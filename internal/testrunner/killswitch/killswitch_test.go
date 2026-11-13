// SPDX-License-Identifier: Apache-2.0
package killswitch

import (
	"slices"
	"strings"
	"testing"
)

// RNC-V1-008 (the narrowing-measurement grammar): exactly `full` or
// `full:<runner>[,<runner>]` with known runner tokens; an empty value is
// off; anything else is refused with the value and the accepted forms.
func TestRNCV1008KillSwitchGrammar(t *testing.T) {
	for value, want := range map[string]Switch{
		"":                    {},
		"full":                {Raw: "full", All: true},
		"full:jest":           {Raw: "full:jest", Runners: []string{"jest"}},
		"full:pytest,go":      {Raw: "full:pytest,go", Runners: []string{"go", "pytest"}},
		"full:jest,jest":      {Raw: "full:jest,jest", Runners: []string{"jest"}},
		"full:minitest,rspec": {Raw: "full:minitest,rspec", Runners: []string{"minitest", "rspec"}},
	} {
		got, err := Parse(value)
		if err != nil || got.Raw != want.Raw || got.All != want.All || !slices.Equal(got.Runners, want.Runners) {
			t.Fatalf("Parse(%q) = %+v, %v; want %+v", value, got, err, want)
		}
	}
	for _, value := range []string{"fulll", "fast", "FULL", "full:", "full:jest,", "full:,jest", "full:cypress", "full: jest", " full", "jest=full", "go=full,jest=full", "full:Jest"} {
		_, err := Parse(value)
		if err == nil || !strings.Contains(err.Error(), "CORVINT_SELECTION=") || !strings.Contains(err.Error(), "`full` or `full:<runner>[,<runner>]`") {
			t.Fatalf("Parse(%q) error %v", value, err)
		}
	}
}

// RNC-V1-008: Full applies a runner-name switch to that runner only, and
// `full` to every runner; Describe names the raw value and the reason.
func TestRNCV1008KillSwitchScope(t *testing.T) {
	jest, _ := Parse("full:jest")
	if !jest.Active() || !jest.Full("jest") || jest.Full("vitest") || jest.Describe() != "kill-switch: CORVINT_SELECTION=full:jest makes jest run the full command on every instance" {
		t.Fatalf("full:jest %+v %q", jest, jest.Describe())
	}
	all, _ := Parse("full")
	if !all.Full("go") || !all.Full("minitest") || all.Describe() != "kill-switch: CORVINT_SELECTION=full makes every runner run its full command" {
		t.Fatalf("full %+v %q", all, all.Describe())
	}
	off, err := FromEnv(func(string) string { return "" })
	if err != nil || off.Active() || off.Full("go") {
		t.Fatalf("off %+v %v", off, err)
	}
	if _, err := FromEnv(func(name string) string { return map[string]string{Env: "fulll"}[name] }); err == nil {
		t.Fatal("FromEnv accepted fulll")
	}
}
