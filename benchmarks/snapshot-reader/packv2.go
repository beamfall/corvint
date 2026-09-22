package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Pack v2 keeps pack v1's per-path, per-section blocks but replaces its
// O(paths) gob manifest with a fixed-size, self-checksummed binary manifest
// that is O(sections): the data file name, the key width, the height and
// root of a static checksummed tree over a fixed-width sorted path table,
// the vocabulary block, and one shared gob type descriptor per section.
//
// The path table is the tree's leaf level: rows of (zero-padded path, one
// locator per section), v2LeafRows rows per page. Each interior level holds
// (first key of child page, child page locator) rows, v2Fanout per page. A
// one-path lookup reads the manifest, the four section descriptors, one
// page per tree level (binary search inside each verified page, over
// io.ReaderAt), and the path's four blocks. Every range is checked against
// a locator from an already-verified parent before it is decoded
// (DNIP-IDX-005), and every length is bounded before it is allocated.
//
// Blocks carry no gob type descriptor: each section's gob.Encoder first
// encodes a fixed primer value, whose bytes (type descriptor included) are
// stored once as the section descriptor; every block is only the value
// message the same encoder wrote next. A reader replays the descriptor
// into a fresh decoder, discards the primer, then decodes the block.

const (
	v2Magic         = "ATLSPK2\x00"
	v2DataNameWidth = 64
	v2LocatorWidth  = 8 + 4 + sha256.Size
	v2LeafRows      = 32
	v2Fanout        = 64
	v2MaxHeight     = 8
	v2MaxKeyWidth   = 4096
	v2MaxBlockBytes = 16 << 20
	v2SectionCount  = 4
	v2ManifestSize  = len(v2Magic) + v2DataNameWidth + 2 + 2 + v2LocatorWidth*(2+v2SectionCount) + sha256.Size
)

// Section indexes, parallel to kindOrder.
const (
	v2SecFilesMeta = iota
	v2SecSymbols
	v2SecImports
	v2SecMarkers
)

// errPackV2Malformed reports a v2 manifest, page, or length that violates
// the format's fixed shape or bounds, detected before any allocation or
// decode it would otherwise drive.
var errPackV2Malformed = errors.New("snapshot-reader: malformed pack v2")

// errPathNotFound reports a one-path lookup for a path the pack does not hold.
var errPathNotFound = errors.New("snapshot-reader: path not found")

func manifestV2Path(dir string) string { return filepath.Join(dir, "manifest.v2") }

// v2Locator addresses one checksummed byte range of the data file.
type v2Locator struct {
	Offset uint64
	Length uint32
	SHA256 [32]byte
}

func (l v2Locator) appendTo(b []byte) []byte {
	b = binary.BigEndian.AppendUint64(b, l.Offset)
	b = binary.BigEndian.AppendUint32(b, l.Length)
	return append(b, l.SHA256[:]...)
}

func getV2Locator(b []byte) v2Locator {
	return v2Locator{
		Offset: binary.BigEndian.Uint64(b[0:8]),
		Length: binary.BigEndian.Uint32(b[8:12]),
		SHA256: [32]byte(b[12:v2LocatorWidth]),
	}
}

type packV2Manifest struct {
	DataFile    string
	KeyWidth    int
	Height      int
	Root        v2Locator
	Vocabulary  v2Locator
	Descriptors [v2SectionCount]v2Locator
}

func (m packV2Manifest) encode() []byte {
	b := make([]byte, 0, v2ManifestSize)
	b = append(b, v2Magic...)
	name := make([]byte, v2DataNameWidth)
	copy(name, m.DataFile)
	b = append(b, name...)
	b = binary.BigEndian.AppendUint16(b, uint16(m.KeyWidth))
	b = binary.BigEndian.AppendUint16(b, uint16(m.Height))
	b = m.Root.appendTo(b)
	b = m.Vocabulary.appendTo(b)
	for _, d := range m.Descriptors {
		b = d.appendTo(b)
	}
	sum := sha256.Sum256(b)
	return append(b, sum[:]...)
}

func malformedV2(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", errPackV2Malformed, fmt.Sprintf(format, args...))
}

func decodePackV2Manifest(b []byte) (packV2Manifest, error) {
	if len(b) != v2ManifestSize {
		return packV2Manifest{}, malformedV2("manifest is %d bytes, want %d", len(b), v2ManifestSize)
	}
	body := b[:len(b)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(b[len(body):]) {
		return packV2Manifest{}, fmt.Errorf("%w: manifest", errChecksumMismatch)
	}
	if string(body[:len(v2Magic)]) != v2Magic {
		return packV2Manifest{}, malformedV2("bad manifest magic")
	}
	pos := len(v2Magic)
	var m packV2Manifest
	m.DataFile = string(bytes.TrimRight(body[pos:pos+v2DataNameWidth], "\x00"))
	pos += v2DataNameWidth
	m.KeyWidth = int(binary.BigEndian.Uint16(body[pos:]))
	m.Height = int(binary.BigEndian.Uint16(body[pos+2:]))
	pos += 4
	m.Root = getV2Locator(body[pos:])
	m.Vocabulary = getV2Locator(body[pos+v2LocatorWidth:])
	pos += 2 * v2LocatorWidth
	for k := range m.Descriptors {
		m.Descriptors[k] = getV2Locator(body[pos+k*v2LocatorWidth:])
	}
	if m.DataFile == "" {
		return packV2Manifest{}, malformedV2("empty data file name")
	}
	if filepath.Base(m.DataFile) != m.DataFile {
		return packV2Manifest{}, malformedV2("data file name %q is not a base name", m.DataFile)
	}
	if m.KeyWidth < 1 {
		return packV2Manifest{}, malformedV2("key width %d below 1", m.KeyWidth)
	}
	if m.KeyWidth > v2MaxKeyWidth {
		return packV2Manifest{}, malformedV2("key width %d above bound", m.KeyWidth)
	}
	if m.Height < 1 {
		return packV2Manifest{}, malformedV2("height %d below 1", m.Height)
	}
	if m.Height > v2MaxHeight {
		return packV2Manifest{}, malformedV2("height %d above bound", m.Height)
	}
	return m, nil
}

func (m packV2Manifest) leafWidth() int     { return m.KeyWidth + v2SectionCount*v2LocatorWidth }
func (m packV2Manifest) interiorWidth() int { return m.KeyWidth + v2LocatorWidth }

// v2PrimerRecord supplies each section's primer value: the value whose
// encoding carries that section's shared gob type descriptor.
var v2PrimerRecord = Record{
	BlobHash: "x",
	Symbols:  []Symbol{{Name: "x", Line: 1}},
	Imports:  []string{"x"},
	Markers:  []Marker{{Line: 1, Text: "x"}},
}

// v2SectionValue returns the value section k stores for rec.
func v2SectionValue(k int, rec Record) interface{} {
	values := [v2SectionCount]interface{}{rec.BlobHash, rec.Symbols, rec.Imports, rec.Markers}
	return values[k]
}

type v2Writer struct {
	f      *os.File
	offset uint64
}

func (w *v2Writer) write(p []byte) (v2Locator, error) {
	if len(p) > v2MaxBlockBytes {
		return v2Locator{}, malformedV2("block of %d bytes exceeds bound", len(p))
	}
	if _, err := w.f.Write(p); err != nil {
		return v2Locator{}, err
	}
	loc := v2Locator{Offset: w.offset, Length: uint32(len(p)), SHA256: sha256.Sum256(p)}
	w.offset += uint64(len(p))
	return loc, nil
}

// writeLevel writes rows perPage at a time and returns one parent row
// (first key, page locator) per page written.
func (w *v2Writer) writeLevel(rows [][]byte, perPage, keyWidth int) ([][]byte, error) {
	parents := make([][]byte, 0, (len(rows)+perPage-1)/perPage)
	for start := 0; start < len(rows); start += perPage {
		end := min(start+perPage, len(rows))
		loc, err := w.write(bytes.Join(rows[start:end], nil))
		if err != nil {
			return nil, err
		}
		parent := append([]byte(nil), rows[start][:keyWidth]...)
		parents = append(parents, loc.appendTo(parent))
	}
	return parents, nil
}

// writeTree bulk-loads sorted leaf rows, then interior levels, until one
// root page remains; it returns the root locator and the tree height.
func (w *v2Writer) writeTree(rows [][]byte, keyWidth int) (v2Locator, int, error) {
	perPage := v2LeafRows
	for height := 1; ; height++ {
		parents, err := w.writeLevel(rows, perPage, keyWidth)
		if err != nil {
			return v2Locator{}, 0, err
		}
		if len(parents) == 1 {
			return getV2Locator(parents[0][keyWidth:]), height, nil
		}
		rows, perPage = parents, v2Fanout
	}
}

// v2SortedKeys returns c's paths sorted and the fixed key width they need.
func v2SortedKeys(c *Corpus) ([]string, int, error) {
	if len(c.Paths) == 0 {
		return nil, 0, malformedV2("empty corpus")
	}
	sorted := append([]string(nil), c.Paths...)
	sort.Strings(sorted)
	width := 0
	for _, p := range sorted {
		width = max(width, len(p))
	}
	if width > v2MaxKeyWidth {
		return nil, 0, malformedV2("path of %d bytes exceeds key bound", width)
	}
	for _, p := range sorted {
		if strings.IndexByte(p, 0) >= 0 {
			return nil, 0, malformedV2("path %q contains NUL", p)
		}
	}
	return sorted, width, nil
}

// BuildPackV2 writes pack v2 for c into dir. Like BuildPack, sections go to
// a freshly named data file and the manifest is renamed into place last
// (DNIP-IDX-008), so a failed republish leaves the old snapshot readable.
func BuildPackV2(dir string, c *Corpus) error {
	paths, keyWidth, err := v2SortedKeys(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "data-*.bin")
	if err != nil {
		return err
	}
	defer f.Close()
	w := &v2Writer{f: f}
	m := packV2Manifest{DataFile: filepath.Base(f.Name()), KeyWidth: keyWidth}

	rows := make([][]byte, len(paths))
	for i, p := range paths {
		rows[i] = append(make([]byte, 0, m.leafWidth()), p...)
		rows[i] = rows[i][:keyWidth]
	}
	for k := range kindOrder {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(v2SectionValue(k, v2PrimerRecord)); err != nil {
			return err
		}
		if m.Descriptors[k], err = w.write(buf.Bytes()); err != nil {
			return err
		}
		for i, p := range paths {
			buf.Reset()
			if err := enc.Encode(v2SectionValue(k, c.Files[p])); err != nil {
				return err
			}
			loc, err := w.write(buf.Bytes())
			if err != nil {
				return err
			}
			rows[i] = loc.appendTo(rows[i])
		}
	}

	vocabPayload, err := gobEncode(c.Vocabulary)
	if err != nil {
		return err
	}
	if m.Vocabulary, err = w.write(vocabPayload); err != nil {
		return err
	}
	if m.Root, m.Height, err = w.writeTree(rows, keyWidth); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	tmp := manifestV2Path(dir) + ".tmp"
	if err := os.WriteFile(tmp, m.encode(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, manifestV2Path(dir))
}

// PackV2Reader reads pack v2 through one counting io.ReaderAt per data file.
type PackV2Reader struct {
	manifest          packV2Manifest
	descriptors       [v2SectionCount][]byte
	ra                *CountingReaderAt
	file              *os.File
	manifestBytesRead int64
}

// OpenPackV2 opens the pack v2 rooted at dir, verifying the manifest and
// reading the four section descriptors. It returns errNoSnapshot when no
// manifest has been published.
func OpenPackV2(dir string) (*PackV2Reader, error) {
	mf, err := os.Open(manifestV2Path(dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, errNoSnapshot
	}
	if err != nil {
		return nil, err
	}
	defer mf.Close()
	manifestRA := &CountingReaderAt{R: mf}
	buf := make([]byte, v2ManifestSize+1) // one spare byte detects an over-long file
	n, err := manifestRA.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return nil, err
	}
	m, err := decodePackV2Manifest(buf[:n])
	if err != nil {
		return nil, err
	}
	f, err := os.Open(dataPath(dir, m.DataFile))
	if err != nil {
		return nil, err
	}
	r := &PackV2Reader{manifest: m, ra: &CountingReaderAt{R: f}, file: f, manifestBytesRead: manifestRA.BytesRead()}
	for k := range r.descriptors {
		if r.descriptors[k], err = r.readVerified(m.Descriptors[k], v2MaxBlockBytes); err != nil {
			f.Close()
			return nil, err
		}
	}
	return r, nil
}

// Close releases the data file handle.
func (r *PackV2Reader) Close() error { return r.file.Close() }

// BytesRead returns bytes read since Open, manifest included.
func (r *PackV2Reader) BytesRead() int64 { return r.manifestBytesRead + r.ra.BytesRead() }

// readVerified bounds loc's length, reads exactly that range, and verifies
// its checksum before returning it.
func (r *PackV2Reader) readVerified(loc v2Locator, maxLen int) ([]byte, error) {
	if int64(loc.Length) > int64(maxLen) {
		return nil, malformedV2("range of %d bytes exceeds bound %d", loc.Length, maxLen)
	}
	buf := make([]byte, loc.Length)
	n, err := r.ra.ReadAt(buf, int64(loc.Offset))
	if n < len(buf) {
		return nil, fmt.Errorf("snapshot-reader: short read at offset %d: %w", loc.Offset, errors.Join(io.ErrUnexpectedEOF, err))
	}
	if sha256.Sum256(buf) != loc.SHA256 {
		return nil, fmt.Errorf("%w: offset %d length %d", errChecksumMismatch, loc.Offset, loc.Length)
	}
	return buf, nil
}

func (r *PackV2Reader) readPage(loc v2Locator, rowWidth, maxRows int) ([]byte, error) {
	page, err := r.readVerified(loc, rowWidth*maxRows)
	if err != nil {
		return nil, err
	}
	if len(page) == 0 {
		return nil, malformedV2("empty page at offset %d", loc.Offset)
	}
	if len(page)%rowWidth != 0 {
		return nil, malformedV2("page of %d bytes is not whole rows of %d", len(page), rowWidth)
	}
	return page, nil
}

// floorRow returns the index of the last row whose key is <= key, or -1.
func floorRow(page []byte, rowWidth int, key []byte) int {
	rows := len(page) / rowWidth
	return sort.Search(rows, func(i int) bool {
		return bytes.Compare(page[i*rowWidth:i*rowWidth+len(key)], key) > 0
	}) - 1
}

// lookup descends the tree to target's leaf row; ok is false when absent.
func (r *PackV2Reader) lookup(target string) (row []byte, ok bool, err error) {
	kw := r.manifest.KeyWidth
	if len(target) > kw {
		return nil, false, nil
	}
	if strings.IndexByte(target, 0) >= 0 {
		return nil, false, nil
	}
	key := make([]byte, kw)
	copy(key, target)
	loc := r.manifest.Root
	iw := r.manifest.interiorWidth()
	for level := r.manifest.Height; level > 1; level-- {
		page, err := r.readPage(loc, iw, v2Fanout)
		if err != nil {
			return nil, false, err
		}
		i := floorRow(page, iw, key)
		if i < 0 {
			return nil, false, nil
		}
		loc = getV2Locator(page[i*iw+kw:])
	}
	lw := r.manifest.leafWidth()
	page, err := r.readPage(loc, lw, v2LeafRows)
	if err != nil {
		return nil, false, err
	}
	i := floorRow(page, lw, key)
	if i < 0 {
		return nil, false, nil
	}
	row = page[i*lw : (i+1)*lw]
	return row, bytes.Equal(row[:kw], key), nil
}

// walk calls fn for every leaf row under loc, in key order.
func (r *PackV2Reader) walk(loc v2Locator, level int, fn func(path string, row []byte) error) error {
	kw := r.manifest.KeyWidth
	if level == 1 {
		lw := r.manifest.leafWidth()
		page, err := r.readPage(loc, lw, v2LeafRows)
		if err != nil {
			return err
		}
		for i := 0; i < len(page)/lw; i++ {
			row := page[i*lw : (i+1)*lw]
			if err := fn(string(bytes.TrimRight(row[:kw], "\x00")), row); err != nil {
				return err
			}
		}
		return nil
	}
	iw := r.manifest.interiorWidth()
	page, err := r.readPage(loc, iw, v2Fanout)
	if err != nil {
		return err
	}
	for i := 0; i < len(page)/iw; i++ {
		if err := r.walk(getV2Locator(page[i*iw+kw:]), level-1, fn); err != nil {
			return err
		}
	}
	return nil
}

// decodeV2Block decodes block after replaying its section descriptor.
func decodeV2Block[T any](descriptor, block []byte, out *T) error {
	dec := gob.NewDecoder(io.MultiReader(bytes.NewReader(descriptor), bytes.NewReader(block)))
	var primer T
	if err := dec.Decode(&primer); err != nil {
		return err
	}
	return dec.Decode(out)
}

func readV2Section[T any](r *PackV2Reader, row []byte, k int, out *T) error {
	loc := getV2Locator(row[r.manifest.KeyWidth+k*v2LocatorWidth:])
	block, err := r.readVerified(loc, v2MaxBlockBytes)
	if err != nil {
		return err
	}
	return decodeV2Block(r.descriptors[k], block, out)
}

func (r *PackV2Reader) record(path string, row []byte) (Record, error) {
	rec := Record{Path: path}
	if err := readV2Section(r, row, v2SecFilesMeta, &rec.BlobHash); err != nil {
		return Record{}, err
	}
	if err := readV2Section(r, row, v2SecSymbols, &rec.Symbols); err != nil {
		return Record{}, err
	}
	if err := readV2Section(r, row, v2SecImports, &rec.Imports); err != nil {
		return Record{}, err
	}
	if err := readV2Section(r, row, v2SecMarkers, &rec.Markers); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// ReadWhole decodes every block in the pack.
func (r *PackV2Reader) ReadWhole() (*Corpus, error) {
	c := &Corpus{Files: map[string]Record{}}
	err := r.walk(r.manifest.Root, r.manifest.Height, func(path string, row []byte) error {
		rec, err := r.record(path, row)
		c.Files[path] = rec
		c.Paths = append(c.Paths, path)
		return err
	})
	if err != nil {
		return nil, err
	}
	vocabBytes, err := r.readVerified(r.manifest.Vocabulary, v2MaxBlockBytes)
	if err != nil {
		return nil, err
	}
	if err := gob.NewDecoder(bytes.NewReader(vocabBytes)).Decode(&c.Vocabulary); err != nil {
		return nil, err
	}
	return c, nil
}

// ReadOneTable decodes only the imports section, across every path.
func (r *PackV2Reader) ReadOneTable() (map[string][]string, error) {
	out := map[string][]string{}
	err := r.walk(r.manifest.Root, r.manifest.Height, func(path string, row []byte) error {
		var imps []string
		err := readV2Section(r, row, v2SecImports, &imps)
		out[path] = imps
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReadOnePath decodes only target's blocks, found by tree descent.
func (r *PackV2Reader) ReadOnePath(target string) (Record, error) {
	row, ok, err := r.lookup(target)
	if err != nil {
		return Record{}, err
	}
	if !ok {
		return Record{}, fmt.Errorf("%w: %q", errPathNotFound, target)
	}
	return r.record(target, row)
}
