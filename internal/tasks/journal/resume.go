package journal

import (
	"encoding/hex"
	"os"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ErrPrefixDeclined is the SNAPSHOT_MOVED a PrefixFallback audit returns
// when its prefix walk refused on an unchanged observation (PSR-V0-050,
// proposed). The prefix walk never decides a refusal for a lease: the caller
// runs the complete audit, which derives the refusal itself.
var ErrPrefixDeclined = wire.Errorf(wire.CodeSnapshotMoved, "receipts", "prefix walk declined; the complete audit decides")

// resumeQuiescence is how long before a base capture began a file's
// modification and change times must lie for ResumeBase to trust its stat
// stamps instead of its bytes (PSR-V0-052). It is receiptsQuiescence; the
// focused tests set it to zero or to an hour to select one path.
var resumeQuiescence = receiptsQuiescence

// ResumableAudits reports whether this platform supports resumed lease
// audits: Result.ResumeBase needs a change time, so elsewhere every lease
// keeps the complete audit on each round.
func ResumableAudits() bool { return haveChangeTime }

// auditPrefixFallback is AuditForWriteObserved with PrefixFallback; see
// Reader.PrefixFallback. The complete attempt is AuditWriterRefresh's, with
// the early stop on a moved head.json; the prefix walk is AuditWriterRefresh's
// unchanged.
func (r Reader) auditPrefixFallback() (result *Result, observation PhysicalObservation, err error) {
	r.PrefixFallback = false
	var walks WalkStats
	r.walks = &walks
	defer func() { observation.Walks = walks }()
	switch r.Source.(type) {
	case Native, *Native:
	default:
		r.writerCache, r.physical = true, &observation
		result, err := r.Audit()
		return result, observation, err
	}
	full := r
	full.writerCache, full.physical, full.fullAttempts, full.abortMoved = true, &observation, 1, true
	result, err = full.Audit()
	if observation.Cleanup != nil || wire.CodeOf(err) != wire.CodeSnapshotMoved {
		return result, observation, err
	}
	observation = PhysicalObservation{}
	r.physical, r.prefix, r.Checkpoint, r.handoffPolicy = &observation, true, nil, nil
	for attempt := 0; attempt < 4; attempt++ {
		result, err := r.auditAttempt(nil, "", profileLimits, true, nil)
		if observation.Cleanup != nil {
			return nil, observation, err
		}
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			continue
		}
		if err != nil {
			return nil, observation, ErrPrefixDeclined
		}
		return result, observation, nil
	}
	return nil, observation, moved("receipts")
}

// headMoved reports whether head.json is no longer the file o captured, for
// the early stop of a PrefixFallback complete attempt. That attempt is then
// SNAPSHOT_MOVED whatever the rest of its walk finds, because its second
// capture compares the same file identity and stamps.
func (r Reader) headMoved(o *observation) bool {
	native, ok := nativeOf(r.Source)
	if !ok || !r.abortMoved {
		return false
	}
	before := o.files["head.json"]
	if before == nil {
		return false
	}
	full, err := native.native.path("head.json")
	if err != nil {
		return false
	}
	now, err := os.Lstat(full)
	return err != nil || !sameStat(before, now)
}

// abortEvery is how many receipts or projections a PrefixFallback complete
// attempt walks between head.json checks.
const abortEvery = 512

// ResumeBase is what one lease command's own settled audit from receipt 1
// established (PSR-V0-051, proposed): the writer checkpoint it derives and,
// for every receipt, request, evidence and pinned file the audit read
// (receipts 1..H and every afterimage latest at H among them), the stat
// stamps the audit's capture observed and the digest of the bytes it read.
// It is held only by that command, never written, and
// never shared with another command or process. On a segmented journal the
// sealed receipts 1..through and the requests they posted have no files of
// their own: the base holds the bound manifest and every segment it names
// instead, with the digests the manifest and the head record (JSG-V0-011).
type ResumeBase struct {
	wc       *WriterCheckpoint
	seq      uint64
	files    map[string]resumeFile
	requests int
	through  uint64
	segments []string
	pre      *resumePre
	// sealedBase says the base is a retained writer checkpoint's sealed
	// base, which no audit of this command read (LGL-V0-009): every
	// segment, the manifest and every evidence and pinned blob it holds is
	// re-hashed before the resumed walk is trusted.
	sealedBase bool
}

// resumePre is what PrecheckResume established before the preparation gate
// (PSR-V0-225): the settled bound segments it re-hashed against the base's
// digests, and the names of each request shard whose stamps had settled
// when it began. The gated checks use an entry only after re-validating
// the stamps it was taken under; it is an optimization, never a decision.
type resumePre struct {
	hashed map[string]bool
	shards map[string]resumeShard
}

type resumeShard struct {
	info  os.FileInfo
	names []string
}

func (p *resumePre) rehashed(path string) bool { return p != nil && p.hashed[path] }

func (p *resumePre) shard(dir string) (resumeShard, bool) {
	if p == nil {
		return resumeShard{}, false
	}
	sh, ok := p.shards[dir]
	return sh, ok
}

type resumeFile struct {
	info  os.FileInfo
	sum   wire.Digest
	bytes int64
	quiet bool
}

// WriterCheckpoint is the checkpoint a resumed audit walks its tail from.
func (b *ResumeBase) WriterCheckpoint() *WriterCheckpoint { return b.wc }

// Seq is the head the base audited.
func (b *ResumeBase) Seq() uint64 { return b.seq }

// ResumeBase derives a resume base from a settled complete (ModeFull) or
// prefix (ModePrefix) audit and the physical observation it returned. Every
// other result, a platform without change times, and any receipt or
// afterimage the audit did not read return an error: the caller keeps the
// complete audit.
func (res *Result) ResumeBase(obs PhysicalObservation) (*ResumeBase, error) {
	if !haveChangeTime {
		return nil, errCheckpoint("resume", "platform reports no change time")
	}
	if res == nil || (res.Mode != ModeFull && res.Mode != ModePrefix) || res.StructuralConsistency != "CONSISTENT" || obs.Cleanup != nil || obs.Files == nil || obs.capture == nil || obs.capture.identity != res.Identity {
		return nil, errCheckpoint("resume", "not a settled audit from receipt 1")
	}
	wc, err := res.WriterCheckpoint(obs.Files)
	if err != nil {
		return nil, err
	}
	o := obs.capture
	bound := o.at.Add(-resumeQuiescence)
	b := &ResumeBase{wc: wc, seq: res.LastSeq.Uint64(), files: map[string]resumeFile{}}
	stamp := func(p string, sum wire.Digest, size int64) error {
		info := o.files[p]
		if info == nil || !info.Mode().IsRegular() || info.Size() != size {
			return errCheckpoint(p, "base file was not read by the audit")
		}
		change, ok := changeTime(info)
		if !ok {
			return errCheckpoint(p, "no change time")
		}
		b.files[p] = resumeFile{info: info, sum: sum, bytes: size, quiet: !info.ModTime().After(bound) && !time.Unix(0, change).After(bound)}
		return nil
	}
	if m := res.manifest; res.Head != nil && res.Head.Segments != nil {
		// The audit read every sealed receipt and request from a segment
		// whose digest the manifest binds, and the manifest whose digest
		// the head binds (JSG-V0-011).
		if m == nil || m.Through != res.Head.Segments.Through.Uint64() || m.Through > b.seq {
			return nil, errCheckpoint(snapshot.SegmentsDir, "the audit read no bound manifest")
		}
		b.through = m.Through
		name := snapshot.SegmentManifestName(m.Through)
		info := o.files[name]
		if info == nil {
			return nil, errCheckpoint(name, "base file was not read by the audit")
		}
		if err := stamp(name, res.Head.Segments.ManifestSha256, info.Size()); err != nil {
			return nil, err
		}
		for _, e := range m.Segments {
			p := snapshot.SegmentName(e.First, e.Last)
			if err := stamp(p, e.Sha256, int64(e.Bytes)); err != nil {
				return nil, err
			}
			b.segments = append(b.segments, p)
		}
		b.segments = append(b.segments, name)
	}
	add := func(p string) error {
		f, read := obs.Files[p]
		if !read {
			return errCheckpoint(p, "base file was not read by the audit")
		}
		return stamp(p, f.Sha256, int64(f.Bytes))
	}
	// Every receipt, request, evidence and pinned file the audit read is a
	// base file, including blobs a prefix receipt's posts name. A sealed
	// receipt or request was read from its segment instead.
	for _, p := range sortedPaths(obs.Files) {
		if resumeFamily(p) && !b.sealed(p, o, res) {
			if err := add(p); err != nil {
				return nil, err
			}
		}
	}
	for seq := b.through + 1; seq <= b.seq; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return nil, err
		}
		if _, ok := b.files["receipts/"+name]; !ok {
			return nil, errCheckpoint("receipts/"+name, "base receipt was not read by the audit")
		}
	}
	for _, p := range sortedPaths(res.chain.canonical) {
		l := res.chain.canonical[p]
		if l.digest == nil || !resumeFamily(p) || b.sealed(p, o, res) {
			continue
		}
		if f, ok := b.files[p]; !ok || f.sum != *l.digest {
			return nil, errCheckpoint(p, "base afterimage was not read by the audit")
		}
	}
	for p, info := range o.files {
		if strings.HasPrefix(p, "requests/") && info.Mode().IsRegular() {
			if _, ok := b.files[p]; !ok {
				return nil, errCheckpoint(p, "base request file was not read by the audit")
			}
			b.requests++
		}
	}
	return b, nil
}

// SealedResumeBase derives a resume base, without an audit from receipt 1,
// from w's sealed base: the writer checkpoint a seal rebased at the sealed
// sequence S that head binds (LGL-V0-009, proposed; decision 0549 R11/F3).
// w must be the retained checkpoint with FullSeq S and a base at S whose
// segment digests are exactly the bound manifest's, and the manifest must
// have the digest head binds. The base files are every bound segment, the
// manifest, and every evidence and pinned blob latest at S, stat'ed now;
// their digests are the manifest's, the head's and the checkpoint's. It
// holds no receipt or request file: a seal removed those of receipts 1..S.
//
// Nothing here is trusted until AuditForResume's base check re-hashes every
// one of those files (or PrecheckResume did, with the stamps still the
// ones recorded here) and the resumed walk reads receipts S+1..head by
// bytes. Every decline leaves the caller on the complete audit. A failed
// close of the base's read lifetime is cleanup, not a decline: cleanup is
// UNSUPPORTED_FILESYSTEM (`read lifetime`) and the command terminates on
// it (PSR-V0-057).
func (r Reader) SealedResumeBase(w *WriterCheckpoint, head *snapshot.Head) (base *ResumeBase, decline, cleanup error) {
	if !haveChangeTime {
		return nil, errCheckpoint("resume", "platform reports no change time"), nil
	}
	n, ok := r.Source.(Native)
	if !ok {
		return nil, errCheckpoint("resume", "sealed base needs a native source"), nil
	}
	if head == nil || head.Segments == nil || head.QueueID != r.QueueID {
		return nil, errCheckpoint(snapshot.SegmentsDir, "store has no sealed prefix"), nil
	}
	s := head.Segments.Through.Uint64()
	if w == nil || w.base == nil || w.FullSeq != s || w.base.seq != s || w.Seq.Uint64() < s || w.QueueID != r.QueueID {
		return nil, errCheckpoint("writer checkpoint", "no base at the sealed sequence"), nil
	}
	m := &w.base.meta
	bc := &WriterCheckpoint{Checkpoint: m.header, FullSeq: s, ReceiptBytes: m.receiptBytes, Cost: m.cost, Intent: m.intent, Invalidation: w.Invalidation, base: w.base, total: w.base.n}
	bc.Entries = nil
	if bc.Seq.Uint64() != s || bc.QueueID != w.QueueID || bc.PrimaryWorktree != w.PrimaryWorktree || bc.InitSha256 != w.InitSha256 {
		return nil, errCheckpoint("writer checkpoint", "base header differs from the checkpoint"), nil
	}
	rd := newNativeRead(n)
	b, decline := sealedBase(rd, bc, head)
	if cleanup = rd.close(); cleanup != nil {
		return nil, nil, wire.Errorf(wire.CodeUnsupportedFilesystem, "read lifetime", "sealed base: %v; close: %v", decline, cleanup)
	}
	if decline != nil {
		return nil, decline, nil
	}
	return b, nil, nil
}

// sealedBase is SealedResumeBase's body over its own read lifetime.
func sealedBase(rd *nativeRead, bc *WriterCheckpoint, head *snapshot.Head) (*ResumeBase, error) {
	s := head.Segments.Through.Uint64()
	bound := time.Now().Add(-resumeQuiescence)
	b := &ResumeBase{wc: bc, seq: s, through: s, files: map[string]resumeFile{}, sealedBase: true}
	stamp := func(p string, sum wire.Digest, size int64) error {
		full, err := rd.native.path(p)
		if err != nil {
			return errCheckpoint(p, err.Error())
		}
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() || size >= 0 && info.Size() != size {
			return errCheckpoint(p, "sealed base file is absent or differs in size")
		}
		if _, ok := changeTime(info); !ok {
			return errCheckpoint(p, "no change time")
		}
		b.files[p] = resumeFile{info: info, sum: sum, bytes: info.Size(), quiet: settledBefore(info, bound)}
		return nil
	}
	name := snapshot.SegmentManifestName(s)
	// Stamped before it is read: the base check re-hashes it either way.
	if err := stamp(name, head.Segments.ManifestSha256, -1); err != nil {
		return nil, err
	}
	raw, err := rd.Read(name, snapshot.MaxSegmentManifestBytes)
	if err != nil || wire.Sum(raw) != head.Segments.ManifestSha256 {
		return nil, errCheckpoint(name, "segment manifest differs from the digest the head binds")
	}
	man, err := snapshot.DecodeSegmentManifest(raw, 1)
	if err != nil || man.Through != s || man.QueueID != head.QueueID || len(man.Segments) == 0 || len(man.Segments) != len(bc.base.meta.segments) {
		return nil, errCheckpoint(name, "segment manifest differs from the sealed base")
	}
	for i, e := range man.Segments {
		if hex.EncodeToString(bc.base.meta.segments[i][:]) != string(e.Sha256) {
			return nil, errCheckpoint(name, "sealed base covers other segments than the manifest binds")
		}
		p := snapshot.SegmentName(e.First, e.Last)
		if err := stamp(p, e.Sha256, int64(e.Bytes)); err != nil {
			return nil, err
		}
		b.segments = append(b.segments, p)
	}
	b.segments = append(b.segments, name)
	err = bc.each(nil, func(p string, l latest) error {
		if l.digest == nil || !blobFamily(p) {
			return nil
		}
		return stamp(p, *l.digest, -1)
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

func blobFamily(p string) bool {
	return strings.HasPrefix(p, "evidence/") || strings.HasPrefix(p, "pinned/")
}

// blobSame says base evidence or pinned blob p is still the file the base
// recorded (same) and, when rehash is set, that its bytes still have the
// digest the sealed base holds for it (LGL-V0-009).
func (b *ResumeBase) blobSame(s *nativeRead, p string, rehash bool) error {
	full, err := s.native.path(p)
	if err != nil {
		return errCheckpoint(p, err.Error())
	}
	info, err := os.Lstat(full)
	if err != nil {
		info = nil
	}
	if err := b.same(s, p, info); err != nil {
		return err
	}
	f := b.files[p]
	if !rehash || !f.quiet {
		// same read an unsettled file's bytes already.
		return nil
	}
	bound, err := snapshot.PostBound(p)
	if err != nil {
		return err
	}
	raw, err := s.Read(p, bound)
	if err != nil || int64(len(raw)) != f.bytes || wire.Sum(raw) != f.sum {
		return errCheckpoint(p, "blob bytes differ from the digest the sealed base holds")
	}
	return nil
}

// sealed says p is a receipt or request a segment of the base holds: a
// receipt at or below through, or a request without a file of its own that
// a sealed receipt posted.
func (b *ResumeBase) sealed(p string, o *observation, res *Result) bool {
	if b.through == 0 {
		return false
	}
	if name, ok := strings.CutPrefix(p, "receipts/"); ok {
		seq, ok := receiptNameSeq(name)
		return ok && seq <= b.through
	}
	if !strings.HasPrefix(p, "requests/") {
		return false
	}
	if o.files[p] != nil {
		return false
	}
	l, ok := res.chain.canonical[p]
	return ok && l.seq.Uint64() <= b.through
}

func resumeFamily(p string) bool {
	return strings.HasPrefix(p, "receipts/") || strings.HasPrefix(p, "requests/") || strings.HasPrefix(p, "evidence/") || strings.HasPrefix(p, "pinned/")
}

// AuditForResume is AuditForWriter from a resume base instead of a retained
// writer checkpoint (PSR-V0-052, proposed). On top of every AuditForWriter
// check it requires each base file to be what the base audit read: the same
// file with the same mode, size, modification time and change time, and,
// for a file whose stamps were not yet settled when the base capture began,
// the same bytes. Every request shard is listed: a request file must be a
// base file or one the walked tail posted. Evidence and pinned afterimages
// posted by the tail are read and compared by digest. Any difference is a
// decline, and the caller's complete audit decides.
func (r Reader) AuditForResume(base *ResumeBase, requestID string) (*Result, PhysicalObservation, error) {
	if base == nil {
		return nil, PhysicalObservation{}, errCheckpoint("resume", "absent")
	}
	// Every attempt AuditForWriter runs is one resumed walk, its
	// SNAPSHOT_MOVED retry included (PSR-V0-151).
	var walks WalkStats
	r.resume, r.walks = base, &walks
	result, observation, err := r.AuditForWriter(base.wc, requestID, false)
	observation.Walks = walks
	return result, observation, err
}

// PrecheckResume does, before the caller takes the preparation gate, the
// work of the base check that does not depend on the snapshot the gated
// write observes (PSR-V0-225): it re-hashes every settled bound segment and
// lists the names of every request shard whose modification and change
// times lie at least resumeQuiescence before it began. AuditForResume and
// RecheckResume then skip a segment's re-hash only while its stamps,
// change time included, are still the base's, and reuse a shard's names
// only while the shard is still that directory with the same stamps; every
// file is still stat'ed under the gate. Any fault here records nothing, so
// the gated check does all its work and decides. As in the base check, a
// failed close of its read lifetime is cleanup (PSR-V0-057).
func (r Reader) PrecheckResume(base *ResumeBase) (cleanup error) {
	if base == nil {
		return nil
	}
	base.pre = nil
	n, ok := r.Source.(Native)
	if !ok {
		return nil
	}
	s := newNativeRead(n)
	pre := base.precheck(s, profileLimits)
	if cleanup = s.close(); cleanup != nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "read lifetime", "resume precheck close: %v", cleanup)
	}
	base.pre = pre
	return nil
}

// precheck is PrecheckResume's body. A shard is trusted only when its
// stamps had settled before the precheck began and its handle reports the
// stamps its parent's listing did: an entry made, removed or renamed in it
// at or after that time moves its modification time off the recorded one.
func (b *ResumeBase) precheck(s *nativeRead, lim limits) *resumePre {
	pre := &resumePre{hashed: map[string]bool{}, shards: map[string]resumeShard{}}
	for _, p := range b.segments {
		if b.files[p].quiet && b.segmentSame(s, p, true) == nil {
			pre.hashed[p] = true
		}
	}
	if b.sealedBase {
		// A sealed base's blobs are re-hashed here too (LGL-V0-009).
		for _, p := range sortedPaths(b.files) {
			if blobFamily(p) && b.files[p].quiet && b.blobSame(s, p, true) == nil {
				pre.hashed[p] = true
			}
		}
	}
	bound := time.Now().Add(-resumeQuiescence)
	shards, err := s.List("requests", lim.scan)
	if err != nil {
		return pre
	}
	for _, shard := range shards.Entries {
		if shard.Info == nil || !shard.Info.IsDir() || !settledBefore(shard.Info, bound) {
			continue
		}
		dir := "requests/" + shard.Name
		info, names, err := s.Names(dir, lim.scan)
		if err != nil || len(names) > lim.scan || !sameStamps(shard.Info, info) {
			continue
		}
		pre.shards[dir] = resumeShard{info: shard.Info, names: names}
	}
	return pre
}

// settledBefore says info's modification and change times lie at or before
// bound.
func settledBefore(info os.FileInfo, bound time.Time) bool {
	change, ok := changeTime(info)
	return ok && !info.ModTime().After(bound) && !time.Unix(0, change).After(bound)
}

// shardEntries is s.List(dir), or, for a shard PrecheckResume trusted that
// is still the same directory with the same stamps by both its parent's
// listing (info) and its own handle, the precheck's names with each entry
// stat'ed now (PSR-V0-225). Each entry is recorded as listed, as List does.
// reused says the names were the precheck's: such a shard was reached
// through the pinned requests/ handle without the pathname binding a
// listing makes, so the caller binds requests/ after its last shard.
func (b *ResumeBase) shardEntries(s *nativeRead, dir string, info os.FileInfo, lim limits) (entries []Entry, reused bool, err error) {
	pre, ok := b.pre.shard(dir)
	if ok && sameStamps(pre.info, info) {
		root, err := s.parent(dir)
		if err != nil {
			return nil, false, err
		}
		if now, err := root.Stat("."); err == nil && sameStamps(pre.info, now) {
			out := make([]Entry, 0, len(pre.names))
			for _, name := range pre.names {
				e, err := root.Lstat(name)
				if os.IsNotExist(err) {
					return nil, false, moved(dir + "/" + name)
				}
				if err != nil {
					return nil, false, err
				}
				out = append(out, Entry{name, e})
				s.listed[dir+"/"+name] = e
			}
			return out, true, nil
		}
	}
	listing, err := s.List(dir, lim.scan)
	return listing.Entries, false, err
}

// rebind says the requests/ handle shardEntries reused shards through is
// still the directory its pathname names, so a requests/ directory, or any
// ancestor, renamed or replaced since the listing is moved (PSR-V0-225).
func rebind(s *nativeRead) error {
	root, err := s.parent("requests")
	if err != nil {
		return err
	}
	return s.binding("requests", root)
}

// RecheckResume repeats AuditForResume's base check (PSR-V0-052) against the
// files as they are now. A caller that resumed holds the writer lock and runs
// it after modelling, immediately before it commits, so no base file can
// change between the check and the commit unseen (PSR-V0-056, proposed).
// result is what AuditForResume returned. The tail the resumed walk read,
// every receipt after the base and every request file it posted, must also
// still be the bytes that walk consumed (PSR-V0-116, proposed). A difference
// is a decline. As in an audit, a failed close of the check's own read
// lifetime is cleanup, which terminates the command and is never a decline
// (PSR-V0-057, proposed).
func (r Reader) RecheckResume(base *ResumeBase, result *Result) (decline, cleanup error) {
	n, ok := r.Source.(Native)
	if !ok || base == nil {
		return errCheckpoint("resume", "recheck needs a base and a native source"), nil
	}
	s := newNativeRead(n)
	decline = base.held(s, result, profileLimits, false)
	if decline == nil {
		decline = base.tailHeld(s, result)
	}
	if cleanup = s.close(); cleanup != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, "read lifetime", "%v; close: %v", decline, cleanup)
	}
	return decline, nil
}

// held is the base check after a successful walkWriter, and RecheckResume's.
// Its listings and reads go through s, a read lifetime of its own that the
// caller closes: they are not physical reads of the audit, but a failed close
// is the caller's cleanup failure (PSR-V0-057, proposed). On a segmented
// base, rehash (the resumed audit's check) re-reads every bound segment and
// the manifest and compares their size and SHA-256 with the digests the base
// recorded; RecheckResume's gated check stats them only (JSG-V0-011).
func (b *ResumeBase) held(s *nativeRead, result *Result, lim limits, rehash bool) error {
	if result == nil || result.chain == nil || result.LastSeq.Uint64() < b.seq {
		return errCheckpoint("resume", "head precedes the base")
	}
	if b.through != 0 && (result.Head == nil || result.Head.Segments == nil || result.Head.Segments.Through.Uint64() != b.through) {
		// A seal after the base removed base receipts; the next round's
		// audit is a new base.
		return errCheckpoint(snapshot.SegmentsDir, "the sealed prefix changed since the base audit")
	}
	if b.sealedBase && result.Head.Segments.ManifestSha256 != b.files[snapshot.SegmentManifestName(b.through)].sum {
		return errCheckpoint(snapshot.SegmentsDir, "the bound manifest changed since the sealed base")
	}
	for _, p := range b.segments {
		// A segment PrecheckResume re-hashed is stat'ed only (PSR-V0-225).
		if err := b.segmentSame(s, p, rehash && !b.pre.rehashed(p)); err != nil {
			return err
		}
	}
	listing, err := s.List("receipts", lim.scan)
	if err != nil {
		return errCheckpoint("receipts", err.Error())
	}
	now := make(map[string]os.FileInfo, len(listing.Entries))
	for _, e := range listing.Entries {
		now[e.Name] = e.Info
	}
	for seq := b.through + 1; seq <= b.seq; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return err
		}
		if err := b.same(s, "receipts/"+name, now[name]); err != nil {
			return err
		}
	}
	// Every shard is listed as it is now, so a shard made after the
	// audit's capture is seen too.
	shards, err := s.List("requests", lim.scan)
	if err != nil && !(os.IsNotExist(err) && b.requests == 0) {
		return errCheckpoint("requests", err.Error())
	}
	seen, reused := 0, false
	for _, shard := range shards.Entries {
		dir := "requests/" + shard.Name
		if shard.Info == nil || !shard.Info.IsDir() {
			return errCheckpoint(dir, "request shard is not a directory")
		}
		entries, pre, err := b.shardEntries(s, dir, shard.Info, lim)
		if err != nil {
			return errCheckpoint(dir, err.Error())
		}
		reused = reused || pre
		for _, e := range entries {
			p := dir + "/" + e.Name
			if _, ok := b.files[p]; ok {
				if err := b.same(s, p, e.Info); err != nil {
					return err
				}
				seen++
				continue
			}
			if l, ok := result.chain.canonical[p]; ok && l.seq.Uint64() > b.seq && l.digest != nil && e.Info != nil && e.Info.Mode().IsRegular() {
				// Posted by the walked tail; walkWriter compared its bytes.
				continue
			}
			return errCheckpoint(p, "request file the base audit did not read")
		}
	}
	if seen != b.requests {
		return errCheckpoint("requests", "base request files differ")
	}
	if reused {
		if err := rebind(s); err != nil {
			return errCheckpoint("requests", err.Error())
		}
	}
	for _, p := range sortedPaths(b.files) {
		if !blobFamily(p) {
			continue
		}
		if l, ok := result.chain.canonical[p]; b.sealedBase && ok && l.seq.Uint64() > b.seq && l.digest == nil {
			// The walked tail removed it; the walk checked it is absent.
			continue
		}
		// No audit of this command read a sealed base's blobs: its resumed
		// audit re-hashes each one PrecheckResume did not (LGL-V0-009).
		if err := b.blobSame(s, p, b.sealedBase && rehash && !b.pre.rehashed(p)); err != nil {
			return err
		}
	}
	for _, p := range sortedPaths(result.chain.canonical) {
		l := result.chain.canonical[p]
		if l.digest == nil || !strings.HasPrefix(p, "evidence/") && !strings.HasPrefix(p, "pinned/") {
			continue
		}
		if f, ok := b.files[p]; ok && f.sum == *l.digest {
			continue
		}
		bound, err := snapshot.PostBound(p)
		if err != nil {
			return err
		}
		raw, err := s.Read(p, bound)
		if err != nil || wire.Sum(raw) != *l.digest {
			return errCheckpoint(p, "afterimage differs from latest canonical afterimage")
		}
	}
	return nil
}

// tailHeld says every receipt after the base, and every request file the
// walked tail posted, still has the bytes the resumed walk consumed. held
// runs first, so result is a resumed walk at or past the base.
func (b *ResumeBase) tailHeld(s *nativeRead, result *Result) error {
	if result.resumeRead == nil {
		return errCheckpoint("resume", "the resumed walk's reads are absent")
	}
	for seq := b.seq + 1; seq <= result.LastSeq.Uint64(); seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return err
		}
		if err := tailSame(s, result.resumeRead, "receipts/"+name, wire.MaxReceiptFileBytes); err != nil {
			return err
		}
	}
	for _, p := range sortedPaths(result.chain.canonical) {
		if l := result.chain.canonical[p]; !strings.HasPrefix(p, "requests/") || l.seq.Uint64() <= b.seq || l.digest == nil {
			continue
		}
		bound, err := snapshot.PostBound(p)
		if err != nil {
			return err
		}
		if err := tailSame(s, result.resumeRead, p, bound); err != nil {
			return err
		}
	}
	return nil
}

// tailSame rereads tail file p and compares it to what the walk consumed.
func tailSame(s *nativeRead, consumed map[string]PhysicalFile, p string, bound int) error {
	was, ok := consumed[p]
	if !ok {
		return errCheckpoint(p, "tail file the resumed walk did not read")
	}
	raw, err := s.Read(p, bound)
	if err != nil || len(raw) != was.Bytes || wire.Sum(raw) != was.Sha256 {
		return errCheckpoint(p, "tail file changed since the resumed walk read it")
	}
	return nil
}

// segmentSame says bound segment or manifest p is still the file the base
// audit read, by its stamps and, when rehash is set or its stamps had not
// settled at the base capture, by its size and SHA-256.
func (b *ResumeBase) segmentSame(s *nativeRead, p string, rehash bool) error {
	full, err := s.native.path(p)
	if err != nil {
		return errCheckpoint(p, err.Error())
	}
	info, err := os.Lstat(full)
	if err != nil {
		info = nil
	}
	f := b.files[p]
	if err := f.stamped(p, info); err != nil {
		return err
	}
	if f.quiet && !rehash {
		return nil
	}
	bound := snapshot.MaxSegmentBytes
	if p == snapshot.SegmentManifestName(b.through) {
		bound = snapshot.MaxSegmentManifestBytes
	}
	raw, err := s.Read(p, bound)
	if err != nil || int64(len(raw)) != f.bytes || wire.Sum(raw) != f.sum {
		return errCheckpoint(p, "sealed segment bytes differ from the digest the base recorded")
	}
	return nil
}

// stamped says info is the file f recorded, with the same mode, size,
// modification time and change time. A file whose stamps had not settled at
// the base capture may differ in change time alone: every caller re-reads it
// against its digest, which a link count change (a writer unlinking the
// staging name it published from, after the base capture) leaves intact
// (PSR-V0-247, proposed).
func (f resumeFile) stamped(p string, info os.FileInfo) error {
	if f.quiet && !sameStamps(f.info, info) || !f.quiet && !sameStat(f.info, info) {
		return errCheckpoint(p, "base file changed since the base audit")
	}
	return nil
}

// sameStamps says b is file a with the same mode, size, modification time
// and change time.
func sameStamps(a, b os.FileInfo) bool {
	if !sameStat(a, b) {
		return false
	}
	was, ok := changeTime(a)
	is, ok2 := changeTime(b)
	return ok && ok2 && was == is
}

// same says base file p is still the file the base audit read; see
// AuditForResume.
func (b *ResumeBase) same(s *nativeRead, p string, info os.FileInfo) error {
	f := b.files[p]
	if err := f.stamped(p, info); err != nil {
		return err
	}
	if f.quiet {
		return nil
	}
	bound := wire.MaxReceiptFileBytes
	if !strings.HasPrefix(p, "receipts/") {
		var err error
		if bound, err = snapshot.PostBound(p); err != nil {
			return err
		}
	}
	raw, err := s.Read(p, bound)
	if err != nil || wire.Sum(raw) != f.sum {
		return errCheckpoint(p, "base file bytes changed since the base audit")
	}
	return nil
}
