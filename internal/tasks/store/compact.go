package store

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CompactReport is what one liveness compaction found and did
// (CAL-V0-302). Byte counts are the physical sizes of receipts 1..HeadSeq.
type CompactReport struct {
	DryRun bool
	// HeadSeq is the settled head the compaction audited and walked.
	HeadSeq uint64
	// Elided counts the receipts this run rewrote (or, on a dry run, would
	// rewrite); AlreadyElided counts those an earlier run rewrote.
	Elided, AlreadyElided   uint64
	BytesBefore, BytesAfter int64
}

// candidate is one elidable liveness receipt not yet known to be superseded.
type candidate struct {
	name      string
	seq       uint64
	original  int
	tombstone []byte
}

// compactStage is a test seam run after each tombstone is published.
var compactStage = func(string) {}

// CompactLiveness rewrites every superseded liveness receipt of a settled
// journal as its taskman-receipt/1 form (CAL-V0-300..CAL-V0-302, decision
// 0538). It is opt-in and runs only when an operator asks: no write path
// calls it. Under the writer lock it requires a complete settled audit,
// drops both checkpoints, walks receipts 1..head with the restorer, and
// replaces a liveness receipt only once a later receipt posts the same
// attempt, so the head and the latest record of every attempt stay whole.
// Each tombstone is proven to restore to the original bytes before it is
// published by rename. A tombstone and its original are interchangeable to
// this binary, so an interrupted run leaves a valid journal and a rerun
// continues. The closing complete audit proves the result. On a segmented
// store the sealed receipts 1..S are read through the segment view and stay
// whole; only active receipts are rewritten (JSG-V0-012).
func CompactLiveness(ctx context.Context, repo *intent.Repository, dryRun bool) (*CompactReport, error) {
	return compactLiveness(ctx, repo, dryRun, snapshot.MaxElidedRun)
}

// compactLiveness keeps a superseded liveness receipt whole when the latest
// whole record of its attempt is maxRun or more receipts before it, so every
// tombstone has a whole record of its attempt fewer than maxRun receipts
// back and a bounded backward reader such as doctor can restore it.
func compactLiveness(ctx context.Context, repo *intent.Repository, dryRun bool, maxRun uint64) (*CompactReport, error) {
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	if err := settleSegmentSeal(repo); err != nil {
		return nil, err
	}
	if on, err := sealed(repo); err != nil || on {
		if err == nil {
			err = wire.Errorf(wire.CodeUnsupported, journal.RetiredPrefixFile, "compaction is refused on a store with a retired prefix (JRT-V0-010)")
		}
		return nil, err
	}
	head, err := readHead(repo)
	if err != nil {
		return nil, err
	}
	if err := compactAudit(repo, head); err != nil {
		return nil, err
	}
	report := &CompactReport{DryRun: dryRun, HeadSeq: head.LastSeq.Uint64()}
	if !dryRun {
		invalidateWriterCheckpoint(ctx, repo)
		if err := os.Remove(journal.CheckpointPath(repo.StateDir)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	restore := &snapshot.ReceiptRestorer{}
	pending := map[string]candidate{}
	// whole is the sequence number of the latest receipt that keeps each
	// attempt's record whole.
	whole := map[string]uint64{}
	var saved int64
	files := newReceiptFiles(repo)
	defer files.close(true)
	// Sealed receipts 1..S are read once each through the segment view and
	// stay whole: a segment's bytes are bound by digest (JSG-V0-012).
	sealedThrough := through(head)
	logical := journal.Logical(journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.IntentRoot()})
	for seq := uint64(1); seq <= report.HeadSeq; seq++ {
		var raw []byte
		if seq <= sealedThrough {
			raw, err = logical.Read(receiptPath(seq), wire.MaxReceiptFileBytes)
		} else {
			raw, err = files.read(seq)
		}
		if err != nil {
			return nil, err
		}
		report.BytesBefore += int64(len(raw))
		header, _, elided, err := snapshot.DecodeRetainedReceipt(raw)
		if err != nil {
			return nil, err
		}
		var path string
		var base []byte
		if header.AttemptID != nil {
			path = "attempts/" + *header.AttemptID + ".json"
			base = restore.Base(path)
		}
		_, original, err := restore.Next(raw)
		if err != nil {
			return nil, err
		}
		if elided {
			report.AlreadyElided++
		}
		// A later post of an attempt supersedes the candidate before it.
		for _, p := range header.Post {
			c, ok := pending[p.Path]
			if !ok {
				continue
			}
			delete(pending, p.Path)
			if c.seq-whole[p.Path] >= maxRun {
				whole[p.Path] = c.seq
				continue
			}
			if err := publishTombstone(repo, c, dryRun); err != nil {
				return nil, err
			}
			report.Elided++
			saved += int64(c.original - len(c.tombstone))
		}
		if elided {
			continue
		}
		ok := false
		if path != "" && seq > sealedThrough {
			var tombstone []byte
			if tombstone, ok, err = snapshot.ElideLiveness(original, base); err != nil {
				return nil, err
			}
			if ok {
				name, err := snapshot.ReceiptName(seq)
				if err != nil {
					return nil, err
				}
				pending[path] = candidate{name: name, seq: seq, original: len(raw), tombstone: tombstone}
			}
		}
		for _, p := range header.Post {
			if p.Path != path || !ok {
				whole[p.Path] = seq
			}
		}
	}
	report.BytesAfter = report.BytesBefore - saved
	if dryRun || report.Elided == 0 {
		return report, nil
	}
	dir, err := authority.OpenDir(repo, "receipts")
	if err != nil {
		return report, err
	}
	err = dir.Sync()
	if closeErr := dir.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return report, err
	}
	return report, compactAudit(repo, head)
}

// compactAudit requires a complete settled audit of head with no staging
// slot: compaction never runs over a pending receipt or a half-published
// write.
func compactAudit(repo *intent.Repository, head *snapshot.Head) error {
	proof, err := lockedJournalReader(repo, head).Audit()
	if err != nil {
		return err
	}
	if proof.StagingPresent || proof.LastSeq != head.LastSeq {
		return wire.Errorf(wire.CodeRedoPending, "receipts", "liveness compaction needs a settled journal with no staging slot")
	}
	return nil
}

// publishTombstone replaces one receipt with its proven taskman-receipt/1
// form: written and synced under a fixed name beside the state directory,
// never inside it, then renamed over the receipt.
func publishTombstone(repo *intent.Repository, c candidate, dryRun bool) error {
	if dryRun {
		return nil
	}
	tmp := repo.StateDir + ".compact.tmp"
	os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(c.tombstone)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(repo.StateDir, "receipts", c.name))
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	compactStage(c.name)
	return nil
}
