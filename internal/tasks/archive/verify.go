package archive

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// VerifyResult describes a verified stream.
type VerifyResult struct {
	Manifest *Manifest
	Head     *snapshot.Head
	Files    int
	Bytes    uint64 // sum of files-entry payload bytes; excludes manifest and tar framing
}

// tailReader counts consumed bytes and remembers the last 1024 so the
// end-of-archive marker (two zero blocks) can be proved rather than
// inferred from an io.EOF that a truncated stream also produces.
type tailReader struct {
	r    io.Reader
	n    int64
	last [1024]byte
	fill int
}

func (t *tailReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		t.n += int64(n)
		chunk := p[:n]
		if len(chunk) >= len(t.last) {
			copy(t.last[:], chunk[len(chunk)-len(t.last):])
			t.fill = len(t.last)
		} else {
			keep := len(t.last) - len(chunk)
			copy(t.last[:keep], t.last[len(chunk):])
			copy(t.last[keep:], chunk)
			if t.fill+len(chunk) > len(t.last) {
				t.fill = len(t.last)
			} else {
				t.fill += len(chunk)
			}
		}
	}
	return n, err
}

func (t *tailReader) lastZero() bool {
	if t.fill < len(t.last) {
		return false
	}
	for _, b := range t.last {
		if b != 0 {
			return false
		}
	}
	return true
}

// Verify reads a taskman-archive/0 stream, recomputes every digest, the
// receipt chain and the head generation, and fails on truncation, a missing
// end-of-archive marker, trailing bytes, an entry that differs from `files`
// in order, path, size or digest, a receiptCount that differs from
// head.lastSeq, or any receipt beyond head.lastSeq (JOURNAL_FORKED).
func Verify(r io.Reader) (*VerifyResult, error) { return verifyLimit(r, wire.MaxArchiveBytes) }

func verifyLimit(input io.Reader, limit uint64) (result *VerifyResult, retErr error) {
	bounded := &streamReader{r: input, limit: limit}
	defer func() {
		if bounded.sourceErr != nil {
			result = nil
			retErr = wire.Errorf(wire.CodeMalformed, "stream", "source read failed: %v", bounded.sourceErr)
		}
		if bounded.exceeded {
			result = nil
			retErr = streamLimitError(limit)
		}
	}()
	var r io.Reader = bounded
	tail := &tailReader{r: r}
	tr := tar.NewReader(tail)
	hdr, err := tr.Next()
	if err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, ManifestName, "stream does not start with a tar entry: %v", err)
	}
	if hdr.Name != ManifestName || hdr.Typeflag != tar.TypeReg {
		return nil, wire.Errorf(wire.CodeMalformed, ManifestName, "first entry must be manifest.json, got %q", hdr.Name)
	}
	// The manifest byte cap is checked on the tar header, before any
	// manifest byte is read into memory (§3.5).
	if hdr.Size < 0 || hdr.Size > MaxManifestBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, ManifestName, "manifest larger than %d bytes", MaxManifestBytes)
	}
	if uint64(hdr.Size) > bounded.limit-bounded.n {
		return nil, streamLimitError(limit)
	}
	mraw, err := readExact(tr, hdr.Size)
	if err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, ManifestName, "truncated manifest: %v", err)
	}
	m, err := DecodeManifest(mraw)
	if err != nil {
		return nil, err
	}
	ordered := make([]FileEntry, len(m.Files))
	copy(ordered, m.Files)
	SortFiles(ordered)
	retained := map[string][]byte{}
	var total uint64
	var dataEnd int64
	for i, fe := range ordered {
		hdr, err := tr.Next()
		if err != nil {
			return nil, wire.Errorf(wire.CodeMalformed, fe.Path, "stream ends before entry %d of %d: %v", i+1, len(ordered), err)
		}
		if hdr.Name != fe.Path {
			return nil, wire.Errorf(wire.CodeMalformed, fe.Path, "entry order differs from files: got %q", hdr.Name)
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil, wire.Errorf(wire.CodeMalformed, fe.Path, "entry is not a regular file")
		}
		if uint64(hdr.Size) != fe.Bytes.Uint64() || hdr.Size < 0 {
			return nil, wire.Errorf(wire.CodeMalformed, fe.Path, "entry size %d differs from files.bytes %s", hdr.Size, fe.Bytes)
		}
		bound := boundFor(fe.Path)
		if hdr.Size > int64(bound) {
			return nil, wire.Errorf(wire.CodeLimitExceeded, fe.Path, "entry larger than its §1 bound of %d bytes", bound)
		}
		if uint64(hdr.Size) > bounded.limit-bounded.n {
			return nil, streamLimitError(limit)
		}
		keep := retainPath(fe.Path)
		h := sha256.New()
		var buf []byte
		if keep {
			buf, err = readExact(tr, hdr.Size)
			if err != nil {
				return nil, wire.Errorf(wire.CodeMalformed, fe.Path, "truncated entry: %v", err)
			}
			h.Write(buf)
		} else {
			n, err := io.Copy(h, tr)
			if err != nil || n != hdr.Size {
				return nil, wire.Errorf(wire.CodeMalformed, fe.Path, "truncated entry")
			}
		}
		if wire.Digest(hex.EncodeToString(h.Sum(nil))) != fe.Sha256 {
			return nil, wire.Errorf(wire.CodeMalformed, fe.Path, "entry digest differs from files.sha256")
		}
		if err := checkContentName(fe.Path, wire.Digest(hex.EncodeToString(h.Sum(nil)))); err != nil {
			return nil, err
		}
		if keep {
			retained[fe.Path] = buf
		}
		total += uint64(hdr.Size)
		if total > wire.MaxArchiveBytes {
			return nil, wire.Errorf(wire.CodeLimitExceeded, fe.Path, "archive larger than %d bytes", wire.MaxArchiveBytes)
		}
		dataEnd = tail.n + (512-hdr.Size%512)%512
	}
	if _, err := tr.Next(); err != io.EOF {
		if err == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "stream", "an entry follows the last files entry")
		}
		return nil, wire.Errorf(wire.CodeMalformed, "stream", "corrupt trailer: %v", err)
	}
	if tail.n != dataEnd+1024 || !tail.lastZero() {
		return nil, wire.Errorf(wire.CodeMalformed, "stream", "missing end-of-archive marker (stream truncated)")
	}
	var one [1]byte
	if n, err := io.ReadFull(r, one[:]); n != 0 {
		return nil, wire.Errorf(wire.CodeMalformed, "stream", "bytes follow the end-of-archive marker")
	} else if err != io.EOF {
		return nil, wire.Errorf(wire.CodeMalformed, "stream", "terminal read failed: %v", err)
	}
	res := &VerifyResult{Manifest: m, Files: len(ordered), Bytes: total}
	if err := checkStore(m, retained, res); err != nil {
		return nil, err
	}
	return res, nil
}

func readExact(r io.Reader, size int64) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func retainPath(p string) bool {
	return p == "VERSION" || p == "head.json" || p == "barrier.json" ||
		strings.HasPrefix(p, "receipts/") || strings.HasPrefix(p, "intent/")
}

// checkStore proves the manifest against the retained store files: VERSION,
// head, barrier, the receipt chain and the intent tree.
func checkStore(m *Manifest, files map[string][]byte, res *VerifyResult) error {
	ver, ok := files["VERSION"]
	if !ok {
		return wire.Errorf(wire.CodeMalformed, "VERSION", "archive carries no VERSION")
	}
	if string(ver) != snapshot.VersionBytes {
		return wire.Errorf(wire.CodeUnsupportedVersion, "VERSION", "state version %q is not %q", string(ver), snapshot.VersionBytes)
	}
	if wire.Sum(ver) != m.VersionSha256 {
		return wire.Errorf(wire.CodeMalformed, "/versionSha256", "does not equal the VERSION entry digest")
	}
	hraw, ok := files["head.json"]
	if !ok {
		return wire.Errorf(wire.CodeMalformed, "head.json", "archive carries no head.json")
	}
	if wire.Sum(hraw) != m.HeadSha256 {
		return wire.Errorf(wire.CodeMalformed, "/headSha256", "does not equal the head.json entry digest")
	}
	head, err := snapshot.DecodeHead(hraw)
	if err != nil {
		return err
	}
	res.Head = head
	if head.QueueID.Raw != m.QueueID.Raw {
		return wire.Errorf(wire.CodeMalformed, "/queueId", "manifest queue %s differs from head.json %s", m.QueueID.Raw, head.QueueID.Raw)
	}
	if head.PrimaryWorktree != m.PrimaryWorktree {
		return wire.Errorf(wire.CodeMalformed, "/primaryWorktree", "differs from head.json")
	}
	if head.VersionSha256 != m.VersionSha256 {
		return wire.Errorf(wire.CodeMalformed, "head.json/versionSha256", "differs from the VERSION entry digest")
	}
	braw, hasBarrier := files["barrier.json"]
	if hasBarrier != (m.BarrierSha256 != nil) {
		return wire.Errorf(wire.CodeMalformed, "/barrierSha256", "barrier.json presence differs from the manifest")
	}
	if hasBarrier {
		if wire.Sum(braw) != *m.BarrierSha256 {
			return wire.Errorf(wire.CodeMalformed, "/barrierSha256", "does not equal the barrier.json entry digest")
		}
		if _, err := snapshot.DecodeBarrier(braw); err != nil {
			return err
		}
	}
	// Receipt chain: exactly 1..lastSeq, each linking to its predecessor,
	// headGeneration never decreasing and ending at head.generation.
	lastSeq := head.LastSeq.Uint64()
	if m.ReceiptCount.Uint64() != lastSeq {
		return wire.Errorf(wire.CodeJournalForked, "/receiptCount", "receiptCount %s differs from head.lastSeq %s", m.ReceiptCount, head.LastSeq)
	}
	if m.ExportedAtSeq.Uint64() != lastSeq {
		return wire.Errorf(wire.CodeJournalForked, "/exportedAtSeq", "exportedAtSeq %s differs from head.lastSeq %s", m.ExportedAtSeq, head.LastSeq)
	}
	receiptCount := uint64(0)
	for p := range files {
		if strings.HasPrefix(p, "receipts/") {
			receiptCount++
			seq, ok := parseReceiptName(strings.TrimPrefix(p, "receipts/"))
			if !ok {
				return wire.Errorf(wire.CodeMalformed, p, "receipt file name is not <12 digits>.json")
			}
			if seq > lastSeq {
				return wire.Errorf(wire.CodeJournalForked, p, "receipt %d is beyond head.lastSeq %d; an archive never carries a pending or forked receipt", seq, lastSeq)
			}
		}
	}
	if receiptCount != lastSeq {
		return wire.Errorf(wire.CodeJournalForked, "receipts", "%d receipt entries but head.lastSeq is %d", receiptCount, lastSeq)
	}
	var prev *wire.Digest
	var prevGen uint64
	for seq := uint64(1); seq <= lastSeq; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return err
		}
		raw, ok := files["receipts/"+name]
		if !ok {
			return wire.Errorf(wire.CodeJournalForked, "receipts/"+name, "receipt %d is missing from the chain", seq)
		}
		rc, err := snapshot.DecodeReceipt(raw)
		if err != nil {
			return prefixWhere(err, "receipts/"+name)
		}
		if rc.Seq.Uint64() != seq {
			return wire.Errorf(wire.CodeJournalForked, "receipts/"+name, "receipt seq %s differs from its file name", rc.Seq)
		}
		if (prev == nil) != (rc.Prev == nil) || (prev != nil && *prev != *rc.Prev) {
			return wire.Errorf(wire.CodeJournalForked, "receipts/"+name, "prev does not link to receipt %d", seq-1)
		}
		if rc.HeadGeneration.Uint64() < prevGen {
			return wire.Errorf(wire.CodeJournalForked, "receipts/"+name, "headGeneration decreases")
		}
		prevGen = rc.HeadGeneration.Uint64()
		d := wire.Sum(raw)
		prev = &d
	}
	if prev == nil || *prev != m.LastReceiptSha256 || *prev != *head.LastReceiptSha256 {
		return wire.Errorf(wire.CodeJournalForked, "/lastReceiptSha256", "does not equal the digest of receipt %d and head.lastReceiptSha256", lastSeq)
	}
	if prevGen != m.HeadGeneration.Uint64() || prevGen != head.Generation.Uint64() {
		return wire.Errorf(wire.CodeJournalForked, "/headGeneration", "generation re-derived from receipts (%d) differs from the manifest (%s) or head (%s)", prevGen, m.HeadGeneration, head.Generation)
	}
	// Intent tree: the queue manifest must be present and name the queue;
	// the tree digest is recomputed from the intent/ entries.
	qraw, ok := files["intent/"+intent.QueueFile]
	if !ok {
		return wire.Errorf(wire.CodeMalformed, "intent/"+intent.QueueFile, "archive carries no queue.json")
	}
	q, err := intent.DecodeQueue(qraw)
	if err != nil {
		return prefixWhere(err, "intent/"+intent.QueueFile)
	}
	if q.QueueID.Raw != m.QueueID.Raw {
		return wire.Errorf(wire.CodeMalformed, "intent/"+intent.QueueFile+"/queueId", "queue %s differs from the manifest %s", q.QueueID.Raw, m.QueueID.Raw)
	}
	var tree []intent.File
	for _, fe := range m.Files {
		if strings.HasPrefix(fe.Path, "intent/") {
			tree = append(tree, intent.File{Path: strings.TrimPrefix(fe.Path, "intent/"), Sha256: fe.Sha256, Bytes: int(fe.Bytes.Uint64())})
		}
	}
	if intent.DigestOfFiles(tree) != m.IntentTreeSha256 {
		return wire.Errorf(wire.CodeMalformed, "/intentTreeSha256", "does not equal the digest recomputed from the intent/ entries")
	}
	return nil
}

func prefixWhere(err error, file string) error {
	e, ok := err.(*wire.Error)
	if !ok {
		return err
	}
	where := file
	if e.Where != "" && e.Where != "/" {
		where = file + e.Where
	}
	return &wire.Error{Code: e.Code, Where: where, Msg: e.Msg}
}
