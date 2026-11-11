package linecov

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// magicV4 starts every linecov-index/4 file (GLS-V0-001).
const magicV4 = Format + "\n"

// envAuditMarker versions the completeness claim, not the index encoding.
// NUL cannot occur in an environment variable name. The final, unreferenced
// Env row carries this metadata without adding a wire field (GLS-V0-017).
// Only this exact version certifies syscall and collector-modified reads; all other
// versions, including an absent marker, require recollection.
var envAuditMarker = EnvBinding{Name: "\x00linecov-env-audit", Value: "3"}

// maxHeader bounds the header a reader accepts, so a damaged length cannot
// make it allocate without limit.
const maxHeader = 1 << 30

// fileRow is one file-table row: where its block section lies, relative to
// the first section, and the SHA-256 the section must have.
type fileRow struct {
	offset, length int64
	blocks         int
	sum            [sha256.Size]byte
}

// lazySource is the open file a lazily read index decodes sections from
// (GLS-V0-002). loaded[i] reports Files[i].Blocks is decoded and verified.
type lazySource struct {
	r      io.ReaderAt
	closer io.Closer
	base   int64 // file offset of the first section
	rows   []fileRow
	loaded []bool
	all    bool
}

// extension carries the rarely present parts of the index in the header:
// segments and divergent classes, the recorded deadlines, the packages whose
// in-order run kept the suite union and why, and the language collector
// identity of a non-Go index. It is gob-encoded, so a field a reader does not
// know is skipped and a missing one stays zero.
type extension struct {
	Segments       []Segment
	Divergent      []Divergence
	TestTimeout    time.Duration
	SuiteTimeout   time.Duration
	WindowFallback map[string]string
	Language       string
	Command        []string
	Executable     string
	RunnerFamily   string
	Config         string
	// EnvBound, Env and TestEnv (one list per test, in Tests order) are the
	// environment bindings (GLS-V0-017).
	EnvBound bool
	Env      []EnvBinding
	TestEnv  [][]int32
}

// extensionOf is ix's header extension, and whether any part of it is set.
func extensionOf(ix *Index) (extension, bool) {
	e := extension{Segments: ix.Segments, Divergent: ix.Divergent, TestTimeout: ix.TestTimeout, SuiteTimeout: ix.SuiteTimeout, WindowFallback: ix.WindowFallback, Language: ix.Language, Command: ix.Command, Executable: ix.Executable, RunnerFamily: ix.RunnerFamily, Config: ix.Config, EnvBound: ix.EnvBound, Env: ix.Env}
	if e.EnvBound {
		// Never upgrade an unbound decoded index merely by writing it again.
		e.Env = append(append([]EnvBinding(nil), e.Env...), envAuditMarker)
	}
	set := len(e.Segments) > 0 || len(e.Divergent) > 0 || e.TestTimeout != 0 || e.SuiteTimeout != 0 || len(e.WindowFallback) > 0 || e.Language != "" || len(e.Command) > 0 || e.Executable != "" || e.RunnerFamily != "" || e.Config != "" || e.EnvBound || len(e.Env) > 0
	for _, t := range ix.Tests {
		if len(t.Env) > 0 {
			e.TestEnv = make([][]int32, len(ix.Tests))
			for i, t := range ix.Tests {
				e.TestEnv[i] = t.Env
			}
			set = true
			break
		}
	}
	return e, set
}

// apply copies the extension into ix; a TestEnv of another length than the
// tests table, or an index out of the bindings table, is malformed.
func (e extension) apply(ix *Index) error {
	ix.Segments, ix.Divergent = e.Segments, e.Divergent
	ix.TestTimeout, ix.SuiteTimeout, ix.WindowFallback = e.TestTimeout, e.SuiteTimeout, e.WindowFallback
	ix.Language, ix.Command, ix.Executable, ix.RunnerFamily, ix.Config = e.Language, e.Command, e.Executable, e.RunnerFamily, e.Config
	ix.EnvBound, ix.Env = e.EnvBound, e.Env
	// EnvBound alone was written by the old audit, which omitted reads of
	// GOFLAGS and GOCOVERDIR. Admit a closed singleton of audit versions.
	ix.EnvBound = false
	if n := len(ix.Env); n > 0 && ix.Env[n-1].Name == envAuditMarker.Name {
		ix.EnvBound = e.EnvBound && ix.Env[n-1] == envAuditMarker
		ix.Env = ix.Env[:n-1]
	}
	for _, b := range ix.Env {
		if b.Name == envAuditMarker.Name {
			ix.EnvBound = false // duplicate or misplaced metadata
		}
	}
	if e.TestEnv == nil {
		return nil
	}
	if len(e.TestEnv) != len(ix.Tests) {
		return errors.New("linecov: malformed index: test environment table length")
	}
	for i, env := range e.TestEnv {
		prev := int32(-1)
		for _, id := range env {
			if id <= prev || int(id) >= len(ix.Env) {
				return errors.New("linecov: malformed index: test environment index")
			}
			prev = id
		}
		ix.Tests[i].Env = env
	}
	return nil
}

// encodeCompact writes ix as linecov-index/4.
func (ix *Index) encodeCompact(w io.Writer) error {
	var sections bytes.Buffer
	rows := make([]fileRow, len(ix.Files))
	var sec []byte
	for i, f := range ix.Files {
		sec = sec[:0]
		prev := int32(0)
		for _, b := range f.Blocks {
			if b.Start < prev || b.End < b.Start || b.StartCol < 0 || b.EndCol < 0 {
				return fmt.Errorf("linecov: encode: %s blocks out of order", f.Path)
			}
			sec = binary.AppendUvarint(sec, uint64(b.Start-prev))
			sec = binary.AppendUvarint(sec, uint64(b.StartCol))
			sec = binary.AppendUvarint(sec, uint64(b.End-b.Start))
			sec = binary.AppendUvarint(sec, uint64(b.EndCol))
			prev = b.Start
			var err error
			if sec, err = appendAscending(sec, b.Tests); err != nil {
				return fmt.Errorf("linecov: encode: %s: tests %w", f.Path, err)
			}
			if sec, err = appendAscending(sec, b.Segs); err != nil {
				return fmt.Errorf("linecov: encode: %s: segments %w", f.Path, err)
			}
		}
		rows[i] = fileRow{offset: int64(sections.Len()), length: int64(len(sec)), blocks: len(f.Blocks), sum: sha256.Sum256(sec)}
		sections.Write(sec)
	}
	var h []byte
	str := func(s string) {
		h = binary.AppendUvarint(h, uint64(len(s)))
		h = append(h, s...)
	}
	strs := func(list []string) {
		h = binary.AppendUvarint(h, uint64(len(list)))
		for _, s := range list {
			str(s)
		}
	}
	str(ix.Module)
	str(ix.Commit)
	str(ix.Runner)
	pkgs := map[string]int{}
	var pkgList []string
	for _, t := range ix.Tests {
		if _, ok := pkgs[t.Package]; !ok {
			pkgs[t.Package] = len(pkgList)
			pkgList = append(pkgList, t.Package)
		}
	}
	strs(pkgList)
	h = binary.AppendUvarint(h, uint64(len(ix.Tests)))
	for _, t := range ix.Tests {
		h = binary.AppendUvarint(h, uint64(pkgs[t.Package]))
		str(t.Name)
		h = binary.AppendVarint(h, int64(t.Elapsed))
		str(t.Level)
		flags := byte(0)
		if t.Incomplete {
			flags = 1
		}
		h = append(h, flags)
	}
	strs(ix.Unordered)
	strs(ix.InOrderRan)
	strs(ix.InOrderSkipped)
	h = binary.AppendUvarint(h, uint64(len(ix.Files)))
	for i, f := range ix.Files {
		str(f.Path)
		str(f.OID)
		h = binary.AppendUvarint(h, uint64(rows[i].blocks))
		h = binary.AppendUvarint(h, uint64(rows[i].offset))
		h = binary.AppendUvarint(h, uint64(rows[i].length))
		h = append(h, rows[i].sum[:]...)
	}
	var ext []byte
	if e, set := extensionOf(ix); set {
		var b bytes.Buffer
		if err := gob.NewEncoder(&b).Encode(e); err != nil {
			return err
		}
		ext = b.Bytes()
	}
	h = binary.AppendUvarint(h, uint64(len(ext)))
	h = append(h, ext...)
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(h)))
	sum := sha256.Sum256(h)
	for _, part := range [][]byte{[]byte(magicV4), lenBuf[:], h, sum[:], sections.Bytes()} {
		if _, err := w.Write(part); err != nil {
			return err
		}
	}
	return nil
}

// appendAscending appends len(ids) and the strictly ascending ids as deltas.
func appendAscending(dst []byte, ids []int32) ([]byte, error) {
	dst = binary.AppendUvarint(dst, uint64(len(ids)))
	prev := int32(-1)
	for _, id := range ids {
		if id <= prev {
			return nil, errors.New("not strictly ascending")
		}
		dst = binary.AppendUvarint(dst, uint64(id-prev-1))
		prev = id
	}
	return dst, nil
}

// reader decodes one byte slice and remembers the first error.
type reader struct {
	b   []byte
	err error
}

func (r *reader) fail(what string) {
	if r.err == nil {
		r.err = fmt.Errorf("linecov: malformed index: %s", what)
	}
}

func (r *reader) uvarint(what string) uint64 {
	if r.err != nil {
		return 0
	}
	v, n := binary.Uvarint(r.b)
	if n <= 0 {
		r.fail(what)
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *reader) varint(what string) int64 {
	if r.err != nil {
		return 0
	}
	v, n := binary.Varint(r.b)
	if n <= 0 {
		r.fail(what)
		return 0
	}
	r.b = r.b[n:]
	return v
}

// count reads a length that must fit what remains, at min bytes per element.
func (r *reader) count(what string, min int) int {
	v := r.uvarint(what)
	if r.err == nil && (min > 0 && v > uint64(len(r.b)/min) || v > 1<<31) {
		r.fail(what + " count")
		return 0
	}
	return int(v)
}

func (r *reader) bytes(n int, what string) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.b) {
		r.fail(what)
		return nil
	}
	out := r.b[:n]
	r.b = r.b[n:]
	return out
}

func (r *reader) str(what string) string {
	return string(r.bytes(r.count(what, 1), what))
}

func (r *reader) strs(what string) []string {
	n := r.count(what, 1)
	if n == 0 {
		return nil
	}
	out := make([]string, n)
	for i := range out {
		out[i] = r.str(what)
	}
	return out
}

// int32 reads a value that must fit in a non-negative int32.
func (r *reader) int32(what string) int32 {
	v := r.uvarint(what)
	if v > 1<<31-1 {
		r.fail(what)
		return 0
	}
	return int32(v)
}

// parseHeader decodes a linecov-index/4 header into an Index whose files
// carry no blocks yet, and returns its file-table rows.
func parseHeader(h []byte) (*Index, []fileRow, error) {
	r := &reader{b: h}
	ix := &Index{Format: Format}
	ix.Module = r.str("module")
	ix.Commit = r.str("commit")
	ix.Runner = r.str("runner")
	pkgs := r.strs("packages")
	nTests := r.count("tests", 4)
	if nTests > 0 {
		ix.Tests = make([]Test, nTests)
	}
	for i := range ix.Tests {
		p := r.uvarint("test package")
		if r.err == nil && p >= uint64(len(pkgs)) {
			r.fail("test package index")
		}
		t := &ix.Tests[i]
		if r.err == nil {
			t.Package = pkgs[p]
		}
		t.Name = r.str("test name")
		t.Elapsed = time.Duration(r.varint("test elapsed"))
		t.Level = r.str("test level")
		flags := r.bytes(1, "test flags")
		if r.err == nil {
			if flags[0]&^1 != 0 {
				r.fail("test flags")
			}
			t.Incomplete = flags[0]&1 != 0
		}
	}
	ix.Unordered = r.strs("unordered")
	ix.InOrderRan = r.strs("in-order ran")
	ix.InOrderSkipped = r.strs("in-order skipped")
	nFiles := r.count("files", sha256.Size)
	rows := make([]fileRow, nFiles)
	if nFiles > 0 {
		ix.Files = make([]File, nFiles)
	}
	for i := range ix.Files {
		ix.Files[i].Path = r.str("file path")
		ix.Files[i].OID = r.str("file oid")
		blocks := r.uvarint("block count")
		offset := r.uvarint("section offset")
		length := r.uvarint("section length")
		sum := r.bytes(sha256.Size, "section sum")
		if r.err != nil {
			break
		}
		if blocks > 1<<31 || offset > 1<<62 || length > 1<<62 {
			r.fail("section bounds")
			break
		}
		rows[i] = fileRow{offset: int64(offset), length: int64(length), blocks: int(blocks)}
		copy(rows[i].sum[:], sum)
	}
	ext := r.bytes(r.count("extension", 1), "extension")
	if r.err == nil && len(r.b) != 0 {
		r.fail("trailing header bytes")
	}
	if r.err != nil {
		return nil, nil, r.err
	}
	if len(ext) > 0 {
		var e extension
		if err := gob.NewDecoder(bytes.NewReader(ext)).Decode(&e); err != nil {
			return nil, nil, fmt.Errorf("linecov: malformed index extension: %w", err)
		}
		if err := e.apply(ix); err != nil {
			return nil, nil, err
		}
	}
	if ix.Module == "" || ix.Commit == "" || ix.Runner == "" {
		return nil, nil, errors.New("linecov: index identity incomplete")
	}
	for i := 1; i < len(ix.Files); i++ {
		if ix.Files[i-1].Path >= ix.Files[i].Path {
			return nil, nil, errors.New("linecov: index files out of order")
		}
	}
	return ix, rows, nil
}

// decodeSection decodes and verifies one file's block section: its SHA-256,
// block order, and test and segment indexes in range and ascending.
func decodeSection(sec []byte, row fileRow, nTests, nSegs int) ([]Block, error) {
	if sha256.Sum256(sec) != row.sum {
		return nil, errors.New("linecov: index section checksum mismatch")
	}
	r := &reader{b: sec}
	if row.blocks > len(sec)/6 {
		return nil, errors.New("linecov: index section block count")
	}
	blocks := make([]Block, row.blocks)
	prev := int32(0)
	ids := func(limit int, what string) []int32 {
		n := r.count(what, 1)
		if n == 0 || r.err != nil {
			return nil
		}
		out := make([]int32, n)
		next := 0 // the least id the next delta can name; next <= limit
		for i := range out {
			// Bound the delta as an unsigned number by the room below the
			// limit before converting it, so no delta wraps or overflows
			// into an id the range check would admit (V1-5163).
			d := r.uvarint(what)
			if r.err != nil {
				return nil
			}
			if d >= uint64(limit-next) {
				r.fail(what + " out of range")
				return nil
			}
			v := next + int(d)
			out[i], next = int32(v), v+1
		}
		return out
	}
	const maxLine = 1<<31 - 1
	for i := range blocks {
		b := &blocks[i]
		// Each delta is bounded, unsigned, by the room below maxLine before
		// it is added, so 0 <= Start <= End <= maxLine (V1-5163).
		ds := r.uvarint("block start")
		if r.err == nil && ds > uint64(maxLine-prev) {
			r.fail("block start out of range")
		}
		b.StartCol = r.int32("block start column")
		de := r.uvarint("block end")
		b.EndCol = r.int32("block end column")
		if r.err != nil {
			return nil, r.err
		}
		b.Start = prev + int32(ds)
		if de > uint64(maxLine-b.Start) {
			r.fail("block span")
			return nil, r.err
		}
		b.End = b.Start + int32(de)
		prev = b.Start
		b.Tests = ids(nTests, "block test")
		b.Segs = ids(nSegs, "block segment")
		if r.err != nil {
			return nil, r.err
		}
		if i > 0 && blockLess(*b, blocks[i-1]) {
			return nil, errors.New("linecov: index section blocks out of order")
		}
		if nSegs == 0 && b.Segs != nil {
			return nil, errors.New("linecov: block names a segment of an unsegmented index")
		}
	}
	if len(r.b) != 0 {
		return nil, errors.New("linecov: index section trailing bytes")
	}
	return blocks, nil
}

// blockLess orders blocks by Start, StartCol, End, EndCol.
func blockLess(a, b Block) bool {
	if a.Start != b.Start {
		return a.Start < b.Start
	}
	if a.StartCol != b.StartCol {
		return a.StartCol < b.StartCol
	}
	if a.End != b.End {
		return a.End < b.End
	}
	return a.EndCol < b.EndCol
}

// decodeCompact reads a whole linecov-index/4 file and verifies every section.
func decodeCompact(data []byte) (*Index, error) {
	h, base, err := splitHeader(data)
	if err != nil {
		return nil, err
	}
	ix, rows, err := parseHeader(h)
	if err != nil {
		return nil, err
	}
	sections := data[base:]
	for i, row := range rows {
		if row.offset+row.length > int64(len(sections)) {
			return nil, errors.New("linecov: index section outside the file")
		}
		blocks, err := decodeSection(sections[row.offset:row.offset+row.length], row, len(ix.Tests), len(ix.Segments))
		if err != nil {
			return nil, fmt.Errorf("%w (%s)", err, ix.Files[i].Path)
		}
		ix.Files[i].Blocks = blocks
	}
	if err := ix.checkSegments(); err != nil {
		return nil, err
	}
	ix.digest = headerDigest(h)
	ix.reindex()
	return ix, nil
}

// splitHeader checks the magic and the header's SHA-256, which follows it,
// and returns the header and the offset of the first section.
func splitHeader(data []byte) ([]byte, int64, error) {
	if !bytes.HasPrefix(data, []byte(magicV4)) {
		return nil, 0, errors.New("linecov: not a linecov-index/4 file")
	}
	rest := data[len(magicV4):]
	if len(rest) < 8 {
		return nil, 0, errors.New("linecov: truncated index header")
	}
	n := binary.LittleEndian.Uint64(rest)
	if n > maxHeader || n+sha256.Size > uint64(len(rest)-8) {
		return nil, 0, errors.New("linecov: index header length outside the file")
	}
	h := rest[8 : 8+n]
	if err := checkHeader(h, rest[8+n:8+n+sha256.Size]); err != nil {
		return nil, 0, err
	}
	return h, int64(len(magicV4)) + 8 + int64(n) + sha256.Size, nil
}

// checkHeader verifies the stored SHA-256 of a header.
func checkHeader(h, sum []byte) error {
	got := sha256.Sum256(h)
	if !bytes.Equal(got[:], sum) {
		return errors.New("linecov: index header checksum mismatch")
	}
	return nil
}

func headerDigest(h []byte) string {
	sum := sha256.Sum256(h)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Digest is the index identity a selection cites as its record digest
// (GLS-V0-001, GLS-V0-014): the SHA-256 of a linecov-index/4 header, which
// holds every section's SHA-256, or of the whole encoding of an older index.
func (ix *Index) Digest() string { return ix.digest }

// Open reads an index file for selection (GLS-V0-002). A linecov-index/4
// file without segments is read lazily: Open verifies the header and each
// file's block section is decoded and verified when Analyze first needs it.
// Any other index, including one with segments, is decoded whole. Close
// releases the file.
func Open(path string) (*Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		f.Close()
		if err == nil {
			err = fmt.Errorf("linecov: %s is not a regular file", path)
		}
		return nil, err
	}
	head := make([]byte, len(magicV4)+8)
	if _, err := io.ReadFull(f, head); err != nil || !bytes.HasPrefix(head, []byte(magicV4)) {
		// Not /4: decode the whole file as an older index.
		if _, serr := f.Seek(0, io.SeekStart); serr != nil {
			f.Close()
			return nil, serr
		}
		ix, derr := Decode(f)
		f.Close()
		return ix, derr
	}
	n := binary.LittleEndian.Uint64(head[len(magicV4):])
	if n > maxHeader || int64(n)+sha256.Size > st.Size()-int64(len(head)) {
		f.Close()
		return nil, errors.New("linecov: index header length outside the file")
	}
	buf := make([]byte, n+sha256.Size)
	if _, err := io.ReadFull(f, buf); err != nil {
		f.Close()
		return nil, fmt.Errorf("linecov: read index header: %w", err)
	}
	h := buf[:n]
	if err := checkHeader(h, buf[n:]); err != nil {
		f.Close()
		return nil, err
	}
	ix, rows, err := parseHeader(h)
	if err != nil {
		f.Close()
		return nil, err
	}
	base := int64(len(head)) + int64(n) + sha256.Size
	for _, row := range rows {
		if row.offset+row.length > st.Size()-base {
			f.Close()
			return nil, errors.New("linecov: index section outside the file")
		}
	}
	ix.digest = headerDigest(h)
	ix.lazy = &lazySource{r: f, closer: f, base: base, rows: rows, loaded: make([]bool, len(rows))}
	if len(ix.Segments) > 0 || len(ix.Divergent) > 0 {
		if err := ix.loadAll(); err != nil {
			f.Close()
			return nil, err
		}
		if err := ix.checkSegments(); err != nil {
			f.Close()
			return nil, err
		}
	}
	ix.reindex()
	return ix, nil
}

// Close releases a lazily read index's file; it is a no-op otherwise.
func (ix *Index) Close() error {
	if ix.lazy == nil || ix.lazy.closer == nil {
		return nil
	}
	c := ix.lazy.closer
	ix.lazy.closer = nil
	return c.Close()
}

// ensureFile decodes and verifies Files[i].Blocks of a lazily read index.
func (ix *Index) ensureFile(i int) error {
	l := ix.lazy
	if l == nil || l.loaded[i] {
		return nil
	}
	row := l.rows[i]
	sec := make([]byte, row.length)
	if _, err := l.r.ReadAt(sec, l.base+row.offset); err != nil {
		return fmt.Errorf("linecov: read index section of %s: %w", ix.Files[i].Path, err)
	}
	blocks, err := decodeSection(sec, row, len(ix.Tests), len(ix.Segments))
	if err != nil {
		return fmt.Errorf("%w (%s)", err, ix.Files[i].Path)
	}
	ix.Files[i].Blocks = blocks
	l.loaded[i] = true
	return nil
}

// loadAll decodes every section of a lazily read index.
func (ix *Index) loadAll() error {
	if ix.lazy == nil || ix.lazy.all {
		return nil
	}
	for i := range ix.Files {
		if err := ix.ensureFile(i); err != nil {
			return err
		}
	}
	ix.lazy.all = true
	return nil
}

// full makes a lazily read index whole for a method that walks every file;
// a section that fails verification there is a caller's misuse of Open, so
// it panics rather than answer from a partial index.
func (ix *Index) full() {
	if err := ix.loadAll(); err != nil {
		panic("linecov: a lazily opened index needs Decode for this method: " + err.Error())
	}
}
