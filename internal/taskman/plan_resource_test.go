package taskman

import (
	"encoding/json"
	"testing"
)

// CAL-V0-097: Core decodes a Tasks default plan that carries the optional
// resourceDeferred rows and pool-blocked DEFERRED RESOURCE_COLLISION
// entries, and still refuses malformed rows and pool blockers it cannot
// bind to a row.
func TestCALV0097_CoreDecodesResourceDeferredPlan(t *testing.T) {
	raw, err := encode(testPlan(t, testCapture(t)))
	if err != nil {
		t.Fatal(err)
	}
	row := func() map[string]any {
		return map[string]any{"poolId": "lanes", "availability": "OBSERVED", "freeEligibleMembers": "2", "selected": "2", "deferred": "1"}
	}
	build := func(edit func(plan map[string]any, deferred map[string]any)) error {
		var plan map[string]any
		if err := json.Unmarshal(raw, &plan); err != nil {
			t.Fatal(err)
		}
		var deferred map[string]any
		for _, e := range plan["entries"].([]any) {
			if entry := e.(map[string]any); entry["state"] == "DEFERRED" && deferred == nil {
				deferred = entry
			}
		}
		if deferred == nil {
			t.Fatal("fixture plan has no DEFERRED entry")
		}
		edit(plan, deferred)
		_, err := decodePlan(testValue(t, plan))
		return err
	}
	poolWait := func(plan, entry map[string]any) {
		plan["resourceDeferred"] = []any{row()}
		entry["reason"], entry["blockers"] = "RESOURCE_COLLISION", []any{"lanes"}
	}
	if err := build(func(map[string]any, map[string]any) {}); err != nil {
		t.Fatalf("plan without resourceDeferred: %v", err)
	}
	if err := build(poolWait); err != nil {
		t.Fatalf("resource-aware plan: %v", err)
	}
	if err := build(func(plan, entry map[string]any) {
		r := row()
		r["availability"], r["freeEligibleMembers"] = "NOT_OBSERVED", nil
		plan["resourceDeferred"] = []any{r}
		entry["reason"], entry["blockers"] = "RESOURCE_COLLISION", []any{"lanes"}
	}); err != nil {
		t.Fatalf("unobserved pool row: %v", err)
	}
	for name, edit := range map[string]func(plan, entry map[string]any){
		"pool blocker without row": func(plan, entry map[string]any) {
			entry["reason"], entry["blockers"] = "RESOURCE_COLLISION", []any{"lanes"}
		},
		"pool blocker on another reason": func(plan, entry map[string]any) {
			poolWait(plan, entry)
			entry["reason"] = "LIMIT_EXCEEDED"
		},
		"extra row member": func(plan, entry map[string]any) {
			poolWait(plan, entry)
			r := row()
			r["extra"] = "1"
			plan["resourceDeferred"] = []any{r}
		},
		"observed row without count": func(plan, entry map[string]any) {
			poolWait(plan, entry)
			r := row()
			r["freeEligibleMembers"] = nil
			plan["resourceDeferred"] = []any{r}
		},
		"unobserved row with count": func(plan, entry map[string]any) {
			poolWait(plan, entry)
			r := row()
			r["availability"] = "NOT_OBSERVED"
			plan["resourceDeferred"] = []any{r}
		},
		"duplicate pool row": func(plan, entry map[string]any) {
			poolWait(plan, entry)
			plan["resourceDeferred"] = []any{row(), row()}
		},
		"unknown availability": func(plan, entry map[string]any) {
			poolWait(plan, entry)
			r := row()
			r["availability"] = "MAYBE"
			plan["resourceDeferred"] = []any{r}
		},
	} {
		if err := build(edit); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
