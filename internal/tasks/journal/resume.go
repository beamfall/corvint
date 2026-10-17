package journal

import (
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
func (r Reader) auditPrefixFallback() (*Result, PhysicalObservation, error) {
	observation := PhysicalObservation{}
	r.PrefixFallback = false
	switch r.Source.(type) {
	case Native, *Native:
	default:
		r.writerCache, r.physical = true, &observation
		result, err := r.Audit()
		return result, observation, err
	}
	full := r
	full.writerCache, full.physical, full.fullAttempts, full.abortMoved = true, &observation, 1, true
	result, err := full.Audit()
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
	native, ok := r.Source.(*nativeRead)
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
// never shared with another command or process.
type ResumeBase struct {
	wc       *WriterCheckpoint
	seq      uint64
	files    map[string]resumeFile
	requests int
}

type resumeFile struct {
	info  os.FileInfo
	sum   wire.Digest
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
	add := func(p string) error {
		info := o.files[p]
		f, read := obs.Files[p]
		if info == nil || !info.Mode().IsRegular() || !read {
			return errCheckpoint(p, "base file was not read by the audit")
		}
		change, ok := changeTime(info)
		if !ok {
			return errCheckpoint(p, "no change time")
		}
		b.files[p] = resumeFile{info: info, sum: f.Sha256, quiet: !info.ModTime().After(bound) && !time.Unix(0, change).After(bound)}
		return nil
	}
	// Every receipt, request, evidence and pinned file the audit read is a
	// base file, including blobs a prefix receipt's posts name.
	for _, p := range sortedPaths(obs.Files) {
		if resumeFamily(p) {
			if err := add(p); err != nil {
				return nil, err
			}
		}
	}
	for seq := uint64(1); seq <= b.seq; seq++ {
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
		if l.digest == nil || !resumeFamily(p) {
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
	r.resume = base
	return r.AuditForWriter(base.wc, requestID, false)
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
	decline = base.held(s, result, profileLimits)
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
// is the caller's cleanup failure (PSR-V0-057, proposed).
func (b *ResumeBase) held(s *nativeRead, result *Result, lim limits) error {
	if result == nil || result.chain == nil || result.LastSeq.Uint64() < b.seq {
		return errCheckpoint("resume", "head precedes the base")
	}
	listing, err := s.List("receipts", lim.scan)
	if err != nil {
		return errCheckpoint("receipts", err.Error())
	}
	now := make(map[string]os.FileInfo, len(listing.Entries))
	for _, e := range listing.Entries {
		now[e.Name] = e.Info
	}
	for seq := uint64(1); seq <= b.seq; seq++ {
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
	seen := 0
	for _, shard := range shards.Entries {
		dir := "requests/" + shard.Name
		if shard.Info == nil || !shard.Info.IsDir() {
			return errCheckpoint(dir, "request shard is not a directory")
		}
		listing, err := s.List(dir, lim.scan)
		if err != nil {
			return errCheckpoint(dir, err.Error())
		}
		for _, e := range listing.Entries {
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
	for _, p := range sortedPaths(b.files) {
		if !strings.HasPrefix(p, "evidence/") && !strings.HasPrefix(p, "pinned/") {
			continue
		}
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

// same says base file p is still the file the base audit read; see
// AuditForResume.
func (b *ResumeBase) same(s *nativeRead, p string, info os.FileInfo) error {
	f := b.files[p]
	if info == nil || !sameStat(f.info, info) {
		return errCheckpoint(p, "base file changed since the base audit")
	}
	was, ok := changeTime(f.info)
	is, ok2 := changeTime(info)
	if !ok || !ok2 || was != is {
		return errCheckpoint(p, "base file changed since the base audit")
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
