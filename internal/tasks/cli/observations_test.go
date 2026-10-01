package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
	"time"
)

func TestCALV0048_HolderObservationBoundaries(t *testing.T) {
	at := wire.Timestamp("2026-10-01T00:00:00Z")
	base, e := time.Parse(time.RFC3339, string(at))
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name           string
		delta          time.Duration
		phase, expires string
		heartbeat      bool
		want           string
	}{
		{"unobserved", 0, "RUNNING", "2026-10-01T01:00:00Z", false, "NOT_OBSERVED"},
		{"fresh", 10*time.Minute - time.Second, "RUNNING", "2026-10-01T01:00:00Z", true, "FRESH_HOLDER"},
		{"TTL boundary", 10 * time.Minute, "RUNNING", "2026-10-01T01:00:00Z", true, "STALE_HOLDER"},
		{"expired", 10 * time.Minute, "RUNNING", "2026-10-01T00:10:00Z", true, "LEASE_EXPIRED"},
		{"terminal", time.Minute, "CANCELLED", "2026-10-01T01:00:00Z", true, "TERMINAL"},
		{"backward clock", -time.Second, "RUNNING", "2026-10-01T01:00:00Z", true, "CLOCK_BEFORE_HEARTBEAT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &snapshot.Attempt{Phase: tc.phase, Lease: &snapshot.Lease{ExpiresAt: wire.Timestamp(tc.expires)}}
			if tc.heartbeat {
				a.LastHeartbeatAt = &at
			}
			o := wire.NewObject()
			addHolderObservation(o, a, base.Add(tc.delta))
			status, _ := o.Get("holderStatus")
			if status.Str != tc.want {
				t.Fatalf("got %s want %s", status.Str, tc.want)
			}
		})
	}
}
