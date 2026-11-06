// SPDX-License-Identifier: Apache-2.0
package localcompletion

import (
	"context"
	"encoding/json"
	"testing"
)

func enrollmentEvidence(t *testing.T, result Evaluation) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	evidence, _ := value["enrollmentEvidence"].(string)
	return evidence
}

// LCP-V0-8209: source-only restoration cannot prove historical inactivity;
// an explicit cancelled record, unlike missing private state, is observed.
func TestLCPV08209MissingEnrollmentIsNotHistoricalInactivity(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	missing, err := Evaluate(ctx, root, key)
	if err != nil || enrollmentEvidence(t, missing) != "NOT_OBSERVED" || missing.Lifecycle != "inactive" || missing.Satisfied {
		t.Fatalf("missing: %#v %v", missing, err)
	}
	beginFixture(t, root, key, plan)
	repo, err := open(root, key)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	saved.Lifecycle = "cancelled"
	if err = repo.save(saved); err != nil {
		t.Fatal(err)
	}
	cancelled, err := Evaluate(ctx, root, key)
	if err != nil || enrollmentEvidence(t, cancelled) != "PRESENT" || cancelled.Lifecycle != "cancelled" || cancelled.Satisfied {
		t.Fatalf("cancelled: %#v %v", cancelled, err)
	}
}
