package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// CEM-V1-644: the counter `cem defer` installs counts an intent past the
// per-map OCM obligation bound, so a spec too large to declare can still be
// deferred, and keeps refusing a document that is not an intent.
func TestCEMV1644CounterAdmitsAnIntentOverTheOCMBound(t *testing.T) {
	var spec strings.Builder
	spec.WriteString("# Oversized\n\n## Requirements\n\n")
	want := lrfrepo.MaxOCMObligations + 1
	for number := 1; number <= want; number++ {
		fmt.Fprintf(&spec, "- `BIG-V0-%03d`: MUST hold.\n", number)
	}
	if refusal := lrfrepo.IntentRequirementsRefusal([]byte(spec.String())); refusal != "too-many-obligations" {
		t.Fatalf("fixture must exceed the OCM bound, refusal %q", refusal)
	}
	count, err := countIntentObligations([]byte(spec.String()))
	if err != nil || count != want {
		t.Fatalf("count = %d, %v; want %d", count, err, want)
	}
	if _, err := countIntentObligations([]byte("# Not an intent\n")); err == nil {
		t.Fatal("a document with no Requirements section was counted")
	}
}
