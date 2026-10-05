package store

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// reviewAudit re-audits a REVIEW_* request with every attempt record, so the
// model judges leases and subject currency from journal-authoritative bytes
// (ERG-V0-009). Other operations keep their read boundary unchanged.
func reviewAudit(reader journal.Reader, inv *transaction.Inventory, env *mutation.Envelope, paths []string, canonical *journal.Result) ([]string, *journal.Result, error) {
	if !mutation.IsReviewOperation(env.Operation) {
		return paths, canonical, nil
	}
	for _, file := range inv.Files() {
		if strings.HasPrefix(file.Path, "attempts/") {
			paths = append(paths, file.Path)
		}
	}
	canonical, err := reader.Audit(paths...)
	if err != nil {
		return paths, nil, err
	}
	if canonical.StagingPresent {
		return paths, nil, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	return paths, canonical, nil
}

// reviewInputs reads, for a REVIEW_* request, the target gate's head event at
// evidence/<head>, the request's subject receipt plus its successor when it
// is not the head receipt, and the submission history after the subject:
// every later receipt, each linked to its predecessor and the last to the
// head, streamed one at a time (ERG-V0-006). The model re-hashes every byte
// it is given; absent bytes make it refuse rather than infer.
func reviewInputs(repo *intent.Repository, env *mutation.Envelope, records map[string]journal.Record, head *snapshot.Head) (prior []byte, subject [][]byte, later *transaction.ExternalSubmissionHistory, err error) {
	if !mutation.IsReviewOperation(env.Operation) || env.TargetID == nil || head == nil {
		return nil, nil, nil, nil
	}
	p, ok := env.Payload.(*mutation.ReviewPayload)
	if !ok {
		return nil, nil, nil, nil
	}
	q, err := snapshot.DecodeExternalReviewRequest(wire.EncodeFile(p.Request))
	if err != nil {
		return nil, nil, nil, nil
	}
	if record, ok := records["intent/tickets/"+env.TargetID.Local+".json"]; ok {
		rec, err := ticket.Decode(record.Raw)
		if err != nil {
			return nil, nil, nil, err
		}
		if ref, ok := rec.ExternalReviews[q.GateID]; ok {
			prior, err = intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(ref.Head)), snapshot.MaxExternalReviewBytes)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, nil, nil, err
			}
		}
	}
	seq := q.Subject.ReceiptSeq.Uint64()
	if seq == 0 || seq > head.LastSeq.Uint64() {
		return prior, nil, nil, nil
	}
	later = &transaction.ExternalSubmissionHistory{}
	blob := ExternalReviewBlob(repo)
	var previous wire.Digest
	for at := seq; at <= head.LastSeq.Uint64(); at++ {
		raw, err := readReceiptBytes(repo, at)
		if err != nil {
			return nil, nil, nil, err
		}
		if at <= seq+1 {
			subject = append(subject, raw)
		}
		if at > seq {
			rc, err := snapshot.DecodeReceipt(raw)
			if err != nil {
				return nil, nil, nil, err
			}
			if rc.Prev == nil || *rc.Prev != previous {
				return nil, nil, nil, wire.Errorf(wire.CodeJournalForked, receiptPath(at), "receipt does not chain to its predecessor")
			}
			built, err := transaction.ExternalBuiltPosts(rc, blob)
			if err != nil {
				return nil, nil, nil, err
			}
			for _, b := range built {
				if b.TicketID == q.TicketID {
					later.Built = append(later.Built, b)
				}
			}
			for _, l := range transaction.ExternalArtifactPosts(rc, blob) {
				if l.TicketID == q.TicketID {
					later.Artifacts = append(later.Artifacts, l)
				}
			}
		}
		previous = wire.Sum(raw)
	}
	if head.LastReceiptSha256 == nil || previous != *head.LastReceiptSha256 {
		return nil, nil, nil, wire.Errorf(wire.CodeJournalForked, "head.json", "receipt chain differs from the head")
	}
	if q.Candidate.Kind == "EVIDENCE" {
		// An absent or unreadable artifact leaves Artifact nil, so the
		// candidate link reads UNKNOWN and the request refuses.
		later.Artifact, _ = blob(q.Candidate.Sha256)
	}
	return prior, subject, later, nil
}

// ExternalReviewBlob reads one retained blob by digest from evidence/: a
// review event, a blob-backed post or an EVIDENCE candidate artifact (a
// captured gate output, hence the gate-output bound). The pure readers
// re-hash every byte and bound each decoded kind, so an absent or altered
// file is UNKNOWN.
func ExternalReviewBlob(repo *intent.Repository) transaction.ExternalReviewBlob {
	return func(d wire.Digest) ([]byte, bool) {
		if _, err := wire.ParseDigest("head", string(d)); err != nil {
			return nil, false
		}
		raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(d)), wire.MaxGateOutputBytes)
		return raw, err == nil
	}
}

// FoldExternalReviews folds receipts 1..last, then pending when it is not
// nil, through the ERG-V0-009 binding audit. Every reader that acts on a
// review reference (receipt audit, the dispatcher observation) and redo of a
// pending receipt use it, so a review event that does not reproduce its
// receipt's transition refuses as JOURNAL_FORKED wherever it is consumed.
func FoldExternalReviews(repo *intent.Repository, last uint64, pending []byte) (*transaction.ExternalReviewReceiptAudit, error) {
	blob := ExternalReviewBlob(repo)
	fold := &transaction.ExternalReviewReceiptAudit{}
	step := func(raw []byte) error {
		rc, err := snapshot.DecodeReceipt(raw)
		if err != nil {
			return err
		}
		return fold.Step(rc, wire.Sum(raw), blob)
	}
	for seq := uint64(1); seq <= last; seq++ {
		raw, err := readReceiptBytes(repo, seq)
		if err != nil {
			return nil, err
		}
		if err = step(raw); err != nil {
			return nil, err
		}
	}
	if pending != nil {
		if err := step(pending); err != nil {
			return nil, err
		}
	}
	return fold, nil
}

// redoReviewBinding refuses redo of a pending receipt that posts a ticket
// record unless the whole history, the pending receipt included, passes the
// binding fold: the generic journal audit treats the review event as an
// opaque blob, so this is where a rehashed but untrue event is caught.
func redoReviewBinding(repo *intent.Repository, receipt *snapshot.Receipt, raw []byte) error {
	for _, p := range receipt.Post {
		if strings.HasPrefix(p.Path, "intent/tickets/") {
			_, err := FoldExternalReviews(repo, receipt.Seq.Uint64()-1, raw)
			return err
		}
	}
	return nil
}
