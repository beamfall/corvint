package doccorpus

import (
	"slices"
	"strings"
	"testing"
)

// behaviorAdapterUseMatcher rewrites the fixture's one assertion, in the reviewed
// outcome, the test's declared check and the flow's ordered events, to the given
// matcher and value, then rebinds the runtime witness to the new contract with
// the runtime assertion event carrying runtimeValue.
func behaviorAdapterUseMatcher(t *testing.T, fixture *behaviorAdapterTestFixture, matcher, value, runtimeValue string) {
	t.Helper()
	behaviorAdapterEditRow(t, &fixture.request, "variations", func(row map[string]any) {
		for _, outcome := range row["outcomes"].([]any) {
			outcome.(map[string]any)["matcher"], outcome.(map[string]any)["value"] = matcher, value
		}
	})
	behaviorAdapterEditRow(t, &fixture.request, "tests", func(row map[string]any) {
		for _, check := range row["checks"].([]any) {
			check.(map[string]any)["matcher"], check.(map[string]any)["value"] = matcher, value
		}
	})
	behaviorAdapterEditRow(t, &fixture.request, "flows", func(row map[string]any) {
		for _, event := range row["sequence"].([]any) {
			if event.(map[string]any)["kind"] == "assertion" {
				event.(map[string]any)["matcher"], event.(map[string]any)["value"] = matcher, value
			}
		}
	})
	contract := buildBehaviorAdapter(t, fixture.request, nil).Provider.BehaviorContracts.ContractSHA256
	behaviorAdapterUpdateRun(t, &fixture.request, func(run *BehaviorRun) {
		run.ContractSHA256 = contract
		for index := range run.Events {
			if run.Events[index].Kind == "assertion" {
				run.Events[index].Matcher, run.Events[index].Value = matcher, runtimeValue
			}
		}
	})
}

// DCP-V1-083 DCP-V1-084: an assertion event that behavior-events.cjs records for a valueless
// state matcher (matcher toBeVisible, value visible) equals a reviewed outcome
// that declares the same matcher and canonical value, so the variation keeps its
// runtime witness; any other non-empty value stays a mismatch, and an empty
// reviewed outcome or declared check value is refused as an invalid declaration.
func TestDCPV1084StateMatcherAssertionIsWitnessed(t *testing.T) {
	t.Parallel()
	assertionGaps := func(result BehaviorAdapterResult) []string {
		kinds := []string{}
		for _, d := range result.Frontier {
			if d.Kind == "semantic-mismatch" || d.Kind == "missing-assertion-event" || d.Kind == "assertion-contradiction" {
				kinds = append(kinds, d.Kind)
			}
		}
		return kinds
	}
	for _, tc := range []struct{ matcher, value string }{
		{"toBeVisible", "visible"}, {"toBeHidden", "hidden"}, {"toBeEnabled", "enabled"}, {"toBeChecked", "checked"},
	} {
		t.Run(tc.matcher, func(t *testing.T) {
			fixture := behaviorAdapterFixture(t)
			behaviorAdapterUseMatcher(t, &fixture, tc.matcher, tc.value, tc.value)
			result := buildBehaviorAdapter(t, fixture.request, nil)
			if gaps := assertionGaps(result); len(gaps) != 0 {
				t.Fatalf("state matcher assertion not equal to its reviewed outcome: %v %+v", gaps, result.Frontier)
			}
			if len(result.Coverage) != 5 || result.Coverage[4].Value != 1 {
				t.Fatalf("state matcher variation lost its runtime witness: %+v", result.Coverage)
			}
		})
	}
	fixture := behaviorAdapterFixture(t)
	behaviorAdapterUseMatcher(t, &fixture, "toBeVisible", "visible", "true")
	result := buildBehaviorAdapter(t, fixture.request, nil)
	if gaps := assertionGaps(result); !slices.Contains(gaps, "assertion-contradiction") || !slices.Contains(gaps, "missing-assertion-event") {
		t.Fatalf("non-canonical runtime value was not a mismatch: %v", gaps)
	}
	for _, input := range []struct{ id, field, refusal string }{
		{"variations", "outcomes", "variation identity or semantic contract is invalid"},
		{"tests", "checks", "mapped assertion identity is incomplete"},
	} {
		fixture := behaviorAdapterFixture(t)
		behaviorAdapterEditRow(t, &fixture.request, input.id, func(row map[string]any) {
			for _, item := range row[input.field].([]any) {
				item.(map[string]any)["matcher"], item.(map[string]any)["value"] = "toBeVisible", ""
			}
		})
		raw, err := Encode(fixture.request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), input.refusal) {
			t.Fatalf("empty %s value was not refused as an invalid declaration: %v", input.field, err)
		}
	}
}
