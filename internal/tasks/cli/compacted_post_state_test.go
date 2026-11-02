//go:build darwin || linux

package cli_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0302_CompactedStepsKeepTheirPostState is the retained
// reproduce-or-refute check of V1-3669 ("compacted post-records unreadable
// by the CLI"). It records a claim and three heartbeats, reads each step's
// recorded result through a same-request-id retry, runs `receipt compact`
// (which elides the two superseded heartbeats), and reads every step again:
// each retry still replays with the same items, attempt show still reads the
// latest heartbeat, and receipt audit and doctor still read the journal.
// The defect does not reproduce: compaction elides only the attempt record
// of a superseded heartbeat or renew, and every CLI read restores it.
func TestCALV0302_CompactedStepsKeepTheirPostState(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	root, claims := leaseCLIStore(t, 1, now.Add(-11*time.Minute))
	c := claims[0]
	heartbeat := func(i int) run {
		return atm(t, root, nil, "attempt", "heartbeat", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--request-id", "hb-"+strconv.Itoa(i))
	}
	for i := 0; i < 3; i++ {
		if h := heartbeat(i); h.code != 0 {
			t.Fatalf("heartbeat %d: %s", i, h.stdout)
		}
	}
	recorded := func() []string {
		t.Helper()
		var out []string
		for i := 0; i < 3; i++ {
			r := heartbeat(i)
			if r.code != 0 || r.res.Outcome != wire.OutcomeOK || len(r.res.Items) != 1 || !field(r.res.Items[0], "replayed").Bool {
				t.Fatalf("replay hb-%d: %s", i, r.stdout)
			}
			out = append(out, string(wire.EncodeFile(r.res.Items[0])))
		}
		show := atm(t, root, nil, "attempt", "show", c.AttemptID)
		if show.code != 0 || len(show.res.Items) != 1 {
			t.Fatalf("attempt show: %s", show.stdout)
		}
		return append(out, string(wire.EncodeFile(show.res.Items[0])))
	}
	before := recorded()
	compact := atm(t, root, nil, "receipt", "compact")
	if compact.code != 0 || len(compact.res.Items) != 1 || field(compact.res.Items[0], "elided").Str != "2" {
		t.Fatalf("compact: %s", compact.stdout)
	}
	after := recorded()
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("step %d read differs after compaction:\nbefore %s\nafter  %s", i, before[i], after[i])
		}
	}
	if a := atm(t, root, nil, "receipt", "audit"); a.code != 0 || a.res.Outcome != wire.OutcomeOK {
		t.Fatalf("audit after compact: %s", a.stdout)
	}
	doctorOK(t, root)
}
