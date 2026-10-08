package store

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// TestCALV0198_SupervisedAcquireIsNotPrepared pins that a supervised
// attempt's QUIESCENCE_UNPROVED acquire refusal starts no health preparation,
// so a repeated refused acquire cannot probe and quarantine pool members.
func TestCALV0198_SupervisedAcquireIsNotPrepared(t *testing.T) {
	live := &snapshot.Attempt{Lease: &snapshot.Lease{}}
	if !acquirePreparable(live) {
		t.Fatal("an unsupervised live attempt must be preparable")
	}
	supervised := &snapshot.Attempt{Lease: &snapshot.Lease{}, Supervision: &snapshot.Supervision{}}
	for name, a := range map[string]*snapshot.Attempt{"missing": nil, "unleased": {}, "supervised": supervised} {
		if acquirePreparable(a) {
			t.Fatalf("%s attempt prepared", name)
		}
	}
}
