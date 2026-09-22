package contextindex

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// The sectioned snapshot (proposed IDX-SNAP-V0-014, experimental) is the same
// Index as the gob snapshot laid out so that a reader decodes only the tables
// it needs. It is written beside the gob file only when CORVINT_SNAPSHOT_FORMAT
// is "sectioned"; the gob file stays the accepted format and every reader
// falls back to it. Layout: a fixed 4 KiB header slot holding the magic, a
// gob-encoded section table (name, offset, length, sha256 of the bytes), and
// the table's own sha256; then the sections, each independently decodable.
// Every table except the source bodies is one gob value wrapped in a
// single-field struct so gob's zero-value omission matches the whole-Index
// encoding exactly. The bodies are the raw source bytes concatenated in path
// order and addressed by offset from the sources table, so a loaded Source
// aliases the mapped file and a body is paged in only where a verb reads it.
const (
	sectionedMagic      = "corvint-index-snapshot-sectioned/1"
	sectionedExtension  = ".sect"
	sectionedHeaderSlot = 4096
	snapshotFormatEnv   = "CORVINT_SNAPSHOT_FORMAT"
	snapshotFormatValue = "sectioned"

	sectionIdentity   = "identity"
	sectionSources    = "sources"
	sectionBodies     = "bodies"
	sectionExclusions = "exclusions"
	sectionFeatures   = "features"
	sectionScenarios  = "scenarios"
	sectionDocuments  = "documents"
	sectionMarkers    = "markers"
	sectionSymbols    = "symbols"
	sectionImports    = "imports"
	sectionUnparsed   = "unparsed"
	sectionNotes      = "notes"
	sectionTracked    = "tracked"
	sectionSkipped    = "skipped"
	sectionVocabulary = "vocabulary"
)

// snapshotLoad names how much of the snapshot a verb reads.
type snapshotLoad int

const (
	loadFull snapshotLoad = iota
	loadEvent
	loadCompact
)

// loadDeferredBodies marks a load whose pack bodies are verified on first read
// (packBody) instead of at open; every other format ignores it.
const loadDeferredBodies snapshotLoad = 1 << 4

const (
	loadContext       = loadFull | loadDeferredBodies
	loadEventDeferred = loadEvent | loadDeferredBodies
)

// tables is the load without its body-verification mark.
func (load snapshotLoad) tables() snapshotLoad { return load &^ loadDeferredBodies }

func (load snapshotLoad) deferredBodies() bool { return load&loadDeferredBodies != 0 }

var sectionsByLoad = map[snapshotLoad][]string{
	loadCompact: {sectionIdentity},
	loadEvent: {sectionIdentity, sectionSources, sectionBodies, sectionExclusions, sectionFeatures, sectionScenarios,
		sectionDocuments, sectionMarkers, sectionSymbols, sectionImports, sectionUnparsed, sectionNotes},
	loadFull: {sectionIdentity, sectionSources, sectionBodies, sectionExclusions, sectionFeatures, sectionScenarios,
		sectionDocuments, sectionMarkers, sectionSymbols, sectionImports, sectionUnparsed, sectionNotes,
		sectionTracked, sectionSkipped, sectionVocabulary},
}

func sectionedEnabled() bool { return runtimeenv.Value("SNAPSHOT_FORMAT") == snapshotFormatValue }

func sectionedPath(root, objectFormat, tree, engineID string) string {
	return strings.TrimSuffix(snapshotPath(root, objectFormat, tree, engineID), ".gob") + sectionedExtension
}

type sectionEntry struct {
	Name           string
	Offset, Length uint64
	SHA256         [sha256.Size]byte
}

type sectionedHeader struct {
	Header   snapshotHeader
	Sections []sectionEntry
}

type sectionIdentityValue struct {
	ObjectFormat, CommitRevision, Revision, ProfileID, Module string
	ApproximateImports, UnsupportedSuffixCount                int
}

// sourceMeta is one Sources entry without its body; Offset and Length
// address the body inside the bodies section.
type sourceMeta struct {
	Path, BlobHash, Mode string
	Checked, Valid       bool
	Offset, Length       uint64
}

type sectionWriter struct {
	name  string
	write func(io.Writer) error
}

func gobSection[T any](name string, value T) sectionWriter {
	return sectionWriter{name, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(&struct{ V T }{value})
	}}
}

func sourceMetas(index *Index) []sourceMeta {
	paths := make([]string, 0, len(index.Sources))
	for path := range index.Sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	metas := make([]sourceMeta, 0, len(paths))
	offset := uint64(0)
	for _, path := range paths {
		source := index.Sources[path]
		metas = append(metas, sourceMeta{
			Path: source.Path, BlobHash: source.BlobHash, Mode: source.Mode,
			Checked: source.Checked, Valid: source.Valid, Offset: offset, Length: uint64(len(source.Data)),
		})
		offset += uint64(len(source.Data))
	}
	return metas
}

func sectionWriters(index *Index, metas []sourceMeta) []sectionWriter {
	bodies := sectionWriter{sectionBodies, func(w io.Writer) error {
		for _, meta := range metas {
			if _, err := w.Write(index.Sources[meta.Path].Data); err != nil {
				return err
			}
		}
		return nil
	}}
	return []sectionWriter{
		gobSection(sectionIdentity, sectionIdentityValue{
			ObjectFormat: index.ObjectFormat, CommitRevision: index.CommitRevision, Revision: index.Revision,
			ProfileID: index.ProfileID, Module: index.Module, ApproximateImports: index.ApproximateImports,
			UnsupportedSuffixCount: index.UnsupportedSuffixCount,
		}),
		gobSection(sectionSources, metas),
		bodies,
		gobSection(sectionExclusions, index.Exclusions),
		gobSection(sectionFeatures, index.Features),
		gobSection(sectionScenarios, index.Scenarios),
		gobSection(sectionDocuments, index.Documents),
		gobSection(sectionMarkers, index.Markers),
		gobSection(sectionSymbols, index.Symbols),
		gobSection(sectionImports, index.Imports),
		gobSection(sectionUnparsed, index.Unparsed),
		gobSection(sectionNotes, index.ExtractionNotes),
		gobSection(sectionTracked, index.Tracked),
		gobSection(sectionSkipped, index.Skipped),
		gobSection(sectionVocabulary, index.Vocabulary),
	}
}

type countingWriter struct {
	w io.Writer
	n uint64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += uint64(n)
	return n, err
}

// encodeSectionedSnapshot writes index in the sectioned layout and returns
// the file size. Sections stream through one digest each; the table is
// written into the reserved slot last.
func encodeSectionedSnapshot(file *os.File, index *Index, engineID string) (int64, error) {
	if _, err := file.Seek(sectionedHeaderSlot, io.SeekStart); err != nil {
		return 0, err
	}
	buffered := bufio.NewWriterSize(file, 1<<20)
	offset := uint64(sectionedHeaderSlot)
	entries := make([]sectionEntry, 0, 16)
	for _, section := range sectionWriters(index, sourceMetas(index)) {
		digest := sha256.New()
		counter := &countingWriter{w: io.MultiWriter(buffered, digest)}
		if err := section.write(counter); err != nil {
			return 0, err
		}
		entry := sectionEntry{Name: section.name, Offset: offset, Length: counter.n}
		copy(entry.SHA256[:], digest.Sum(nil))
		entries = append(entries, entry)
		offset += counter.n
	}
	if err := buffered.Flush(); err != nil {
		return 0, err
	}
	header, err := encodeSectionedHeader(sectionedHeader{
		Header:   snapshotHeader{Format: snapshotFormat, ObjectFormat: index.ObjectFormat, Tree: index.Revision, Engine: engineID},
		Sections: entries,
	})
	if err != nil {
		return 0, err
	}
	if _, err := file.WriteAt(header, 0); err != nil {
		return 0, err
	}
	return int64(offset), nil
}

// The header slot is: magic, big-endian uint32 table length, the gob table,
// and the table's sha256.
func encodeSectionedHeader(header sectionedHeader) ([]byte, error) {
	var table bytes.Buffer
	if err := gob.NewEncoder(&table).Encode(&header); err != nil {
		return nil, err
	}
	encoded := make([]byte, 0, sectionedHeaderSlot)
	encoded = append(encoded, sectionedMagic...)
	encoded = binary.BigEndian.AppendUint32(encoded, uint32(table.Len()))
	encoded = append(encoded, table.Bytes()...)
	digest := sha256.Sum256(table.Bytes())
	encoded = append(encoded, digest[:]...)
	if len(encoded) > sectionedHeaderSlot {
		return nil, errors.New("sectioned snapshot header exceeds its slot")
	}
	return encoded, nil
}

// sectionedFile serves sections from a ReaderAt, or from a read-only mapping
// of the file when the platform provides one, so a body is read only where
// it is touched.
type sectionedFile struct {
	at      io.ReaderAt
	size    int64
	mapping []byte
	close   func()
	key     packKey
}

func openSectionedFile(path string) (*sectionedFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	mapping := mapReadOnly(file, info.Size())
	key := packKey{path: path, size: info.Size(), modified: info.ModTime().UnixNano()}
	return &sectionedFile{at: file, size: info.Size(), mapping: mapping, close: func() { file.Close() }, key: key}, nil
}

// release drops the mapping after a refused read. A successful read keeps
// it: the returned Sources alias the mapped bodies for the process's life,
// so sectionedCache retains it and later reads of the same file reuse it.
func (f *sectionedFile) release() { unmapReadOnly(f.mapping) }

// withinBounds reports whether [offset, offset+length) lies inside size
// without computing offset+length, which a corrupt table can make wrap.
func withinBounds(offset, length, size uint64) bool {
	if offset > size {
		return false
	}
	return length <= size-offset
}

func (f *sectionedFile) bytesAt(offset, length uint64) ([]byte, error) {
	if !withinBounds(offset, length, uint64(f.size)) {
		return nil, errors.New("sectioned snapshot section lies outside the file")
	}
	if f.mapping != nil {
		return f.mapping[offset : offset+length : offset+length], nil
	}
	buffer := make([]byte, length)
	if _, err := f.at.ReadAt(buffer, int64(offset)); err != nil {
		return nil, err
	}
	return buffer, nil
}

// headerTable returns the section table bytes and the digest the file stores
// over them. That digest covers every section's sha256, so two files carrying
// it hold the same verified content.
func (f *sectionedFile) headerTable() ([]byte, [sha256.Size]byte, error) {
	var none [sha256.Size]byte
	prefix, err := f.bytesAt(0, uint64(len(sectionedMagic))+4)
	if err != nil {
		return nil, none, err
	}
	if string(prefix[:len(sectionedMagic)]) != sectionedMagic {
		return nil, none, errors.New("sectioned snapshot magic mismatch")
	}
	length := uint64(binary.BigEndian.Uint32(prefix[len(sectionedMagic):]))
	if uint64(len(prefix))+length+sha256.Size > sectionedHeaderSlot {
		return nil, none, errors.New("sectioned snapshot table exceeds its slot")
	}
	table, err := f.bytesAt(uint64(len(prefix)), length+sha256.Size)
	if err != nil {
		return nil, none, err
	}
	digest := [sha256.Size]byte(table[length:])
	if sha256.Sum256(table[:length]) != digest {
		return nil, none, errors.New("sectioned snapshot table digest mismatch")
	}
	return table[:length], digest, nil
}

func (f *sectionedFile) header() (sectionedHeader, error) {
	table, _, err := f.headerTable()
	if err != nil {
		return sectionedHeader{}, err
	}
	var header sectionedHeader
	if err := gob.NewDecoder(bytes.NewReader(table)).Decode(&header); err != nil {
		return sectionedHeader{}, err
	}
	return header, nil
}

// identity names the file by path, length, modification time and header
// digest (the pack key), so a file rewritten at the same path is never served
// from the mapping the previous one left behind.
func (f *sectionedFile) identity() (packKey, error) {
	_, digest, err := f.headerTable()
	if err != nil {
		return packKey{}, err
	}
	key := f.key
	key.digest = digest
	return key, nil
}

// sectionedCacheCapacity bounds the sectioned mappings one process retains,
// for the same reason and at the same size as packCacheCapacity.
const sectionedCacheCapacity = packCacheCapacity

// sectionedCache retains the mappings of the last sectionedCacheCapacity
// distinct sectioned files and evicts the oldest retention first. Eviction
// drops the reference without unmapping: an Index handed out earlier still
// aliases those bytes, and a later read of that file maps it again.
var sectionedCache = struct {
	sync.Mutex
	mapped map[packKey]*sectionedFile
	ring   [sectionedCacheCapacity]packKey
	next   int
}{mapped: map[packKey]*sectionedFile{}}

func retainedSectioned(key packKey) (*sectionedFile, bool) {
	sectionedCache.Lock()
	defer sectionedCache.Unlock()
	file, ok := sectionedCache.mapped[key]
	return file, ok
}

// retainSectioned retains a mapped file and returns the file holding the
// key's retention: file, or the one a concurrent first read retained before
// it. An unmapped read copies its bytes, so there is nothing to reuse.
func retainSectioned(key packKey, file *sectionedFile) *sectionedFile {
	if file.mapping == nil {
		return file
	}
	sectionedCache.Lock()
	defer sectionedCache.Unlock()
	if retained, ok := sectionedCache.mapped[key]; ok {
		return retained
	}
	delete(sectionedCache.mapped, sectionedCache.ring[sectionedCache.next])
	sectionedCache.mapped[key] = file
	sectionedCache.ring[sectionedCache.next] = key
	sectionedCache.next = (sectionedCache.next + 1) % sectionedCacheCapacity
	return file
}

func (header sectionedHeader) entry(name string) (sectionEntry, error) {
	for _, entry := range header.Sections {
		if entry.Name == name {
			return entry, nil
		}
	}
	return sectionEntry{}, fmt.Errorf("sectioned snapshot has no %q section", name)
}

// section returns one section's bytes after checking its digest.
func (f *sectionedFile) section(header sectionedHeader, name string) ([]byte, error) {
	entry, err := header.entry(name)
	if err != nil {
		return nil, err
	}
	content, err := f.bytesAt(entry.Offset, entry.Length)
	if err != nil {
		return nil, err
	}
	if sha256.Sum256(content) != entry.SHA256 {
		return nil, fmt.Errorf("sectioned snapshot %q section digest mismatch", name)
	}
	return content, nil
}

func decodeGobSection[T any](f *sectionedFile, header sectionedHeader, name string) (T, error) {
	var wrapper struct{ V T }
	content, err := f.section(header, name)
	if err != nil {
		return wrapper.V, err
	}
	err = gob.NewDecoder(bytes.NewReader(content)).Decode(&wrapper)
	return wrapper.V, err
}

// readSectionedSnapshot decodes the sections load names, refusing the whole
// file on any digest mismatch. The bodies section is verified in parallel
// with the table decodes because it is the largest and is otherwise untouched.
// A file whose mapping is already retained is decoded from it, so repeated
// reads of one file retain one mapping.
func readSectionedSnapshot(path string, identity repositoryIdentity, engineID string, load snapshotLoad) (*Index, error) {
	file, err := openSectionedFile(path)
	if err != nil {
		return nil, err
	}
	key, err := file.identity()
	if err != nil {
		file.close()
		file.release()
		return nil, err
	}
	if retained, ok := retainedSectioned(key); ok {
		file.close()
		file.release()
		return decodeSectionedSnapshot(retained, identity, engineID, load)
	}
	defer file.close()
	index, err := decodeSectionedSnapshot(file, identity, engineID, load)
	if err != nil {
		file.release()
		return nil, err
	}
	// A concurrent first read retained this file first; as for a pack, the
	// index decoded here is dropped with its mapping and the retained one answers.
	if retained := retainSectioned(key, file); retained != file {
		file.release()
		return decodeSectionedSnapshot(retained, identity, engineID, load)
	}
	return index, nil
}

func decodeSectionedSnapshot(file *sectionedFile, identity repositoryIdentity, engineID string, load snapshotLoad) (*Index, error) {
	header, err := file.header()
	if err != nil {
		return nil, err
	}
	expected := snapshotHeader{Format: snapshotFormat, ObjectFormat: identity.objectFormat, Tree: identity.treeRevision, Engine: engineID}
	if header.Header != expected {
		return nil, errors.New("snapshot header mismatch")
	}
	index := &Index{}
	var bodies []byte
	bodiesChecked := make(chan error, 1)
	wanted := sectionsByLoad[load]
	if containsName(wanted, sectionBodies) {
		go func() {
			var err error
			bodies, err = file.section(header, sectionBodies)
			bodiesChecked <- err
		}()
	} else {
		bodiesChecked <- nil
	}
	var metas []sourceMeta
	for _, name := range wanted {
		if name == sectionBodies {
			continue
		}
		if err := decodeSectionInto(file, header, name, index, &metas); err != nil {
			<-bodiesChecked
			return nil, err
		}
	}
	if err := <-bodiesChecked; err != nil {
		return nil, err
	}
	if metas != nil {
		index.Sources, err = sourcesFrom(metas, bodies)
		if err != nil {
			return nil, err
		}
	}
	if load == loadCompact {
		return &Index{ProfileID: index.ProfileID}, nil
	}
	if err := index.checkSymbolWindows(); err != nil {
		return nil, err
	}
	return index, nil
}

func containsName(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func decodeSectionInto(file *sectionedFile, header sectionedHeader, name string, index *Index, metas *[]sourceMeta) error {
	var err error
	switch name {
	case sectionIdentity:
		var identity sectionIdentityValue
		identity, err = decodeGobSection[sectionIdentityValue](file, header, name)
		index.ObjectFormat, index.CommitRevision, index.Revision = identity.ObjectFormat, identity.CommitRevision, identity.Revision
		index.ProfileID, index.Module, index.ApproximateImports = identity.ProfileID, identity.Module, identity.ApproximateImports
		index.UnsupportedSuffixCount = identity.UnsupportedSuffixCount
	case sectionSources:
		*metas, err = decodeGobSection[[]sourceMeta](file, header, name)
	case sectionExclusions:
		index.Exclusions, err = decodeGobSection[[]Exclusion](file, header, name)
	case sectionFeatures:
		index.Features, err = decodeGobSection[map[string]Record](file, header, name)
	case sectionScenarios:
		index.Scenarios, err = decodeGobSection[map[string]Record](file, header, name)
	case sectionDocuments:
		index.Documents, err = decodeGobSection[map[string]Record](file, header, name)
	case sectionMarkers:
		index.Markers, err = decodeGobSection[map[string][]Marker](file, header, name)
	case sectionSymbols:
		index.Symbols, err = decodeGobSection[[]Symbol](file, header, name)
	case sectionImports:
		index.Imports, err = decodeGobSection[map[string]map[string]struct{}](file, header, name)
	case sectionUnparsed:
		index.Unparsed, err = decodeGobSection[[]Unparsed](file, header, name)
	case sectionNotes:
		index.ExtractionNotes, err = decodeGobSection[[]ExtractionNote](file, header, name)
	case sectionTracked:
		index.Tracked, err = decodeGobSection[map[string]struct{}](file, header, name)
	case sectionSkipped:
		index.Skipped, err = decodeGobSection[map[string]struct{}](file, header, name)
	case sectionVocabulary:
		index.Vocabulary, err = decodeGobSection[*TermTable](file, header, name)
		if err == nil {
			err = index.Vocabulary.check()
		}
	default:
		err = fmt.Errorf("sectioned snapshot reader has no decoder for %q", name)
	}
	return err
}

// sourcesFrom rebuilds the Sources map; a body aliases the bodies section,
// and an empty body is nil exactly as gob decodes it.
func sourcesFrom(metas []sourceMeta, bodies []byte) (map[string]Source, error) {
	sources := make(map[string]Source, len(metas))
	for _, meta := range metas {
		if !withinBounds(meta.Offset, meta.Length, uint64(len(bodies))) {
			return nil, errors.New("sectioned snapshot source body lies outside the bodies section")
		}
		end := meta.Offset + meta.Length
		var data []byte
		if meta.Length != 0 {
			data = bodies[meta.Offset:end:end]
		}
		sources[meta.Path] = Source{Path: meta.Path, BlobHash: meta.BlobHash, Data: data, Mode: meta.Mode, Checked: meta.Checked, Valid: meta.Valid}
	}
	return sources, nil
}
