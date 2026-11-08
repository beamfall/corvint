// SPDX-License-Identifier: Apache-2.0

// Package shaderrunner supplies shader pieces for declared T13 script commands.
// Inventory, command construction and terminal reconciliation use T3 unchanged;
// include dependencies supplement core trace reads and never confer qualification.
package shaderrunner

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/shaderdialect"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// Read resolves exactly one project-declared shader command. Both compiler
// checks and shader tests are aggregate script identities; no tool discovery,
// glob expansion or compiler invocation occurs here. The caller supplies the
// declaration entry ID rather than inferring tests from source filenames.
func Read(ctx context.Context, root, commit, declaration, id, os, arch string) (registry.RunnerPlugin, testrunner.InventoryKey, error) {
	plugins, err := registry.ReadScriptPlugins(ctx, root, commit, declaration)
	if err != nil {
		return nil, testrunner.InventoryKey{}, err
	}
	p, key := plugins.Plugin(id, os, arch)
	inv, err := p.Inventory(ctx, registry.InventoryRequest{Key: key})
	if err != nil || !inv.Complete {
		return nil, key, fmt.Errorf("shader-command-undeclared")
	}
	return shaderPlugin{p}, key, nil
}

// shaderPlugin preserves T13 commands and exposes the retained qualification gap.
type shaderPlugin struct{ registry.RunnerPlugin }

func (p shaderPlugin) Pieces() testrunner.RunnerPieces {
	pieces := p.RunnerPlugin.Pieces()
	pieces.Qualification = testrunner.RunnerPiece{Reason: "unqualified"}
	return pieces
}

const (
	MaxFiles      = 128
	MaxDepth      = 32
	MaxGraphBytes = 8 << 20
)

// Graph is derived from immutable tracked source, not a plugin-supplied trace.
// It has no store writer and is never sufficient to mark a trace present.
type Graph struct {
	Reads   testevidence.Component[testevidence.Blob]
	Regions testevidence.Component[testevidence.RegionsFile]
}

// Includes reads a bounded transitive include graph at the record's exact
// commit. Sources and includeDirs are core-supplied inputs for this command,
// relative to the repository root. An empty or unresolved input is incomplete.
// Search order is quoted-file directory first, then includeDirs in their declared
// order. There is no host SDK or untracked-file fallback.
func Includes(ctx context.Context, repo *gitauth.Repository, repository, commit string, sources, includeDirs []string) Graph {
	out := Graph{Reads: testevidence.Component[testevidence.Blob]{State: testevidence.Present}, Regions: testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present}}
	fail := func(reason string) Graph {
		out.Reads.State, out.Reads.Reason = testevidence.Incomplete, reason
		out.Regions.State, out.Regions.Reason = testevidence.Incomplete, reason
		return out
	}
	if repo == nil || repository == "" || !utf8.ValidString(repository) || strings.ContainsRune(repository, 0) || !wire.IsGitOid(commit) || len(sources) == 0 {
		return fail("shader-graph-input-missing")
	}
	if len(sources) > MaxFiles || len(includeDirs) > MaxFiles {
		return fail("shader-file-cap")
	}
	for _, name := range sources {
		if !relative(name, false) || !shaderSource(name) {
			return fail("shader-source-unsupported")
		}
	}
	for _, dir := range includeDirs {
		if !relative(dir, true) {
			return fail("shader-include-directory-invalid")
		}
	}
	// CommitTree also accepts historical tree/tag objects. Require an exact
	// commit here, as T13 does, before deriving any source evidence.
	resolved, err := repo.Resolve(ctx, commit)
	if err != nil || resolved != commit {
		return fail("shader-checkout-not-pinned-commit")
	}
	tree, err := repo.CommitTree(ctx, commit)
	if err != nil {
		return fail("shader-checkout-unavailable")
	}
	read := func(name string) (gitauth.TreeEntry, bool, error) {
		entry, exists, err := repo.LookupTreeEntry(ctx, tree, name)
		if err != nil {
			return entry, false, err
		}
		if exists && (entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755") {
			return entry, false, fmt.Errorf("shader-include-not-regular")
		}
		return entry, exists, nil
	}
	seen, active := map[string]bool{}, map[string]bool{}
	total := 0
	var visit func(string, int) error
	visit = func(name string, depth int) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("shader-graph-cancelled")
		}
		if active[name] {
			return fmt.Errorf("shader-include-cycle")
		}
		if seen[name] {
			return nil
		}
		if depth > MaxDepth {
			return fmt.Errorf("shader-depth-cap")
		}
		if len(seen) >= MaxFiles {
			return fmt.Errorf("shader-file-cap")
		}
		entry, exists, err := read(name)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("shader-include-unresolved")
		}
		raw, err := repo.BlobBytesBounded(ctx, entry.OID, shaderdialect.MaxBytes)
		if err != nil {
			return fmt.Errorf("shader-source-unavailable")
		}
		total += len(raw)
		if total > MaxGraphBytes {
			return fmt.Errorf("shader-graph-byte-cap")
		}
		seen[name], active[name] = true, true
		defer delete(active, name)
		blob := testevidence.Blob{Path: testevidence.Path{Repository: repository, Lexical: name, Resolved: name}, OID: entry.OID}
		out.Reads.Items = append(out.Reads.Items, blob)
		source := shaderdialect.Parse(string(raw))
		if source.Reason != "" {
			return fmt.Errorf("%s", source.Reason)
		}
		if shaderSource(name) {
			out.Regions.Items = append(out.Regions.Items, testevidence.RegionsFile{Blob: blob, Bodies: source.Bodies})
		}
		for _, inc := range source.Includes {
			if inc.Name == "" || strings.ContainsAny(inc.Name, "\\:\x00\t\n\r <>\"") || path.IsAbs(inc.Name) {
				return fmt.Errorf("shader-include-path-invalid")
			}
			var candidates []string
			if !inc.Angled {
				candidates = append(candidates, path.Join(path.Dir(name), inc.Name))
			}
			for _, dir := range includeDirs {
				candidates = append(candidates, path.Join(dir, inc.Name))
			}
			target := ""
			for _, candidate := range candidates {
				if !relative(candidate, false) {
					return fmt.Errorf("shader-include-outside-repository")
				}
				_, exists, err := read(candidate)
				if err != nil {
					return err
				}
				if exists {
					target = candidate
					break
				}
			}
			if target == "" {
				return fmt.Errorf("shader-include-unresolved")
			}
			if err := visit(target, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, source := range sources {
		if err := visit(source, 0); err != nil {
			return fail(err.Error())
		}
	}
	sort.Slice(out.Reads.Items, func(i, j int) bool { return out.Reads.Items[i].Path.Lexical < out.Reads.Items[j].Path.Lexical })
	sort.Slice(out.Regions.Items, func(i, j int) bool {
		return out.Regions.Items[i].Blob.Path.Lexical < out.Regions.Items[j].Blob.Path.Lexical
	})
	return out
}

func relative(name string, dir bool) bool {
	return utf8.ValidString(name) && name != "" && (dir || name != ".") && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../") && !path.IsAbs(name) && !strings.ContainsAny(name, "\\:\x00\n\r")
}
func shaderSource(name string) bool {
	switch path.Ext(name) {
	case ".metal", ".hlsl", ".glsl":
		return true
	}
	return false
}

// Record supplements T13's core-produced record. Missing trace observations
// remain missing even if the entire static include graph is known. Every shader
// class stays FULL until the core harness supplies retained qualification; this
// function neither supplies nor writes one.
func Record(ctx context.Context, header testevidence.Record, trace proctrace.Result, repo *gitauth.Repository, sources, includeDirs []string) (testevidence.Record, error) {
	record, err := registry.ScriptTraceRecord(ctx, header, trace, repo)
	if err != nil {
		return testevidence.Record{}, err
	}
	graph := Includes(ctx, repo, header.Repository, header.Commit, sources, includeDirs)
	record.Executed = testevidence.Missing[testevidence.ExecutedFile]("shader-no-line-coverage")
	record.Regions = graph.Regions
	// A graph supplements observations; it can only weaken their completeness.
	if graph.Reads.State != testevidence.Present {
		record.Reads.State, record.Reads.Reason = testevidence.Incomplete, graph.Reads.Reason
	} else if record.Reads.State == testevidence.Absent {
		record.Reads.State = testevidence.Incomplete
	}
	byPath := map[testevidence.Path]testevidence.Blob{}
	for _, b := range record.Reads.Items {
		byPath[b.Path] = b
	}
	for _, b := range graph.Reads.Items {
		if old, ok := byPath[b.Path]; ok && old != b {
			return testevidence.Record{}, fmt.Errorf("shader-read-binding-conflict")
		}
		byPath[b.Path] = b
	}
	record.Reads.Items = nil
	for _, b := range byPath {
		record.Reads.Items = append(record.Reads.Items, b)
	}
	sort.Slice(record.Reads.Items, func(i, j int) bool { return record.Reads.Items[i].Path.Lexical < record.Reads.Items[j].Path.Lexical })
	if _, err := testevidence.Encode(record); err != nil {
		return testevidence.Record{}, err
	}
	return record, nil
}
