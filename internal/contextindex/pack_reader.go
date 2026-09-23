package contextindex

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
)

// packFile serves sections from a read-only private mapping of the file
// (unix) or from ReadAt elsewhere. A successful read keeps the mapping for
// the process's life because the returned Index aliases it, so packCache
// retains it and every later read of the same pack reuses it; a refused read
// releases the mapping it opened. A digest mismatch found in a retained
// mapping, including a deferred body's, marks it refused, so every later read
// of those bytes refuses at once instead of redoing the work the mismatch
// discards.
type packFile struct {
	at      io.ReaderAt
	size    int64
	mapping []byte
	file    *os.File
	key     packKey
	refused atomic.Bool

	mutex   sync.Mutex
	history []historyEntry
}

func openPackFile(path string) (*packFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	key := packKey{path: path, size: info.Size(), modified: info.ModTime().UnixNano()}
	return &packFile{at: file, size: info.Size(), mapping: mapReadOnly(file, info.Size()), file: file, key: key}, nil
}

func (f *packFile) close()   { f.file.Close() }
func (f *packFile) release() { unmapReadOnly(f.mapping) }

// bytesAt returns the file bytes at offset without verifying them; every
// caller verifies before decoding.
func (f *packFile) bytesAt(offset, length uint64) ([]byte, error) {
	end := offset + length
	if end < offset || end > uint64(f.size) {
		return nil, errors.New("pack range lies outside the file")
	}
	if f.mapping != nil {
		return f.mapping[offset:end:end], nil
	}
	buffer := make([]byte, length)
	if _, err := f.at.ReadAt(buffer, int64(offset)); err != nil {
		return nil, err
	}
	return buffer, nil
}

// headerTable returns the header table bytes and the digest the file stores
// over them. That digest covers every section's block digest table, so two
// packs carrying it hold the same verified content.
func (f *packFile) headerTable() ([]byte, [sha256.Size]byte, error) {
	var none [sha256.Size]byte
	prefixLength := uint64(len(packMagic)) + 4
	prefix, err := f.bytesAt(0, prefixLength)
	if err != nil {
		return nil, none, err
	}
	if string(prefix[:len(packMagic)]) != packMagic {
		return nil, none, errors.New("pack magic mismatch")
	}
	length := uint64(binary.LittleEndian.Uint32(prefix[len(packMagic):]))
	if prefixLength+length+sha256.Size > packHeaderSlot {
		return nil, none, errors.New("pack table exceeds its slot")
	}
	table, err := f.bytesAt(prefixLength, length+sha256.Size)
	if err != nil {
		return nil, none, err
	}
	digest := [sha256.Size]byte(table[length:])
	if sha256.Sum256(table[:length]) != digest {
		return nil, none, errors.New("pack table digest mismatch")
	}
	return table[:length], digest, nil
}

func (f *packFile) header() (packHeader, error) {
	table, _, err := f.headerTable()
	if err != nil {
		return packHeader{}, err
	}
	var header packHeader
	if err := gob.NewDecoder(bytes.NewReader(table)).Decode(&header); err != nil {
		return packHeader{}, err
	}
	if err := header.checkSections(uint64(f.size)); err != nil {
		return packHeader{}, err
	}
	return header, nil
}

// packRange is a half-open byte range [start, end) of the pack file.
type packRange struct{ start, end uint64 }

// checkSections refuses a table naming a section twice, or whose sections,
// digest tables and header slot share bytes or run past the file: the
// writer lays them out disjoint, and a forged table that aliases one
// section's verified bytes under another name is not the pack it signs.
func (header packHeader) checkSections(size uint64) error {
	names := map[string]bool{}
	ranges := []packRange{{0, packHeaderSlot}}
	for _, entry := range header.Sections {
		if names[entry.Name] {
			return fmt.Errorf("pack section %q is listed twice", entry.Name)
		}
		names[entry.Name] = true
		if entry.Length > size {
			return fmt.Errorf("pack section %q is longer than the file", entry.Name)
		}
		ranges = append(ranges, packRange{entry.Offset, entry.Offset + entry.Length}, packRange{entry.DigestsOffset, entry.DigestsOffset + packBlockCount(entry.Length)*sha256.Size})
	}
	slices.SortFunc(ranges, func(a, b packRange) int { return cmp.Or(cmp.Compare(a.start, b.start), cmp.Compare(a.end, b.end)) })
	previous := uint64(0)
	for _, item := range ranges {
		if item.end < item.start || item.end > size {
			return errors.New("pack section range runs past the file")
		}
		if item.start == item.end {
			continue
		}
		if item.start < previous {
			return errors.New("pack sections overlap")
		}
		previous = item.end
	}
	return nil
}

// identity names the pack by path, length, modification time and header
// digest, so a pack rewritten at the same path is never served from the
// mapping the previous one left behind.
func (f *packFile) identity() (packKey, error) {
	_, digest, err := f.headerTable()
	if err != nil {
		return packKey{}, err
	}
	key := f.key
	key.digest = digest
	return key, nil
}

func (header packHeader) entry(name string) (packSectionEntry, error) {
	for _, entry := range header.Sections {
		if entry.Name == name {
			return entry, nil
		}
	}
	return packSectionEntry{}, fmt.Errorf("pack has no %q section", name)
}

// packSection is one section with its verified block digest table. Blocks
// are verified on first touch: bytes verifies the blocks a range covers
// before returning it, and all verifies every block. failure holds the first
// mismatch a deferred body read found (see packBody).
type packSection struct {
	name     string
	data     []byte
	digests  []byte
	verified []atomic.Bool
	failure  atomic.Pointer[error]
	refused  *atomic.Bool
}

// ErrSnapshotRefused reports that a read of a deferred load touched a pack
// body whose blocks failed verification. The result is withheld; the caller
// reloads through a loader that verifies every section up front, which
// refuses the pack as the IDX-SNAP-V0-003 miss and falls back to the gob
// snapshot, then the build.
var ErrSnapshotRefused = errors.New("snapshot body failed verification")

// packBody is a deferred-load source body whose blocks are verified when a
// read first asks for its text, not when the pack is opened: a packet, query
// or event reads a few dozen of a repository's bodies, and the bodies are
// most of the pack.
type packBody struct {
	section        *packSection
	offset, length uint64
}

// text verifies the blocks the body covers and answers Source.Text over
// them. A mismatch records the refusal on the section and reports the body
// unloaded, so no unverified byte is ever returned.
func (body packBody) text(source Source) (string, bool, bool) {
	data, err := body.section.bytes(body.offset, body.length)
	if err != nil {
		body.section.failure.CompareAndSwap(nil, &err)
		return "", false, false
	}
	source.Data, source.body = data, packBody{}
	return source.Text()
}

// SnapshotRefusal reports the first body verification failure a read of this
// deferred load found, wrapped in ErrSnapshotRefused. A caller of a deferred
// loader checks it after every read of the index and, when it is non-nil,
// discards what it computed.
func (index *Index) SnapshotRefusal() error {
	if index.bodies == nil {
		return nil
	}
	if failure := index.bodies.failure.Load(); failure != nil {
		return fmt.Errorf("%w: %w", ErrSnapshotRefused, *failure)
	}
	return nil
}

func (f *packFile) section(header packHeader, name string) (*packSection, error) {
	entry, err := header.entry(name)
	if err != nil {
		return nil, err
	}
	blocks := packBlockCount(entry.Length)
	digests, err := f.bytesAt(entry.DigestsOffset, blocks*sha256.Size)
	if err != nil {
		return nil, err
	}
	if sha256.Sum256(digests) != entry.DigestsSHA256 {
		f.refused.Store(true)
		return nil, fmt.Errorf("pack %q block digest table mismatch", name)
	}
	data, err := f.bytesAt(entry.Offset, entry.Length)
	if err != nil {
		return nil, err
	}
	return &packSection{name: name, data: data, digests: digests, verified: make([]atomic.Bool, blocks), refused: &f.refused}, nil
}

func (s *packSection) verifyBlock(block int) error {
	if s.verified[block].Load() {
		return nil
	}
	low := block * packBlockSize
	high := min(low+packBlockSize, len(s.data))
	if sha256.Sum256(s.data[low:high]) != [sha256.Size]byte(s.digests[block*sha256.Size:]) {
		s.refused.Store(true)
		return fmt.Errorf("pack %q block %d digest mismatch", s.name, block)
	}
	s.verified[block].Store(true)
	return nil
}

// bytes returns the section bytes in [offset, offset+length) after verifying
// every block the range touches.
func (s *packSection) bytes(offset, length uint64) ([]byte, error) {
	end := offset + length
	if end < offset || end > uint64(len(s.data)) {
		return nil, fmt.Errorf("pack %q range lies outside the section", s.name)
	}
	if length == 0 {
		return s.data[offset:offset:offset], nil
	}
	for block := int(offset / packBlockSize); block <= int((end-1)/packBlockSize); block++ {
		if err := s.verifyBlock(block); err != nil {
			return nil, err
		}
	}
	return s.data[offset:end:end], nil
}

func (s *packSection) all() ([]byte, error) { return s.bytes(0, uint64(len(s.data))) }

// verifyAll checks every block of every section on all CPUs and reports the
// first mismatch. The whole-array views the ranking reads cannot be
// intercepted per touch from here, so their sections are verified up front,
// in parallel, before any view is built.
func verifyAll(sections []*packSection) error {
	type job struct {
		section *packSection
		block   int
	}
	jobs := make(chan job, 64)
	var failure atomic.Value
	var pending sync.WaitGroup
	for worker := 0; worker < min(runtime.NumCPU(), 8); worker++ {
		pending.Add(1)
		go func() {
			defer pending.Done()
			for item := range jobs {
				if failure.Load() != nil {
					continue
				}
				if err := item.section.verifyBlock(item.block); err != nil {
					failure.CompareAndSwap(nil, err)
				}
			}
		}()
	}
	for _, section := range sections {
		for block := range section.verified {
			jobs <- job{section, block}
		}
	}
	close(jobs)
	pending.Wait()
	if err, _ := failure.Load().(error); err != nil {
		return err
	}
	return nil
}

func decodePackGob[T any](section *packSection) (T, error) {
	var wrapper struct{ V T }
	content, err := section.all()
	if err != nil {
		return wrapper.V, err
	}
	err = gob.NewDecoder(bytes.NewReader(content)).Decode(&wrapper)
	return wrapper.V, err
}

// packStringTable views the string section: offsets over one byte run.
type packStringTable struct {
	offsets []uint32
	data    []byte
}

func viewStrings(data []byte) (packStringTable, error) {
	if len(data) < 4 {
		return packStringTable{}, errors.New("pack string table is truncated")
	}
	count := uint64(binary.LittleEndian.Uint32(data))
	offsets, end, err := u32Array(data, 4, count+1)
	if err != nil {
		return packStringTable{}, err
	}
	table := packStringTable{offsets: offsets, data: data[end:]}
	limit := uint32(len(table.data))
	for index, offset := range offsets {
		if offset > limit || index > 0 && offset < offsets[index-1] {
			return packStringTable{}, errors.New("pack string offsets are not ascending within the bytes")
		}
	}
	return table, nil
}

func (table packStringTable) get(id uint32) (string, error) {
	if int(id)+1 >= len(table.offsets) {
		return "", errors.New("pack names a string outside the string table")
	}
	return stringView(table.data[table.offsets[id]:table.offsets[id+1]]), nil
}

func (table packStringTable) strings(ids []uint32) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	values := make([]string, len(ids))
	for index, id := range ids {
		value, err := table.get(id)
		if err != nil {
			return nil, err
		}
		values[index] = value
	}
	return values, nil
}

func (table packStringTable) set(ids []uint32, present bool) (map[string]struct{}, error) {
	if !present {
		return nil, nil
	}
	values := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		value, err := table.get(id)
		if err != nil {
			return nil, err
		}
		values[value] = struct{}{}
	}
	return values, nil
}

// u32Columns views count values of each of the named columns after a
// header of headerBytes.
func u32Columns(data []byte, headerBytes uint64, count uint64, columns int) ([][]uint32, error) {
	views := make([][]uint32, columns)
	offset := headerBytes
	for column := range views {
		array, next, err := u32Array(data, offset, count)
		if err != nil {
			return nil, err
		}
		views[column], offset = array, next
	}
	if offset != uint64(len(data)) {
		return nil, errors.New("pack table has trailing bytes")
	}
	return views, nil
}

// viewIDList views [u32 n][u32 present][u32 id x n] (idListSection).
func viewIDList(data []byte) ([]uint32, bool, error) {
	if len(data) < 8 {
		return nil, false, errors.New("pack id list is truncated")
	}
	columns, err := u32Columns(data, 8, uint64(binary.LittleEndian.Uint32(data)), 1)
	if err != nil {
		return nil, false, err
	}
	return columns[0], binary.LittleEndian.Uint32(data[4:]) != 0, nil
}

// decodeSources builds the source table. A nil deferred aliases verified
// bodies; otherwise each body is left to packBody verification on first read.
func decodeSources(data []byte, bodies []byte, deferred *packSection, strings packStringTable) (map[string]Source, error) {
	if len(data) < 8 {
		return nil, errors.New("pack sources header is truncated")
	}
	count := uint64(binary.LittleEndian.Uint32(data))
	offsetsEnd := 8 + 8*count
	if offsetsEnd > uint64(len(data)) {
		return nil, errors.New("pack source offsets lie outside the section")
	}
	offsets := u64View(data[8:offsetsEnd])
	columns, err := u32Columns(data, offsetsEnd, count, 5)
	if err != nil {
		return nil, err
	}
	sources := make(map[string]Source, count)
	for index := uint64(0); index < count; index++ {
		path, err := strings.get(columns[0][index])
		if err != nil {
			return nil, err
		}
		blob, err := strings.get(columns[1][index])
		if err != nil {
			return nil, err
		}
		mode, err := strings.get(columns[2][index])
		if err != nil {
			return nil, err
		}
		flags, length := columns[3][index], uint64(columns[4][index])
		end := offsets[index] + length
		if end < offsets[index] || end > uint64(len(bodies)) {
			return nil, errors.New("pack source body lies outside the bodies section")
		}
		source := Source{Path: path, BlobHash: blob, Mode: mode, Checked: flags&1 != 0, Valid: flags&2 != 0}
		switch {
		case length == 0:
		case deferred != nil:
			source.body = packBody{section: deferred, offset: offsets[index], length: length}
		default:
			source.Data = bodies[offsets[index]:end:end]
		}
		sources[path] = source
	}
	return sources, nil
}

func decodeSymbols(data []byte, strings packStringTable) ([]Symbol, error) {
	if len(data) < 4 {
		return nil, errors.New("pack symbols header is truncated")
	}
	count := uint64(binary.LittleEndian.Uint32(data))
	if count == 0 {
		return nil, nil
	}
	columns, err := u32Columns(data, 4, count, 6)
	if err != nil {
		return nil, err
	}
	symbols := make([]Symbol, count)
	for index := range symbols {
		fields := [4]*string{&symbols[index].Kind, &symbols[index].Name, &symbols[index].Path, &symbols[index].BlobHash}
		for column, field := range fields {
			if *field, err = strings.get(columns[column][index]); err != nil {
				return nil, err
			}
		}
		symbols[index].Line, symbols[index].EndLine = int(columns[4][index]), int(columns[5][index])
	}
	return symbols, nil
}

// packCacheCapacity bounds the mappings one process retains. Each retained
// mapping is a whole pack, and a reader works over one tree at a time, so a
// handful of slots absorbs a tree switch without holding more.
const packCacheCapacity = 4

// packKey identifies the bytes behind a retained mapping.
type packKey struct {
	path     string
	size     int64
	modified int64
	digest   [sha256.Size]byte
}

// packCache retains the mappings of the last packCacheCapacity distinct packs
// and evicts the oldest retention first. Eviction drops the reference without
// unmapping: an Index handed out earlier still aliases those bytes, and a
// later read of that pack maps it again.
var packCache = struct {
	sync.Mutex
	mapped map[packKey]*packFile
	ring   [packCacheCapacity]packKey
	next   int
}{mapped: map[packKey]*packFile{}}

func retainedPack(key packKey) (*packFile, bool) {
	packCache.Lock()
	defer packCache.Unlock()
	file, ok := packCache.mapped[key]
	return file, ok
}

// retainPack retains a mapped pack and returns the file holding the key's
// retention: file, or the one a concurrent first read retained before it. An
// unmapped read (a platform without mmap) copies its bytes and closes the
// file, so there is nothing to reuse.
func retainPack(key packKey, file *packFile) *packFile {
	if file.mapping == nil {
		return file
	}
	packCache.Lock()
	defer packCache.Unlock()
	if retained, ok := packCache.mapped[key]; ok {
		return retained
	}
	delete(packCache.mapped, packCache.ring[packCache.next])
	packCache.mapped[key] = file
	packCache.ring[packCache.next] = key
	packCache.next = (packCache.next + 1) % packCacheCapacity
	return file
}

// readPackSnapshot maps the pack and builds the Index the load names,
// refusing the whole file on any digest mismatch, truncation, or
// out-of-range offset. A refusal is the caller's gob fallback. A pack whose
// mapping is already retained is decoded from it, so repeated reads of one
// pack retain one mapping.
func readPackSnapshot(path string, identity repositoryIdentity, engineID string, load snapshotLoad) (*Index, error) {
	file, err := openPackFile(path)
	if err != nil {
		return nil, err
	}
	key, err := file.identity()
	if err != nil {
		file.close()
		file.release()
		return nil, err
	}
	if retained, ok := retainedPack(key); ok {
		file.close()
		file.release()
		return decodeRetainedPack(retained, identity, engineID, load)
	}
	defer file.close()
	index, err := decodePackSnapshot(file, identity, engineID, load)
	if err != nil {
		file.release()
		return nil, err
	}
	// A concurrent first read retained this pack first. The index decoded
	// here is not handed out, so its history registration and mapping are
	// dropped and the retained mapping answers; one pack keeps one mapping.
	if retained := retainPack(key, file); retained != file {
		forgetPackHistory(index)
		file.release()
		return decodeRetainedPack(retained, identity, engineID, load)
	}
	return index, nil
}

func decodeRetainedPack(retained *packFile, identity repositoryIdentity, engineID string, load snapshotLoad) (*Index, error) {
	if retained.refused.Load() {
		return nil, errors.New("pack failed a digest check earlier in this process")
	}
	return decodePackSnapshot(retained, identity, engineID, load)
}

func decodePackSnapshot(file *packFile, identity repositoryIdentity, engineID string, load snapshotLoad) (*Index, error) {
	header, err := file.header()
	if err != nil {
		return nil, err
	}
	expected := snapshotHeader{Format: snapshotFormat, ObjectFormat: identity.objectFormat, Tree: identity.treeRevision, Engine: engineID}
	if header.Header != expected {
		return nil, errors.New("snapshot header mismatch")
	}
	sections := map[string]*packSection{}
	tables := load.tables()
	ordered := make([]*packSection, 0, len(packSectionsByLoad[tables]))
	for _, name := range packSectionsByLoad[tables] {
		section, err := file.section(header, name)
		if err != nil {
			return nil, err
		}
		sections[name] = section
		ordered = append(ordered, section)
	}
	reader := packReader{sections: sections}
	if load.deferredBodies() {
		reader.deferred = sections[packSectionBodies]
		ordered = slices.DeleteFunc(ordered, func(section *packSection) bool { return section == reader.deferred })
	}
	if err := verifyAll(ordered); err != nil {
		return nil, err
	}
	packed, err := decodePackGob[packIdentity](sections[packSectionIdentity])
	if err != nil {
		return nil, err
	}
	if tables == loadCompact {
		return &Index{ProfileID: packed.ProfileID}, nil
	}
	index := &Index{
		ObjectFormat: packed.ObjectFormat, CommitRevision: packed.CommitRevision, Revision: packed.Revision,
		ProfileID: packed.ProfileID, Module: packed.Module, ApproximateImports: packed.ApproximateImports,
		UnsupportedSuffixCount: packed.UnsupportedSuffixCount,
		bodies:                 reader.deferred,
	}
	if err := reader.eventTables(index); err != nil {
		return nil, err
	}
	if tables == loadEvent {
		return index, nil
	}
	if err := reader.fullTables(index, packed.Vocabulary); err != nil {
		return nil, err
	}
	if header.HistoryCommit != "" && header.HistoryCommit == identity.commitRevision && !identity.historyCut {
		history, err := file.cochange(reader.data(packSectionCochange), reader.strings)
		if err != nil {
			return nil, err
		}
		registerPackHistory(index, history)
	}
	return index, nil
}

// packReader decodes verified sections into an Index; every section it
// touches was verified by verifyAll, so data never fails. The one exception
// is deferred, the bodies of a deferred load, which no decode reads.
type packReader struct {
	sections map[string]*packSection
	strings  packStringTable
	deferred *packSection
}

func (reader *packReader) data(name string) []byte { return reader.sections[name].data }

func (reader *packReader) eventTables(index *Index) error {
	var err error
	if reader.strings, err = viewStrings(reader.data(packSectionStrings)); err != nil {
		return err
	}
	if index.Sources, err = decodeSources(reader.data(packSectionSources), reader.data(packSectionBodies), reader.deferred, reader.strings); err != nil {
		return err
	}
	if index.Symbols, err = decodeSymbols(reader.data(packSectionSymbols), reader.strings); err != nil {
		return err
	}
	if index.Exclusions, err = decodePackGob[[]Exclusion](reader.sections[sectionExclusions]); err != nil {
		return err
	}
	if index.Features, err = decodePackGob[map[string]Record](reader.sections[sectionFeatures]); err != nil {
		return err
	}
	if index.Scenarios, err = decodePackGob[map[string]Record](reader.sections[sectionScenarios]); err != nil {
		return err
	}
	if index.Documents, err = decodePackGob[map[string]Record](reader.sections[sectionDocuments]); err != nil {
		return err
	}
	if index.Markers, err = decodePackGob[map[string][]Marker](reader.sections[sectionMarkers]); err != nil {
		return err
	}
	if index.Imports, err = decodePackGob[map[string]map[string]struct{}](reader.sections[sectionImports]); err != nil {
		return err
	}
	if index.Unparsed, err = decodePackGob[[]Unparsed](reader.sections[sectionUnparsed]); err != nil {
		return err
	}
	index.ExtractionNotes, err = decodePackGob[[]ExtractionNote](reader.sections[sectionNotes])
	return err
}

func (reader *packReader) fullTables(index *Index, vocabulary bool) error {
	tracked, present, err := viewIDList(reader.data(packSectionTracked))
	if err != nil {
		return err
	}
	if index.Tracked, err = reader.strings.set(tracked, present); err != nil {
		return err
	}
	skipped, present, err := viewIDList(reader.data(packSectionSkipped))
	if err != nil {
		return err
	}
	if index.Skipped, err = reader.strings.set(skipped, present); err != nil {
		return err
	}
	if !vocabulary {
		return nil
	}
	pathIDs, _, err := viewIDList(reader.data(packSectionPaths))
	if err != nil {
		return err
	}
	table := &TermTable{}
	if table.Paths, err = reader.strings.strings(pathIDs); err != nil {
		return err
	}
	targets := [3]struct {
		name    string
		target  *termPostings
		counted bool
	}{{packSectionTerms, &table.Terms, true}, {packSectionWords, &table.Words, false}, {packSectionPathTerms, &table.PathTerms, false}}
	for _, item := range targets {
		view, err := viewTermTable(reader.data(item.name))
		if err != nil {
			return err
		}
		if err := view.check(len(table.Paths), item.counted); err != nil {
			return err
		}
		*item.target = view.postings()
	}
	if table.SymbolWindows, err = decodePackGob[windowPostings](reader.sections[packSectionWindows]); err != nil {
		return err
	}
	if err := table.SymbolWindows.check(len(index.Symbols)); err != nil {
		return err
	}
	if table.IdentGraph, err = decodePackGob[*identGraph](reader.sections[packSectionGraph]); err != nil {
		return err
	}
	if err := table.IdentGraph.check(len(table.Paths)); err != nil {
		return err
	}
	index.Vocabulary = table
	return nil
}
