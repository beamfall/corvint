package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Journal retention (JRT-V0, docs/specs/corvint-tasks-journal-retention-v0.md).
//
// A retired store keeps one seal, RETIRED_PREFIX, in its state directory. It
// names an anchor receipt S that stays retained, the digest of receipt S-1 its
// Prev binds, the archive that holds receipts 1..S-1, and the canonical
// afterimage bytes of every non-blob path latest at S. Receipts 1..S-1 are
// gone from the live store; every complete audit of a sealed store starts at
// S (JRT-V0-004).
const (
	// RetiredPrefixFile is the seal (JRT-V0-003).
	RetiredPrefixFile = "RETIRED_PREFIX"
	// RetireIncompleteFile marks a retire or unretire between its first and
	// last write; every reader and writer refuses while it exists (JRT-V0-009).
	RetireIncompleteFile = "RETIRE_INCOMPLETE"
	// ProfileRetiredPrefix names the seal encoding.
	ProfileRetiredPrefix = "taskman-retired-prefix/1"
	// MaxRetiredPrefixBytes bounds the seal a reader consumes.
	MaxRetiredPrefixBytes = 512 * wire.MiB
)

const sealMagic = ProfileRetiredPrefix + "\n"

// RetiredPrefix is the decoded seal. Anchor is the writer checkpoint a
// complete settled audit derived at S, so its entries name every latest
// non-request afterimage and its request records every request afterimage
// receipts 1..S posted. Seeds holds the exact afterimage bytes of every entry
// with a digest outside evidence/ and pinned/, keyed by path.
type RetiredPrefix struct {
	Anchor *WriterCheckpoint
	// ElidedSha256 is the SHA-256 of receipt S-1, the last archived receipt;
	// receipt S's Prev must equal it.
	ElidedSha256 wire.Digest
	// ArchiveSha256 and ArchiveBytes name the verified archive export that
	// holds receipts 1..S-1 (JRT-V0-006).
	ArchiveSha256 wire.Digest
	ArchiveBytes  uint64
	Seeds         map[string][]byte
}

// Seq is the anchor sequence S.
func (s *RetiredPrefix) Seq() uint64 { return s.Anchor.Seq.Uint64() }

// Retired is the number of receipts the seal stands for, 1..S-1.
func (s *RetiredPrefix) Retired() uint64 { return s.Seq() - 1 }

// seeded reports whether p's afterimage is a seed: every anchor entry with a
// digest outside the content-addressed blob trees.
func seeded(p string) bool {
	return !strings.HasPrefix(p, "evidence/") && !strings.HasPrefix(p, "pinned/") && !strings.HasPrefix(p, "requests/")
}

// Seed returns the anchor afterimage of p when the seal holds one.
func (s *RetiredPrefix) Seed(p string) ([]byte, bool) {
	raw, ok := s.Seeds[p]
	return raw, ok
}

func sealForked(why string, args ...any) error {
	return wire.Errorf(wire.CodeJournalForked, RetiredPrefixFile, why, args...)
}

// Encode renders the seal bytes: magic, the anchor's writer checkpoint
// encoding, the elided and archive digests, the archive size, the sorted
// seeds and a SHA-256 trailer over all of it.
func (s *RetiredPrefix) Encode() ([]byte, error) {
	elided, err := digestBytes(s.ElidedSha256)
	if err != nil {
		return nil, err
	}
	archived, err := digestBytes(s.ArchiveSha256)
	if err != nil {
		return nil, err
	}
	anchor, err := encodeAnchor(s.Anchor)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(s.Seeds))
	for p := range s.Seeds {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	buf := append([]byte(nil), sealMagic...)
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(anchor)))
	buf = append(buf, anchor...)
	buf = append(buf, elided...)
	buf = append(buf, archived...)
	buf = binary.BigEndian.AppendUint64(buf, s.ArchiveBytes)
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(paths)))
	for _, p := range paths {
		buf = binary.BigEndian.AppendUint32(buf, uint32(len(p)))
		buf = append(buf, p...)
		buf = binary.BigEndian.AppendUint64(buf, uint64(len(s.Seeds[p])))
		buf = append(buf, s.Seeds[p]...)
	}
	if len(buf)+sha256.Size > MaxRetiredPrefixBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, RetiredPrefixFile, "seal exceeds its byte bound")
	}
	sum := sha256.Sum256(buf)
	out := append(buf, sum[:]...)
	if _, err := DecodeRetiredPrefix(out); err != nil {
		return nil, err
	}
	return out, nil
}

func digestBytes(d wire.Digest) ([]byte, error) {
	if _, err := wire.ParseDigest(RetiredPrefixFile, string(d)); err != nil {
		return nil, err
	}
	out := make([]byte, sha256.Size)
	for i := range out {
		out[i] = unhex(d[2*i])<<4 | unhex(d[2*i+1])
	}
	return out, nil
}

func unhex(c byte) byte {
	if c >= 'a' {
		return c - 'a' + 10
	}
	return c - '0'
}

func hexDigest(b []byte) wire.Digest {
	const digits = "0123456789abcdef"
	out := make([]byte, 2*len(b))
	for i, c := range b {
		out[2*i], out[2*i+1] = digits[c>>4], digits[c&15]
	}
	return wire.Digest(out)
}

// DecodeRetiredPrefix parses and checks a seal. Every failure is
// JOURNAL_FORKED: a store that carries a seal is audited only through it, so
// an altered seal is never skipped (JRT-V0-003). Each seed must hash to its
// anchor entry and every anchor entry with a digest outside the blob trees
// must have a seed.
func DecodeRetiredPrefix(raw []byte) (*RetiredPrefix, error) {
	if len(raw) > MaxRetiredPrefixBytes {
		return nil, sealForked("larger than its bound")
	}
	// A seal another build wrote at another version of this profile is
	// refused before any layout check (CAL-V0-131).
	if name, _, _ := strings.Cut(ProfileRetiredPrefix, "/"); bytes.HasPrefix(raw, []byte(name+"/")) && !bytes.HasPrefix(raw, []byte(sealMagic)) {
		return nil, wire.Errorf(wire.CodeUnsupportedVersion, RetiredPrefixFile, "another %s version", name)
	}
	if len(raw) < len(sealMagic)+8+2*sha256.Size+3*8+sha256.Size {
		return nil, sealForked("truncated")
	}
	body, trailer := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	if sum := sha256.Sum256(body); !bytes.Equal(sum[:], trailer) {
		return nil, sealForked("trailer digest differs")
	}
	if !bytes.HasPrefix(body, []byte(sealMagic)) {
		return nil, sealForked("profile differs")
	}
	rest := body[len(sealMagic):]
	n := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	if n > uint64(len(rest)) {
		return nil, sealForked("anchor length")
	}
	anchor, err := decodeAnchor(rest[:n])
	if err != nil {
		return nil, sealForked("anchor: %v", err)
	}
	rest = rest[n:]
	if len(rest) < 2*sha256.Size+2*8 {
		return nil, sealForked("truncated digests")
	}
	s := &RetiredPrefix{Anchor: anchor, ElidedSha256: hexDigest(rest[:sha256.Size]), ArchiveSha256: hexDigest(rest[sha256.Size : 2*sha256.Size])}
	rest = rest[2*sha256.Size:]
	s.ArchiveBytes = binary.BigEndian.Uint64(rest)
	count := binary.BigEndian.Uint64(rest[8:])
	rest = rest[16:]
	if anchor.Seq.Uint64() < 2 {
		return nil, sealForked("anchor must leave at least one receipt retired")
	}
	entries := map[string]*wire.Digest{}
	for _, e := range anchor.Entries {
		if e.Sha256 != nil && seeded(e.Path) {
			entries[e.Path] = e.Sha256
		}
	}
	if count != uint64(len(entries)) {
		return nil, sealForked("seed count differs from the anchor entries")
	}
	s.Seeds = make(map[string][]byte, count)
	prev := ""
	for i := uint64(0); i < count; i++ {
		if len(rest) < 4 {
			return nil, sealForked("truncated seed")
		}
		pl := uint64(binary.BigEndian.Uint32(rest))
		rest = rest[4:]
		if pl > uint64(len(rest)) || uint64(len(rest))-pl < 8 {
			return nil, sealForked("truncated seed path")
		}
		p := string(rest[:pl])
		rl := binary.BigEndian.Uint64(rest[pl:])
		rest = rest[pl+8:]
		if rl > uint64(len(rest)) {
			return nil, sealForked("truncated seed bytes")
		}
		seed := rest[:rl]
		rest = rest[rl:]
		if i > 0 && p <= prev {
			return nil, sealForked("seeds are not strictly ordered")
		}
		prev = p
		want, ok := entries[p]
		if !ok || wire.Sum(seed) != *want {
			return nil, sealForked("seed %s differs from its anchor entry", p)
		}
		s.Seeds[p] = seed
	}
	if len(rest) != 0 {
		return nil, sealForked("trailing bytes")
	}
	return s, nil
}

// RetiredThrough is the anchor sequence S of the seal a complete audit
// started from, or 0 when the audit walked from receipt 1.
func (res *Result) RetiredThrough() uint64 {
	if res == nil || res.chain == nil || res.chain.seal == nil {
		return 0
	}
	return res.chain.seal.Seq()
}

// RetiredRequest reports whether path names a request that a receipt
// retired with the prefix posted. Its receipt is only in the retained
// archive, so a writer refuses its replay UNSUPPORTED (JRT-V0-010).
func (res *Result) RetiredRequest(path string) bool {
	return res != nil && res.chain != nil && res.chain.seal != nil && res.chain.seal.Anchor.HasRequest(path)
}

// walkSealed is the complete audit of a sealed store (JRT-V0-004). It rebinds
// the anchor receipt S to the seal and its Prev to the retired prefix,
// requires the receipt names on disk to be exactly S..head (or head+1
// pending), seeds the chain with the anchor's latest afterimages, requires
// the retained request names to be exactly the ones the anchor records, then
// walks every receipt after S with the complete audit's per-receipt
// validators. The settled tail is the complete audit's own (settle).
func (r Reader) walkSealed(o *observation, result *Result, selected map[string]bool, request string, lim limits, headSeq uint64) (*chain, error) {
	s := o.seal
	a := s.Anchor
	from := a.Seq.Uint64()
	head := o.head
	if head == nil {
		return nil, sealForked("a retired prefix needs a settled head")
	}
	// The anchor records the primary worktree as of the seal; a later
	// RELOCATE receipt in the walked tail moves it (TRL-V0-002).
	if a.QueueID != r.QueueID || a.InitSha256 != head.InitSha256 || from > headSeq || a.Generation.Uint64() > head.Generation.Uint64() || (from == headSeq && a.PrimaryWorktree != head.PrimaryWorktree) {
		return nil, sealForked("anchor is not a prefix of head")
	}
	count := uint64(len(o.receipts))
	if count != headSeq-from+1 && count != headSeq-from+2 {
		return nil, wire.Errorf(wire.CodeJournalForked, "receipts", "receipt names must be the anchor through head, or head+1")
	}
	for i, name := range o.receipts {
		seq, err := receiptSeq(name)
		if err != nil {
			return nil, err
		}
		if seq != from+uint64(i) {
			return nil, wire.Errorf(wire.CodeJournalForked, name, "receipt gap, late extra or receipt inside the retired prefix")
		}
	}
	result.Pending = count == headSeq-from+2
	if result.Pending && (r.divergentIntent != "" || (r.unpauseTickets && !r.pendingUnpause)) {
		return nil, wire.Errorf(wire.CodeRedoPending, "receipts", "this observation requires a settled journal")
	}
	name := o.receipts[0]
	raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
	if err != nil {
		return nil, err
	}
	if wire.Sum(raw) != a.ReceiptSha256 {
		return nil, wire.Errorf(wire.CodeJournalForked, name, "anchor receipt differs from the seal")
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil || rc.Seq.Uint64() != from || rc.HeadGeneration != a.Generation || rc.Prev == nil || *rc.Prev != s.ElidedSha256 {
		return nil, wire.Errorf(wire.CodeJournalForked, name, "anchor receipt sequence, generation or prefix link differs from the seal")
	}
	if from == headSeq && (*head.LastReceiptSha256 != a.ReceiptSha256 || head.Generation != a.Generation) {
		return nil, wire.Errorf(wire.CodeJournalForked, "head.json", "head digest/generation differs from the anchor")
	}
	if request != "" {
		if rp, err := snapshot.RequestPath(request); err == nil && a.HasRequest(rp) {
			return nil, wire.Errorf(wire.CodeUnsupported, rp, "request was retired with the prefix; its receipt is in the retained archive")
		}
	}
	st := &chain{canonical: make(map[string]latest, len(a.Entries)), prev: &a.ReceiptSha256, generation: a.Generation.Uint64(), seal: s, primary: a.PrimaryWorktree,
		notes: &noteState{refs: map[string]*ticket.OperatorNoteReference{}, policyWalked: true}}
	for _, e := range a.Entries {
		st.canonical[e.Path] = latest{seq: e.Seq, digest: e.Sha256, bytes: -1}
	}
	// Requests receipts 1..S posted stay on disk but are not reinstated as
	// canonical paths: their names must be exactly the ones the anchor
	// records, and their bytes, which the anchor digests and the archive
	// holds, are neither read nor hashed, so the audit's cost does not grow
	// with retired history (JRT-V0-013). No later operation reads them:
	// their replay is refused (JRT-V0-010) and unretire's closing complete
	// audit verifies each against its receipt again.
	var requests uint64
	for p, info := range o.files {
		if strings.HasPrefix(p, "requests/") && !info.IsDir() && !isTemp(p) && a.HasRequest(p) {
			requests++
		}
	}
	if requests != a.Requests() {
		return nil, wire.Errorf(wire.CodeJournalForked, "requests", "retained request afterimages differ from the seal")
	}
	result.SemanticCoverage = a.SemanticCoverage
	result.LastSeq, result.LastReceiptSha256 = a.Seq, a.ReceiptSha256
	for i := 1; i < len(o.receipts); i++ {
		if i%abortEvery == abortEvery-1 && r.headMoved(o) {
			return nil, moved("head.json")
		}
		if err := r.step(o, st, result, o.receipts[i], from+uint64(i), headSeq, selected, request, lim); err != nil {
			return nil, err
		}
	}
	// Selected paths no later receipt posted are served from the anchor:
	// the seed bytes, or the projection for a path the seal does not seed.
	for _, p := range sortedPaths(st.canonical) {
		l := st.canonical[p]
		if _, have := result.Records[p]; have || l.seq.Uint64() > from || result.selectErr != nil || !r.selects(p, selected) {
			continue
		}
		var body []byte
		if l.digest != nil {
			if seed, ok := s.Seed(p); ok {
				body = seed
			} else {
				digest, raw, err := r.projected(o, p, true)
				if err != nil {
					return nil, err
				}
				if !equalDigest(digest, l.digest) {
					return nil, wire.Errorf(wire.CodeJournalForked, p, "projection differs from latest canonical afterimage")
				}
				body = raw
			}
		}
		if len(body) > lim.selected-st.selectedBytes {
			err := wire.Errorf(wire.CodeLimitExceeded, p, "selected canonical bytes exceed aggregate live intent-state budget; select fewer paths")
			if !r.observedIntent {
				return nil, err
			}
			result.selectErr = err
			continue
		}
		st.selectedBytes += len(body)
		result.Records[p] = Record{Seq: l.seq, Sha256: l.digest, Raw: body}
	}
	return st, nil
}

// sealedNote seeds the operator-note state of a ticket whose latest
// afterimage is the anchor's, so the note audit binds it as the complete
// audit from receipt 1 would (ON-V0-006).
func (st *chain) sealedNote(p string, prior latest) error {
	if st.seal == nil || st.notes == nil || prior.digest == nil || prior.seq.Uint64() > st.seal.Seq() {
		return nil
	}
	if _, known := st.notes.refs[p]; known {
		return nil
	}
	raw, ok := st.seal.Seed(p)
	if !ok || wire.Sum(raw) != *prior.digest {
		return nil
	}
	rec, err := ticket.Decode(raw)
	if err != nil {
		return err
	}
	st.notes.refs[p] = rec.OperatorNote
	return nil
}

// sealedPost returns the anchor afterimage a canonical latest at or before
// the anchor names; the receipt that posted it was retired.
func (st *chain) sealedPost(prior latest, p string) ([]byte, bool) {
	if st.seal == nil || prior.digest == nil || prior.seq.Uint64() > st.seal.Seq() {
		return nil, false
	}
	raw, ok := st.seal.Seed(p)
	if !ok || wire.Sum(raw) != *prior.digest {
		return nil, false
	}
	return raw, true
}

// priorPost serves an anchor afterimage from the seal and every later one
// from the receipt that posted it.
func (st *chain) priorPost(r Reader, prior latest, p string) ([]byte, error) {
	if raw, ok := st.sealedPost(prior, p); ok {
		return raw, nil
	}
	return r.priorPost(prior, p, st.generation, st.resumed, st.resumedSha)
}

// NewRetiredPrefix derives the seal that retires receipts 1..S-1 of the
// complete settled audit res, whose head is S (JRT-V0-003). The anchor is the
// writer checkpoint res supports; every seed is read back from the state
// directory and must hash to its anchor entry. archiveSha and archiveBytes
// name the verified export that holds the retired receipts. The caller holds
// the writer lock; any other observation is refused.
func (r Reader) NewRetiredPrefix(res *Result, physical map[string]PhysicalFile, archiveSha wire.Digest, archiveBytes uint64) (*RetiredPrefix, error) {
	if res == nil || res.chain == nil || res.Mode != ModeFull || res.StructuralConsistency != "CONSISTENT" || res.chain.seal != nil || res.Pending || res.StagingPresent || res.IntentError != nil {
		return nil, wire.Errorf(wire.CodeRedoPending, "receipts", "retiring a prefix needs a complete settled audit of an unsealed journal")
	}
	wc, err := res.WriterCheckpoint(physical)
	if err != nil {
		return nil, err
	}
	if wc.Seq.Uint64() < 2 {
		return nil, wire.Errorf(wire.CodeUnsupported, "receipts", "a journal of one receipt has no prefix to retire")
	}
	name, err := snapshot.ReceiptName(wc.Seq.Uint64())
	if err != nil {
		return nil, err
	}
	raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
	if err != nil {
		return nil, err
	}
	if wire.Sum(raw) != wc.ReceiptSha256 {
		return nil, wire.Errorf(wire.CodeJournalForked, name, "anchor receipt is not the whole receipt the audit walked")
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		return nil, err
	}
	if rc.Prev == nil {
		return nil, wire.Errorf(wire.CodeJournalForked, name, "anchor receipt has no prefix link")
	}
	s := &RetiredPrefix{Anchor: wc, ElidedSha256: *rc.Prev, ArchiveSha256: archiveSha, ArchiveBytes: archiveBytes, Seeds: map[string][]byte{}}
	for _, e := range wc.Entries {
		if e.Sha256 == nil || !seeded(e.Path) {
			continue
		}
		bound, err := snapshot.PostBound(e.Path)
		if err != nil {
			return nil, err
		}
		body, present, err := optionalRead(r.Source, e.Path, bound)
		if err != nil {
			return nil, err
		}
		if !present || wire.Sum(body) != *e.Sha256 {
			return nil, wire.Errorf(wire.CodeJournalForked, e.Path, "retained afterimage differs from the audited latest")
		}
		if body == nil {
			body = []byte{}
		}
		s.Seeds[e.Path] = body
	}
	if _, err := s.Encode(); err != nil {
		return nil, err
	}
	return s, nil
}
