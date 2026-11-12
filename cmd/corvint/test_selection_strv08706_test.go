package main

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
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

// STR-V0-8706, APK-V0-001: the files the joined plan.e2e member selects join
// the selected units, one per path whatever its project; a member that is
// UNKNOWN and selects nothing is a reason, never an empty set of tests.
func TestSTRV08706PlanE2EUnits(t *testing.T) {
	invalid := []affected.Unknown{{Reason: "FLOWS_DECLARATION_INVALID", Detail: ".corvint/flows.json"}}
	for name, test := range map[string]struct {
		e2e    *affected.E2EPlan
		units  []string
		reason string
	}{
		"absent":  {nil, nil, ""},
		"bounded": {&affected.E2EPlan{Scope: affected.E2EScopeBounded}, []string{}, ""},
		"selected": {&affected.E2EPlan{Scope: affected.E2EScopeUnknown, Unknown: invalid, Selected: []affected.E2ESelection{
			{Path: "e2e/login.spec.ts", Project: "chromium"}, {Path: "e2e/cart.spec.ts"}, {Path: "e2e/login.spec.ts", Project: "firefox"}}},
			[]string{"typescript:playwright:e2e/cart.spec.ts", "typescript:playwright:e2e/login.spec.ts"}, ""},
		"invalid declaration": {&affected.E2EPlan{Scope: affected.E2EScopeUnknown, Unknown: invalid}, nil,
			"plan.e2e is UNKNOWN and selects nothing (FLOWS_DECLARATION_INVALID .corvint/flows.json; its E2E tests cannot be selected)"},
		"underivable inventory": {&affected.E2EPlan{Scope: affected.E2EScopeFull,
			Unknown:  []affected.Unknown{{Reason: affected.UnknownE2EInventoryUnderivable, Detail: "discovery:MISSING"}},
			Selected: []affected.E2ESelection{{Path: "e2e/login.spec.ts"}}},
			[]string{"typescript:playwright:e2e/login.spec.ts"},
			"plan.e2e inventory is underivable (discovery:MISSING): only flow-linked E2E files are selected, so an E2E test no flow links is not run"},
		// FSV-V1-004 (V1-5410): an unsupported flows schema version selects
		// every E2E test and lists none, so the run is INCOMPLETE for them.
		"unsupported flows schema": {&affected.E2EPlan{Scope: affected.E2EScopeFull, Selected: []affected.E2ESelection{},
			Unknown: []affected.Unknown{{Reason: affected.UnknownFlowsSchemaUnsupported, Detail: ".corvint/flows.json version 7; supported 0"}}}, nil,
			"plan.e2e is FULL and lists nothing (FLOWS_SCHEMA_UNSUPPORTED .corvint/flows.json version 7; supported 0): every E2E test must run and this build cannot name them"},
		"unknown without a reason": {&affected.E2EPlan{Scope: affected.E2EScopeUnknown}, nil,
			"plan.e2e is UNKNOWN and selects nothing (its E2E tests cannot be selected)"},
	} {
		units, reason := testE2EUnits(test.e2e)
		if !reflect.DeepEqual(units, test.units) || reason != test.reason {
			t.Errorf("%s: units %q reason %q, want %q and %q", name, units, reason, test.units, test.reason)
		}
	}
}
