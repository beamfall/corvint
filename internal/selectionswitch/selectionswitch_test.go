// SPDX-License-Identifier: Apache-2.0
package selectionswitch

import (
	"slices"
	"strings"
	"testing"
)

// PYR-V1-009 (grammar of owner decision R13): exactly `full` and
// `full:<runner>[,<runner>]` are accepted; an empty value forces nothing,
// and any other value or runner is refused with kill-switch-value-unknown.
func TestPYRV1009SelectionGrammar(t *testing.T) {
	for value, want := range map[string]Switch{
		"":                      {},
		"full":                  {All: true, Value: "full"},
		"full:pytest":           {Runners: []string{"pytest"}, Value: "full:pytest"},
		"full:pytest,go,pytest": {Runners: []string{"go", "pytest"}, Value: "full:pytest,go,pytest"},
	} {
		got, err := Parse(value)
		if err != nil || got.All != want.All || !slices.Equal(got.Runners, want.Runners) || got.Value != want.Value {
			t.Errorf("%q: %+v %v, want %+v", value, got, err, want)
		}
	}
	for _, value := range []string{"FULL", "full:", "full:cypress", "full:pytest,", "narrow", "full: pytest", "full:pytest;jest"} {
		if _, err := Parse(value); err == nil || !strings.HasPrefix(err.Error(), ReasonUnknown+": ") {
			t.Errorf("%q: %v, want %s", value, err, ReasonUnknown)
		}
	}
	forced, _ := Parse("full:pytest")
	if !forced.Forces("pytest") || forced.Forces("go") || (Switch{}).Forces("pytest") {
		t.Fatalf("Forces %+v", forced)
	}
	if all, _ := Parse("full"); !all.Forces("minitest") {
		t.Fatal("full forces every runner")
	}
	if got := forced.Line("x"); got != "kill-switch: CORVINT_SELECTION=full:pytest forces x" {
		t.Fatalf("line %q", got)
	}
}
