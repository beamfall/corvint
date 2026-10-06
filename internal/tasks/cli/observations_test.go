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
			addHolderObservation(o, a, base.Add(tc.delta), 600)
			status, _ := o.Get("holderStatus")
			if status.Str != tc.want {
				t.Fatalf("got %s want %s", status.Str, tc.want)
			}
		})
	}
}

// TestCALV0120_HolderObservationUsesPolicyTTL classifies the same recorded
// signal against the effective policy TTL and reports that TTL; legacy
// absence stays NOT_OBSERVED, never STALE_HOLDER, at any TTL.
func TestCALV0120_HolderObservationUsesPolicyTTL(t *testing.T) {
	at := wire.Timestamp("2026-10-01T00:00:00Z")
	base, e := time.Parse(time.RFC3339, string(at))
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name      string
		ttl       int64
		delta     time.Duration
		heartbeat bool
		want      string
	}{
		{"short fresh", 300, 299 * time.Second, true, "FRESH_HOLDER"},
		{"short boundary", 300, 300 * time.Second, true, "STALE_HOLDER"},
		{"long fresh", 1800, 1799 * time.Second, true, "FRESH_HOLDER"},
		{"long boundary", 1800, 1800 * time.Second, true, "STALE_HOLDER"},
		{"legacy short", 300, 59 * time.Minute, false, "NOT_OBSERVED"},
		{"legacy long", 86400, 59 * time.Minute, false, "NOT_OBSERVED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &snapshot.Attempt{Phase: "RUNNING", Lease: &snapshot.Lease{ExpiresAt: "2026-10-01T01:00:00Z"}}
			if tc.heartbeat {
				a.LastHeartbeatAt = &at
			}
			o := wire.NewObject()
			addHolderObservation(o, a, base.Add(tc.delta), tc.ttl)
			status, _ := o.Get("holderStatus")
			ttl, _ := o.Get("heartbeatTTLSeconds")
			if status.Str != tc.want || ttl.Str != string(wire.CountOf(tc.ttl)) {
				t.Fatalf("got %s/%s want %s/%d", status.Str, ttl.Str, tc.want, tc.ttl)
			}
		})
	}
}
