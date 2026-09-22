package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func sectionedFixture(t *testing.T) (*Index, SnapshotReceipt) {
	t.Helper()
	t.Setenv(snapshotFormatEnv, snapshotFormatValue)
	index := taskContextFixture(t)
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.SectionedPath == "" || receipt.SectionedBytes <= sectionedHeaderSlot {
		t.Fatalf("sectioned receipt = %+v", receipt)
	}
	return index, receipt
}

func fixtureIdentity(index *Index) repositoryIdentity {
	return repositoryIdentity{objectFormat: index.ObjectFormat, commitRevision: index.CommitRevision, treeRevision: index.Revision}
}

func gobDecoded(t *testing.T, receipt SnapshotReceipt, identity repositoryIdentity, load snapshotLoad) *Index {
	t.Helper()
	file, err := os.Open(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var index *Index
	switch load {
	case loadCompact:
		index, err = decodeCompactEventSnapshot(file, identity, receipt.Engine)
	case loadEvent:
		index, err = decodeEventSnapshot(file, identity, receipt.Engine)
	default:
		index, err = decodeSnapshot(file, identity, receipt.Engine)
	}
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func TestSectionedSnapshotDecodesEverySectionToTheGobValues(t *testing.T) {
	index, receipt := sectionedFixture(t)
	identity := fixtureIdentity(index)
	t.Run("IDX-SNAP-V0-014", func(t *testing.T) {
		for _, load := range []snapshotLoad{loadFull, loadEvent, loadCompact} {
			gob := gobDecoded(t, receipt, identity, load)
			sectioned, err := readSectionedSnapshot(receipt.SectionedPath, identity, receipt.Engine, load)
			if err != nil {
				t.Fatalf("load %d: %v", load, err)
			}
			if !reflect.DeepEqual(gob, sectioned) {
				t.Errorf("load %d: gob and sectioned snapshots differ", load)
			}
		}
	})
	loaded, hit, err := LoadSnapshot(context.Background(), index.Root)
	if err != nil || !hit {
		t.Fatalf("sectioned LoadSnapshot: hit=%v err=%v", hit, err)
	}
	if !reflect.DeepEqual(loaded, index) {
		t.Fatal("sectioned LoadSnapshot differs from the built index")
	}
	compact, hit, err := LoadEventSnapshot(context.Background(), index.Root, true)
	if err != nil || !hit || compact.ProfileID != index.ProfileID || compact.Sources != nil {
		t.Fatalf("sectioned compact load: hit=%v err=%v index=%+v", hit, err, compact)
	}
}

type countingReaderAt struct {
	at   io.ReaderAt
	read atomic.Uint64
}

func (c *countingReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	n, err := c.at.ReadAt(p, offset)
	c.read.Add(uint64(n))
	return n, err
}

func countedSectionedFile(t *testing.T, path string) (*sectionedFile, *countingReaderAt) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	counter := &countingReaderAt{at: file}
	return &sectionedFile{at: counter, size: info.Size(), close: func() {}}, counter
}

func TestSectionedFileChangeReadsOnlyItsSections(t *testing.T) {
	index, receipt := sectionedFixture(t)
	identity := fixtureIdentity(index)
	headerFile, headerCounter := countedSectionedFile(t, receipt.SectionedPath)
	header, err := headerFile.header()
	if err != nil {
		t.Fatal(err)
	}
	headerBytes := headerCounter.read.Load()
	expected := func(load snapshotLoad) uint64 {
		total := headerBytes
		for _, name := range sectionsByLoad[load] {
			entry, err := header.entry(name)
			if err != nil {
				t.Fatal(err)
			}
			total += entry.Length
		}
		return total
	}
	for _, load := range []snapshotLoad{loadEvent, loadCompact} {
		file, counter := countedSectionedFile(t, receipt.SectionedPath)
		if _, err := decodeSectionedSnapshot(file, identity, receipt.Engine, load); err != nil {
			t.Fatal(err)
		}
		if counter.read.Load() != expected(load) {
			t.Fatalf("load %d read %d bytes, want %d", load, counter.read.Load(), expected(load))
		}
		if counter.read.Load() >= uint64(receipt.SectionedBytes) {
			t.Fatalf("load %d read the whole %d-byte file", load, receipt.SectionedBytes)
		}
	}
	if expected(loadCompact) >= expected(loadEvent) {
		t.Fatal("compact must read strictly less than file-change")
	}
}

func corruptSection(t *testing.T, path, name string) {
	t.Helper()
	file, err := openSectionedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header, err := file.header()
	file.close()
	file.release()
	if err != nil {
		t.Fatal(err)
	}
	entry, err := header.entry(name)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content[entry.Offset+entry.Length/2] ^= 0xff
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSectionedSnapshotRefusesACorruptSectionAsAMiss(t *testing.T) {
	index, receipt := sectionedFixture(t)
	identity := fixtureIdentity(index)
	corruptSection(t, receipt.SectionedPath, sectionTracked)
	if _, err := readSectionedSnapshot(receipt.SectionedPath, identity, receipt.Engine, loadEvent); err != nil {
		t.Fatalf("file-change does not read tracked, so its corruption must not refuse: %v", err)
	}
	_, err := readSectionedSnapshot(receipt.SectionedPath, identity, receipt.Engine, loadFull)
	if err == nil || !strings.Contains(err.Error(), `"tracked" section digest mismatch`) {
		t.Fatalf("full load must refuse the corrupt section: %v", err)
	}
	// Corruption accumulates, so the sequentially decoded symbols table is
	// corrupted after the concurrently checked bodies.
	for _, name := range []string{sectionBodies, sectionSymbols} {
		corruptSection(t, receipt.SectionedPath, name)
		_, err := readSectionedSnapshot(receipt.SectionedPath, identity, receipt.Engine, loadEvent)
		if err == nil || !strings.Contains(err.Error(), name+`" section digest mismatch`) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// With the gob file gone the refusal is the snapshot's existing miss: no
	// error, no partial index, the caller builds.
	if err := os.Remove(receipt.Path); err != nil {
		t.Fatal(err)
	}
	loaded, hit, err := LoadEventSnapshot(context.Background(), index.Root, false)
	if err != nil || hit || loaded != nil {
		t.Fatalf("corrupt sectioned snapshot must miss: hit=%v err=%v", hit, err)
	}
}

// rewriteSectioned copies the sectioned file to a fresh path, appends each
// replacement section, lets mutate edit the table, and rewrites the header
// slot with a matching digest: the corruption a self-attested file permits.
func rewriteSectioned(t *testing.T, source string, replacements []sectionWriter, mutate func(*sectionedHeader)) string {
	t.Helper()
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := openSectionedFile(source)
	if err != nil {
		t.Fatal(err)
	}
	header, err := opened.header()
	opened.close()
	opened.release()
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range replacements {
		var section bytes.Buffer
		if err := replacement.write(&section); err != nil {
			t.Fatal(err)
		}
		for index := range header.Sections {
			if header.Sections[index].Name == replacement.name {
				header.Sections[index] = sectionEntry{Name: replacement.name, Offset: uint64(len(content)), Length: uint64(section.Len()), SHA256: sha256.Sum256(section.Bytes())}
			}
		}
		content = append(content, section.Bytes()...)
	}
	mutate(&header)
	slot, err := encodeSectionedHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	copy(content, slot)
	path := filepath.Join(t.TempDir(), filepath.Base(source))
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A self-attested section table or source table can name bytes outside the
// file; the read refuses it as the IDX-SNAP-V0-014 miss instead of panicking
// on a wrapped offset+length.
func TestSectionedSnapshotRefusesOutOfRangeOffsetsAsAMiss(t *testing.T) {
	index, receipt := sectionedFixture(t)
	identity := fixtureIdentity(index)
	metas := sourceMetas(index)
	metas[0].Offset, metas[0].Length = math.MaxUint64, 2
	vocabulary := *index.Vocabulary
	vocabulary.Terms.KeyOffsets = slices.Clone(vocabulary.Terms.KeyOffsets)
	vocabulary.Terms.KeyOffsets[len(vocabulary.Terms.KeyOffsets)-1] = uint32(len(vocabulary.Terms.KeyBytes)) + 9
	cases := map[string]string{
		"section offset wraps": rewriteSectioned(t, receipt.SectionedPath, nil, func(header *sectionedHeader) {
			header.Sections[0].Offset, header.Sections[0].Length = math.MaxUint64, 2
		}),
		"source body offset wraps":      rewriteSectioned(t, receipt.SectionedPath, []sectionWriter{gobSection(sectionSources, metas)}, func(*sectionedHeader) {}),
		"term key offset past the keys": rewriteSectioned(t, receipt.SectionedPath, []sectionWriter{gobSection(sectionVocabulary, &vocabulary)}, func(*sectionedHeader) {}),
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			loaded, err := readSectionedSnapshot(path, identity, receipt.Engine, loadFull)
			if err == nil {
				walkTermTable(loaded.Vocabulary)
				t.Fatal("an out-of-range offset must refuse the file")
			}
		})
	}
}

// retainedSectionedPaths counts the retained sectioned mappings by path.
func retainedSectionedPaths() map[string]int {
	sectionedCache.Lock()
	defer sectionedCache.Unlock()
	paths := make(map[string]int, len(sectionedCache.mapped))
	for key := range sectionedCache.mapped {
		paths[key.path]++
	}
	return paths
}

// TestSectionedRepeatedOpensRetainBoundedMappings bounds what a long-lived
// reader keeps. Before the bound every successful read retained a new
// whole-file mapping and nothing released it.
func TestSectionedRepeatedOpensRetainBoundedMappings(t *testing.T) {
	index, receipt := sectionedFixture(t)
	identity := fixtureIdentity(index)
	sectionedCache.Lock()
	sectionedCache.mapped, sectionedCache.ring, sectionedCache.next = map[packKey]*sectionedFile{}, [sectionedCacheCapacity]packKey{}, 0
	sectionedCache.Unlock()
	const opens = 64
	loaded := make([]*Index, 0, opens)
	for open := 0; open < opens; open++ {
		sectioned, err := readSectionedSnapshot(receipt.SectionedPath, identity, receipt.Engine, loadFull)
		if err != nil {
			t.Fatalf("open %d: %v", open, err)
		}
		loaded = append(loaded, sectioned)
	}
	if retained := retainedSectionedPaths(); len(retained) != 1 || retained[receipt.SectionedPath] != 1 {
		t.Fatalf("%d opens of one file retained %v, want one mapping of %s", opens, retained, receipt.SectionedPath)
	}
	if !reflect.DeepEqual(loaded[0], loaded[opens-1]) {
		t.Fatal("the reused mapping decoded a different index")
	}
	// Distinct files fill the ring and the oldest retention is evicted first.
	content, err := os.ReadFile(receipt.SectionedPath)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	paths := make([]string, 0, sectionedCacheCapacity+1)
	for open := 0; open <= sectionedCacheCapacity; open++ {
		path := filepath.Join(directory, fmt.Sprintf("copy-%d%s", open, sectionedExtension))
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readSectionedSnapshot(path, identity, receipt.Engine, loadFull); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		paths = append(paths, path)
	}
	retained := retainedSectionedPaths()
	if len(retained) != sectionedCacheCapacity {
		t.Fatalf("%d distinct files retained %d mappings, want %d", len(paths), len(retained), sectionedCacheCapacity)
	}
	if _, ok := retained[paths[0]]; ok {
		t.Fatal("the oldest retention survived the eviction")
	}
	for _, path := range paths[1:] {
		if _, ok := retained[path]; !ok {
			t.Fatalf("%s was evicted before the oldest retention", path)
		}
	}
}

// TestSectionedConcurrentFirstReadsShareOneMapping is the sectioned twin of
// TestPackConcurrentFirstReadsShareOneMapping.
func TestSectionedConcurrentFirstReadsShareOneMapping(t *testing.T) {
	index, receipt := sectionedFixture(t)
	identity := fixtureIdentity(index)
	sectionedCache.Lock()
	sectionedCache.mapped, sectionedCache.ring, sectionedCache.next = map[packKey]*sectionedFile{}, [sectionedCacheCapacity]packKey{}, 0
	sectionedCache.Unlock()
	const racers = 16
	loaded := make([]*Index, racers)
	failures := make([]error, racers)
	start := make(chan struct{})
	var pending sync.WaitGroup
	for racer := range loaded {
		pending.Add(1)
		go func() {
			defer pending.Done()
			<-start
			loaded[racer], failures[racer] = readSectionedSnapshot(receipt.SectionedPath, identity, receipt.Engine, loadFull)
		}()
	}
	close(start)
	pending.Wait()
	if err := errors.Join(failures...); err != nil {
		t.Fatal(err)
	}
	sectionedCache.Lock()
	retained := make([]*sectionedFile, 0, 1)
	for _, file := range sectionedCache.mapped {
		retained = append(retained, file)
	}
	sectionedCache.Unlock()
	if len(retained) != 1 {
		t.Fatalf("%d racers retained %d mappings, want 1", racers, len(retained))
	}
	for racer, sectioned := range loaded {
		if !aliasesMapping(sectioned, retained[0].mapping) {
			t.Fatalf("racer %d returned an index outside the retained mapping", racer)
		}
	}
}
