package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestPSRV0227_RecordedRunAndChildTolerateJitter (V1-1091): a detached
// run's supervisor or command, and a detached lease child, recorded by
// another process and read again with btime one second off, are live;
// another start tick is gone; a matching start tick with btime further off
// is QUIESCENCE_UNPROVED for a run and, for a lease child, unconfirmed: live
// only inside the startup window, never ended on identity alone.
func TestPSRV0227_RecordedRunAndChildTolerateJitter(t *testing.T) {
	const recorded = "linux:1000:77"
	prev := attemptProcessIdentity
	t.Cleanup(func() { attemptProcessIdentity = prev })
	now := time.Now().UTC()
	for _, c := range []struct {
		name, fresh string
		live        bool
		code        string
		childLive   bool // started an hour ago
		childFresh  bool // started now
	}{
		{"jitter tolerated", "linux:1001:77", true, "", true, true},
		{"start ticks mismatch", "linux:1001:78", false, "", false, false},
		{"more than 1 s", "linux:1002:77", false, wire.CodeQuiescenceUnproved, false, true},
	} {
		attemptProcessIdentity = func(int) (string, error) { return c.fresh, nil }
		live, err := processLive(4000001, recorded)
		if live != c.live || wire.CodeOf(err) != c.code {
			t.Fatalf("%s: processLive = %v %v", c.name, live, err)
		}
		restore := SetLeaseDetachIdentity(func(int) (string, error) { return c.fresh, nil })
		for _, started := range []struct {
			at   time.Time
			want bool
		}{{now.Add(-time.Hour), c.childLive}, {now, c.childFresh}} {
			dir := t.TempDir()
			if err := writeLeaseDetachJSON(dir, leaseDetachChildName, leaseDetachChild{Pid: 4000002, Identity: recorded, StartedAt: started.at.Format(time.RFC3339), CeilingAt: now.Add(time.Hour).Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
			var child leaseDetachChild
			got, err := leaseDetachChildLive(dir, &child, now)
			if err != nil || got != started.want {
				t.Fatalf("%s: child started %s live = %v %v, want %v", c.name, started.at, got, err, started.want)
			}
		}
		restore()
	}
}
