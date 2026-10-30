//go:build darwin || linux

package store

import "testing"

// TestLGLV0015_GateLoadDefaultsToLiveSizedQueue (V1-2866): the lease
// gate-load harness runs over a live-sized queue unless a size is requested,
// so the per-ticket decode under the gate is part of every default run, and
// it never builds fewer tickets than its writers need.
func TestLGLV0015_GateLoadDefaultsToLiveSizedQueue(t *testing.T) {
	if gateLoadTicketsDefault < 800 {
		t.Fatalf("default %d tickets, want at least 800", gateLoadTicketsDefault)
	}
	for _, c := range []struct{ requested, writers, want int }{
		{0, 15, gateLoadTicketsDefault},
		{31, 15, 31},
		{10, 15, 31},
		{2000, 15, 2000},
		{0, 1000, 1016},
	} {
		if got := gateLoadTickets(c.requested, c.writers); got != c.want {
			t.Fatalf("gateLoadTickets(%d, %d) = %d, want %d", c.requested, c.writers, got, c.want)
		}
	}
}
