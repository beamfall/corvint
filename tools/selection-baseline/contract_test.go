// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

// AFP-V0-9501: no undeclared difference can pass the replay. Slicing,
// ordering and non-root adapters are the explicit execution differences.
func TestSelectionContractReplay_AFPV09501(t *testing.T) {
	goUnit := func(pkg string) manifestUnit { return manifestUnit{Adapter: "go", Unit: "go:" + pkg} }
	for _, tc := range []struct {
		name, audit string
		manifest    manifest
		pass        bool
	}{
		{"same", "selected m/a\nrun m/a m/b", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/b"), goUnit("m/a")}}, true},
		{"profiles", "run m/a", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/a"), {Adapter: "ruby", Unit: "ruby:spec/x"}, {Adapter: "go", Unit: "go-module:nested"}}}, true},
		{"non-Go only", "NOTHING no root work", manifest{Scope: "NARROWED", Units: []manifestUnit{{Adapter: "ruby", Unit: "ruby:spec/x"}}}, true},
		{"full", "FALLBACK unknown frontier", manifest{Scope: "FULL", Units: []manifestUnit{goUnit("m/a")}}, true},
		{"nothing", "NOTHING no change", manifest{Scope: "NOTHING"}, true},
		{"driver extra", "run m/a m/b", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/a")}}, false},
		{"driver omission", "run m/a", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/a"), goUnit("m/b")}}, false},
		{"scope", "run m/a", manifest{Scope: "FULL", Units: []manifestUnit{goUnit("m/a")}}, false},
		{"unknown verdict", "future m/a", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/a")}}, false},
		{"empty run", "run ", manifest{Scope: "NARROWED"}, false},
		{"bad unit", "run m/a", manifest{Scope: "NARROWED", Units: []manifestUnit{{Adapter: "go", Unit: "m/a"}}}, false},
		{"nothing hides units", "NOTHING no change", manifest{Scope: "NOTHING", Units: []manifestUnit{goUnit("m/a")}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := contractAgreement(tc.audit, tc.manifest); (err == nil) != tc.pass {
				t.Fatalf("pass=%v, error=%v", tc.pass, err)
			}
		})
	}
}
