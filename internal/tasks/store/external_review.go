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

// reviewAudit re-audits a REVIEW_* request, and a KNOWHOW_ADD or RECONFIRM naming an
// attempt or generation (KHN-V0-008), with every attempt record, so the model
// judges leases, subject currency and provenance from journal-authoritative
// bytes (ERG-V0-009). Other operations keep their read boundary unchanged.
func reviewAudit(reader journal.Reader, inv *transaction.Inventory, env *mutation.Envelope, paths []string, canonical *journal.Result) ([]string, *journal.Result, error) {
	if !mutation.IsReviewOperation(env.Operation) && !mutation.KnowHowNamesAttempt(env) {
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
		rc, digest, elided, err := snapshot.DecodeRetainedReceipt(raw)
		if err != nil {
			return nil, nil, nil, err
		}
		// An elided liveness receipt (CAL-V0-300) changes only its
		// attempt's liveness members, so it posts no BUILT phase and no
		// artifact link; its chain link is still checked.
		if at > seq {
			if rc.Prev == nil || *rc.Prev != previous {
				return nil, nil, nil, wire.Errorf(wire.CodeJournalForked, receiptPath(at), "receipt does not chain to its predecessor")
			}
		}
		if at > seq && !elided {
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
		previous = digest
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
	b := newSealBlob(repo)
	fold := &transaction.ExternalReviewReceiptAudit{}
	err := foldReceipts(repo, b, last, pending, fold.Seed, func(rc *snapshot.Receipt, sum wire.Digest) error { return fold.Step(rc, sum, b.blob) })
	if err != nil {
		return nil, err
	}
	return fold, nil
}

// FoldReceiptBindings folds receipts 1..last, then pending when it is not
// nil, through every material binding audit in one pass: the ERG-V0-009
// review binding, the ESC-V0-010 escalation binding, the TOL-V0-013
// obligation-ledger binding and the CAL-V0-243 gate reuse binding. Receipt
// audit and redo use it, so a rehashed but untrue review, escalation, ledger
// event or gate reuse refuses as JOURNAL_FORKED.
func FoldReceiptBindings(repo *intent.Repository, last uint64, pending []byte) error {
	f := newBindingFold(repo)
	return foldReceipts(repo, f.b, last, pending, f.seed, f.step)
}

// ReceiptBindingFold returns a fresh FoldReceiptBindings step for receipts
// supplied in sequence order from 1, so a complete journal audit can fold
// the receipts it has just validated instead of reading them again. Each
// ticket post is read and decoded once for the review and escalation audits.
func ReceiptBindingFold(repo *intent.Repository) func(*snapshot.Receipt, wire.Digest) error {
	f := newBindingFold(repo)
	started := false
	return func(rc *snapshot.Receipt, sum wire.Digest) error {
		if !started {
			// A complete audit of a sealed store supplies receipts from the
			// anchor's successor: seed the folds there first (JRT-V0-008).
			started = true
			if err := seedFold(repo, f.b, rc, f.seed); err != nil {
				f.err = err
			}
		}
		if f.err != nil {
			return f.err
		}
		return f.step(rc, sum)
	}
}

type bindingFold struct {
	b           *sealBlob
	reviews     *transaction.ExternalReviewReceiptAudit
	escalations *transaction.EscalationReceiptAudit
	obligations *transaction.ObligationReceiptAudit
	memo        *transaction.TicketPosts
	reuses      *transaction.GateReuseReceiptAudit
	err         error
}

func newBindingFold(repo *intent.Repository) *bindingFold {
	return &bindingFold{
		b:           newSealBlob(repo),
		reviews:     &transaction.ExternalReviewReceiptAudit{},
		escalations: &transaction.EscalationReceiptAudit{},
		obligations: &transaction.ObligationReceiptAudit{Source: func(dir, commit string, paths []string) (string, map[string]bool, map[string][]byte, error) {
			root := repo.PrimaryWorktree
			if dir != "" {
				var err error
				if root, err = ExactWorktreeRoot(intent.ResolveTestRepository(repo.PrimaryWorktree, dir)); err != nil {
					return "", nil, nil, err
				}
			}
			return FilesAtCommit(root, commit, paths)
		}},
		memo:   &transaction.TicketPosts{},
		reuses: &transaction.GateReuseReceiptAudit{Descent: descentOracle(descentRoot(repo)), CIDescent: ciDescentOracle(descentRoot(repo))},
	}
}

func (f *bindingFold) seed(posts []snapshot.PostEntry, blob transaction.ExternalReviewBlob) error {
	if err := f.reviews.Seed(posts, blob); err != nil {
		return err
	}
	if err := f.escalations.Seed(posts, blob); err != nil {
		return err
	}
	if err := f.obligations.Seed(posts, blob); err != nil {
		return err
	}
	return f.reuses.Seed(posts, blob)
}

func (f *bindingFold) step(rc *snapshot.Receipt, sum wire.Digest) error {
	blob := f.b.blob
	if err := f.reviews.StepPosts(rc, sum, blob, f.memo); err != nil {
		return err
	}
	if err := f.escalations.StepPosts(rc, sum, blob, f.memo); err != nil {
		return err
	}
	if err := f.obligations.StepPosts(rc, sum, blob, f.memo); err != nil {
		return err
	}
	return f.reuses.StepPosts(rc, sum, blob, f.memo)
}

// foldReceipts folds receipts 1..last, then pending. On a sealed store it
// seeds the fold with the anchor afterimages and folds from the anchor's
// successor, which must link to the anchor (JRT-V0-008); a fold that ends
// inside the retired prefix refuses.
func foldReceipts(repo *intent.Repository, b *sealBlob, last uint64, pending []byte, seed func([]snapshot.PostEntry, transaction.ExternalReviewBlob) error, step func(*snapshot.Receipt, wire.Digest) error) (err error) {
	seal, err := readSeal(repo)
	if err != nil {
		return err
	}
	first := uint64(1)
	var link *wire.Digest
	if seal != nil {
		if last < seal.Seq() {
			return wire.Errorf(wire.CodeUnsupported, journal.RetiredPrefixFile, "receipts through %d were retired; their bindings are in the retained archive", seal.Retired())
		}
		if err := seed(b.sealPosts(seal), b.blob); err != nil {
			return err
		}
		first, link = seal.Seq()+1, &seal.Anchor.ReceiptSha256
	}
	// The walk starts at receipt 1 (or the anchor of a sealed store, whose
	// later receipts are never elided: compaction refuses there), so an
	// elided liveness receipt is folded as its restored original
	// (CAL-V0-301).
	restore := &snapshot.ReceiptRestorer{}
	fold := func(raw []byte) error {
		rc, original, err := restore.Next(raw)
		if err != nil {
			return err
		}
		if link != nil {
			if rc.Seq.Uint64() != first || rc.Prev == nil || *rc.Prev != *link {
				return wire.Errorf(wire.CodeJournalForked, "receipts", "the receipt after a retired prefix must link to its anchor")
			}
			link = nil
		}
		return step(rc, wire.Sum(original))
	}
	files := newReceiptFiles(repo)
	defer func() {
		if closeErr := files.close(err == nil); err == nil {
			err = closeErr
		}
	}()
	for seq := first; seq <= last; seq++ {
		raw, err := files.read(seq)
		if err != nil {
			return err
		}
		if err = fold(raw); err != nil {
			return err
		}
	}
	if pending != nil {
		return fold(pending)
	}
	return nil
}

// redoReviewBinding refuses redo of a pending receipt that posts a ticket
// record, or of a GATE_RESULT receipt (whose result may be a reuse,
// CAL-V0-243), unless the whole history, the pending receipt included,
// passes the material binding folds: the generic journal audit treats their
// events and results as opaque blobs, so this is where a rehashed but untrue
// one is caught.
func redoReviewBinding(repo *intent.Repository, receipt *snapshot.Receipt, raw []byte) error {
	if receipt.Kind == "GATE_RESULT" {
		return FoldReceiptBindings(repo, receipt.Seq.Uint64()-1, raw)
	}
	for _, p := range receipt.Post {
		if strings.HasPrefix(p.Path, "intent/tickets/") {
			return FoldReceiptBindings(repo, receipt.Seq.Uint64()-1, raw)
		}
	}
	return nil
}
