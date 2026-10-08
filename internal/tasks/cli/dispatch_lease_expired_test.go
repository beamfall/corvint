package cli_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func observedAttempt(t *testing.T, q dispatch.Queue, id string) dispatch.Attempt {
	t.Helper()
	obs, err := q.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range obs.Attempts {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("attempt %s not observed", id)
	return dispatch.Attempt{}
}

// TestCALV0191_NativeReapExpiredReportsOnlyAnActualReap drives the
// dispatcher's native ReapExpired through the real CLI and store: a reap
// fenced on a lease expiry the store no longer holds refuses and leaves the
// attempt live; the reap at the observed expiry reports reaped, and so does
// its replay, and the observation then carries the reap's cause and holder
// (the durable record a lost stop is recovered from); a reap of an attempt released since the observation reports
// nothing reaped and no error.
func TestCALV0191_NativeReapExpiredReportsOnlyAnActualReap(t *testing.T) {
	t.Run("CAL-V0-191 fenced, reaped and replayed", func(t *testing.T) {
		root, claimed := expiredCLIStore(t, 1)
		q := cli.DispatchQueueForTest(root)
		a := observedAttempt(t, q, claimed[0].AttemptID)
		if !a.Live || a.LeaseExpires.IsZero() || a.LeaseExpires.After(time.Now()) {
			t.Fatalf("observed attempt %+v", a)
		}
		moved := a
		moved.LeaseExpires = a.LeaseExpires.Add(-time.Minute)
		if reaped, err := q.ReapExpired(context.Background(), moved, "reap-moved"); reaped || err == nil || !strings.Contains(err.Error(), wire.CodeFenced) {
			t.Fatalf("reap at a moved expiry: reaped %v err %v", reaped, err)
		}
		if again := observedAttempt(t, q, a.ID); !again.Live {
			t.Fatalf("the fenced reap moved the attempt: %+v", again)
		}
		for _, try := range []string{"first", "replay"} {
			if reaped, err := q.ReapExpired(context.Background(), a, "reap-observed"); !reaped || err != nil {
				t.Fatalf("%s reap: reaped %v err %v", try, reaped, err)
			}
		}
		if after := observedAttempt(t, q, a.ID); after.Live || after.Phase != "FAILED" || after.Cause != "LEASE_EXPIRED" || after.Holder != a.Holder {
			t.Fatalf("reaped attempt %+v", after)
		}
	})
	t.Run("CAL-V0-191 released before the reap", func(t *testing.T) {
		root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-10*time.Minute).Truncate(time.Second))
		q := cli.DispatchQueueForTest(root)
		a := observedAttempt(t, q, claimed[0].AttemptID)
		release := atm(t, root, nil, "release", "--attempt", a.ID, "--generation", a.Generation, "--request-id", "release-first")
		if release.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release: %+v", release.res)
		}
		if reaped, err := q.ReapExpired(context.Background(), a, "reap-late"); reaped || err != nil {
			t.Fatalf("reap after release: reaped %v err %v", reaped, err)
		}
		if after := observedAttempt(t, q, a.ID); after.Phase != "CANCELLED" || after.Cause == "LEASE_EXPIRED" {
			t.Fatalf("released attempt %+v", after)
		}
	})
}
