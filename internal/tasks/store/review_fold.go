package store

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ReviewFold carries one long-running reader's ERG-V0-009 review binding
// fold from one observation to the next (CAL-V0-138), so a dispatcher tick
// folds only the receipts appended since its previous tick. The fold is a
// pure left fold over the hash-chained history: it continues only while the
// receipt it last folded keeps its digest and every new receipt chains to
// its predecessor. Any other observation (a shorter or rewritten history, an
// unreadable receipt, or a binding refusal) discards the carried state and
// answers exactly as FoldExternalReviews does from receipt 1. The state is
// process memory only; nothing is written.
type ReviewFold struct {
	audit *transaction.ExternalReviewReceiptAudit
	seq   uint64
	sum   wire.Digest
}

// Fold returns the review binding fold of receipts 1..last. The returned
// audit is owned by f and valid until the next Fold call.
func (f *ReviewFold) Fold(repo *intent.Repository, last uint64) (*transaction.ExternalReviewReceiptAudit, error) {
	if f.advance(repo, last) {
		return f.audit, nil
	}
	f.audit, f.seq, f.sum = nil, 0, ""
	audit := &transaction.ExternalReviewReceiptAudit{}
	blob := ExternalReviewBlob(repo)
	var sum wire.Digest
	err := foldReceipts(repo, last, nil, func(rc *snapshot.Receipt, s wire.Digest) error {
		sum = s
		return audit.Step(rc, s, blob)
	})
	if err != nil {
		return nil, err
	}
	if last > 0 {
		f.audit, f.seq, f.sum = audit, last, sum
	}
	return audit, nil
}

// advance continues the carried fold to last, reporting false when it
// cannot prove the carried prefix is still the history's prefix.
func (f *ReviewFold) advance(repo *intent.Repository, last uint64) (ok bool) {
	if f.audit == nil || f.seq == 0 || last < f.seq {
		return false
	}
	files := newReceiptFiles(repo)
	defer func() {
		if files.close(ok) != nil {
			ok = false
		}
	}()
	raw, err := files.read(f.seq)
	if err != nil || wire.Sum(raw) != f.sum {
		return false
	}
	blob := ExternalReviewBlob(repo)
	prev := f.sum
	for seq := f.seq + 1; seq <= last; seq++ {
		raw, err := files.read(seq)
		if err != nil {
			return false
		}
		rc, err := snapshot.DecodeReceipt(raw)
		if err != nil || rc.Prev == nil || *rc.Prev != prev {
			return false
		}
		sum := wire.Sum(raw)
		if err := f.audit.Step(rc, sum, blob); err != nil {
			return false
		}
		prev = sum
	}
	f.seq, f.sum = last, prev
	return true
}
