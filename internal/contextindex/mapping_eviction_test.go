package contextindex

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// escapedAliases keeps what consumers keep after the Index is gone: whole
// Source values (as internal/doccorpus does) and bare strings taken from the
// Source.Text view and the symbol table, which hold no pointer to any owner.
// want holds a heap copy of each to compare against later.
type escapedAliases struct {
	sources map[string]Source
	strings []string
	want    map[string]string
	wantStr []string
}

func escape(index *Index) escapedAliases {
	kept := escapedAliases{sources: map[string]Source{}, want: map[string]string{}}
	for path, source := range index.Sources {
		kept.sources[path] = source
		kept.want[path] = strings.Clone(string(source.Data))
		if text, valid, _ := source.Text(); valid && text != "" {
			kept.strings = append(kept.strings, text)
		}
	}
	for _, symbol := range index.Symbols {
		kept.strings = append(kept.strings, symbol.Path, symbol.Name)
	}
	for _, value := range kept.strings {
		kept.wantStr = append(kept.wantStr, strings.Clone(value))
	}
	return kept
}

// check reads every escaped alias, which faults if its mapping was unmapped
// and differs if the address was reused by another file.
func (kept escapedAliases) check(t *testing.T) {
	t.Helper()
	for path, source := range kept.sources {
		if string(source.Data) != kept.want[path] {
			t.Fatalf("%s: an escaped body changed after its mapping was evicted", path)
		}
	}
	for at, value := range kept.strings {
		if value != kept.wantStr[at] {
			t.Fatalf("escaped string %d changed after its mapping was evicted", at)
		}
	}
}

// collect runs the collector and gives runtime cleanups, which run
// asynchronously after a collection, time to run. It is best effort: an
// unmap on eviction is caught deterministically, a cleanup-driven one only
// when its callback has run by the time the aliases are read.
func collect() {
	for range 3 {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
}

// TestEvictedMappingsStayValidForEscapedAliases is the use-after-unmap guard
// of the IDX-SNAP-V0-014 and IDX-SNAP-V0-015 retention (V1-0947). Values
// copied out of an Index alias the mapping without keeping the Index or any
// owner reachable, so neither the Index becoming unreachable nor eviction
// proves the bytes unused. An evicted mapping must therefore stay readable;
// a change that unmaps on eviction faults here, and one that unmaps from a
// runtime cleanup faults once that cleanup has run (see collect). Where the
// platform or the mmap call yields no mapping the read copies its bytes and
// there is nothing to guard, so the subtest skips.
func TestEvictedMappingsStayValidForEscapedAliases(t *testing.T) {
	t.Run("pack", func(t *testing.T) {
		index, receipt := packFixture(t)
		identity := fixtureIdentity(index)
		resetPackRetention()
		defer resetPackRetention()
		loaded, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull)
		if err != nil {
			t.Fatal(err)
		}
		file, ok := retainedPackFor(receipt.PackPath)
		if !ok {
			t.Skip("no mapping: the read copied the pack, so no alias can dangle")
		}
		if !aliasesMapping(loaded, file.mapping) {
			t.Fatal("the loaded bodies do not alias the mapping, so this guard proves nothing")
		}
		kept := escape(loaded)
		file = nil
		forgetPackHistory(loaded)
		loaded = nil
		for open := 0; open < packCacheCapacity; open++ {
			other, err := readPackSnapshot(packWithVocabulary(t, index, func(*TermTable) {}), identity, analyzerEngine(), loadFull)
			if err != nil {
				t.Fatal(err)
			}
			forgetPackHistory(other)
		}
		if _, ok := retainedPackFor(receipt.PackPath); ok {
			t.Fatal("the first pack's mapping was not evicted")
		}
		collect()
		kept.check(t)
	})
	t.Run("sectioned", func(t *testing.T) {
		index, receipt := sectionedFixture(t)
		identity := fixtureIdentity(index)
		resetSectionedRetention()
		defer resetSectionedRetention()
		loaded, err := readSectionedSnapshot(receipt.SectionedPath, identity, receipt.Engine, loadFull)
		if err != nil {
			t.Fatal(err)
		}
		file, ok := retainedSectionedFor(receipt.SectionedPath)
		if !ok {
			t.Skip("no mapping: the read copied the file, so no alias can dangle")
		}
		if !aliasesMapping(loaded, file.mapping) {
			t.Fatal("the loaded bodies do not alias the mapping, so this guard proves nothing")
		}
		kept := escape(loaded)
		file, loaded = nil, nil
		content, err := os.ReadFile(receipt.SectionedPath)
		if err != nil {
			t.Fatal(err)
		}
		directory := t.TempDir()
		for open := 0; open < sectionedCacheCapacity; open++ {
			path := filepath.Join(directory, fmt.Sprintf("copy-%d%s", open, sectionedExtension))
			if err := os.WriteFile(path, content, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := readSectionedSnapshot(path, identity, receipt.Engine, loadFull); err != nil {
				t.Fatal(err)
			}
		}
		if retained := retainedSectionedPaths(); retained[receipt.SectionedPath] != 0 {
			t.Fatal("the first file's mapping was not evicted")
		}
		collect()
		kept.check(t)
	})
}

// retainedPackFor returns the retained mapping of the pack at path.
func retainedPackFor(path string) (*packFile, bool) {
	packCache.Lock()
	defer packCache.Unlock()
	for key, file := range packCache.mapped {
		if key.path == path {
			return file, true
		}
	}
	return nil, false
}

// retainedSectionedFor returns the retained mapping of the sectioned file at path.
func retainedSectionedFor(path string) (*sectionedFile, bool) {
	sectionedCache.Lock()
	defer sectionedCache.Unlock()
	for key, file := range sectionedCache.mapped {
		if key.path == path {
			return file, true
		}
	}
	return nil, false
}

// TestEvictedReadsAdoptOneMappingPerFile bounds live mappings by distinct
// files, not cache misses (V1-0947). Before adoption, twenty reads cycling
// five unchanged files through the four-slot ring missed every time and left
// twenty mappings; now a read of an evicted file adopts the mapping its
// first read retained, and the index it returns aliases that mapping.
func TestEvictedReadsAdoptOneMappingPerFile(t *testing.T) {
	const files, reads = sectionedCacheCapacity + 1, 4 * (sectionedCacheCapacity + 1)
	check := func(t *testing.T, mappings map[packKey][]byte, loaded map[string][]*Index) {
		t.Helper()
		if len(mappings) != files {
			t.Fatalf("%d reads of %d files kept %d mappings, want %d", reads, files, len(mappings), files)
		}
		for key, mapping := range mappings {
			for at, index := range loaded[key.path] {
				if !aliasesMapping(index, mapping) {
					t.Fatalf("%s read %d does not alias the mapping its first read kept", key.path, at)
				}
			}
		}
	}
	t.Run("pack", func(t *testing.T) {
		index, _ := packFixture(t)
		identity := fixtureIdentity(index)
		resetPackRetention()
		defer resetPackRetention()
		resetPackMappings()
		defer resetPackMappings()
		paths := make([]string, files)
		for at := range paths {
			paths[at] = packWithVocabulary(t, index, func(*TermTable) {})
		}
		loaded := map[string][]*Index{}
		for read := 0; read < reads; read++ {
			path := paths[read%files]
			packed, err := readPackSnapshot(path, identity, analyzerEngine(), loadFull)
			if err != nil {
				t.Fatal(err)
			}
			forgetPackHistory(packed)
			loaded[path] = append(loaded[path], packed)
		}
		mappings := packMappings.mappings()
		if len(mappings) == 0 {
			t.Skip("no mapping: the reads copied the packs")
		}
		check(t, mappings, loaded)
	})
	t.Run("sectioned", func(t *testing.T) {
		index, receipt := sectionedFixture(t)
		identity := fixtureIdentity(index)
		resetSectionedRetention()
		defer resetSectionedRetention()
		resetSectionedMappings()
		defer resetSectionedMappings()
		content, err := os.ReadFile(receipt.SectionedPath)
		if err != nil {
			t.Fatal(err)
		}
		directory := t.TempDir()
		paths := make([]string, files)
		for at := range paths {
			paths[at] = filepath.Join(directory, fmt.Sprintf("copy-%d%s", at, sectionedExtension))
			if err := os.WriteFile(paths[at], content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		loaded := map[string][]*Index{}
		for read := 0; read < reads; read++ {
			path := paths[read%files]
			sectioned, err := readSectionedSnapshot(path, identity, receipt.Engine, loadFull)
			if err != nil {
				t.Fatal(err)
			}
			loaded[path] = append(loaded[path], sectioned)
		}
		mappings := sectionedMappings.mappings()
		if len(mappings) == 0 {
			t.Skip("no mapping: the reads copied the files")
		}
		check(t, mappings, loaded)
	})
}

// resetPackMappings forgets the live pack mappings without releasing them, so
// a count belongs to one test; indexes from earlier tests still hold theirs.
func resetPackMappings() { packMappings.clear() }

// resetSectionedMappings is resetPackMappings for sectioned files.
func resetSectionedMappings() { sectionedMappings.clear() }

func (table *snapshotMappings) clear() {
	table.mutex.Lock()
	table.live = map[packKey]*snapshotMapping{}
	table.mutex.Unlock()
}

// mappings returns the bytes of each live mapping by key.
func (table *snapshotMappings) mappings() map[packKey][]byte {
	table.mutex.Lock()
	defer table.mutex.Unlock()
	mappings := make(map[packKey][]byte, len(table.live))
	for key, mapping := range table.live {
		mappings[key] = mapping.bytes
	}
	return mappings
}

// TestRetainKeepsOneMappingWhenEvictedDuringDecode replays a race: a read
// opens a file and adopts its key before a concurrent first read of it, whose
// retention is evicted while the first read still decodes. The later read
// shares the mapping the first registered, so retention keeps that one and
// the key never holds a second.
func TestRetainKeepsOneMappingWhenEvictedDuringDecode(t *testing.T) {
	t.Run("pack", func(t *testing.T) {
		index, _ := packFixture(t)
		identity := fixtureIdentity(index)
		resetPackRetention()
		defer resetPackRetention()
		resetPackMappings()
		defer resetPackMappings()
		path := packWithVocabulary(t, index, func(*TermTable) {})
		late, err := openPackFile(path)
		if err != nil {
			t.Fatal(err)
		}
		defer late.close()
		if late.mapping == nil {
			late.release()
			t.Skip("no mapping: the read copies the pack")
		}
		key, err := late.identity()
		if err != nil {
			t.Fatal(err)
		}
		late.adopt(key)
		for _, read := range append([]string{path}, packPaths(t, index, packCacheCapacity)...) {
			packed, err := readPackSnapshot(read, identity, analyzerEngine(), loadFull)
			if err != nil {
				t.Fatal(err)
			}
			forgetPackHistory(packed)
		}
		mappings := packMappings.mappings()
		kept, count := mappings[key], len(mappings)
		if &kept[0] != &late.mapping[0] {
			t.Fatal("a concurrent read of the pack kept a second mapping")
		}
		if retained, lease := retainPack(key, late); retained != nil || lease != nil {
			t.Fatal("retention answered with another file after the first was evicted")
		}
		mappings = packMappings.mappings()
		if len(mappings) != count || &mappings[key][0] != &kept[0] {
			t.Fatal("the live mapping table changed")
		}
	})
	t.Run("sectioned", func(t *testing.T) {
		index, receipt := sectionedFixture(t)
		identity := fixtureIdentity(index)
		resetSectionedRetention()
		defer resetSectionedRetention()
		resetSectionedMappings()
		defer resetSectionedMappings()
		content, err := os.ReadFile(receipt.SectionedPath)
		if err != nil {
			t.Fatal(err)
		}
		directory := t.TempDir()
		paths := make([]string, sectionedCacheCapacity+1)
		for at := range paths {
			paths[at] = filepath.Join(directory, fmt.Sprintf("copy-%d%s", at, sectionedExtension))
			if err := os.WriteFile(paths[at], content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		late, err := openSectionedFile(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		defer late.close()
		if late.mapping == nil {
			late.release()
			t.Skip("no mapping: the read copies the file")
		}
		key, err := late.identity()
		if err != nil {
			t.Fatal(err)
		}
		late.adopt(key)
		for _, path := range paths {
			if _, err := readSectionedSnapshot(path, identity, receipt.Engine, loadFull); err != nil {
				t.Fatal(err)
			}
		}
		mappings := sectionedMappings.mappings()
		kept, count := mappings[key], len(mappings)
		if &kept[0] != &late.mapping[0] {
			t.Fatal("a concurrent read of the file kept a second mapping")
		}
		if retained, lease := retainSectioned(key, late); retained != nil || lease != nil {
			t.Fatal("retention answered with another file after the first was evicted")
		}
		mappings = sectionedMappings.mappings()
		if len(mappings) != count || &mappings[key][0] != &kept[0] {
			t.Fatal("the live mapping table changed")
		}
	})
}

// packPaths writes count distinct packs of index.
func packPaths(t *testing.T, index *Index, count int) []string {
	t.Helper()
	paths := make([]string, count)
	for at := range paths {
		paths[at] = packWithVocabulary(t, index, func(*TermTable) {})
	}
	return paths
}
