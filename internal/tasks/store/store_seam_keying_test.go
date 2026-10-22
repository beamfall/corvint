package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// TestV10961_TestSeamsAreKeyedByStore checks that a fixed attempt-ID source
// and a seal stop set for one store leave every other store on the
// production behaviour, which is what lets tests over different stores run
// in parallel (V1-0961).
func TestV10961_TestSeamsAreKeyedByStore(t *testing.T) {
	t.Parallel()
	fixed := &intent.Repository{StateDir: t.TempDir()}
	other := &intent.Repository{StateDir: t.TempDir()}
	fixAttemptEntropy(t, fixed, fixedEntropy(0xab))
	withSealStop(t, fixed, "segment")

	want := "attempt:q:" + strings.Repeat("ab", 16)
	if id, err := mintAttemptID(fixed, "queue:q"); err != nil || id != want {
		t.Fatalf("fixed store minted %q %v, want %q", id, err, want)
	}
	a, errA := mintAttemptID(other, "queue:q")
	b, errB := mintAttemptID(other, "queue:q")
	if errA != nil || errB != nil || a == want || a == b {
		t.Fatalf("other store minted %q and %q (%v %v), want two fresh IDs", a, b, errA, errB)
	}
	if err := segmentSealStage(fixed, "segment"); !errors.Is(err, errSegmentSealStop) {
		t.Fatalf("fixed store seal stage: %v, want the stop", err)
	}
	if err := segmentSealStage(other, "segment"); err != nil {
		t.Fatalf("other store seal stage: %v, want none", err)
	}
}
