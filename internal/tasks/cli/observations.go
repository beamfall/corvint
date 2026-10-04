package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"time"
)

// Heartbeat freshness describes a recorded signal, never physical quiescence.
func addHolderObservation(o *wire.Object, a *snapshot.Attempt, now time.Time) {
	status := "NOT_OBSERVED"
	last := wire.Null()
	if a.LastHeartbeatAt != nil {
		last = wire.String(string(*a.LastHeartbeatAt))
		at, _ := time.Parse(time.RFC3339, string(*a.LastHeartbeatAt))
		switch {
		case !a.Live():
			status = "TERMINAL"
		case a.Lease != nil && now.UTC().Format(time.RFC3339) >= string(a.Lease.ExpiresAt):
			status = "LEASE_EXPIRED"
		case now.Before(at):
			status = "CLOCK_BEFORE_HEARTBEAT"
		case now.Sub(at) >= 10*time.Minute:
			status = "STALE_HOLDER"
		default:
			status = "FRESH_HOLDER"
		}
	}
	o.Set("lastHeartbeatAt", last).Set("holderStatus", wire.String(status)).Set("observedAt", wire.String(now.UTC().Truncate(time.Second).Format(time.RFC3339))).Set("heartbeatTTLSeconds", wire.String("600"))
}
func retryObservation(rc *readCtx, attempts map[string]*snapshot.Attempt, rec *ticket.Record) wire.Value {
	if rc.journalAbsent {
		return wire.ObjectValue(wire.NewObject().Set("remainingMeaning", wire.String("RETRY_CAPACITY")).Set("retryAdmissionReason", wire.String("NOT_OBSERVED")).Set("charged", wire.String("NOT_OBSERVED")).Set("limit", wire.String(string(rc.store.Policy.AdmissionsPerRevision))).Set("remaining", wire.String("NOT_OBSERVED")).Set("exhausted", wire.Null()).Set("byReason", wire.Null()).Set("reasonHistory", wire.String("NOT_OBSERVED")))
	}
	return transaction.RetryObservation(attempts, rec, rc.store.Policy.AdmissionsPerRevision.Int())
}
