package journal

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// commitOnRead commits one more receipt the first time receipt 2 is read,
// so the attempt that read it ends SNAPSHOT_MOVED and the audit retries.
type commitOnRead struct {
	Source
	repo *fixture.Repo
	t    *testing.T
	done bool
}

func (s *commitOnRead) Read(p string, n int) ([]byte, error) {
	raw, err := s.Source.Read(p, n)
	if p == "receipts/000000000002.json" && !s.done {
		s.done = true
		fixture.Commit(s.t, s.repo, "MUTATION")
	}
	return raw, err
}

// TestV11053_ReceiptFoldFollowsCompleteAudit pins Reader.ReceiptFold: one
// fresh fold per complete audit attempt sees every validated receipt once,
// in order, with the digest of its retained bytes; its first error stops the
// fold without changing the audit; a checkpoint read never folds.
func TestV11053_ReceiptFoldFollowsCompleteAudit(t *testing.T) {
	repo, r := setup(t)
	for i := 0; i < 3; i++ {
		fixture.Commit(t, repo, "MUTATION")
	}
	var seqs []uint64
	var sums []wire.Digest
	starts, failAt := 0, uint64(0)
	boom := errors.New("fold refused")
	r.ReceiptFold = func() func(*snapshot.Receipt, wire.Digest) error {
		starts++
		seqs, sums = nil, nil
		return func(rc *snapshot.Receipt, sum wire.Digest) error {
			seqs = append(seqs, rc.Seq.Uint64())
			sums = append(sums, sum)
			if rc.Seq.Uint64() == failAt {
				return boom
			}
			return nil
		}
	}
	check := func(res *Result, err error, wantStarts int) {
		t.Helper()
		if err != nil || res.Mode != ModeFull || starts != wantStarts {
			t.Fatalf("audit %v mode %s starts %d", err, res.Mode, starts)
		}
		last := res.LastSeq.Uint64()
		if uint64(len(seqs)) != last {
			t.Fatalf("folded %v of %d receipts", seqs, last)
		}
		for i, seq := range seqs {
			name, _ := snapshot.ReceiptName(seq)
			if seq != uint64(i+1) || sums[i] != wire.Sum(read(t, filepath.Join(repo.StateDir, "receipts", name))) {
				t.Fatalf("fold %d saw receipt %d with digest %s", i, seq, sums[i])
			}
		}
		if folded, ferr := res.ReceiptFold(); folded != last || ferr != nil {
			t.Fatalf("ReceiptFold %d %v, want %d", folded, ferr, last)
		}
	}
	res, err := r.Audit()
	check(res, err, 1)

	// Movement retries the attempt, and the retry folds afresh.
	moving := r
	moving.Source = &commitOnRead{Source: r.Source, repo: repo, t: t}
	starts = 0
	res, err = moving.Audit()
	check(res, err, 2)

	// The fold's first error ends the fold, not the audit.
	starts, failAt = 0, 2
	res, err = r.Audit()
	if err != nil || len(seqs) != 2 {
		t.Fatalf("audit %v folded %v", err, seqs)
	}
	if folded, ferr := res.ReceiptFold(); folded != 1 || ferr != boom {
		t.Fatalf("ReceiptFold %d %v", folded, ferr)
	}

	// A checkpoint read does not walk every receipt and never folds.
	starts, failAt = 0, 0
	plain := r
	plain.ReceiptFold = nil
	r.Checkpoint = checkpointed(t, repo, plain)
	res, err = r.Audit()
	if err != nil || res.Mode != ModeCheckpoint || starts != 0 {
		t.Fatalf("checkpoint audit %v mode %s starts %d", err, res.Mode, starts)
	}
	if folded, ferr := res.ReceiptFold(); folded != 0 || ferr != nil {
		t.Fatalf("checkpoint ReceiptFold %d %v", folded, ferr)
	}
}
