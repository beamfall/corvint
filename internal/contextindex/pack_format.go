package contextindex

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"hash"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// The Corvint pack (proposed IDX-SNAP-V0-015, experimental) is the snapshot of
// one committed tree laid out so that a reader maps the file once and views
// its tables in place instead of decoding them into the heap. It is written
// beside the gob snapshot only when CORVINT_SNAPSHOT_FORMAT is "pack"; the gob
// file stays the accepted format and every refused or absent pack falls back
// to it, then to the build (IDX-SNAP-V0-003).
//
// Layout: a fixed header slot holding the magic, a gob-encoded section table
// (name, data offset, length, the offset of the section's 64 KiB block digest
// table, and the sha256 of that table), and the table's own sha256; then the
// sections, each starting on a 64-byte boundary; then the block digest tables.
// A block is verified against its digest before any byte of it is decoded. The
// fixed-width tables are little-endian u32/u64 arrays behind small counts, so
// a little-endian reader aliases them through unsafe.Slice and a big-endian
// one copies; every string lives in one string table and is aliased from the
// mapping. The small tables (exclusions, features, scenarios, documents,
// markers, imports, unparsed, notes) are gob values inside verified sections,
// as in the sectioned prototype.
const (
	packMagic       = "corvint-index-pack/1"
	packExtension   = ".aip"
	packFormatValue = "pack"
	packHeaderSlot  = 16384
	packBlockSize   = 64 * 1024
	packAlignment   = 64

	packSectionIdentity  = "identity"
	packSectionStrings   = "strings"
	packSectionSources   = "sources"
	packSectionBodies    = "bodies"
	packSectionTracked   = "tracked"
	packSectionSkipped   = "skipped"
	packSectionSymbols   = "symbols"
	packSectionPaths     = "vocab.paths"
	packSectionTerms     = "vocab.terms"
	packSectionWords     = "vocab.words"
	packSectionPathTerms = "vocab.pathterms"
	packSectionWindows   = "vocab.symbolwindows"
	packSectionCochange  = "cochange"
	packSectionGraph     = "vocab.identgraph"
)

// packFullSections is every section; a deferred load names the same sections
// as its tables and defers only the verification of bodies.
var packFullSections = []string{packSectionIdentity, packSectionStrings, packSectionSources, packSectionBodies,
	sectionExclusions, sectionFeatures, sectionScenarios, sectionDocuments, sectionMarkers, packSectionSymbols,
	sectionImports, sectionUnparsed, sectionNotes, packSectionTracked, packSectionSkipped, packSectionPaths,
	packSectionTerms, packSectionWords, packSectionPathTerms, packSectionWindows, packSectionCochange, packSectionGraph}

var packSectionsByLoad = map[snapshotLoad][]string{
	loadCompact: {packSectionIdentity},
	loadEvent: {packSectionIdentity, packSectionStrings, packSectionSources, packSectionBodies, sectionExclusions,
		sectionFeatures, sectionScenarios, sectionDocuments, sectionMarkers, packSectionSymbols, sectionImports,
		sectionUnparsed, sectionNotes},
	loadFull: packFullSections,
}

func packEnabled() bool { return runtimeenv.Value("SNAPSHOT_FORMAT") == packFormatValue }

func packPath(root, objectFormat, tree, engineID string) string {
	return strings.TrimSuffix(snapshotPath(root, objectFormat, tree, engineID), ".gob") + packExtension
}

type packSectionEntry struct {
	Name           string
	Offset, Length uint64
	DigestsOffset  uint64
	DigestsSHA256  [sha256.Size]byte
}

// packHeader is the section table. HistoryCommit names the commit whose
// recent history the cochange section holds, or is empty when the pack
// carries none; a reader whose HEAD commit differs uses the section's
// spawn fallback and nothing else changes.
type packHeader struct {
	Header        snapshotHeader
	HistoryCommit string
	Sections      []packSectionEntry
}

type packIdentity struct {
	ObjectFormat, CommitRevision, Revision, ProfileID, Module string
	ApproximateImports, UnsupportedSuffixCount                int
	Vocabulary                                                bool
}

// packStrings interns every string the fixed-width tables reference. The
// section is [u32 count][u32 offsets, count+1][bytes].
type packStrings struct {
	ids    map[string]uint32
	values []string
	size   uint64
}

func newPackStrings() *packStrings { return &packStrings{ids: map[string]uint32{}} }

func (table *packStrings) id(value string) uint32 {
	if id, ok := table.ids[value]; ok {
		return id
	}
	id := uint32(len(table.values))
	table.ids[value] = id
	table.values = append(table.values, value)
	table.size += uint64(len(value))
	return id
}

func (table *packStrings) write(w io.Writer) error {
	header := make([]byte, 0, 4*(len(table.values)+2))
	header = binary.LittleEndian.AppendUint32(header, uint32(len(table.values)))
	offset := uint32(0)
	for _, value := range table.values {
		header = binary.LittleEndian.AppendUint32(header, offset)
		offset += uint32(len(value))
	}
	header = binary.LittleEndian.AppendUint32(header, offset)
	if _, err := w.Write(header); err != nil {
		return err
	}
	for _, value := range table.values {
		if _, err := io.WriteString(w, value); err != nil {
			return err
		}
	}
	return nil
}

// u32Section writes one or more u32 arrays behind a u32 count header.
func u32Section(name string, counts []uint32, arrays ...[]uint32) sectionWriter {
	return sectionWriter{name, func(w io.Writer) error {
		size := 4 * len(counts)
		for _, array := range arrays {
			size += 4 * len(array)
		}
		data := make([]byte, 0, size)
		for _, count := range counts {
			data = binary.LittleEndian.AppendUint32(data, count)
		}
		for _, array := range arrays {
			for _, value := range array {
				data = binary.LittleEndian.AppendUint32(data, value)
			}
		}
		_, err := w.Write(data)
		return err
	}}
}

func stringIDs(table *packStrings, values []string) []uint32 {
	ids := make([]uint32, len(values))
	for index, value := range values {
		ids[index] = table.id(value)
	}
	return ids
}

// idListSection is [u32 n][u32 present][u32 string id x n]. present is 0
// for a nil map and 1 otherwise: gob sends an empty non-nil map and omits a
// nil one, so the reader must give back the same shape.
func idListSection(name string, set map[string]struct{}, table *packStrings) sectionWriter {
	present := uint32(0)
	if set != nil {
		present = 1
	}
	return u32Section(name, []uint32{uint32(len(set)), present}, stringIDs(table, packSortedKeys(set)))
}

func packSortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sourcesSection is [u32 n][u32 pad][u64 body offset x n][u32 path, blob,
// mode, flags, body length x n each]; flags bit 0 is Checked, bit 1 Valid.
func sourcesSection(metas []sourceMeta, table *packStrings) sectionWriter {
	return sectionWriter{packSectionSources, func(w io.Writer) error {
		n := len(metas)
		data := make([]byte, 0, 8+8*n+20*n)
		data = binary.LittleEndian.AppendUint32(data, uint32(n))
		data = binary.LittleEndian.AppendUint32(data, 0)
		for _, meta := range metas {
			data = binary.LittleEndian.AppendUint64(data, meta.Offset)
		}
		columns := [5]func(sourceMeta) uint32{
			func(meta sourceMeta) uint32 { return table.id(meta.Path) },
			func(meta sourceMeta) uint32 { return table.id(meta.BlobHash) },
			func(meta sourceMeta) uint32 { return table.id(meta.Mode) },
			func(meta sourceMeta) uint32 { return sourceFlags(meta) },
			func(meta sourceMeta) uint32 { return uint32(meta.Length) },
		}
		for _, column := range columns {
			for _, meta := range metas {
				data = binary.LittleEndian.AppendUint32(data, column(meta))
			}
		}
		_, err := w.Write(data)
		return err
	}}
}

func sourceFlags(meta sourceMeta) uint32 {
	flags := uint32(0)
	if meta.Checked {
		flags |= 1
	}
	if meta.Valid {
		flags |= 2
	}
	return flags
}

// symbolsSection is [u32 n][u32 kind, name, path, blob, line, endline x n each].
func symbolsSection(symbols []Symbol, table *packStrings) sectionWriter {
	n := len(symbols)
	columns := make([][]uint32, 6)
	for column := range columns {
		columns[column] = make([]uint32, n)
	}
	for index, symbol := range symbols {
		columns[0][index] = table.id(symbol.Kind)
		columns[1][index] = table.id(symbol.Name)
		columns[2][index] = table.id(symbol.Path)
		columns[3][index] = table.id(symbol.BlobHash)
		columns[4][index] = uint32(symbol.Line)
		columns[5][index] = uint32(symbol.EndLine)
	}
	return u32Section(packSectionSymbols, []uint32{uint32(n)}, columns...)
}

func packIdentityOf(index *Index) packIdentity {
	return packIdentity{
		ObjectFormat: index.ObjectFormat, CommitRevision: index.CommitRevision, Revision: index.Revision,
		ProfileID: index.ProfileID, Module: index.Module, ApproximateImports: index.ApproximateImports,
		UnsupportedSuffixCount: index.UnsupportedSuffixCount,
		Vocabulary:             index.Vocabulary != nil,
	}
}

// packSectionWriters lists every section in file order. The string table is
// written last because the tables before it intern their strings as they
// are encoded; the reader locates sections by name, not position.
func packSectionWriters(index *Index, history []historyEntry) []sectionWriter {
	table := newPackStrings()
	metas := sourceMetas(index)
	vocabulary := index.Vocabulary
	if vocabulary == nil {
		vocabulary = &TermTable{}
	}
	bodies := sectionWriter{packSectionBodies, func(w io.Writer) error {
		for _, meta := range metas {
			if _, err := w.Write(index.Sources[meta.Path].Data); err != nil {
				return err
			}
		}
		return nil
	}}
	writers := []sectionWriter{
		gobSection(packSectionIdentity, packIdentityOf(index)),
		sourcesSection(metas, table),
		bodies,
		gobSection(sectionExclusions, index.Exclusions),
		gobSection(sectionFeatures, index.Features),
		gobSection(sectionScenarios, index.Scenarios),
		gobSection(sectionDocuments, index.Documents),
		gobSection(sectionMarkers, index.Markers),
		symbolsSection(index.Symbols, table),
		gobSection(sectionImports, index.Imports),
		gobSection(sectionUnparsed, index.Unparsed),
		gobSection(sectionNotes, index.ExtractionNotes),
		idListSection(packSectionTracked, index.Tracked, table),
		idListSection(packSectionSkipped, index.Skipped, table),
		u32Section(packSectionPaths, []uint32{uint32(len(vocabulary.Paths)), 1}, stringIDs(table, vocabulary.Paths)),
		termTableSection(packSectionTerms, vocabulary.Terms),
		termTableSection(packSectionWords, vocabulary.Words),
		termTableSection(packSectionPathTerms, vocabulary.PathTerms),
		gobSection(packSectionWindows, vocabulary.SymbolWindows),
		cochangeSection(history, table),
		gobSection(packSectionGraph, vocabulary.IdentGraph),
	}
	return append(writers, sectionWriter{packSectionStrings, table.write})
}

// blockDigester hashes what passes through it in packBlockSize blocks and
// keeps one sha256 per block, the last block possibly short.
type blockDigester struct {
	w       io.Writer
	n       uint64
	filled  int
	block   hash.Hash
	digests []byte
}

func newBlockDigester(w io.Writer) *blockDigester {
	return &blockDigester{w: w, block: sha256.New()}
}

func (d *blockDigester) Write(p []byte) (int, error) {
	written, err := d.w.Write(p)
	d.n += uint64(written)
	for rest := p[:written]; len(rest) > 0; {
		take := min(packBlockSize-d.filled, len(rest))
		d.block.Write(rest[:take])
		d.filled += take
		rest = rest[take:]
		if d.filled == packBlockSize {
			d.seal()
		}
	}
	return written, err
}

func (d *blockDigester) seal() {
	d.digests = d.block.Sum(d.digests)
	d.block.Reset()
	d.filled = 0
}

// finish returns the block digest table, sealing a trailing short block.
func (d *blockDigester) finish() []byte {
	if d.filled > 0 {
		d.seal()
	}
	return d.digests
}

func packBlockCount(length uint64) uint64 {
	return (length + packBlockSize - 1) / packBlockSize
}

func padTo(w io.Writer, offset uint64, alignment uint64) (uint64, error) {
	pad := (alignment - offset%alignment) % alignment
	if pad == 0 {
		return offset, nil
	}
	if _, err := w.Write(make([]byte, pad)); err != nil {
		return 0, err
	}
	return offset + pad, nil
}

// encodePackSnapshot writes index and its co-change history in the pack
// layout and returns the file size. Sections stream through a block
// digester; the digest tables follow the sections and the header is written
// into its slot last, so a reader never sees a table that names bytes not
// yet written.
func encodePackSnapshot(file *os.File, index *Index, engineID string, history []historyEntry, historyCommit string) (int64, error) {
	if _, err := file.Seek(packHeaderSlot, io.SeekStart); err != nil {
		return 0, err
	}
	buffered := bufio.NewWriterSize(file, 1<<20)
	offset := uint64(packHeaderSlot)
	entries := make([]packSectionEntry, 0, 24)
	digestTables := make([][]byte, 0, 24)
	for _, section := range packSectionWriters(index, history) {
		digester := newBlockDigester(buffered)
		if err := section.write(digester); err != nil {
			return 0, err
		}
		entries = append(entries, packSectionEntry{Name: section.name, Offset: offset, Length: digester.n})
		digestTables = append(digestTables, digester.finish())
		next, err := padTo(buffered, offset+digester.n, packAlignment)
		if err != nil {
			return 0, err
		}
		offset = next
	}
	for position, table := range digestTables {
		entries[position].DigestsOffset = offset
		entries[position].DigestsSHA256 = sha256.Sum256(table)
		if _, err := buffered.Write(table); err != nil {
			return 0, err
		}
		offset += uint64(len(table))
	}
	if err := buffered.Flush(); err != nil {
		return 0, err
	}
	header, err := encodePackHeader(packHeader{
		Header:        snapshotHeader{Format: snapshotFormat, ObjectFormat: index.ObjectFormat, Tree: index.Revision, Engine: engineID},
		HistoryCommit: historyCommit,
		Sections:      entries,
	})
	if err != nil {
		return 0, err
	}
	if _, err := file.WriteAt(header, 0); err != nil {
		return 0, err
	}
	return int64(offset), nil
}

// The header slot is: magic, little-endian uint32 table length, the gob
// table, and the table's sha256.
func encodePackHeader(header packHeader) ([]byte, error) {
	var table bytes.Buffer
	if err := gob.NewEncoder(&table).Encode(&header); err != nil {
		return nil, err
	}
	encoded := make([]byte, 0, packHeaderSlot)
	encoded = append(encoded, packMagic...)
	encoded = binary.LittleEndian.AppendUint32(encoded, uint32(table.Len()))
	encoded = append(encoded, table.Bytes()...)
	digest := sha256.Sum256(table.Bytes())
	encoded = append(encoded, digest[:]...)
	if len(encoded) > packHeaderSlot {
		return nil, errors.New("pack header exceeds its slot")
	}
	return encoded, nil
}
