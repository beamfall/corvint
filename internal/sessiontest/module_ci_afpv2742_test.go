// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"reflect"
	"slices"
	"testing"
)

// AFP-V0-2742 (V1-4932): on the ci profile a well-formed unobserved-module
// frontier keeps the scope the rest of the plan earns and names the module's
// own suite as a check still required, the decision the deciding driver makes
// for the same entry (CIN-V0-860), so the AFP-V0-098 superset guard no longer
// forces FULL (beamfall/corvint#105). The root selection is still run: the
// module entry never drops a selected root package. A malformed module detail
// and any other unknown still force FULL, and FULL adds no module unit on the
// ci profile. Restoring the profile condition on the allowlist bypass makes the
// first case FULL again.
func TestCIProfileNamesTheNestedModuleSuite_AFPV2742(t *testing.T) {
	module := func(detail string) Unknown { return Unknown{Reason: "LANGUAGE_FRONTIER", Detail: detail} }
	full := []string{"0 go:example.test/m/a whole", "0 go:example.test/m/b whole", "0 go:example.test/m/c whole"}
	cases := []struct {
		name    string
		unknown []Unknown
		scope   string
		units   []string
		checks  []string
	}{
		{name: "module frontier narrows and names the suite", unknown: []Unknown{
			module("go:unobserved-module:nested/mod:nested/mod/a_test.go"),
			module("go:unobserved-module:nested/mod:nested/mod/b.go"),
		}, scope: ScopeNarrowed, units: []string{"0 go:example.test/m/a whole"}, checks: []string{ModuleCheck("nested/mod")}},
		{name: "malformed detail stays FULL", unknown: []Unknown{module("go:unobserved-module:../x:../x/y.go")}, scope: ScopeFull, units: full},
		{name: "another unknown stays FULL", unknown: []Unknown{
			module("go:unobserved-module:nested/mod:nested/mod/b.go"),
			{Reason: "UNINDEXED_SOURCE_PATH", Detail: "x/y.go"},
		}, scope: ScopeFull, units: full, checks: []string{ModuleCheck("nested/mod")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := fixtureInput()
			input.RunnerProfile = ProfileCI
			input.Unknown = tc.unknown
			m, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if m.Scope != tc.scope || !reflect.DeepEqual(unitIDs(m), tc.units) {
				t.Fatalf("got scope=%s reason=%q units=%v", m.Scope, m.Reason, unitIDs(m))
			}
			for _, check := range tc.checks {
				if !slices.Contains(m.Checks, check) {
					t.Fatalf("checks %q do not name %q", m.Checks, check)
				}
			}
			if len(tc.checks) == 0 && slices.Contains(m.Checks, ModuleCheck("../x")) {
				t.Fatalf("a malformed detail is named as a module check: %q", m.Checks)
			}
		})
	}
}
