package journal

import (
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// pendingOnce shows a linked receipt beyond the head to the first read of
// it only: a writer between its receipt link-in and its head rename that
// then finishes (a removed link stands in for the applied head).
type pendingOnce struct {
	Native
	name string
	hits int
}

func (s *pendingOnce) Read(p string, n int) ([]byte, error) {
	if p == "receipts/"+s.name {
		s.hits++
		if s.hits == 1 {
			return []byte("{}"), nil
		}
	}
	return s.Native.Read(p, n)
}

// fullWalks counts complete audit attempts through Reader.ReceiptFold, which
// starts once per complete walk.
func fullWalks(r *Reader) *int {
	n := new(int)
	r.ReceiptFold = func() func(*snapshot.Receipt, wire.Digest) error {
		*n++
		return nil
	}
	return n
}

func headSum(t *testing.T, repo *fixture.Repo) wire.Digest {
	t.Helper()
	return wire.Sum(read(t, filepath.Join(repo.StateDir, "head.json")))
}

// TestV11060_InFlightTailIsRetriedBeforeCompleteAudit: a writer in flight
// during a checkpoint-resumed read is the head moving, so the read retries
// the resumed path within the CAL-V0-061 bound instead of walking every
// receipt at once. Before V1-1060 the first in-flight refusal fell through to
// the complete audit.
func TestV11060_InFlightTailIsRetriedBeforeCompleteAudit(t *testing.T) {
	repo, r := setup(t)
	for i := 0; i < 3; i++ {
		fixture.Commit(t, repo, "MUTATION")
	}
	r.Checkpoint = checkpointed(t, repo, r)
	fixture.Commit(t, repo, "MUTATION")
	full, err := r.Audit()
	if err != nil {
		t.Fatal(err)
	}
	next, _ := snapshot.ReceiptName(full.LastSeq.Uint64() + 1)
	src := &pendingOnce{Native: r.Source.(Native), name: next}
	r.Source = src
	walks := fullWalks(&r)
	got, err := r.Audit()
	if err != nil || got.Mode != ModeCheckpoint || *walks != 0 || src.hits != 2 {
		t.Fatalf("in-flight tail: %v mode %s, %d complete walks, %d reads of %s", err, got.Mode, *walks, src.hits, next)
	}
	if got.LastSeq != full.LastSeq || got.LastReceiptSha256 != full.LastReceiptSha256 || got.Identity.HeadSha256 != full.Identity.HeadSha256 {
		t.Fatalf("retried tail differs from the complete audit: seq %s/%s", got.LastSeq, full.LastSeq)
	}
}

// TestV11060_HeadPastCallerSnapshotStopsTheAudit: once a capture shows a head
// other than the caller's outer snapshot, no further attempt and no complete
// walk runs; the caller's outer reader re-probes. Without ExpectHeadSha256 the
// audit keeps retrying and walks again, as before.
func TestV11060_HeadPastCallerSnapshotStopsTheAudit(t *testing.T) {
	for _, tc := range []struct {
		name       string
		checkpoint bool
		expect     bool
		walks      int
	}{
		{"checkpoint, bound", true, true, 0},
		{"complete, bound", false, true, 1},
		{"complete, unbound", false, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, r := setup(t)
			for i := 0; i < 3; i++ {
				fixture.Commit(t, repo, "MUTATION")
			}
			if tc.checkpoint {
				r.Checkpoint = checkpointed(t, repo, r)
			}
			if tc.expect {
				r.ExpectHeadSha256 = headSum(t, repo)
			}
			walks := fullWalks(&r)
			captures, committed := 0, false
			r.afterCapture = func() {
				captures++
				if !committed {
					committed = true
					fixture.Commit(t, repo, "MUTATION")
				}
			}
			res, err := r.Audit()
			if *walks != tc.walks {
				t.Fatalf("%d complete walks, want %d", *walks, tc.walks)
			}
			if !tc.expect {
				if err != nil || res.Mode != ModeFull || res.Identity.HeadSha256 != headSum(t, repo) {
					t.Fatalf("unbound audit: %v", err)
				}
				return
			}
			if res != nil || wire.CodeOf(err) != wire.CodeSnapshotMoved || captures != 2 {
				t.Fatalf("bound audit: %v after %d captures", err, captures)
			}
			// The same reader bound to the new head resumes normally.
			r.ExpectHeadSha256 = headSum(t, repo)
			if res, err = r.Audit(); err != nil || res.Identity.HeadSha256 != r.ExpectHeadSha256 {
				t.Fatalf("rebound audit: %v", err)
			}
		})
	}
}
