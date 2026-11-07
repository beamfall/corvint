package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// ASD-V0-576 (V1-3390): the repository's AGENTS.md Verify section declares
// `make changed-checks`, so `corvint affected` names it, conditional on a change
// that reaches the inputs of the three doc gates it runs.
func TestASDV0576AffectedListsChangedChecks(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	checks, unknown, truncated := mandatoryAffectedChecks(root)
	if truncated || len(unknown) != 0 {
		t.Fatalf("declarations truncated=%v unknown=%q", truncated, unknown)
	}
	for _, check := range checks {
		if check.Command != "make changed-checks BASE=FULL_SHA" {
			continue
		}
		for _, gate := range []string{"line-citations-check", "unbounded-readers-check", "test-read-scopes-check"} {
			if check.Kind != adviceKindConditional || check.Source != adviceAgentsName || !strings.Contains(check.Reason, gate) {
				t.Fatalf("make changed-checks check = %+v, want a conditional AGENTS.md check naming %s", check, gate)
			}
		}
		return
	}
	t.Fatalf("affected advice lists no make changed-checks: %+v", checks)
}
