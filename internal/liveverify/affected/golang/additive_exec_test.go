// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// additiveExecBase is the review's fixture (AFP-V0-3140): `quiet` is a
// command that locates nothing, `harness` runs it from its code with
// exec.Command("go", "run", ...), and `consumer` calls harness.Run, so
// consumer's tests observe quiet's behaviour through harness.
var additiveExecBase = map[string]string{
	"quiet/main.go":      "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"a\") }\n",
	"quiet/main_test.go": "package main\n",
	"harness/h.go":       "package harness\n\nimport \"os/exec\"\n\n// Run runs quiet.\nfunc Run() ([]byte, error) { return exec.Command(\"go\", \"run\", \"example.test/m/quiet\").Output() }\n",
	"harness/h_test.go":  "package harness\n",
	"consumer/c.go":      "package consumer\n\nimport \"example.test/m/harness\"\n\n// Use runs quiet through harness.\nfunc Use() ([]byte, error) { return harness.Run() }\n",
	"consumer/c_test.go": "package consumer\n",
}

// additiveExecHead changes quiet's behaviour and adds an unused exported
// function to harness.
var additiveExecHead = map[string]string{
	"quiet/main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"b\") }\n",
	"harness/h.go":  additiveExecBase["harness/h.go"] + "\n// Fresh is new and unused.\nfunc Fresh() {}\n",
}

// additiveExecPlan applies base and head over additiveExecBase and returns the
// narrowed plan for dirty with harness's record.
func additiveExecPlan(t *testing.T, base, head map[string]string, dirty ...string) (affected.Plan, affected.GoAdditive) {
	t.Helper()
	merged := map[string]string{}
	for relative, body := range additiveExecBase {
		merged[relative] = body
	}
	for relative, body := range base {
		merged[relative] = body
	}
	records, graph := additiveRun(t, merged, head, dirty...)
	if record := additiveRecordFor(t, records, "go:example.test/m/harness"); record.State != affected.GoAdditiveNarrowed || strings.Join(record.Declarations, " ") != "Fresh" {
		t.Fatalf("the plugin's harness record %+v, want NARROWED [Fresh]", record)
	}
	plan := affected.Select(graph.WithGoAdditive(records), dirty)
	for _, record := range plan.GoAdditive {
		if record.Unit == "go:example.test/m/harness" {
			return plan, record
		}
	}
	t.Fatalf("the plan reports no harness record: %+v", plan.GoAdditive)
	return plan, affected.GoAdditive{}
}

// TestAdditiveNarrowingFollowsCodeRuns_AFPV03140 is the review's case
// (AFP-V0-3140): quiet's behaviour changes and, in the same change, harness,
// whose existing Run executes quiet, gains an unused exported function. The
// walk reaches harness from quiet by a code-run edge, so harness's record
// falls back with quiet as the unit it was reached through, and consumer,
// which calls Run but names no new declaration, is selected. Before the fix
// the record stayed NARROWED and consumer was excluded. A run declared in
// .corvint/test-binary-execs.json is a code run too.
func TestAdditiveNarrowingFollowsCodeRuns_AFPV03140(t *testing.T) {
	declared := map[string]string{
		"harness/h.go":                    "package harness\n\n// Run runs quiet, as the binary-execs declaration says; no literal names it.\nfunc Run() error { return nil }\n",
		".corvint/test-binary-execs.json": "{\n  \"profile\": \"corvint-test-binary-execs/0\",\n  \"packages\": {\n    \"harness\": [\"quiet\"]\n  }\n}\n", //corvint:fixture-path
		"consumer/c.go":                   "package consumer\n\nimport \"example.test/m/harness\"\n\n// Use runs quiet through harness.\nfunc Use() error { return harness.Run() }\n",
	}
	declaredHead := map[string]string{
		"quiet/main.go": additiveExecHead["quiet/main.go"],
		"harness/h.go":  declared["harness/h.go"] + "\n// Fresh is new and unused.\nfunc Fresh() {}\n",
	}
	for _, tc := range []struct {
		name       string
		base, head map[string]string
	}{
		{"literal go run", nil, additiveExecHead},
		{"declared binary exec", declared, declaredHead},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, record := additiveExecPlan(t, tc.base, tc.head, "harness/h.go", "quiet/main.go")
			if record.State != affected.GoAdditiveFallback || record.Reason != affected.GoAdditiveReachedThrough+": go:example.test/m/quiet" {
				t.Errorf("harness record %+v, want FALLBACK %s: go:example.test/m/quiet", record, affected.GoAdditiveReachedThrough)
			}
			selected := selectedIDs(plan)
			for _, want := range []string{"consumer", "harness", "quiet"} {
				if !slices.Contains(selected, "go:example.test/m/"+want) {
					t.Errorf("%s was not selected: %s", want, trimGo(selected))
				}
			}
			for _, exclusion := range plan.GoAdditiveExcluded {
				t.Errorf("unit %s excluded through %s, although the change reaches harness through a code run", exclusion.UnitID, exclusion.Through)
			}
		})
	}
}

// TestAdditiveNarrowingKeepsUnrunRecords_AFPV03141 holds the narrowing where
// no code run reaches the record's package (AFP-V0-3141): harness's additive
// change alone keeps its record NARROWED and excludes consumer, and so does a
// change of quiet when only harness's tests run quiet, because a test-only
// run reaches harness's tests, not its importers.
func TestAdditiveNarrowingKeepsUnrunRecords_AFPV03141(t *testing.T) {
	testOnly := map[string]string{
		"harness/h.go":      "package harness\n\n// Run does nothing.\nfunc Run() ([]byte, error) { return nil, nil }\n",
		"harness/h_test.go": runnerTest("harness", "example.test/m/quiet"),
	}
	testOnlyHead := map[string]string{
		"quiet/main.go": additiveExecHead["quiet/main.go"],
		"harness/h.go":  testOnly["harness/h.go"] + "\n// Fresh is new and unused.\nfunc Fresh() {}\n",
	}
	for _, tc := range []struct {
		name       string
		base, head map[string]string
		dirty      []string
	}{
		{"additive change alone", nil, map[string]string{"harness/h.go": additiveExecHead["harness/h.go"]}, []string{"harness/h.go"}},
		{"test-only run", testOnly, testOnlyHead, []string{"harness/h.go", "quiet/main.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, record := additiveExecPlan(t, tc.base, tc.head, tc.dirty...)
			if record.State != affected.GoAdditiveNarrowed {
				t.Errorf("harness record %+v, want NARROWED", record)
			}
			selected := selectedIDs(plan)
			if slices.Contains(selected, "go:example.test/m/consumer") {
				t.Errorf("consumer was selected, although it names no new declaration and no code run reaches harness: %s", trimGo(selected))
			}
			if !slices.Contains(selected, "go:example.test/m/harness") {
				t.Errorf("harness's own tests were not selected: %s", trimGo(selected))
			}
		})
	}
}
