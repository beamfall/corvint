// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// AFP-V0-2401: partial or malformed receipts cannot establish that any step is safe to skip.
func TestAFPV02401IncompleteReceiptsRunEveryStep(t *testing.T) {
	steps, err := ReadSteps(strings.NewReader(fixtureMakefile))
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, members map[string]any) {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"ok": true, "profile": "affected-plan/1", "plan": members})
		if err != nil {
			t.Fatal(err)
		}
		run, _, full := Select(raw, steps)
		if full == "" || !slices.Equal(run, []string{"a-test", "b-test", "c-test"}) {
			t.Fatalf("receipt %s: run=%v full=%q; want FULL with every step", raw, run, full)
		}
	}
	t.Run("empty plan", func(t *testing.T) { check(t, map[string]any{}) })
	for _, field := range []string{"dirty", "selected", "unknown"} {
		for _, value := range []string{"missing", "null", "object", "string", "null member", "empty member"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				members := map[string]any{"dirty": []any{}, "selected": []any{}, "unknown": []any{}}
				switch value {
				case "missing":
					delete(members, field)
				case "null":
					members[field] = nil
				case "object":
					members[field] = map[string]any{}
				case "string":
					members[field] = "broken"
				case "null member":
					members[field] = []any{nil}
				case "empty member":
					members[field] = []any{map[string]any{}}
				}
				check(t, members)
			})
		}
	}
	for _, id := range []any{nil, "", "shell", ":script/a_test.sh", "shell:", "shell:/script/a_test.sh", "shell:../script/a_test.sh", "shell:script\\a_test.sh", "shell:script/\na_test.sh", 42} {
		t.Run("unitId/"+fmt.Sprint(id), func(t *testing.T) {
			check(t, map[string]any{"dirty": []any{}, "selected": []any{map[string]any{"unitId": id}}, "unknown": []any{}})
		})
	}
	for _, entry := range []any{map[string]any{"detail": "go:build-constraint-variants"}, map[string]any{"reason": "NO_SELECTABLE_TEST"}, map[string]any{"reason": "NO_SELECTABLE_TEST", "detail": nil}} {
		t.Run(fmt.Sprint(entry), func(t *testing.T) {
			check(t, map[string]any{"dirty": []any{}, "selected": []any{}, "unknown": []any{entry}})
		})
	}
	// Complete empty arrays are valid; only the unmodelled recipe must run.
	if run, _, full := Select(planJSON("", "", ""), steps); full != "" || !slices.Equal(run, []string{"c-test"}) {
		t.Fatalf("complete empty receipt: run=%v full=%q", run, full)
	}
}

const fixtureMakefile = "shell-test: \\\n  a-test \\\n  b-test \\\n  c-test \\\n  # end\n\n.PHONY: a-test\na-test:\n\t@script/a_test.sh\nb-test:\n\t@script/b_test.sh && script/b2_test.sh\nc-test:\n\tnode --test integrations/x.test.mjs\n"

func planJSON(dirty, selected, unknown string) []byte {
	return []byte(`{"ok":true,"profile":"affected-plan/1","plan":{"dirty":[` + dirty + `],"selected":[` + selected + `],"unknown":[` + unknown + `]}}`)
}

// TestAFPV02401SelectsTheShellTestStepsThePlanSelects (V1-3489): a selected shell test runs the
// step whose recipe runs it, a step naming no script test always runs, and the rest are skipped;
// the plan's allowlisted unknowns narrow. Negative controls: an unreadable plan, a dirty Makefile,
// an UNOWNED_DIRTY_PATH or a module-level Go frontier runs every step, naming why.
func TestAFPV02401SelectsTheShellTestStepsThePlanSelects(t *testing.T) {
	steps, err := ReadSteps(strings.NewReader(fixtureMakefile))
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 || !slices.Equal(steps[1].Scripts, []string{"script/b2_test.sh", "script/b_test.sh"}) || len(steps[2].Scripts) != 0 {
		t.Fatalf("steps = %+v", steps)
	}
	ok := `{"reason":"LANGUAGE_FRONTIER","detail":"shell:foreign-invocation-unmodelled"},{"reason":"LANGUAGE_FRONTIER","detail":"go:build-constraint-variants"},{"reason":"NO_SELECTABLE_TEST","detail":"shell:script/lib.sh"},{"reason":"UNSELECTED_BINARY_CONSUMER","detail":"tools/e2e.mjs"}`
	run, unstepped, full := Select(planJSON(`"script/b.sh"`, `{"unitId":"shell:script/b2_test.sh"},{"unitId":"shell:script/loose_test.sh"},{"unitId":"go:m/x"}`, ok), steps)
	if full != "" || !slices.Equal(run, []string{"b-test", "c-test"}) || !slices.Equal(unstepped, []string{"script/loose_test.sh"}) {
		t.Fatalf("narrowed run=%v unstepped=%v full=%q", run, unstepped, full)
	}
	for name, raw := range map[string][]byte{
		"unreadable":     []byte("{"),
		"refused":        []byte(`{"ok":false,"profile":"affected-plan/1"}`),
		"makefile":       planJSON(`"Makefile"`, ``, ``),
		"unowned":        planJSON(`"LICENSE.txt"`, ``, `{"reason":"UNOWNED_DIRTY_PATH","detail":"LICENSE.txt"}`),
		"go frontier":    planJSON(`"x.go"`, ``, `{"reason":"LANGUAGE_FRONTIER","detail":"go:gated-root-locators-changed"}`),
		"bad consumer":   planJSON(`"x.go"`, ``, `{"reason":"UNSELECTED_BINARY_CONSUMER","detail":"../x.mjs"}`),
		"unindexed path": planJSON(`"x.go"`, ``, `{"reason":"UNINDEXED_SOURCE_PATH","detail":"x.go"}`),
		"inputs invalid": planJSON(`"x.go"`, ``, `{"reason":"LANGUAGE_FRONTIER","detail":"shell:test-inputs-invalid"}`),
		"scopes changed": planJSON(`".corvint/shell-read-scopes.json"`, ``, `{"reason":"LANGUAGE_FRONTIER","detail":"shell:read-scopes-changed"}`),
		"inputs changed": planJSON(`".corvint/shell-test-inputs.json"`, ``, `{"reason":"LANGUAGE_FRONTIER","detail":"shell:test-inputs-changed"}`),
	} {
		if run, _, full := Select(raw, steps); full == "" || len(run) != 3 {
			t.Errorf("%s: run=%v full=%q, want every step with a reason", name, run, full)
		}
	}
}

// TestAFPV02401RepositoryStepsMapToTheirScripts: every shell-test step of this repository's
// Makefile is found, and the GOC-V0-008 self-test maps to its own step.
func TestAFPV02401RepositoryStepsMapToTheirScripts(t *testing.T) {
	file, err := os.Open("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	steps, err := ReadSteps(file)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, step := range steps {
		if step.Target == "no-python-runtime-dependency-test" {
			found = slices.Equal(step.Scripts, []string{"script/no-python-runtime-dependency_test.sh"})
		}
	}
	if len(steps) < 40 || !found {
		t.Fatalf("%d steps, no-python step mapped %v: %+v", len(steps), found, steps)
	}
}
