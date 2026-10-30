package store

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestPSRV0227_RecordedOwnerAndRemoverTolerateJitter (V1-1091): a program
// owner or worktree remover recorded by another process and read again with
// btime one second off is still live; another start tick is gone; a
// matching start tick with btime further off is QUIESCENCE_UNPROVED, never
// a gone process. Exact equality stays the same process.
func TestPSRV0227_RecordedOwnerAndRemoverTolerateJitter(t *testing.T) {
	const recorded = "linux:1000:77"
	prev := recordedProcessIdentity
	t.Cleanup(func() { recordedProcessIdentity = prev })
	for _, c := range []struct {
		name, fresh string
		want        string
	}{
		{"exact", recorded, wire.CodeAttemptLive},
		{"jitter tolerated", "linux:1001:77", wire.CodeAttemptLive},
		{"start ticks mismatch", "linux:1001:78", ""},
		{"more than 1 s", "linux:1002:77", wire.CodeQuiescenceUnproved},
	} {
		recordedProcessIdentity = func(int) (string, error) { return c.fresh, nil }
		owner := snapshot.Program{OwnerPID: 4000001, OwnerStarted: recorded}
		if got := wire.CodeOf(ownerNotLive(c.fresh, owner)); got != c.want {
			t.Fatalf("%s: owner %q, want %q", c.name, got, c.want)
		}
		owner.OwnerReleased = true
		if e := ownerNotLive(c.fresh, owner); e != nil {
			t.Fatalf("%s: a released owner refused: %v", c.name, e)
		}
		remover := snapshot.Program{Phase: "WORKTREE_REMOVE", RemoverPID: 4000002, RemoverStarted: recorded}
		if got := wire.CodeOf(removerStopped(remover)); got != c.want {
			t.Fatalf("%s: remover %q, want %q", c.name, got, c.want)
		}
		if got := recordedMatch(c.fresh, recorded) == supervisor.IdentitySame; got != (c.want == wire.CodeAttemptLive) {
			t.Fatalf("%s: recordedMatch same = %v", c.name, got)
		}
	}
	if recordedMatch("", "") != supervisor.IdentitySame {
		t.Fatal("exact equality of two empty identities changed")
	}
}
