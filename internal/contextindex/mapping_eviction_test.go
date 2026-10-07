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
		sectionedCache.Lock()
		sectionedCache.mapped, sectionedCache.ring, sectionedCache.next = map[packKey]*sectionedFile{}, [sectionedCacheCapacity]packKey{}, 0
		sectionedCache.Unlock()
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
