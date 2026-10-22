package lrfrepo

import (
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
)

// TestOCMV0026PrepareRefusalNamesTheBoundThatFired pins that an ocm read or
// prepare refused by the stable verifier for a resource limit names the bound
// that fired and its value, and that other refusals keep their old text.
func TestOCMV0026PrepareRefusalNamesTheBoundThatFired(t *testing.T) {
	code := "unsupported-resource-limit"
	limited := &StableRefusal{Exit: 2, Result: verify.StableResult{
		Outcome: "UNSUPPORTED", Stage: "repository", Code: &code,
		LimitCause: "git-budget-exceeded: Git operation budget exhausted at its 1024-operation bound",
	}}
	err := stableOCMRefusal(limited)
	typed, _ := err.(*Error)
	want := "unsupported-resource-limit: stable CEM verification did not accept: unsupported-resource-limit (git-budget-exceeded: Git operation budget exhausted at its 1024-operation bound)"
	if typed == nil || typed.Code != code || err.Error() != want {
		t.Fatalf("got %q (%#v), want %q", err, err, want)
	}
	if got := limited.Error(); got != "unsupported-resource-limit: stable verification UNSUPPORTED at stage repository (git-budget-exceeded: Git operation budget exhausted at its 1024-operation bound)" {
		t.Fatalf("StableRefusal.Error()=%q", got)
	}
	other := "git-read-failed"
	plain := &StableRefusal{Exit: 2, Result: verify.StableResult{Outcome: "UNSUPPORTED", Stage: "repository", Code: &other}}
	if got := stableOCMRefusal(plain).Error(); got != "git-read-failed: stable CEM verification did not accept: git-read-failed" {
		t.Fatalf("non-limit refusal changed: %q", got)
	}
}
