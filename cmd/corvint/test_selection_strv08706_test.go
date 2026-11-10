package main

import (
	"reflect"
	"testing"
)

// STR-V0-8706: an e2e-safe selection's Playwright files join the selected
// units; a selection that is blocked, of another schema or malformed is a
// reason, never an empty set of tests.
func TestSTRV08706TestSelectionUnits(t *testing.T) {
	selected := []any{
		map[string]any{"path": "e2e/login.spec.ts", "name": "login > signs in"},
		map[string]any{"path": "e2e/cart.spec.ts"},
		map[string]any{"path": "e2e/login.spec.ts", "name": "login > signs out"},
	}
	wantUnits := []string{"typescript:playwright:e2e/cart.spec.ts", "typescript:playwright:e2e/login.spec.ts"}
	for name, test := range map[string]struct {
		selection map[string]any
		units     []string
		reason    string
	}{
		"absent":        {nil, nil, ""},
		"empty":         {map[string]any{}, nil, ""},
		"narrow":        {map[string]any{"schema": "e2e-safe-selection/1", "state": "narrow-selection-allowed", "selected": selected}, wantUnits, ""},
		"full":          {map[string]any{"schema": "e2e-safe-selection/1", "state": "full-relevant-suite-required", "selected": selected}, wantUnits, ""},
		"none selected": {map[string]any{"schema": "e2e-safe-selection/1", "state": "narrow-selection-allowed", "selected": []any{}}, []string{}, ""},
		"other schema": {map[string]any{"schema": "e2e-safe-selection/2", "state": "narrow-selection-allowed", "selected": selected}, nil,
			`advice.test_selection has schema "e2e-safe-selection/2", which corvint test does not run; its tests are not run`},
		"no schema": {map[string]any{"state": "narrow-selection-allowed", "selected": selected}, nil,
			`advice.test_selection has schema "", which corvint test does not run; its tests are not run`},
		"blocked": {map[string]any{"schema": "e2e-safe-selection/1", "state": "blocked", "state_reason": "flow evidence is stale", "selected": selected}, nil,
			`advice.test_selection is "blocked" (flow evidence is stale); its Playwright tests cannot be selected`},
		"no state": {map[string]any{"schema": "e2e-safe-selection/1", "selected": selected}, nil,
			`advice.test_selection is "" (); its Playwright tests cannot be selected`},
		"no selected list": {map[string]any{"schema": "e2e-safe-selection/1", "state": "narrow-selection-allowed"}, nil,
			"advice.test_selection names no selected list; its Playwright tests cannot be selected"},
		"selected is not a list": {map[string]any{"schema": "e2e-safe-selection/1", "state": "full-relevant-suite-required", "selected": "e2e/login.spec.ts"}, nil,
			"advice.test_selection names no selected list; its Playwright tests cannot be selected"},
		"entry without path": {map[string]any{"schema": "e2e-safe-selection/1", "state": "narrow-selection-allowed", "selected": []any{selected[0], map[string]any{"name": "cart"}}}, nil,
			"advice.test_selection selects a test without a path; its Playwright tests cannot be selected"},
		"entry is not an object": {map[string]any{"schema": "e2e-safe-selection/1", "state": "narrow-selection-allowed", "selected": []any{"e2e/login.spec.ts"}}, nil,
			"advice.test_selection selects a test without a path; its Playwright tests cannot be selected"},
	} {
		units, reason := testSelectionUnits(test.selection)
		if !reflect.DeepEqual(units, test.units) || reason != test.reason {
			t.Errorf("%s: units %q reason %q, want %q and %q", name, units, reason, test.units, test.reason)
		}
	}
}
