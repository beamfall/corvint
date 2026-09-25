package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// redoPending completes a transaction whose receipt was linked in but whose
// post files or head were not written (§5.2 crash point C2). It runs under
// the lock, before any new transaction is modelled.
//
// The receipt is the redo record: every post entry carries either the new
// bytes inline or the digest of a blob already published in `evidence/`, so
// completing it needs nothing but the receipt itself. Redo is applied by the
// same rule as a first write, and it never overwrites a third value.
func redoPending(repo *intent.Repository, session *authority.Session) (bool, error) {
	head, err := readHead(repo)
	if err != nil {
		return false, err
	}
	last := head.LastSeq.Uint64()
	if err = checkChainBounds(repo, head); err != nil {
		return false, err
	}
	raw, found, err := receiptBytes(repo, last+1)
	if err != nil || !found {
		return false, err
	}
	// Only the terminal, fully validated PRE_OR_POST result authorizes redo.
	proof, auditErr := journalReader(repo, head).Audit("intent/queue.json")
	if wire.CodeOf(auditErr) != wire.CodeRedoPending || proof == nil || !proof.Pending || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "PRE_OR_POST" {
		if auditErr != nil {
			return false, auditErr
		}
		return false, wire.Errorf(wire.CodeSnapshotMoved, "receipts", "pending receipt observation changed")
	}
	if proof.StagingPresent {
		return false, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	queue, err := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
	if err != nil {
		return false, err
	}
	if err = requireBranch(repo, queue.IntentBranch); err != nil {
		return false, err
	}
	if err = bindObservation(repo, proof.Identity, transaction.Mutate); err != nil {
		return false, err
	}
	current, found, err := receiptBytes(repo, last+1)
	if err != nil {
		return false, err
	}
	if !found || wire.Sum(current) != proof.LastReceiptSha256 || wire.Sum(raw) != proof.LastReceiptSha256 {
		return false, wire.Errorf(wire.CodeSnapshotMoved, "receipts", "validated pending receipt changed")
	}
	receipt, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		return false, err
	}
	if receipt.Seq.Uint64() != last+1 {
		return false, wire.Errorf(wire.CodeJournalForked, receiptPath(last+1), "pending receipt sequence differs from head")
	}
	if receipt.Prev == nil || head.LastReceiptSha256 == nil || *receipt.Prev != *head.LastReceiptSha256 {
		return false, wire.Errorf(wire.CodeJournalForked, receiptPath(last+1), "pending receipt does not chain to the head")
	}
	if err = redoPosts(repo, session, receipt); err != nil {
		return false, err
	}
	return true, advanceHead(repo, session, head, receipt, wire.Sum(raw))
}

// checkChainBounds refuses a store holding more than one unapplied receipt:
// two pending receipts cannot both chain to this head, so the journal forked.
func checkChainBounds(repo *intent.Repository, head *snapshot.Head) error {
	last := head.LastSeq.Uint64()
	settledRaw, found, err := receiptBytes(repo, last)
	if err != nil {
		return err
	}
	if !found {
		return wire.Errorf(wire.CodeJournalForked, receiptPath(last), "the head names a receipt that is not there")
	}
	if head.LastReceiptSha256 == nil || wire.Sum(settledRaw) != *head.LastReceiptSha256 {
		return wire.Errorf(wire.CodeJournalForked, receiptPath(last), "receipt digest differs from the head")
	}
	if _, found, err = receiptBytes(repo, last+2); err != nil {
		return err
	} else if found {
		return wire.Errorf(wire.CodeJournalForked, receiptPath(last+2), "a receipt beyond the pending one exists")
	}
	return nil
}

// redoPosts republishes every post entry of a pending receipt. A deletion
// (null post digest) is the UNPAUSE barrier removal, which this slice does
// not write and therefore refuses rather than guesses at.
func redoPosts(repo *intent.Repository, session *authority.Session, receipt *snapshot.Receipt) error {
	pre := make(map[string]*wire.Digest, len(receipt.Pre))
	for _, entry := range receipt.Pre {
		pre[entry.Path] = entry.Sha256
	}
	slot := 0
	occupied := map[authority.Slot]bool{}
	defer func() {
		for s, live := range occupied {
			if live {
				_ = session.RemoveStage(s)
			}
		}
	}()
	for _, entry := range receipt.Post {
		if entry.Sha256 == nil {
			return wire.Errorf(wire.CodeUnsupported, entry.Path, "redo of a deletion is outside this slice")
		}
		raw, err := postBytes(repo, entry)
		if err != nil {
			return err
		}
		how, err := settled(repo, entry.Path, *entry.Sha256, pre[entry.Path])
		if err != nil {
			return err
		}
		if how == skipPublication {
			continue
		}
		target, err := postTarget(entry.Path)
		if err != nil {
			return err
		}
		if key, dir := parentOf(target); key != "" {
			if err = ensureDir(repo, session, key, dir); err != nil {
				return err
			}
		}
		name := authority.Slot(slotName(slot))
		stage, err := session.Prepare(name, target.Role, raw)
		if err != nil {
			return err
		}
		occupied[name] = true
		slot++
		if err = publish(session, stage, target, how, pre[entry.Path]); err != nil {
			return err
		}
		if how == replacePublication {
			occupied[name] = false
		}
	}
	return nil
}

// postBytes recovers one post entry's bytes: inline for a record, from the
// published blob otherwise. The bytes are checked against the digest the
// receipt names, so a blob that does not match is never published.
func postBytes(repo *intent.Repository, entry snapshot.PostEntry) ([]byte, error) {
	var raw []byte
	switch {
	case entry.Record != nil:
		raw = wire.EncodeFile(*entry.Record)
	case entry.BlobSha256 != nil:
		blob, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(*entry.BlobSha256)), wire.MaxEvidenceBlobBytes)
		if err != nil {
			return nil, err
		}
		raw = blob
	default:
		if entry.Path != "VERSION" {
			return nil, wire.Errorf(wire.CodeMalformed, entry.Path, "post entry carries neither a record nor a blob")
		}
		raw = []byte(snapshot.VersionBytes)
	}
	if wire.Sum(raw) != *entry.Sha256 {
		return nil, wire.Errorf(wire.CodeJournalForked, entry.Path, "redo bytes differ from the digest the receipt names")
	}
	return raw, nil
}

// advanceHead writes the head the pending receipt implies, completing the
// transaction. Every other head field is carried forward unchanged.
func advanceHead(repo *intent.Repository, session *authority.Session, head *snapshot.Head, receipt *snapshot.Receipt, digest wire.Digest) error {
	next := *head
	next.LastSeq = receipt.Seq
	next.LastReceiptSha256 = &digest
	next.Generation = receipt.HeadGeneration
	raw := wire.EncodeFile(next.Value())
	if _, err := snapshot.DecodeHead(raw); err != nil {
		return err
	}
	target := authority.Target{Role: authority.RoleHead, Name: "head.json"}
	current, err := currentDigest(repo, "head.json")
	if err != nil {
		return err
	}
	stage, err := session.Prepare(authority.Slot("a00"), target.Role, raw)
	if err != nil {
		return err
	}
	if current == nil {
		defer func() { _ = session.RemoveStage(authority.Slot("a00")) }()
		return session.Link(stage, target)
	}
	// The rename consumes the slot, so it is not removed afterwards.
	return session.Replace(stage, target, current)
}

func readHead(repo *intent.Repository) (*snapshot.Head, error) {
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), 4096)
	if err != nil {
		return nil, err
	}
	head, err := snapshot.DecodeHead(raw)
	if err != nil {
		return nil, err
	}
	if head.PrimaryWorktree != repo.PrimaryWorktree {
		return nil, wire.Errorf(wire.CodeUnsupported, "head.json", "primary worktree differs; relocation unsupported")
	}
	return head, nil
}

func receiptPath(seq uint64) string {
	name, _ := snapshot.ReceiptName(seq)
	return "receipts/" + name
}

func receiptBytes(repo *intent.Repository, seq uint64) ([]byte, bool, error) {
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		return nil, false, err
	}
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "receipts", name), wire.MaxReceiptFileBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return raw, true, nil
}
