package store

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Journal retention (JRT-V0, docs/specs/corvint-tasks-journal-retention-v0.md).

// RetireReport is what one `receipt retire` or `receipt unretire` found and
// did (JRT-V0-002, JRT-V0-011).
type RetireReport struct {
	DryRun bool
	// Recovered is ROLLED_FORWARD or ROLLED_BACK when the run only finished
	// an interrupted retire or unretire (JRT-V0-009); nothing else ran.
	Recovered string
	// AnchorSeq is S; Retired counts receipts 1..S-1.
	AnchorSeq, Retired uint64
	// ReceiptBytesBefore and ReceiptBytesAfter are the physical receipt
	// payload bytes in the live store before and after.
	ReceiptBytesBefore, ReceiptBytesAfter uint64
	SealBytes                             uint64
	ArchiveSha256                         wire.Digest
	ArchiveBytes                          uint64
}

const (
	recoveredForward = "ROLLED_FORWARD"
	recoveredBack    = "ROLLED_BACK"
)

// retireStage is a test seam run after each durable step; a non-nil error
// stops the run there as a crash would, with no cleanup.
var retireStage = func(string) error { return nil }

func retireTemp(repo *intent.Repository) string { return repo.StateDir + ".retire.tmp" }

func statePath(repo *intent.Repository, name string) string {
	return filepath.Join(repo.StateDir, name)
}

func present(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot observe: %v", err)
	}
	return true, nil
}

// readSeal returns the store's retired prefix, or nil when it has none.
func readSeal(repo *intent.Repository) (*journal.RetiredPrefix, error) {
	raw, err := intent.ReadFile(statePath(repo, journal.RetiredPrefixFile), journal.MaxRetiredPrefixBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return journal.DecodeRetiredPrefix(raw)
}

// sealed reports whether the store carries a retired prefix (JRT-V0-010).
func sealed(repo *intent.Repository) (bool, error) {
	return present(statePath(repo, journal.RetiredPrefixFile))
}

// retainedInventory is transaction.NewInventory over a store that may carry
// a retired prefix, whose receipts below the anchor are absent by design
// (JRT-V0-004).
func retainedInventory(repo *intent.Repository, files []archive.FileEntry, dirs []string) (*transaction.Inventory, error) {
	inv, err := transaction.NewInventory(files, dirs)
	if err != nil {
		return nil, err
	}
	on, err := sealed(repo)
	if err != nil {
		return nil, err
	}
	if on {
		if err := inv.RetirePrefix(); err != nil {
			return nil, err
		}
	}
	return inv, nil
}

// RetirePrefix retires receipts 1..S-1 of a settled journal whose head is S
// (JRT-V0-002..JRT-V0-009). It is operator-invoked only: no write path calls
// it. Under the writer lock it requires a complete settled audit, an
// eligible anchor and passing binding folds; it exports the store to
// archivePath, a new file outside the state directory, verifies that export
// and binds its digest into the seal, and only then drops the prefix.
// --dry-run runs every check except the export and writes nothing.
func RetirePrefix(ctx context.Context, repo *intent.Repository, archivePath string, dryRun bool) (*RetireReport, error) {
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	if recovered, err := recoverRetire(ctx, repo); err != nil || recovered != "" {
		return &RetireReport{Recovered: recovered}, err
	}
	if on, err := sealed(repo); err != nil || on {
		if err == nil {
			err = wire.Errorf(wire.CodeUnsupported, journal.RetiredPrefixFile, "the store already carries a retired prefix; retire is one-shot (unretire first)")
		}
		return nil, err
	}
	if !dryRun {
		if archivePath, err = checkArchiveTarget(repo, archivePath); err != nil {
			return nil, err
		}
	}
	head, err := writerGuards(repo, "RETIRE")
	if err != nil {
		return nil, err
	}
	if head.Segments != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, snapshot.SegmentsDir, "retire is refused on a segmented journal (JSG-V0-013); run journal unseal first")
	}
	if on, err := present(statePath(repo, "barrier.json")); err != nil || on {
		if err == nil {
			err = wire.Errorf(wire.CodePaused, "barrier.json", "a paused store is not retired")
		}
		return nil, err
	}
	reader := lockedJournalReader(repo, head)
	proof, obs, err := reader.AuditWriterRefresh()
	if obs.Cleanup != nil {
		return nil, errors.Join(err, obs.Cleanup)
	}
	if err != nil {
		return nil, err
	}
	if proof.IntentError != nil {
		return nil, proof.IntentError
	}
	if proof.Mode != journal.ModeFull || proof.Pending || proof.StagingPresent || proof.LastSeq != head.LastSeq {
		return nil, wire.Errorf(wire.CodeRedoPending, "receipts", "retire needs a complete settled audit with no pending receipt or staging slot")
	}
	placeholder := wire.Sum(nil)
	seal, err := reader.NewRetiredPrefix(proof, obs.Files, placeholder, 0)
	if err != nil {
		return nil, err
	}
	if err := retireEligible(seal); err != nil {
		return nil, err
	}
	if err := FoldReceiptBindings(repo, seal.Seq(), nil); err != nil {
		return nil, err
	}
	report := &RetireReport{DryRun: dryRun, AnchorSeq: seal.Seq(), Retired: seal.Retired()}
	for seq := uint64(1); seq <= seal.Seq(); seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return nil, err
		}
		f, ok := obs.Files["receipts/"+name]
		if !ok {
			return nil, wire.Errorf(wire.CodeJournalForked, name, "the audit did not read this receipt")
		}
		report.ReceiptBytesBefore += uint64(f.Bytes)
		if seq == seal.Seq() {
			report.ReceiptBytesAfter = uint64(f.Bytes)
		}
	}
	if dryRun {
		raw, err := seal.Encode()
		if err != nil {
			return nil, err
		}
		report.SealBytes = uint64(len(raw))
		return report, nil
	}
	sum, size, err := exportVerified(repo, archivePath, seal, obs.Files)
	if err != nil {
		return nil, err
	}
	seal.ArchiveSha256, seal.ArchiveBytes = sum, size
	report.ArchiveSha256, report.ArchiveBytes = sum, size
	raw, err := seal.Encode()
	if err != nil {
		return nil, err
	}
	report.SealBytes = uint64(len(raw))
	if err := retireStage("exported"); err != nil {
		return report, err
	}
	if err := writeMarker(repo); err != nil {
		return report, err
	}
	if err := retireStage("marked"); err != nil {
		return report, err
	}
	if err := publishSeal(repo, raw); err != nil {
		return report, err
	}
	if err := retireStage("sealed"); err != nil {
		return report, err
	}
	if err := dropPrefix(repo, seal); err != nil {
		return report, err
	}
	if err := retireStage("dropped"); err != nil {
		return report, err
	}
	dropCheckpoints(ctx, repo)
	if err := removeMarker(repo); err != nil {
		return report, err
	}
	if err := retireStage("unmarked"); err != nil {
		return report, err
	}
	if err := closingAudit(repo, seal.Seq()); err != nil {
		// The new store does not audit: restore the prefix from the archive
		// just verified, so the store is the old one again.
		if _, rerr := unretireLocked(ctx, repo, archivePath); rerr != nil {
			return report, errors.Join(err, rerr)
		}
		return report, err
	}
	return report, nil
}

// UnretirePrefix restores receipts 1..S-1 from the archive the seal names and
// removes the seal (JRT-V0-011): the rollback of RetirePrefix.
func UnretirePrefix(ctx context.Context, repo *intent.Repository, archivePath string) (*RetireReport, error) {
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	if recovered, err := recoverRetire(ctx, repo); err != nil || recovered != "" {
		return &RetireReport{Recovered: recovered}, err
	}
	head, err := writerGuards(repo, "UNRETIRE")
	if err != nil {
		return nil, err
	}
	// A store with a retired prefix is never sealed; one that binds segments
	// is refused rather than restored beneath them (JSG-V0-013).
	if head.Segments != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, snapshot.SegmentsDir, "unretire is refused on a segmented journal (JSG-V0-013); run journal unseal first")
	}
	return unretireLocked(ctx, repo, archivePath)
}

func unretireLocked(ctx context.Context, repo *intent.Repository, archivePath string) (*RetireReport, error) {
	seal, err := readSeal(repo)
	if err != nil {
		return nil, err
	}
	if seal == nil {
		return nil, wire.Errorf(wire.CodeUnsupported, journal.RetiredPrefixFile, "the store carries no retired prefix")
	}
	report := &RetireReport{AnchorSeq: seal.Seq(), Retired: seal.Retired(), ArchiveSha256: seal.ArchiveSha256, ArchiveBytes: seal.ArchiveBytes}
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, archivePath, "cannot open the archive: %v", err)
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, archivePath, "cannot read the archive: %v", err)
	}
	if wire.Digest(hex.EncodeToString(h.Sum(nil))) != seal.ArchiveSha256 || uint64(n) != seal.ArchiveBytes {
		return nil, wire.Errorf(wire.CodeJournalForked, archivePath, "archive differs from the export the seal names")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	verified, err := archive.Verify(f)
	if err != nil {
		return nil, err
	}
	if err := archiveMatches(verified.Manifest, seal); err != nil {
		return nil, err
	}
	want := map[string]wire.Digest{}
	for _, e := range verified.Manifest.Files {
		want[e.Path] = e.Sha256
	}
	if err := writeMarker(repo); err != nil {
		return report, err
	}
	if err := retireStage("unretire.marked"); err != nil {
		return report, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return report, err
	}
	dir, err := authority.OpenDir(repo, "receipts")
	if err != nil {
		return report, err
	}
	defer dir.Close()
	restored := uint64(0)
	var last wire.Digest
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return report, wire.Errorf(wire.CodeMalformed, archivePath, "archive changed after verification: %v", err)
		}
		seq, ok := retiredReceipt(hdr.Name, seal.Seq())
		if !ok {
			continue
		}
		if hdr.Size < 0 || hdr.Size > wire.MaxReceiptFileBytes {
			return report, wire.Errorf(wire.CodeLimitExceeded, hdr.Name, "archived receipt larger than its bound")
		}
		raw := make([]byte, hdr.Size)
		if _, err := io.ReadFull(tr, raw); err != nil {
			return report, wire.Errorf(wire.CodeMalformed, hdr.Name, "truncated archived receipt: %v", err)
		}
		if wire.Sum(raw) != want[hdr.Name] {
			return report, wire.Errorf(wire.CodeJournalForked, hdr.Name, "archived receipt differs from the verified manifest")
		}
		name := strings.TrimPrefix(hdr.Name, "receipts/")
		if err := dir.LinkIn(name, raw); errors.Is(err, authority.ErrExists) {
			held, rerr := intent.ReadFile(filepath.Join(dir.Path(), name), wire.MaxReceiptFileBytes)
			if rerr != nil || wire.Sum(held) != wire.Sum(raw) {
				return report, wire.Errorf(wire.CodeJournalForked, hdr.Name, "a different receipt occupies a retired name")
			}
		} else if err != nil {
			return report, err
		}
		restored++
		if seq == seal.Seq()-1 {
			// Receipt S's prev names the original bytes of S-1; a compacted
			// S-1 is a tombstone that records that digest (CAL-V0-300), and
			// the closing audit replays the restored prefix to prove it.
			_, digest, _, err := snapshot.DecodeRetainedReceipt(raw)
			if err != nil {
				return report, wire.Errorf(wire.CodeJournalForked, hdr.Name, "archived receipt does not decode: %v", err)
			}
			last = digest
		}
	}
	if restored != seal.Retired() || last != seal.ElidedSha256 {
		return report, wire.Errorf(wire.CodeJournalForked, archivePath, "archive does not hold exactly the retired receipts the seal links")
	}
	if err := dir.Sync(); err != nil {
		return report, err
	}
	if err := retireStage("unretire.restored"); err != nil {
		return report, err
	}
	if err := os.Remove(statePath(repo, journal.RetiredPrefixFile)); err != nil {
		return report, err
	}
	if err := syncState(repo); err != nil {
		return report, err
	}
	if err := retireStage("unretire.unsealed"); err != nil {
		return report, err
	}
	dropCheckpoints(ctx, repo)
	if err := removeMarker(repo); err != nil {
		return report, err
	}
	return report, closingAudit(repo, 0)
}

// retiredReceipt reports whether an archive entry is a retired receipt.
func retiredReceipt(name string, anchor uint64) (uint64, bool) {
	base, ok := strings.CutPrefix(name, "receipts/")
	if !ok || len(base) != 17 || !strings.HasSuffix(base, ".json") {
		return 0, false
	}
	seq := uint64(0)
	for _, c := range base[:12] {
		if c < '0' || c > '9' {
			return 0, false
		}
		seq = seq*10 + uint64(c-'0')
	}
	if seq == 0 || seq >= anchor {
		return 0, false
	}
	if n, _ := snapshot.ReceiptName(seq); n != base {
		return 0, false
	}
	return seq, true
}

// recoverRetire finishes an interrupted retire or unretire (JRT-V0-009). The
// seal is published only after the archive verified and is removed only after
// every retired receipt is back, so a marker beside a seal rolls forward to
// the sealed store and a marker without one rolls back to the unsealed store.
func recoverRetire(ctx context.Context, repo *intent.Repository) (string, error) {
	on, err := present(statePath(repo, journal.RetireIncompleteFile))
	if err != nil || !on {
		return "", err
	}
	seal, err := readSeal(repo)
	if err != nil {
		return "", err
	}
	outcome := recoveredBack
	if seal != nil {
		if err := dropPrefix(repo, seal); err != nil {
			return "", err
		}
		outcome = recoveredForward
	} else if err := os.Remove(retireTemp(repo)); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	dropCheckpoints(ctx, repo)
	if err := removeMarker(repo); err != nil {
		return "", err
	}
	return outcome, nil
}

// retireEligible is the anchor eligibility (JRT-V0-007): nothing a later
// receipt can bind by receipt sequence may be live at S. Every attempt is in
// a terminal phase, no reservation, pool occupancy or capacity slot is held,
// the store is not paused, and no non-terminal ticket carries an external
// review head or escalation reference into the prefix.
func retireEligible(seal *journal.RetiredPrefix) error {
	if _, ok := seal.Seed("barrier.json"); ok {
		return wire.Errorf(wire.CodePaused, "barrier.json", "a paused store is not retired")
	}
	if raw, ok := seal.Seed("reservations.json"); ok {
		set, err := snapshot.DecodeReservations(raw)
		if err != nil {
			return err
		}
		if len(set.Entries) != 0 {
			return wire.Errorf(wire.CodeAttemptLive, "reservations.json", "a reservation is held")
		}
	}
	if raw, ok := seal.Seed("pools.json"); ok {
		pools, err := snapshot.DecodePools(raw)
		if err != nil {
			return err
		}
		if len(pools.Entries) != 0 || len(pools.Slots) != 0 {
			return wire.Errorf(wire.CodeAttemptLive, "pools.json", "a pool member or capacity slot is occupied")
		}
	}
	paths := make([]string, 0, len(seal.Seeds))
	for p := range seal.Seeds {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		raw := seal.Seeds[p]
		switch {
		case strings.HasPrefix(p, "attempts/"):
			at, err := snapshot.DecodeAttempt(raw)
			if err != nil {
				return err
			}
			if at.Live() {
				return wire.Errorf(wire.CodeAttemptLive, p, "attempt is not in a terminal phase")
			}
		case strings.HasPrefix(p, "intent/tickets/"):
			rec, err := ticket.Decode(raw)
			if err != nil {
				return err
			}
			if rec.Status == ticket.StatusCompleted || rec.Status == ticket.StatusArchived {
				continue
			}
			if len(rec.ExternalReviews) != 0 {
				return wire.Errorf(wire.CodeTicketState, p, "a non-terminal ticket carries external review heads into the prefix")
			}
			if rec.Escalations != nil && len(rec.Escalations.Entries) != 0 {
				return wire.Errorf(wire.CodeEscalationPending, p, "a non-terminal ticket carries escalation references into the prefix")
			}
		}
	}
	return nil
}

// checkArchiveTarget requires a new archive path outside the state
// directory, the repository's common directory and the primary worktree, and
// returns it with its parent's symbolic links resolved. The roots are
// compared both as named and resolved, so a link that aliases any of them
// is refused too.
func checkArchiveTarget(repo *intent.Repository, archivePath string) (string, error) {
	if archivePath == "" {
		return "", wire.Errorf(wire.CodeMalformed, "--archive", "retire needs --archive FILE")
	}
	abs, err := filepath.Abs(archivePath)
	if err != nil {
		return "", wire.Errorf(wire.CodeMalformed, archivePath, "%v", err)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", wire.Errorf(wire.CodeMalformed, archivePath, "cannot resolve the archive directory: %v", err)
	}
	resolved := filepath.Join(parent, filepath.Base(abs))
	for _, root := range []string{repo.StateDir, repo.CommonDir, repo.PrimaryWorktree} {
		if root == "" {
			continue
		}
		roots := []string{root}
		if real, err := filepath.EvalSymlinks(root); err == nil {
			roots = append(roots, real)
		}
		for _, r := range roots {
			for _, p := range []string{abs, resolved} {
				if rel, err := filepath.Rel(r, p); err == nil && filepath.IsLocal(rel) {
					return "", wire.Errorf(wire.CodeMalformed, archivePath, "the archive must live outside the state directory, the common directory and the worktree")
				}
			}
		}
	}
	if on, err := present(resolved); err != nil || on {
		if err == nil {
			err = wire.Errorf(wire.CodeMalformed, archivePath, "the archive path exists; retire never replaces a file")
		}
		return "", err
	}
	return resolved, nil
}

// syncArchiveDir makes the archive's name durable before the seal relies on
// it (JRT-V0-006); a test replaces it to observe or fail the sync.
var syncArchiveDir = authority.SyncDirectory

// exportVerified exports the store to a new file, syncs it, re-reads and
// verifies it, and binds it to the seal: the export is at the anchor and
// every retired receipt in it is the one the audit read (JRT-V0-006).
func exportVerified(repo *intent.Repository, archivePath string, seal *journal.RetiredPrefix, physical map[string]journal.PhysicalFile) (wire.Digest, uint64, error) {
	f, err := os.OpenFile(archivePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", 0, wire.Errorf(wire.CodeMalformed, archivePath, "cannot create the archive: %v", err)
	}
	keep := false
	defer func() {
		if !keep {
			os.Remove(archivePath)
		}
	}()
	_, err = archive.Export(archive.ExportOptions{Repo: repo, Stdout: f})
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		// The prefix is dropped only once the archive's directory entry is
		// durable too, so a crash never leaves a seal naming a lost file.
		err = syncArchiveDir(filepath.Dir(archivePath))
	}
	if err != nil {
		return "", 0, err
	}
	r, err := os.Open(archivePath)
	if err != nil {
		return "", 0, err
	}
	defer r.Close()
	h := sha256.New()
	counted := &countingReader{r: io.TeeReader(r, h)}
	verified, err := archive.Verify(counted)
	if err != nil {
		return "", 0, err
	}
	if err := archiveMatches(verified.Manifest, seal); err != nil {
		return "", 0, err
	}
	files := map[string]wire.Digest{}
	for _, e := range verified.Manifest.Files {
		files[e.Path] = e.Sha256
	}
	for seq := uint64(1); seq < seal.Seq(); seq++ {
		name, _ := snapshot.ReceiptName(seq)
		p := "receipts/" + name
		if got, ok := files[p]; !ok || got != physical[p].Sha256 {
			return "", 0, wire.Errorf(wire.CodeJournalForked, p, "exported receipt differs from the audited receipt")
		}
	}
	keep = true
	return wire.Digest(hex.EncodeToString(h.Sum(nil))), counted.n, nil
}

type countingReader struct {
	r io.Reader
	n uint64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += uint64(n)
	return n, err
}

func archiveMatches(m *archive.Manifest, seal *journal.RetiredPrefix) error {
	a := seal.Anchor
	if !m.Complete || m.QueueID.Raw != a.QueueID.Raw || m.ExportedAtSeq != a.Seq || m.LastReceiptSha256 != a.ReceiptSha256 || m.HeadGeneration != a.Generation {
		return wire.Errorf(wire.CodeJournalForked, archive.ManifestName, "archive is not an export of this store at the anchor")
	}
	return nil
}

// writeMarker creates RETIRE_INCOMPLETE and syncs the state directory.
func writeMarker(repo *intent.Repository) error {
	f, err := os.OpenFile(statePath(repo, journal.RetireIncompleteFile), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	err = f.Sync()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return syncState(repo)
}

func removeMarker(repo *intent.Repository) error {
	if err := os.Remove(statePath(repo, journal.RetireIncompleteFile)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncState(repo)
}

// publishSeal writes the seal beside the state directory, syncs it and
// renames it into place: the seal is absent or whole.
func publishSeal(repo *intent.Repository, raw []byte) error {
	tmp := retireTemp(repo)
	os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, statePath(repo, journal.RetiredPrefixFile))
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return syncState(repo)
}

// dropPrefix removes receipts 1..S-1 and syncs receipts/.
func dropPrefix(repo *intent.Repository, seal *journal.RetiredPrefix) error {
	for seq := uint64(1); seq < seal.Seq(); seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(repo.StateDir, "receipts", name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	dir, err := authority.OpenDir(repo, "receipts")
	if err != nil {
		return err
	}
	err = dir.Sync()
	if closeErr := dir.Close(); err == nil {
		err = closeErr
	}
	return err
}

// dropCheckpoints removes both derived checkpoints: neither may outlive the
// prefix it descends from.
func dropCheckpoints(ctx context.Context, repo *intent.Repository) {
	invalidateWriterCheckpoint(ctx, repo)
	os.Remove(journal.CheckpointPath(repo.StateDir))
}

func syncState(repo *intent.Repository) error {
	dir, err := authority.OpenDir(repo, "")
	if err != nil {
		return err
	}
	err = dir.Sync()
	if closeErr := dir.Close(); err == nil {
		err = closeErr
	}
	return err
}

// closingAudit proves the result: a complete settled audit that starts at
// the anchor (or at receipt 1 when anchor is 0), and the binding folds.
func closingAudit(repo *intent.Repository, anchor uint64) error {
	head, err := readHead(repo)
	if err != nil {
		return err
	}
	proof, err := lockedJournalReader(repo, head).Audit()
	if err != nil {
		return err
	}
	if proof.RetiredThrough() != anchor || proof.Pending || proof.StagingPresent || proof.LastSeq != head.LastSeq {
		return wire.Errorf(wire.CodeJournalForked, journal.RetiredPrefixFile, "closing audit does not start at the expected anchor")
	}
	return FoldReceiptBindings(repo, head.LastSeq.Uint64(), nil)
}

// sealBlob serves the seal's seed bytes by digest to a seeded fold, then
// falls back to the retained evidence blobs.
type sealBlob struct {
	seeds map[wire.Digest][]byte
	base  transaction.ExternalReviewBlob
}

func newSealBlob(repo *intent.Repository) *sealBlob {
	return &sealBlob{seeds: map[wire.Digest][]byte{}, base: ExternalReviewBlob(repo)}
}

func (b *sealBlob) blob(d wire.Digest) ([]byte, bool) {
	if raw, ok := b.seeds[d]; ok {
		return raw, true
	}
	return b.base(d)
}

// sealPosts are the blob-backed posts that stand for the anchor's seeds.
func (b *sealBlob) sealPosts(seal *journal.RetiredPrefix) []snapshot.PostEntry {
	paths := make([]string, 0, len(seal.Seeds))
	for p := range seal.Seeds {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	posts := make([]snapshot.PostEntry, 0, len(paths))
	for _, p := range paths {
		d := wire.Sum(seal.Seeds[p])
		b.seeds[d] = seal.Seeds[p]
		posts = append(posts, snapshot.PostEntry{Path: p, Sha256: &d, BlobSha256: &d})
	}
	return posts
}

// seedFold starts a binding fold whose first receipt is rc. A fold that
// starts at receipt 1 needs no seed; any other start must be the receipt
// right after the seal's anchor and link to it (JRT-V0-008).
func seedFold(repo *intent.Repository, b *sealBlob, rc *snapshot.Receipt, seed func([]snapshot.PostEntry, transaction.ExternalReviewBlob) error) error {
	if rc.Seq.Uint64() == 1 {
		return nil
	}
	seal, err := readSeal(repo)
	if err != nil {
		return err
	}
	if seal == nil || rc.Seq.Uint64() != seal.Seq()+1 || rc.Prev == nil || *rc.Prev != seal.Anchor.ReceiptSha256 {
		return wire.Errorf(wire.CodeJournalForked, "receipts", "a binding fold that does not start at receipt 1 must start after the retired prefix anchor")
	}
	return seed(b.sealPosts(seal), b.blob)
}
