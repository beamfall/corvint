// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// SelectQualified is the common root-Go package contract (AFP-V0-9500).
// The receipt can add obligations, but cannot certify its own exclusions.
// Re-observe Go with the planner's rules and remove a conservative attribution
// only with that observation's exclusion certificate. No set intersection is
// used: every positive observation and every unproved attribution survives.
func SelectQualified(plan Receipt, module, root string) []string {
	return SelectQualifiedIndexed(plan, module, root, nil)
}

// SelectQualifiedIndexed uses a caller's already-started structural scan.
// Index reuse changes neither qualification nor the resulting package set.
func SelectQualifiedIndexed(plan Receipt, module, root string, pending *Index) []string {
	build := func() (*repositoryIndex, error) { return indexRepository(root, module) }
	if pending != nil && pending.root == root && pending.module == module {
		build = func() (*repositoryIndex, error) {
			<-pending.done
			return pending.index, pending.err
		}
	}
	var index *repositoryIndex
	lines := selectPackagesFrom(plan, module, root, true, func() (*repositoryIndex, error) {
		var err error
		index, err = build()
		return index, err
	})
	verdict := lines[len(lines)-1]
	fallback := func(reason string) []string {
		return append(lines[:len(lines)-1], "FALLBACK "+reason)
	}
	// Even an empty candidate set needs a supported input before it can
	// certify NOTHING.
	for _, dirty := range plan.Plan.Dirty {
		if !fs.ValidPath(dirty) || strings.ContainsRune(dirty, '\\') {
			return fallback("the selection input contains an invalid dirty path")
		}
	}
	if plan.Plan.Scope != "BOUNDED" && plan.Plan.Scope != "UNKNOWN" {
		return fallback("the selection input has no supported scope")
	}
	if strings.HasPrefix(verdict, "FALLBACK the selection is empty while ") {
		if directories := moduleCovered(plan); directories != nil {
			// A declared runtime edge is invisible to the Go-only index: a
			// root test may run a unit inside the nested module.
			if reason := runtimeEdgeIntoModule(root, module, directories); reason != "" {
				return fallback(reason)
			}
			lines[len(lines)-1] = "NOTHING every dirty path is inside a nested Go module whose own unit runs it"
			return lines
		}
	}
	if !strings.HasPrefix(verdict, "run ") {
		return lines
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// Other language selections remain in Provider.Go.Packages. A partial
	// observation cannot certify cross-language edges and never removes them.
	graph, err := affected.BuildPartial(ctx, root, golang.New())
	if err != nil {
		return fallback(fmt.Sprintf("the repository could not be indexed for attribution: %q", err.Error()))
	}
	if slices.Contains(plan.Plan.Dirty, readScopesPath) {
		graph = graph.WithReadScopeBase(baseReadScopes(plan, root))
	}
	base := plan.Range.Base
	if base == "" {
		base = plan.Revision
	}
	// Never consume a supplied additive exclusion. Recompute it against the
	// immutable base; an absent base or snapshot only permits wider selection.
	if plan.Snapshot == nil && fullObjectID.MatchString(base) {
		graph = graph.WithGoAdditive(golang.AdditiveChanges(ctx, root, base, graph, plan.Plan.Dirty))
		graph = graph.WithImportSetChanges(golang.ImportSetChangesAt(ctx, root, root, base, graph, plan.Plan.Dirty))
	}
	proof := affected.Select(graph, plan.Plan.Dirty)
	observed := plan
	observed.Plan.Scope = proof.Scope
	observed.Plan.Unknown = nil
	for _, unknown := range proof.Unknown {
		observed.Plan.Unknown = append(observed.Plan.Unknown, struct {
			Reason string `json:"reason"`
			Detail string `json:"detail"`
		}{unknown.Reason, unknown.Detail})
	}
	if reason, _ := unknownNarrowing(observed, index, root); reason != "" {
		return fallback(reason)
	}
	if err := ctx.Err(); err != nil {
		return fallback(fmt.Sprintf("the repository could not be indexed for attribution: %q", err.Error()))
	}
	selected := map[string]bool{}
	for _, pkg := range plan.Provider.Go.Packages {
		selected[pkg] = true
	}
	for _, selection := range proof.Selected {
		if pkg, ok := strings.CutPrefix(selection.UnitID, "go:"); ok {
			selected[pkg] = true
		}
	}
	runtimeReaders := graph.UnobservedRuntimeReaders()
	if len(plan.Plan.Dirty) != 0 {
		for id := range runtimeReaders {
			if pkg, ok := strings.CutPrefix(id, "go:"); ok {
				selected[pkg] = true
			}
		}
	}
	excluded := map[string]bool{}
	for _, exclusion := range proof.Excluded {
		if exclusion.Universe == proof.GraphDigest && exclusion.Reason == affected.ExcludedNoDependencyPath && exclusion.Invalidation != "" {
			excluded[exclusion.UnitID] = true
		}
	}
	for _, exclusion := range proof.GoAdditiveExcluded {
		if exclusion.Universe == proof.GraphDigest && exclusion.Invalidation == affected.GoAdditiveInvalidation {
			excluded[exclusion.UnitID] = true
		}
	}
	for _, pkg := range strings.Fields(strings.TrimPrefix(verdict, "run ")) {
		unit, known := graph.Unit("go:" + pkg)
		if !excluded["go:"+pkg] && !(known && len(unit.Tests) == 0) {
			selected[pkg] = true
		}
	}
	for pkg := range selected {
		if unit, known := graph.Unit("go:" + pkg); known && golang.DependentModuleUnit(unit) {
			delete(selected, pkg) // STR-V0-069: never a root-module execution unit.
		}
	}
	packages := sortedKeys(selected)
	for _, pkg := range packages {
		if !plainImportPath.MatchString(pkg) || !underModule(pkg, module) {
			return fallback(fmt.Sprintf("package path is not a plain import path under the module: %q", pkg))
		}
	}
	// The audit must describe the executed set, not the discarded conservative
	// candidates. Keep non-package decisions (data and language suites).
	audit := []string{}
	for _, line := range lines[:len(lines)-1] {
		if strings.HasPrefix(line, "data ") || strings.HasPrefix(line, "suite ") {
			audit = append(audit, line)
		}
	}
	for _, pkg := range packages {
		if reason := runtimeReaders["go:"+pkg]; reason != "" {
			audit = append(audit, fmt.Sprintf("unresolved %s: %q", pkg, reason))
		} else {
			audit = append(audit, "selected "+pkg)
		}
	}
	return append(audit, PlannedRunSet("run qualified", packages))
}

// moduleCovered returns the nested module directories that hold every dirty
// path, or nil when any dirty path lacks a nested-module certificate.
func moduleCovered(plan Receipt) []string {
	covered := map[string]string{}
	for _, entry := range plan.Plan.Unknown {
		if directory, ok := UnobservedModule(entry.Reason, entry.Detail); ok {
			covered[strings.TrimPrefix(entry.Detail, "go:unobserved-module:"+directory+":")] = directory
		}
	}
	directories := map[string]bool{}
	for _, dirty := range plan.Plan.Dirty {
		directory, ok := covered[dirty]
		if !ok {
			return nil
		}
		directories[directory] = true
	}
	return sortedKeys(directories)
}

// runtimeEdgeIntoModule names why a declared runtime edge may make a root
// test run a unit inside one of the nested module directories, or returns ""
// when no declared edge can. An unreadable or invalid declaration, and a Go
// unit outside the root module, cannot be placed and so fail closed.
func runtimeEdgeIntoModule(root, module string, directories []string) string {
	name := filepath.Join(root, filepath.FromSlash(affected.RuntimeEdgesPath))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	var body []byte
	if err == nil {
		// Only a regular file is opened, so a FIFO or device cannot block the
		// selector; the read is size-capped and refuses a swapped file.
		if !info.Mode().IsRegular() {
			err = errors.New("not a regular file")
		} else {
			body, err = readBounded(name, info)
		}
	}
	var declared struct {
		Edges []affected.RuntimeEdge `json:"edges"`
	}
	if err != nil || json.Unmarshal(body, &declared) != nil {
		return fmt.Sprintf("the nested-module downgrade cannot read %s", affected.RuntimeEdgesPath)
	}
	inside := func(path string) bool {
		for _, directory := range directories {
			if path == directory || strings.HasPrefix(path, directory+"/") {
				return true
			}
		}
		return false
	}
	for _, edge := range declared.Edges {
		namespace, name, _ := strings.Cut(edge.Runs, ":")
		candidates := []string{name, strings.ReplaceAll(name, ".", "/")}
		if namespace == "go" {
			rel, ok := strings.CutPrefix(name, module+"/")
			if !ok && name != module {
				return fmt.Sprintf("a declared runtime edge runs %q, which the nested-module downgrade cannot place", edge.Runs)
			}
			candidates = []string{rel}
		}
		if slices.ContainsFunc(candidates, inside) {
			return fmt.Sprintf("a declared runtime edge from %q runs %q inside a dirty nested module", edge.Tests, edge.Runs)
		}
	}
	return ""
}

// PlannedRunSet projects an already-qualified set into the existing verdict
// grammar. Qualification, including widening, must precede this projection.
func PlannedRunSet(verdict string, planned []string) string {
	if !strings.HasPrefix(verdict, "run ") {
		return verdict
	}
	packages := sortedUnique(planned)
	if len(packages) == 0 {
		return "NOTHING the plan selects no Go package; the AFP-V0-012 attribution names only packages the plan's rules exclude (AFP-V0-2402)"
	}
	return "run " + strings.Join(packages, " ")
}
