//go:build darwin || linux

package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// declinedResume makes every prefix-fallback audit of the command decline,
// the PSR-V0-052 class that keeps a command on the complete audit (V1-1626:
// the shape behind issue 1063), and runs each complete audit for real with
// mid-walk, the fold that starts each complete attempt. walks counts the
// complete attempts the command ran.
func declinedResume(mid func(attempt int), walks *int) func(*inventoryHooks) {
	return func(h *inventoryHooks) {
		h.audit = func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
			if r.PrefixFallback {
				return nil, journal.PhysicalObservation{}, journal.ErrPrefixDeclined
			}
			r.ReceiptFold = func() func(*snapshot.Receipt, wire.Digest) error {
				*walks++
				attempt := *walks
				first := true
				return func(*snapshot.Receipt, wire.Digest) error {
					if first {
						first = false
						mid(attempt)
					}
					return nil
				}
			}
			return r.AuditForWriteObserved()
		}
	}
}

// PSR-V0-150 (V1-1626, issue 1063): once a command's resume is declined,
// each preparation round runs one complete attempt, which ends SNAPSHOT_MOVED
// when the journal moves under it, and the round's guarded audit is not rerun
// against the head the round lost. A dispatcher that commits during every
// walk therefore costs one complete walk a round (16 for the command), not
// four blind attempts and a second four-attempt audit a round; one that
// stops lets the next round settle and commit.
func TestPSRV0150_DeclinedResumeWalksOncePerRound(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, tc := range []struct {
		name   string
		writes int // dispatcher writes, one per complete attempt; negative: every attempt
		rounds int
		moved  int
		done   bool
	}{
		{"writer-outpaces-every-walk", -1, 16, 16, false},
		{"writer-stops-after-three", 3, 4, 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newResumeFixture(t)
			var walks int
			mid := func(attempt int) {
				if tc.writes >= 0 && attempt > tc.writes {
					return
				}
				if d := stagedWrite(writerCreate(f.repo, "mid-walk-"+string(rune('a'+attempt)), f.now), nil); !d.completed() {
					t.Errorf("dispatcher write: %+v %v", d.rep, d.err)
				}
			}
			run := moving(t, f, f.acquire("bounded-acquire"), 0, nil, declinedResume(mid, &walks))
			if run.completed() != tc.done || (!tc.done && wire.CodeOf(run.err) != wire.CodeSnapshotMoved) {
				t.Fatalf("acquire: %+v %v stages %v", run.rep, run.err, run.stages)
			}
			timing := run.timing
			if timing.Rounds != tc.rounds || walks != tc.rounds || timing.CompleteWalks != tc.rounds || timing.CompleteMoved != tc.moved {
				t.Fatalf("rounds %d, fold starts %d, complete walks %d (moved %d), want %d rounds and %d moved: %+v", timing.Rounds, walks, timing.CompleteWalks, timing.CompleteMoved, tc.rounds, tc.moved, timing)
			}
			// PSR-V0-151: the split names the declined resume and charges
			// every walk to the complete column.
			if timing.ResumeDeclines != 1 || timing.PrefixWalks != 0 || timing.ResumedWalks != 0 || timing.Resumed != 0 || timing.CompleteWalk <= 0 || timing.CompleteWalk > timing.SnapshotRead {
				t.Fatalf("walk split: %+v", timing)
			}
		})
	}
}

// PSR-V0-151 (V1-1626): an acquire that resumes reports its one complete
// walk, the prefix walk that became the base and the resumed walk, with no
// decline, and the resumed route serves the write (PSR-V0-052).
func TestPSRV0151_TimingSplitsResumedAcquire(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	run := moving(t, f, f.acquire("split-acquire"), 1, nil)
	if !run.completed() || run.timing.Resumed != 1 {
		t.Fatalf("acquire: %+v %v resumed %d stages %v", run.rep, run.err, run.timing.Resumed, run.stages)
	}
	timing := run.timing
	if timing.CompleteWalks != 1 || timing.CompleteMoved != 0 || timing.PrefixWalks != 0 || timing.ResumedWalks != 1 || timing.ResumeDeclines != 0 || timing.CompleteWalk <= 0 || timing.ResumedWalk <= 0 {
		t.Fatalf("walk split: %+v", timing)
	}
	// A prefix walk shows in its own column when the complete attempt moves.
	moved := newResumeFixture(t)
	wrote := false
	hooks := func(h *inventoryHooks) {
		audit := h.audit
		h.audit = func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
			if r.PrefixFallback && !wrote {
				r.ReceiptFold = func() func(*snapshot.Receipt, wire.Digest) error {
					return func(*snapshot.Receipt, wire.Digest) error {
						if !wrote {
							wrote = true
							if d := stagedWrite(writerCreate(moved.repo, "mid-prefix", moved.now), nil); !d.completed() {
								t.Errorf("dispatcher write: %+v %v", d.rep, d.err)
							}
						}
						return nil
					}
				}
			}
			return audit(r)
		}
	}
	run = moving(t, moved, moved.acquire("split-prefix"), 0, nil, hooks)
	if !run.completed() || run.timing.Resumed != 1 {
		t.Fatalf("acquire over a prefix: %+v %v resumed %d stages %v", run.rep, run.err, run.timing.Resumed, run.stages)
	}
	timing = run.timing
	if timing.CompleteWalks != 1 || timing.CompleteMoved != 1 || timing.PrefixWalks != 1 || timing.PrefixMoved != 0 || timing.ResumedWalks != 1 || timing.PrefixWalk <= 0 {
		t.Fatalf("walk split after a moved complete attempt: %+v", timing)
	}
}

// PSR-V0-151 (V1-1626): a resumed route that declines before its audit,
// here because the writer checkpoint was invalidated after the resume base
// was captured, charges a resume decline and no resumed walk; the complete
// route then serves the write. The retry inside a resumed audit is
// TestPSRV0151_ResumedWalksCountEachAttempt in internal/tasks/journal.
func TestPSRV0151_DeclinedResumeChargesNoResumedWalk(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	invalidate := func(int) { invalidateWriterCheckpoint(context.Background(), f.repo) }
	run := moving(t, f, f.acquire("split-decline"), 1, invalidate)
	if !run.completed() || run.fast() || run.timing.Resumed != 0 {
		t.Fatalf("acquire: %+v %v resumed %d stages %v", run.rep, run.err, run.timing.Resumed, run.stages)
	}
	if !strings.Contains(strings.Join(run.stages, "\n"), "invalidated since the resume base") {
		t.Fatalf("resumed route did not decline on the token: %v", run.stages)
	}
	if timing := run.timing; timing.ResumedWalks != 0 || timing.ResumedWalk != 0 || timing.ResumeDeclines < 1 {
		t.Fatalf("walk split after a declined resume: %+v", timing)
	}
}

// PSR-V0-153 (V1-1626): the bounded complete audit keeps the CAL-V0-116
// rewritten-prefix detection. A receipt rewritten with its stamps restored
// while the first complete attempt walks, as the journal also moves, is
// refused by the next round's attempt: nothing commits, the head stays where
// the tamper left it, and the decision is the complete route's own.
func TestPSRV0153_BoundedAuditKeepsRewrittenPrefixDetection(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	second, _ := snapshot.ReceiptName(2)
	var walks int
	var tampered uint64
	mid := func(attempt int) {
		if attempt != 1 {
			return
		}
		if d := stagedWrite(writerCreate(f.repo, "mid-tamper", f.now), nil); !d.completed() {
			t.Errorf("dispatcher write: %+v %v", d.rep, d.err)
		}
		rewriteReceiptKeepingStamps(t, filepath.Join(f.repo.StateDir, "receipts", second))
		tampered = headSeqOf(t, f.repo)
	}
	run := moving(t, f, f.acquire("bounded-tamper"), 0, nil, declinedResume(mid, &walks))
	if run.completed() || run.fast() || run.timing.Resumed != 0 || wire.CodeOf(run.err) == wire.CodeSnapshotMoved {
		t.Fatalf("acquire over a rewritten prefix: %+v %v stages %v", run.rep, run.err, run.stages)
	}
	// Round 1 moved; round 2's attempt refuses in one complete walk, and the
	// guarded audit reuses that refusal instead of walking again (V1-1703).
	if timing := run.timing; timing.Rounds != 2 || timing.CompleteWalks != 2 || timing.CompleteMoved != 1 {
		t.Fatalf("rounds: %+v", timing)
	}
	if got := headSeqOf(t, f.repo); got != tampered {
		t.Fatalf("head moved from %d to %d over a rewritten prefix", tampered, got)
	}
	oracle := stagedWrite(f.acquire("bounded-oracle"), nil)
	sameDecision(t, run.writerRun, oracle)
}
