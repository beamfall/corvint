//go:build darwin || linux

package dispatch

import (
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// CAL-V0-139 with CAL-V0-129: a lane member reaching minAgeSeconds changes
// the roster without any store change, so the idle gate wakes for it, and a
// minimum age crossed during the full tick leaves the gate unarmed.
func TestCALV0139_IdleGateWakesForALaneMinimumAge(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles = []Role{{Name: "lane", Host: "sh", Cap: 4, Lane: &Lane{Pool: "pool", MinAgeSeconds: 10}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60}}
	t0 := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	clock := t0
	q := &witnessQueue{witness: "w1"}
	q.obs.Members = []Member{{Pool: "pool", Member: "m1", State: "QUARANTINED", Changed: "7"}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Now = func() time.Time { return clock }
	tick := func(want int, why string) {
		t.Helper()
		if err := d.Tick(context.Background()); err != nil {
			t.Fatalf("%s: tick: %v", why, err)
		}
		if q.observes != want {
			t.Fatalf("%s: %d full observations, want %d", why, q.observes, want)
		}
	}
	tick(1, "first tick")
	tick(2, "a tick that records the observation reads in full")
	tick(2, "a young member leaves the tick idle")
	if d.idle == nil || !d.idle.due.Equal(t0.Add(10*time.Second)) {
		t.Fatalf("idle gate %+v, want one due at the member's minimum age", d.idle)
	}

	// A minimum age that falls after the tick started and at or before
	// its end leaves the gate unarmed.
	raw, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	mark := idleMark{witness: q.witness, ok: true, state: sha256.Sum256(raw), since: t0.Add(9 * time.Second)}
	clock = t0.Add(10 * time.Second)
	d.idle = nil
	d.idleSettle(mark, nil)
	if d.idle != nil {
		t.Fatalf("a minimum age crossed during the tick armed the gate %+v", d.idle)
	}
	clock = t0.Add(9500 * time.Millisecond)
	d.idleSettle(mark, nil)
	if d.idle == nil {
		t.Fatal("a future minimum age did not arm the gate")
	}

	clock = t0.Add(11 * time.Second)
	tick(3, "a member reaching its minimum age reads in full")
}
