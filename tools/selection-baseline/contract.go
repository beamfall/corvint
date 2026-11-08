// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// contractAgreement compares execution scopes first and root-Go sets second.
// Non-Go suites, nested module suites, test slicing and placement are declared
// execution-profile differences; none licenses a different root-Go package.
// FULL also needs an independently listed root-Go universe; a label alone
// cannot establish coverage, and unknown unit forms never stand for packages.
func contractAgreement(audit string, m manifest, universe []string) error {
	lines := strings.Split(strings.TrimSpace(audit), "\n")
	verdict := lines[len(lines)-1]
	kind, rest, _ := strings.Cut(verdict, " ")
	scope := map[string]string{"run": "NARROWED", "FALLBACK": "FULL", "NOTHING": "NOTHING"}[kind]
	localScope := m.Scope
	driver, local := map[string]bool{}, map[string]bool{}
	for _, unit := range m.Units {
		if unit.Adapter == "go" {
			if strings.HasPrefix(unit.Unit, "go-module:") && len(unit.Unit) > len("go-module:") {
				continue // a nested module's own suite is not a root package.
			}
			pkg, ok := strings.CutPrefix(unit.Unit, "go:")
			if !ok || pkg == "" {
				return fmt.Errorf("selection contract: invalid Go unit %q", unit.Unit)
			}
			local[pkg] = true
		}
	}
	// A non-Go suite/check can make the manifest NARROWED while the root-Go
	// selection is NOTHING. This is the declared adapter-profile difference.
	if localScope == "NARROWED" && len(local) == 0 && kind == "NOTHING" {
		localScope = "NOTHING"
	}
	if scope == "" || scope != localScope {
		return fmt.Errorf("selection contract scope divergence: driver=%q local=%q", verdict, m.Scope)
	}
	if scope == "FULL" {
		if len(universe) == 0 {
			return fmt.Errorf("selection contract: empty run verdict")
		}
		for _, pkg := range universe {
			driver[pkg] = true
		}
	}
	if kind == "run" {
		for _, pkg := range strings.Fields(rest) {
			driver[pkg] = true
		}
		if len(driver) == 0 {
			return fmt.Errorf("selection contract: empty run verdict")
		}
	}
	var extra, omitted []string
	for pkg := range driver {
		if !local[pkg] {
			extra = append(extra, pkg)
		}
	}
	for pkg := range local {
		if !driver[pkg] {
			omitted = append(omitted, pkg)
		}
	}
	sort.Strings(extra)
	sort.Strings(omitted)
	if len(extra)+len(omitted) != 0 {
		return fmt.Errorf("selection contract package divergence: driver-only=%v local-only=%v", extra, omitted)
	}
	return nil
}

// contractUniverse observes ./... plus the root runner's hidden packages independently of the
// affected receipt and manifest. Only a successful concrete package listing can
// certify FULL coverage; unreadable or unsupported modules fail the check.
func contractUniverse(ctx context.Context, root string) ([]string, error) {
	hidden, err := golang.HiddenSourcePaths(affected.DiskSourceContext(ctx, root))
	if err != nil {
		return nil, err
	}
	directories := map[string]bool{}
	for _, source := range hidden {
		directory := path.Dir(source)
		nested := false
		for parent := directory; parent != "."; parent = path.Dir(parent) {
			info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(parent), "go.mod"))
			if err == nil {
				if !info.Mode().IsRegular() {
					return nil, fmt.Errorf("selection contract: invalid Go unit %q", source)
				}
				nested = true
				break
			}
			if !os.IsNotExist(err) {
				return nil, err
			}
		}
		if !nested {
			directories["./"+directory] = true
		}
	}
	patterns := []string{"./..."}
	for directory := range directories {
		patterns = append(patterns, directory)
	}
	sort.Strings(patterns)
	listed, err := run(ctx, root, "go", append([]string{"list", "-mod=readonly"}, patterns...)...)
	if err != nil {
		return nil, err
	}
	packages := strings.Fields(listed)
	sort.Strings(packages)
	return packages, nil
}
