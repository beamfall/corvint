package lrf

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestRetiredLRF0RefusesEveryContext (CEM-V1-055, CEM-V1-215): lrf/0 carries no
// cem/0.x context row; Evaluate refuses every request and issues no bytes, and
// the stable admission refuses a relabeled cem/0.x context.
func TestRetiredLRF0RefusesEveryContext(t *testing.T) {
	target := strings.Repeat("2", 40)
	excluded := wire.ExcludedCEMPath
	for _, spec := range []string{"cem/0.1", "cem/0.2", wire.StableSpec} {
		request := Request{Context: Context{
			CEMSpec: spec, CEMMapSHA256: strings.Repeat("a", 64), PatchSource: "canonical-derived",
			BaseRevision: strings.Repeat("1", 40), TargetRevision: &target, PatchSHA256: strings.Repeat("b", 64),
			ExcludedPath: &excluded,
		}}
		if _, err := Evaluate(request); err == nil || err.(*Error).Code != "unsupported-lrf-context" {
			t.Fatalf("Evaluate(%s) = %v, want unsupported-lrf-context", spec, err)
		}
		if err := ValidateStableRequest(request); err == nil || err.(*Error).Code != "unsupported-lrf-context" {
			t.Fatalf("ValidateStableRequest(%s) = %v, want unsupported-lrf-context", spec, err)
		}
	}
	if raw := CanonicalBytes(Result{}); raw != nil && len(raw) != 0 {
		t.Fatalf("caller-constructed result serialized as %q", raw)
	}
}
