// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"sort"
	"strings"
)

// contractAgreement compares execution scopes first and root-Go sets second.
// Non-Go suites, nested module suites, test slicing and placement are declared
// execution-profile differences; none licenses a different root-Go package.
// FULL is compared as a scope, never as an empty set or a successful narrowing.
func contractAgreement(audit string, m manifest) error {
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
		return nil
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
