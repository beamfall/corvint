package contextindex

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// escapedSources keeps what a consumer such as internal/doccorpus keeps: the
// Source values copied out of an Index, which outlive it. It also returns a
// heap copy of each body to compare against later.
func escapedSources(index *Index) (map[string]Source, map[string]string) {
	kept := make(map[string]Source, len(index.Sources))
	want := make(map[string]string, len(index.Sources))
	for path, source := range index.Sources {
		kept[path] = source
		want[path] = strings.Clone(string(source.Data))
	}
	return kept, want
}

// checkEscapedSources reads every escaped body, which faults if its mapping
// was unmapped and differs if the address was reused by another file.
func checkEscapedSources(t *testing.T, kept map[string]Source, want map[string]string) {
	t.Helper()
	for path, source := range kept {
		if string(source.Data) != want[path] {
			t.Fatalf("%s: an escaped body changed after its mapping was evicted", path)
		}
	}
}

// TestEvictedMappingsStayValidForEscapedAliases is the use-after-unmap guard
// of the IDX-SNAP-V0-014 and IDX-SNAP-V0-015 retention (V1-0947). Values
// copied out of an Index alias the mapping without keeping the Index or any
// owner reachable, so neither the Index becoming unreachable nor eviction
// proves the bytes unused. An evicted mapping must therefore stay readable;
// a change that unmaps on eviction or on an Index cleanup faults here.
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
		if file, ok := retainedPackFor(receipt.PackPath); !ok {
			t.Fatal("the first read retained no mapping")
		} else if file.mapping != nil && !aliasesMapping(loaded, file.mapping) {
			t.Fatal("the loaded bodies do not alias the mapping, so this guard proves nothing")
		}
		kept, want := escapedSources(loaded)
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
		runtime.GC()
		runtime.GC()
		checkEscapedSources(t, kept, want)
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
		kept, want := escapedSources(loaded)
		loaded = nil
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
		runtime.GC()
		runtime.GC()
		checkEscapedSources(t, kept, want)
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
