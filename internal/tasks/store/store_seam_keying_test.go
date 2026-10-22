package store

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// TestV10961_AttemptEntropyIsKeyedByStore checks that a fixed attempt-ID
// source set for one store leaves every other store on fresh IDs, which is
// what lets the resume tests over different stores run in parallel
// (V1-0961).
func TestV10961_AttemptEntropyIsKeyedByStore(t *testing.T) {
	t.Parallel()
	fixed := &intent.Repository{StateDir: t.TempDir()}
	other := &intent.Repository{StateDir: t.TempDir()}
	fixAttemptEntropy(t, fixed, fixedEntropy(0xab))

	want := "attempt:q:" + strings.Repeat("ab", 16)
	if id, err := mintAttemptID(fixed, "queue:q"); err != nil || id != want {
		t.Fatalf("fixed store minted %q %v, want %q", id, err, want)
	}
	a, errA := mintAttemptID(other, "queue:q")
	b, errB := mintAttemptID(other, "queue:q")
	if errA != nil || errB != nil || a == want || a == b {
		t.Fatalf("other store minted %q and %q (%v %v), want two fresh IDs", a, b, errA, errB)
	}
}
