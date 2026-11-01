package store

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PSR-V0-169 (V1-1653): a scheduled refresh that publishes nothing leaves
// FullSeq behind. The writers after it must not each schedule another
// complete audit: the first refresh is due at WriterRefreshInterval, and
// retries only every writerRefreshRetry receipts up to WriterFullBound.
func TestPSRV0169_NonPublishingRefreshDoesNotRecurPerWrite(t *testing.T) {
	t.Parallel()
	const full = 1000
	due := func(lastSeq uint64) bool {
		w := &writerObservation{
			wc:    &journal.WriterCheckpoint{Checkpoint: journal.Checkpoint{Seq: wire.SizeOf(lastSeq)}, FullSeq: full},
			proof: &journal.Result{Head: &snapshot.Head{LastSeq: wire.SizeOf(lastSeq)}},
		}
		return advanceWriterCheckpoint(context.Background(), nil, w)
	}
	var scheduled []uint64
	previous := false
	// Every write from FullSeq to the bound, with no refresh ever publishing.
	for last := uint64(full); last+1-full < journal.WriterFullBound; last++ {
		d := last + 1 - full
		got := due(last)
		if got {
			scheduled = append(scheduled, d)
			if previous {
				t.Fatalf("consecutive writes at distance %d and %d both scheduled a complete audit", d-1, d)
			}
		}
		previous = got
	}
	want := (journal.WriterFullBound-1-journal.WriterRefreshInterval)/writerRefreshRetry + 1
	if len(scheduled) != want || scheduled[0] != journal.WriterRefreshInterval || scheduled[1] != journal.WriterRefreshInterval+writerRefreshRetry {
		t.Fatalf("scheduled %d refreshes starting %v, want %d starting at %d every %d", len(scheduled), scheduled[:min(3, len(scheduled))], want, journal.WriterRefreshInterval, writerRefreshRetry)
	}
	for _, d := range []uint64{1, journal.WriterRefreshInterval - 1, journal.WriterRefreshInterval + 1, journal.WriterRefreshInterval + writerRefreshRetry - 1} {
		if refreshDue(d) {
			t.Fatalf("distance %d scheduled a refresh", d)
		}
	}
}
