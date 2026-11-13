// SPDX-License-Identifier: Apache-2.0
package selectionswitch

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
)

// MTT-V1-008 with change narrowing-measurement (NMS-V1-008): exactly full and
// full:<runner>[,<runner>] naming known runners are in the grammar; every other
// non-empty value, an unknown runner name among them, is unknown.
func TestMTTV1008Grammar(t *testing.T) {
	for value, want := range map[string]Switch{
		"":             {},
		"full":         {Value: "full", All: true},
		"full:go":      {Value: "full:go", Runners: []string{"go"}},
		"full:jest,go": {Value: "full:jest,go", Runners: []string{"jest", "go"}},
		"full:go,vitest,jest,playwright,pytest,rspec,minitest": {Value: "full:go,vitest,jest,playwright,pytest,rspec,minitest", Runners: testid.Runners},
	} {
		got := Parse(value)
		if got.Value != want.Value || got.All != want.All || !slices.Equal(got.Runners, want.Runners) || got.Unknown {
			t.Fatalf("%q: got %+v want %+v", value, got, want)
		}
	}
	for _, value := range []string{"fastest", "ful", "FULL", " full", "full ", "full:", "full:go,", "full:,go", "full:go,go", "full:Go", "go=full", "full:go jest",
		"full:this-runner-does-not-exist", "full:node-test", "full:go,cargo", "full:mocha,go", "full:go.", "full:GO"} {
		if got := Parse(value); !got.Unknown || got.Reason() != ReasonUnknownValue || !got.On() {
			t.Fatalf("%q accepted: %+v", value, got)
		}
	}
	if Parse("").On() || Parse("").Reason() != "" || Parse("full:go").Reason() != ReasonKillSwitch {
		t.Fatal("reasons")
	}
}
