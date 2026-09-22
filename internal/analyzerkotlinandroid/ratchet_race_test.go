//go:build race

package analyzerkotlinandroid

import (
	"bytes"
	"testing"
)

// Race instrumentation intentionally has no allocation ceiling. It still
// proves the representative request is bounded, deterministic, and framed.
func TestRepresentativeRaceCorrectnessAndBounds(t *testing.T) {
	raw := canonical(t, pinned(t))
	if len(raw) > MaxRequestBytes {
		t.Fatal("representative request exceeds bound")
	}
	first := mustProcess(t, raw)
	if len(first) > MaxOutputBytes || !bytes.HasSuffix(first, []byte("\n")) {
		t.Fatal("representative output is unbounded or unframed")
	}
	for index := 0; index < 32; index++ {
		if got := mustProcess(t, raw); !bytes.Equal(got, first) {
			t.Fatal("race build output drift")
		}
	}
}
