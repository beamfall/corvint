// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"reflect"
	"testing"
)

// TestDriverUnknownIsNotLanguage_AZU_V0_9704 retains Go readers and frontier
// fallbacks without requiring a fictitious driver-language suite. PR 6's
// unparsable workflow produces both unknowns in the second case.
func TestDriverUnknownIsNotLanguage_AZU_V0_9704(t *testing.T) {
	for _, tc := range []struct {
		name       string
		unknown    []Unknown
		scope      string
		units      []string
		incomplete bool
	}{
		{
			name:    "driver without selectable target",
			unknown: []Unknown{{Reason: "NO_SELECTABLE_TEST", Detail: "driver:ci:.github/workflows/ci.yml#*"}},
			scope:   ScopeNarrowed,
			units:   []string{"0 go:example.test/m/a whole"},
		},
		{
			name: "unparsable workflow retains allowed category",
			unknown: []Unknown{
				{Reason: "NO_SELECTABLE_TEST", Detail: "driver:ci:.github/workflows/ci.yml#*"},
				{Reason: "LANGUAGE_FRONTIER", Detail: "driver:unparsable-file"},
			},
			scope: ScopeNarrowed,
			units: []string{"0 go:example.test/m/a whole"},
		},
		{
			name: "driver entry does not bypass unsafe Go frontier",
			unknown: []Unknown{
				{Reason: "NO_SELECTABLE_TEST", Detail: "driver:ci:.github/workflows/ci.yml#*"},
				{Reason: "LANGUAGE_FRONTIER", Detail: "go:module-replace"},
			},
			scope: ScopeFull,
			units: []string{"0 go:example.test/m/a whole", "0 go:example.test/m/b whole", "0 go:example.test/m/c whole"},
		},
		{
			name:       "unadapted language still incomplete",
			unknown:    []Unknown{{Reason: "NO_SELECTABLE_TEST", Detail: "python:pkg/mod.py"}},
			scope:      ScopeNarrowed,
			units:      []string{"0 go:example.test/m/a whole"},
			incomplete: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fixtureInput()
			input.Selected = []string{"go:example.test/m/a", "driver:make:Makefile#test", "driver:ci:.github/workflows/ci.yml#unit"}
			input.Unknown = tc.unknown
			m, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if m.Scope != tc.scope || !reflect.DeepEqual(unitIDs(m), tc.units) || (len(m.Incomplete) != 0) != tc.incomplete {
				t.Fatalf("scope=%s units=%v incomplete=%v; want scope=%s units=%v incomplete=%t", m.Scope, unitIDs(m), m.Incomplete, tc.scope, tc.units, tc.incomplete)
			}
		})
	}
}
