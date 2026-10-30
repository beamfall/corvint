package store

import (
	"context"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sealTrigger is the active receipt count at which a write seals the oldest
// segment (JSG-V0-008): the head receipt is never sealed and the active set
// stays between 512 and 1,023 receipts.
const sealTrigger = 2 * snapshot.SegmentReceipts

// segmentSealStage is a test seam run after each durable seal step of repo;
// a non-nil error stops the seal there as a crash would, with no cleanup.
// It receives the store so parallel tests stop only their own store's seal.
var segmentSealStage = func(*intent.Repository, string) error { return nil }

// errSegmentSealStop is what a test's segmentSealStage returns to leave the
// stopped seal for the next writer to settle.
var errSegmentSealStop = errors.New("segment seal stopped by a test")

// sealAfterCommit runs once a transaction committed. A seal that fails is
// settled at once and never declines the committed write (JSG-V0-008). A
// writer seals only a store that is already segmented: converting a
// taskman-journal-head/0 store is the operator's seal (JSG-V0-013), so no
// write makes a store unreadable to a prior binary.
//
// A seal that fails for any other reason than a test stop also removes the
// writer checkpoint: the seal read the receipts it would seal and found them
// wanting, so the next writer takes the complete audit, which classifies the
// fault (JSG-V0-009).
func sealAfterCommit(repo *intent.Repository, session *authority.Session) {
	if err := sealSegment(repo, session, false); err != nil && !errors.Is(err, errSegmentSealStop) {
		_ = settleSegmentSeal(repo)
		invalidateWriterCheckpoint(context.Background(), repo)
	}
}

func segmentPath(repo *intent.Repository, rel string) string {
	return filepath.Join(repo.StateDir, filepath.FromSlash(rel))
}

func sealTemp(repo *intent.Repository) string { return repo.StateDir + ".seal.tmp" }

// sealSegment seals receipts S+1..S+512 and the request afterimages they
// posted into one segment when the head is at least 1,024 receipts past S.
// The marker lands first, then the segment and the next manifest, then the
// head that binds them: the head publication is the commit point. Only then
// are the sealed physical files and the replaced manifest removed, and last
// the marker. Unless convert is set, an unsegmented store is left as it is.
func sealSegment(repo *intent.Repository, session *authority.Session, convert bool) error {
	head, err := readHead(repo)
	if err != nil {
		return err
	}
	if head.Segments == nil && !convert {
		return nil
	}
	through := uint64(0)
	if head.Segments != nil {
		through = head.Segments.Through.Uint64()
	}
	if head.LastSeq.Uint64()-through < sealTrigger {
		return nil
	}
	if on, err := sealed(repo); err != nil || on {
		return err
	}
	if on, err := present(segmentPath(repo, snapshot.SealMarkerPath)); err != nil || on {
		return err
	}
	headRaw, err := intent.ReadFile(statePath(repo, "head.json"), wire.MaxJournalHeadBytes)
	if err != nil {
		return err
	}
	old := &snapshot.SegmentManifest{QueueID: head.QueueID}
	if head.Segments != nil {
		if old, err = boundManifest(repo, head); err != nil {
			return err
		}
	}
	seg, entry, err := collectSegment(repo, old, through+1, through+snapshot.SegmentReceipts)
	if err != nil {
		return err
	}
	segRaw, err := snapshot.EncodeSegment(seg)
	if err != nil {
		return err
	}
	entry.Sha256, entry.Bytes = wire.Sum(segRaw), uint64(len(segRaw))
	next := &snapshot.SegmentManifest{QueueID: head.QueueID, Through: seg.Last, Segments: append(append([]snapshot.SegmentEntry(nil), old.Segments...), entry)}
	manifestRaw := wire.EncodeFile(next.Value())
	if _, err := snapshot.DecodeSegmentManifest(manifestRaw, 1); err != nil {
		return err
	}
	nextHead := *head
	nextHead.Segments = &snapshot.SegmentBinding{Through: wire.SizeOf(seg.Last), ManifestSha256: wire.Sum(manifestRaw)}
	nextHeadRaw := wire.EncodeFile(nextHead.Value())
	if _, err := snapshot.DecodeHead(nextHeadRaw); err != nil {
		return err
	}
	// A writer seal whose checkpoint descends from a complete audit inside
	// the range runs the complete validators over the receipts it seals,
	// resumed from the checkpoint's base, before anything lands
	// (JSG-V0-009). A refusal declines the seal; the migration verbs
	// removed the checkpoint already.
	var rebased *journal.WriterCheckpoint
	var token writerToken
	if !convert {
		wc, bound := boundWriterCheckpoint(repo)
		if wc.SealValidates(seg.First, seg.Last, old.Segments) {
			if rebased, err = journalReader(repo, head).SealCheckpoint(head, wc, seg, next.Segments); err != nil {
				return err
			}
			token = bound
		}
	}
	if err := os.Mkdir(segmentPath(repo, snapshot.SegmentsDir), 0o700); err == nil {
		if err := syncState(repo); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrExist) {
		return err
	}
	// The marker lands first: a stop at any later step leaves it for the
	// next writer to settle, and no segment file is ever unaccounted for.
	marker := snapshot.EncodeSealMarker(snapshot.SealMarker{First: seg.First, Last: seg.Last, SegmentSha256: entry.Sha256, PriorHead: wire.Sum(headRaw), NextHead: wire.Sum(nextHeadRaw)})
	if err := writeExclusive(segmentPath(repo, snapshot.SealMarkerPath), marker); err != nil {
		return err
	}
	if err := syncDir(repo, snapshot.SegmentsDir); err != nil {
		return err
	}
	if err := segmentSealStage(repo, "marker"); err != nil {
		return err
	}
	for _, f := range []struct {
		name string
		raw  []byte
	}{{snapshot.SegmentName(seg.First, seg.Last), segRaw}, {snapshot.SegmentManifestName(seg.Last), manifestRaw}} {
		if err := writeSynced(sealTemp(repo), f.raw); err != nil {
			return err
		}
		if err := os.Rename(sealTemp(repo), segmentPath(repo, f.name)); err != nil {
			os.Remove(sealTemp(repo))
			return err
		}
	}
	if err := syncDir(repo, snapshot.SegmentsDir); err != nil {
		return err
	}
	if err := segmentSealStage(repo, "segment"); err != nil {
		return err
	}
	current, err := currentDigest(repo, "head.json")
	if err != nil {
		return err
	}
	if current == nil || *current != wire.Sum(headRaw) {
		return wire.Errorf(wire.CodeSnapshotMoved, "head.json", "head changed during the seal")
	}
	stage, err := session.Prepare(authority.Slot("a00"), authority.RoleHead, nextHeadRaw)
	if err != nil {
		return err
	}
	// The rename consumes the slot, so it is not removed afterwards.
	if err := session.Replace(stage, authority.Target{Role: authority.RoleHead, Name: "head.json"}, current); err != nil {
		return err
	}
	if err := segmentSealStage(repo, "head"); err != nil {
		return err
	}
	if err := dropSealed(repo, seg, through); err != nil {
		return err
	}
	if err := segmentSealStage(repo, "dropped"); err != nil {
		return err
	}
	if err := removeSealMarker(repo); err != nil {
		return err
	}
	if rebased != nil {
		publishSealCheckpoint(repo, rebased, token)
	}
	return nil
}

// publishSealCheckpoint retains the checkpoint a seal rebased, once the seal
// is complete. It first replaces the invalidation token the checkpoint was
// bound under, so the writer's own advance after the transaction, derived
// from the checkpoint before the seal, cannot replace it with the older
// FullSeq. Nothing is retained if the token changed since the seal read the
// checkpoint or cannot be replaced.
func publishSealCheckpoint(repo *intent.Repository, wc *journal.WriterCheckpoint, since writerToken) {
	if !since.ok || writerInvalidation(repo) != since {
		return
	}
	if replaceDerived(writerInvalidationPath(repo), []byte(rand.Text()+"\n")) != nil {
		return
	}
	retainWriterCheckpoint(repo, wc, writerInvalidation(repo))
}

// collectSegment reads receipts first..last and the request afterimages they
// posted, checks the chain from the old manifest's last receipt through
// receipt last+1, and checks every afterimage against the digest its receipt
// posted. Anything else declines the seal.
func collectSegment(repo *intent.Repository, old *snapshot.SegmentManifest, first, last uint64) (*snapshot.Segment, snapshot.SegmentEntry, error) {
	seg := &snapshot.Segment{First: first, Last: last}
	entry := snapshot.SegmentEntry{First: first, Last: last}
	var prev *wire.Digest
	if n := len(old.Segments); n > 0 {
		d := old.Segments[n-1].LastReceiptSha256
		prev = &d
	}
	posted := map[string]wire.Digest{}
	for seq := first; seq <= last+1; seq++ {
		raw, found, err := receiptBytes(repo, seq)
		if err != nil {
			return nil, entry, err
		}
		if !found {
			return nil, entry, wire.Errorf(wire.CodeJournalForked, receiptPath(seq), "receipt to seal is absent")
		}
		rc, digest, _, err := snapshot.DecodeRetainedReceipt(raw)
		if err != nil {
			return nil, entry, err
		}
		if rc.Seq.Uint64() != seq || (rc.Prev == nil) != (prev == nil) || (prev != nil && *rc.Prev != *prev) {
			return nil, entry, wire.Errorf(wire.CodeJournalForked, receiptPath(seq), "receipt to seal does not chain")
		}
		d := digest
		prev = &d
		if seq > last {
			break
		}
		seg.Receipts = append(seg.Receipts, snapshot.SegmentFile{Path: receiptPath(seq), Raw: raw})
		entry.LastReceiptSha256 = digest
		for _, p := range rc.Post {
			if !strings.HasPrefix(p.Path, "requests/") {
				continue
			}
			if p.Sha256 == nil {
				return nil, entry, wire.Errorf(wire.CodeUnsupported, p.Path, "a removed request afterimage is not sealed")
			}
			posted[p.Path] = *p.Sha256
		}
	}
	for p, want := range posted {
		bound, err := snapshot.PostBound(p)
		if err != nil {
			return nil, entry, err
		}
		raw, err := intent.ReadFile(segmentPath(repo, p), bound)
		if err != nil {
			return nil, entry, err
		}
		if wire.Sum(raw) != want {
			return nil, entry, wire.Errorf(wire.CodeUnsupported, p, "request afterimage differs from the digest its sealed receipt posted")
		}
		seg.Requests = append(seg.Requests, snapshot.SegmentFile{Path: p, Raw: raw})
	}
	snapshot.SortSegmentFiles(seg.Requests)
	entry.Requests = uint64(len(seg.Requests))
	return seg, entry, nil
}

// boundManifest reads and checks the manifest head binds.
func boundManifest(repo *intent.Repository, head *snapshot.Head) (*snapshot.SegmentManifest, error) {
	name := snapshot.SegmentManifestName(head.Segments.Through.Uint64())
	raw, err := intent.ReadFile(segmentPath(repo, name), snapshot.MaxSegmentManifestBytes)
	if err != nil {
		return nil, err
	}
	if wire.Sum(raw) != head.Segments.ManifestSha256 {
		return nil, wire.Errorf(wire.CodeJournalForked, name, "segment manifest differs from the digest the head binds")
	}
	m, err := snapshot.DecodeSegmentManifest(raw, 1)
	if err != nil {
		return nil, err
	}
	if len(m.Segments) == 0 || m.Through != head.Segments.Through.Uint64() || m.QueueID != head.QueueID {
		return nil, wire.Errorf(wire.CodeJournalForked, name, "segment manifest differs from the head")
	}
	return m, nil
}

// dropSealed removes the sealed physical receipts and request afterimages
// and the manifest the new one replaced, then syncs each directory. Request
// shard directories stay.
func dropSealed(repo *intent.Repository, seg *snapshot.Segment, oldThrough uint64) error {
	dirs := map[string]bool{"receipts": true, snapshot.SegmentsDir: true}
	for _, f := range append(append([]snapshot.SegmentFile(nil), seg.Receipts...), seg.Requests...) {
		if err := os.Remove(segmentPath(repo, f.Path)); err != nil && !os.IsNotExist(err) {
			return err
		}
		dirs[f.Path[:strings.LastIndexByte(f.Path, '/')]] = true
	}
	if oldThrough > 0 {
		if err := os.Remove(segmentPath(repo, snapshot.SegmentManifestName(oldThrough))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return syncDirs(repo, dirs)
}

func syncDirs(repo *intent.Repository, dirs map[string]bool) error {
	names := make([]string, 0, len(dirs))
	for d := range dirs {
		names = append(names, d)
	}
	sort.Strings(names)
	for _, d := range names {
		if err := syncDir(repo, d); err != nil {
			return err
		}
	}
	return nil
}

// settleSegmentSeal finishes or undoes a seal a stop interrupted (JSG-V0-008).
// It runs under the writer lock before any write. With the prior head still
// published, the new segment and manifest are removed; with the next head
// published, the sealed physical files still present must equal the sealed
// bytes and are removed. Any other head is a fork, and nothing is removed.
func settleSegmentSeal(repo *intent.Repository) error {
	path := segmentPath(repo, snapshot.SealMarkerPath)
	raw, err := intent.ReadFile(path, snapshot.MaxSealMarkerBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	m, err := snapshot.DecodeSealMarker(raw)
	if err != nil {
		return err
	}
	headRaw, err := intent.ReadFile(statePath(repo, "head.json"), wire.MaxJournalHeadBytes)
	if err != nil {
		return err
	}
	switch wire.Sum(headRaw) {
	case m.PriorHead:
		for _, name := range []string{snapshot.SegmentName(m.First, m.Last), snapshot.SegmentManifestName(m.Last)} {
			if err := os.Remove(segmentPath(repo, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if err := syncDir(repo, snapshot.SegmentsDir); err != nil {
			return err
		}
	case m.NextHead:
		name := snapshot.SegmentName(m.First, m.Last)
		segRaw, err := intent.ReadFile(segmentPath(repo, name), snapshot.MaxSegmentBytes)
		if err != nil {
			return err
		}
		if wire.Sum(segRaw) != m.SegmentSha256 {
			return wire.Errorf(wire.CodeJournalForked, name, "sealed segment differs from its marker")
		}
		seg, err := snapshot.DecodeSegment(segRaw, m.First, m.Last)
		if err != nil {
			return err
		}
		for _, f := range append(append([]snapshot.SegmentFile(nil), seg.Receipts...), seg.Requests...) {
			got, err := intent.ReadFile(segmentPath(repo, f.Path), len(f.Raw)+1)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil || string(got) != string(f.Raw) {
				return wire.Errorf(wire.CodeJournalForked, f.Path, "a sealed file still linked differs from its sealed bytes")
			}
		}
		if err := dropSealed(repo, seg, m.First-1); err != nil {
			return err
		}
	default:
		return wire.Errorf(wire.CodeJournalForked, snapshot.SealMarkerPath, "the head is neither the seal's prior nor its next head")
	}
	if err := removeSealMarker(repo); err != nil {
		return err
	}
	return dropEmptySegments(repo)
}

// dropEmptySegments removes segments/ once the head binds no segment, so a
// settled first seal rolled back, or a last segment unsealed, leaves the
// unsegmented tree exactly. Anything left in it refuses as it refuses a
// reader.
func dropEmptySegments(repo *intent.Repository) error {
	head, err := readHead(repo)
	if err != nil || head.Segments != nil {
		return err
	}
	if err := os.Remove(segmentPath(repo, snapshot.SegmentsDir)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return wire.Errorf(wire.CodeJournalForked, snapshot.SegmentsDir, "segments/ is not empty under a head that binds no segment: %v", err)
	}
	return syncState(repo)
}

func removeSealMarker(repo *intent.Repository) error {
	if err := os.Remove(segmentPath(repo, snapshot.SealMarkerPath)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDir(repo, snapshot.SegmentsDir)
}

// JournalSealReport is what `journal seal` and `journal unseal` did: the
// sealed prefix S before and after, and the bound segment count after.
type JournalSealReport struct {
	HeadSeq       uint64
	ThroughBefore uint64
	ThroughAfter  uint64
	SegmentsAfter int
}

// SealJournal is `journal seal` (JSG-V0-013): under the writer lock and after
// a complete settled audit it converts an unsegmented store and seals every
// whole 512-receipt range the trigger admits. A fault the audit or a seal's
// input checks find refuses before that seal writes anything; each segment
// is its own marker-guarded transaction, so a stop leaves whole segments
// bound and the next writer settles the one in flight.
func SealJournal(ctx context.Context, repo *intent.Repository) (*JournalSealReport, error) {
	return journalMigrate(ctx, repo, true)
}

// UnsealJournal is `journal unseal` (JSG-V0-013, the rollback of JSG-V0-005):
// under the writer lock and after a complete settled audit it unseals the
// last segment at a time until the head is a taskman-journal-head/0 again,
// then removes the empty segments/ directory. The restored tree is
// byte-identical to the tree the seal started from.
func UnsealJournal(ctx context.Context, repo *intent.Repository) (*JournalSealReport, error) {
	return journalMigrate(ctx, repo, false)
}

func journalMigrate(ctx context.Context, repo *intent.Repository, seal bool) (*JournalSealReport, error) {
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	operation := "JOURNAL_UNSEAL"
	if seal {
		operation = "JOURNAL_SEAL"
	}
	// The guards settle a stopped seal first and refuse a restore or
	// retire in progress and a paused store.
	head, err := writerGuards(repo, operation)
	if err != nil {
		return nil, err
	}
	if on, err := sealed(repo); err != nil || on {
		if err == nil {
			err = wire.Errorf(wire.CodeUnsupported, journal.RetiredPrefixFile, "journal seal and unseal are refused on a store with a retired prefix (JSG-V0-013)")
		}
		return nil, err
	}
	report := &JournalSealReport{HeadSeq: head.LastSeq.Uint64(), ThroughBefore: through(head)}
	if !seal && head.Segments == nil {
		return report, nil
	}
	if err := compactAudit(repo, head); err != nil {
		return nil, err
	}
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	invalidateWriterCheckpoint(ctx, repo)
	if err := os.Remove(journal.CheckpointPath(repo.StateDir)); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for {
		if seal {
			err = sealSegment(repo, session, true)
		} else {
			err = unsealLast(repo, session)
		}
		if err != nil {
			if !errors.Is(err, errSegmentSealStop) {
				err = errors.Join(err, settleSegmentSeal(repo))
			}
			return nil, err
		}
		next, err := readHead(repo)
		if err != nil {
			return nil, err
		}
		if through(next) == through(head) {
			break
		}
		head = next
	}
	report.ThroughAfter = through(head)
	if head.Segments != nil {
		report.SegmentsAfter = int(through(head) / snapshot.SegmentReceipts)
		return report, nil
	}
	if err := dropEmptySegments(repo); err != nil {
		return nil, err
	}
	return report, nil
}

func through(head *snapshot.Head) uint64 {
	if head.Segments == nil {
		return 0
	}
	return head.Segments.Through.Uint64()
}

// unsealLast is a seal run backwards under the same marker: it restores the
// last bound segment's receipts and request afterimages to their physical
// paths, writes the manifest of the segments before it, and publishes the
// head that binds that manifest (or no segments). The marker names the
// unsealed head as PriorHead and the current head as NextHead, so
// settleSegmentSeal rolls a stopped unseal forward once its head is
// published and back, unlinking the restored files, before that.
func unsealLast(repo *intent.Repository, session *authority.Session) error {
	headRaw, err := intent.ReadFile(statePath(repo, "head.json"), wire.MaxJournalHeadBytes)
	if err != nil {
		return err
	}
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil || head.Segments == nil {
		return err
	}
	m, err := boundManifest(repo, head)
	if err != nil {
		return err
	}
	e := m.Segments[len(m.Segments)-1]
	name := snapshot.SegmentName(e.First, e.Last)
	segRaw, err := intent.ReadFile(segmentPath(repo, name), snapshot.MaxSegmentBytes)
	if err != nil {
		return err
	}
	if wire.Sum(segRaw) != e.Sha256 {
		return wire.Errorf(wire.CodeJournalForked, name, "segment differs from the digest the manifest binds")
	}
	seg, err := snapshot.DecodeSegment(segRaw, e.First, e.Last)
	if err != nil {
		return err
	}
	prior := *head
	prior.Segments = nil
	var manifestRaw []byte
	if len(m.Segments) > 1 {
		pm := &snapshot.SegmentManifest{QueueID: head.QueueID, Through: e.First - 1, Segments: m.Segments[:len(m.Segments)-1]}
		manifestRaw = wire.EncodeFile(pm.Value())
		if _, err := snapshot.DecodeSegmentManifest(manifestRaw, 1); err != nil {
			return err
		}
		prior.Segments = &snapshot.SegmentBinding{Through: wire.SizeOf(e.First - 1), ManifestSha256: wire.Sum(manifestRaw)}
	}
	priorRaw := wire.EncodeFile(prior.Value())
	if _, err := snapshot.DecodeHead(priorRaw); err != nil {
		return err
	}
	marker := snapshot.EncodeSealMarker(snapshot.SealMarker{First: e.First, Last: e.Last, SegmentSha256: e.Sha256, PriorHead: wire.Sum(priorRaw), NextHead: wire.Sum(headRaw)})
	if err := writeExclusive(segmentPath(repo, snapshot.SealMarkerPath), marker); err != nil {
		return err
	}
	if err := syncDir(repo, snapshot.SegmentsDir); err != nil {
		return err
	}
	if err := segmentSealStage(repo, "marker"); err != nil {
		return err
	}
	dirs := map[string]bool{"receipts": true, snapshot.SegmentsDir: true}
	for _, f := range append(append([]snapshot.SegmentFile(nil), seg.Receipts...), seg.Requests...) {
		dir := f.Path[:strings.LastIndexByte(f.Path, '/')]
		if strings.HasPrefix(dir, "requests/") && !dirs[dir] {
			if err := os.Mkdir(segmentPath(repo, dir), 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
			dirs["requests"] = true
		}
		dirs[dir] = true
		// A sealed file is never linked under the head that binds it; one
		// that is must hold exactly its sealed bytes.
		got, err := intent.ReadFile(segmentPath(repo, f.Path), len(f.Raw)+1)
		if err == nil {
			if string(got) != string(f.Raw) {
				return wire.Errorf(wire.CodeJournalForked, f.Path, "a sealed file still linked differs from its sealed bytes")
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := writeSynced(sealTemp(repo), f.Raw); err != nil {
			return err
		}
		if err := os.Rename(sealTemp(repo), segmentPath(repo, f.Path)); err != nil {
			os.Remove(sealTemp(repo))
			return err
		}
	}
	if manifestRaw != nil {
		if err := writeSynced(sealTemp(repo), manifestRaw); err != nil {
			return err
		}
		if err := os.Rename(sealTemp(repo), segmentPath(repo, snapshot.SegmentManifestName(e.First-1))); err != nil {
			os.Remove(sealTemp(repo))
			return err
		}
	}
	if err := syncDirs(repo, dirs); err != nil {
		return err
	}
	if err := segmentSealStage(repo, "segment"); err != nil {
		return err
	}
	current, err := currentDigest(repo, "head.json")
	if err != nil {
		return err
	}
	if current == nil || *current != wire.Sum(headRaw) {
		return wire.Errorf(wire.CodeSnapshotMoved, "head.json", "head changed during the unseal")
	}
	stage, err := session.Prepare(authority.Slot("a00"), authority.RoleHead, priorRaw)
	if err != nil {
		return err
	}
	if err := session.Replace(stage, authority.Target{Role: authority.RoleHead, Name: "head.json"}, current); err != nil {
		return err
	}
	if err := segmentSealStage(repo, "head"); err != nil {
		return err
	}
	for _, n := range []string{name, snapshot.SegmentManifestName(e.Last)} {
		if err := os.Remove(segmentPath(repo, n)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := syncDir(repo, snapshot.SegmentsDir); err != nil {
		return err
	}
	if err := segmentSealStage(repo, "dropped"); err != nil {
		return err
	}
	return removeSealMarker(repo)
}

// sealSegmentsLocked seals every whole segment the trigger admits, converting
// an unsegmented store, without the opening audit: for tests and measurement
// harnesses that build a history first. The operator verb is SealJournal.
func sealSegmentsLocked(ctx context.Context, repo *intent.Repository) error {
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return err
	}
	defer lock.Close()
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return err
	}
	defer session.Close()
	for {
		head, err := readHead(repo)
		if err != nil {
			return err
		}
		before := head.Segments
		if err := sealSegment(repo, session, true); err != nil {
			return err
		}
		if head, err = readHead(repo); err != nil {
			return err
		}
		if head.Segments == nil || (before != nil && head.Segments.Through == before.Through) {
			return nil
		}
	}
}

func syncDir(repo *intent.Repository, rel string) error {
	dir, err := authority.OpenDir(repo, rel)
	if err != nil {
		return err
	}
	err = dir.Sync()
	if closeErr := dir.Close(); err == nil {
		err = closeErr
	}
	return err
}

// writeSynced writes raw to a fresh file at path and syncs it.
func writeSynced(path string, raw []byte) error {
	os.Remove(path)
	if err := writeExclusive(path, raw); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

func writeExclusive(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
	return err
}

// readLogical reads a receipt or request afterimage by its logical path,
// from the segment that sealed it once its physical file is gone.
func readLogical(repo *intent.Repository, rel string, max int) ([]byte, error) {
	return journal.Logical(journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.IntentRoot()}).Read(rel, max)
}
