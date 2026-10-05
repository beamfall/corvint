package cli

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// LeaseTimingProfile names the diagnostic `--timing` object (issue 494).
const LeaseTimingProfile = "taskman-lease-timing/0"

// timingVerbs are the lease commands that accept `--timing`.
var timingVerbs = map[string]bool{"claim": true, "renew": true, "attempt heartbeat": true, "release": true}

// timedLease runs one lease command with a phase collector and reports it as
// a `timing` member of the first item, on every outcome. The flag is not
// part of the request, so replay, digests and receipts are unchanged.
func timedLease(ctx context.Context, started time.Time, cmd []string, repo *intent.Repository, actor mutation.Binding, choice store.LeaseChoice, now wire.Timestamp) *wire.Result {
	var t store.LeaseTiming
	report, err := store.Lease(store.WithLeaseTiming(ctx, &t), repo, actor, choice, now)
	timing := leaseTimingValue(&t, time.Since(started))
	if err != nil {
		res := errorResult(cmd, err)
		res.Items = []wire.Value{wire.ObjectValue(wire.NewObject().Set("timing", timing))}
		return res
	}
	res := leaseResult(cmd, report)
	res.Items[0].Obj.Set("timing", timing)
	return res
}

func leaseTimingValue(t *store.LeaseTiming, total time.Duration) wire.Value {
	ms := func(d time.Duration) wire.Value {
		if d < 0 {
			d = 0
		}
		return wire.String(string(wire.SizeOf(uint64(d.Milliseconds()))))
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(LeaseTimingProfile))
	o.Set("totalMillis", ms(total))
	o.Set("transactions", wire.String(string(wire.SizeOf(uint64(t.Transactions)))))
	o.Set("preparationRounds", wire.String(string(wire.SizeOf(uint64(t.Rounds)))))
	o.Set("admissionWaitMillis", ms(t.AdmissionWait))
	o.Set("guardMillis", ms(t.Guards))
	o.Set("snapshotReadMillis", ms(t.SnapshotRead))
	o.Set("validationMillis", ms(t.Validation))
	o.Set("monitorCloseMillis", ms(t.MonitorClose))
	o.Set("lockWaitMillis", ms(t.LockWait))
	o.Set("lockHoldMillis", ms(t.LockHold))
	o.Set("journalWriteMillis", ms(t.JournalWrite))
	o.Set("fsyncMillis", ms(t.Fsync))
	return wire.ObjectValue(o)
}

// timedFailure reports an ERROR raised before the store writer ran; with
// `--timing` on a timing verb it carries the timing item with every phase 0.
func timedFailure(timed bool, started time.Time, cmd []string, err error) *wire.Result {
	res := errorResult(cmd, err)
	if timed {
		res.Items = []wire.Value{wire.ObjectValue(wire.NewObject().Set("timing", leaseTimingValue(&store.LeaseTiming{}, time.Since(started))))}
	}
	return res
}
