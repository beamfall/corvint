package contextindex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

// leaseAliases holds the values a caller can keep after it drops an Index:
// symbol strings, source strings, a Source value as internal/doccorpus keeps
// one, the text a body read returned, a Tracked key and a vocabulary path.
type leaseAliases struct {
	symbolPath, symbolName, symbolKind, symbolBlob string
	source                                         Source
	text                                           string
	tracked, vocabularyPath                        string
}

// keepAliases takes leaseAliases from index and the heap copies they must
// still equal later. The aliases are taken exactly as callers take them, so
// on a mapped read they point into the mapping unless the reader copied them.
func keepAliases(t *testing.T, index *Index) (leaseAliases, leaseAliases) {
	t.Helper()
	symbol := index.Symbols[slices.IndexFunc(index.Symbols, func(symbol Symbol) bool { return symbol.Name != "" })]
	paths := slices.Sorted(func(yield func(string) bool) {
		for path := range index.Sources {
			if !yield(path) {
				return
			}
		}
	})
	var source Source
	for _, path := range paths {
		if text, valid, loaded := index.Sources[path].Text(); loaded && valid && text != "" {
			source = index.Sources[path]
			break
		}
	}
	if source.Path == "" {
		t.Fatal("fixture has no readable source")
	}
	text, _, _ := source.Text()
	var tracked string
	for path := range index.Tracked {
		tracked = path
		break
	}
	kept := leaseAliases{
		symbolPath: symbol.Path, symbolName: symbol.Name, symbolKind: symbol.Kind, symbolBlob: symbol.BlobHash,
		source: source, text: text, tracked: tracked, vocabularyPath: index.Vocabulary.Paths[0],
	}
	want := leaseAliases{
		symbolPath: strings.Clone(symbol.Path), symbolName: strings.Clone(symbol.Name), symbolKind: strings.Clone(symbol.Kind),
		symbolBlob: strings.Clone(symbol.BlobHash), text: strings.Clone(text), tracked: strings.Clone(tracked),
		vocabularyPath: strings.Clone(index.Vocabulary.Paths[0]),
		source:         Source{Path: strings.Clone(source.Path), BlobHash: strings.Clone(source.BlobHash), Mode: strings.Clone(source.Mode)},
	}
	return kept, want
}

// checkAliases reads every kept alias byte by byte; a dangling alias faults
// or reads different bytes.
func checkAliases(t *testing.T, kept, want leaseAliases, withText bool) {
	t.Helper()
	pairs := [][2]string{
		{kept.symbolPath, want.symbolPath}, {kept.symbolName, want.symbolName}, {kept.symbolKind, want.symbolKind},
		{kept.symbolBlob, want.symbolBlob}, {kept.source.Path, want.source.Path}, {kept.source.BlobHash, want.source.BlobHash},
		{kept.source.Mode, want.source.Mode}, {kept.tracked, want.tracked}, {kept.vocabularyPath, want.vocabularyPath},
	}
	if withText {
		pairs = append(pairs, [2]string{kept.text, want.text})
		text, _, _ := kept.source.Text()
		pairs = append(pairs, [2]string{text, want.text})
	}
	for index, pair := range pairs {
		if pair[0] != pair[1] {
			t.Fatalf("alias %d reads %q, want %q", index, pair[0], pair[1])
		}
	}
}

// snapshotCopies writes count copies of path under distinct names; each is a
// distinct retention key.
func snapshotCopies(t *testing.T, path string, count int) []string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	copies := make([]string, count)
	for index := range copies {
		copies[index] = filepath.Join(directory, fmt.Sprintf("copy-%d%s", index, filepath.Ext(path)))
		if err := os.WriteFile(copies[index], content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return copies
}

// evictPacks reads count distinct copies of path, releasing each index it
// reads, so every earlier retention leaves the ring.
func evictPacks(t *testing.T, path string, identity repositoryIdentity, count int) {
	t.Helper()
	for _, copied := range snapshotCopies(t, path, count) {
		evicting, err := readPackSnapshot(copied, identity, analyzerEngine(), loadFull)
		if err != nil {
			t.Fatal(err)
		}
		evicting.Release()
	}
}

// retainedOwner returns the mapping the pack ring retains for path.
func retainedOwner(t *testing.T, path string) *snapshotMapping {
	t.Helper()
	packCache.Lock()
	defer packCache.Unlock()
	for key, file := range packCache.mapped {
		if key.path == path {
			return file.owner
		}
	}
	t.Fatalf("%s is not retained", path)
	return nil
}

// TestEvictedPackMappingKeepsEscapedAliasesValid is the guard for V1-0983: a
// caller keeps aliases from a pack read, drops the Index without releasing it,
// the ring evicts the mapping, and the collector runs. Unmapping on eviction,
// or when the Index becomes unreachable, faults here (IDX-SNAP-V0-028).
func TestEvictedPackMappingKeepsEscapedAliasesValid(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	resetPackRetention()
	defer resetPackRetention()
	loaded, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull)
	if err != nil {
		t.Fatal(err)
	}
	kept, want := keepAliases(t, loaded)
	loaded = nil
	runtime.GC()
	evictPacks(t, receipt.PackPath, identity, packCacheCapacity)
	runtime.GC()
	runtime.GC()
	checkAliases(t, kept, want, true)
}

// TestReleasedEvictedPackMappingUnmaps: once a read is released and the ring
// evicts its pack, the mapping is unmapped, and every string the read handed
// out, plus a Source whose Data the caller copied as internal/doccorpus does,
// still reads correctly (IDX-SNAP-V0-028, IDX-SNAP-V0-029).
func TestReleasedEvictedPackMappingUnmaps(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	resetPackRetention()
	defer resetPackRetention()
	loaded, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull)
	if err != nil {
		t.Fatal(err)
	}
	owner := retainedOwner(t, receipt.PackPath)
	kept, want := keepAliases(t, loaded)
	copied := kept.source
	copied.Data = []byte(kept.text)
	if refs := owner.refs.Load(); refs != 2 {
		t.Fatalf("a retained, unreleased read holds %d references, want 2", refs)
	}
	shared := *loaded
	shared.Release()
	loaded.Release()
	if refs := owner.refs.Load(); refs != 1 {
		t.Fatalf("releasing an index and its copy left %d references, want the ring's 1", refs)
	}
	loaded = nil
	evictPacks(t, receipt.PackPath, identity, packCacheCapacity)
	if refs := owner.refs.Load(); refs != 0 {
		t.Fatalf("the evicted, released mapping holds %d references, want 0 (unmapped)", refs)
	}
	runtime.GC()
	checkAliases(t, kept, want, false)
	if text, valid, loaded := copied.Text(); !valid || !loaded || text != want.text {
		t.Fatal("the copied source no longer reads its text")
	}
}

// TestCompactPackReadTakesNoLease: a compact read copies its one field, so it
// holds no reference and its Release is a no-op.
func TestCompactPackReadTakesNoLease(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	resetPackRetention()
	defer resetPackRetention()
	full, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Release()
	owner := retainedOwner(t, receipt.PackPath)
	compact, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadCompact)
	if err != nil {
		t.Fatal(err)
	}
	if compact.lease != nil || owner.refs.Load() != 2 {
		t.Fatalf("a compact read took a lease (%v, %d references)", compact.lease != nil, owner.refs.Load())
	}
	compact.Release()
}

// TestDistinctSnapshotReadsKeepBoundedMappings reads three ring capacities of
// distinct snapshot files, releasing each read and deleting each file after
// it. At most the ring's capacity stays mapped, and every evicted mapping of a
// deleted file is unmapped, its descriptor already closed, so nothing holds
// the file's blocks (IDX-SNAP-V0-028, IDX-SNAP-V0-030).
func TestDistinctSnapshotReadsKeepBoundedMappings(t *testing.T) {
	cases := []struct {
		name     string
		fixture  func(*testing.T) (*Index, SnapshotReceipt)
		path     func(SnapshotReceipt) string
		read     func(string, repositoryIdentity, SnapshotReceipt) (*Index, error)
		reset    func()
		capacity int
	}{
		{"pack", packFixture, func(receipt SnapshotReceipt) string { return receipt.PackPath },
			func(path string, identity repositoryIdentity, _ SnapshotReceipt) (*Index, error) {
				return readPackSnapshot(path, identity, analyzerEngine(), loadFull)
			}, resetPackRetention, packCacheCapacity},
		{"sectioned", sectionedFixture, func(receipt SnapshotReceipt) string { return receipt.SectionedPath },
			func(path string, identity repositoryIdentity, receipt SnapshotReceipt) (*Index, error) {
				return readSectionedSnapshot(path, identity, receipt.Engine, loadFull)
			}, resetSectionedRetention, sectionedCacheCapacity},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			index, receipt := test.fixture(t)
			identity := fixtureIdentity(index)
			test.reset()
			defer test.reset()
			base := liveSnapshotMappings.Load()
			for _, path := range snapshotCopies(t, test.path(receipt), 3*test.capacity) {
				loaded, err := test.read(path, identity, receipt)
				if err != nil {
					t.Fatal(err)
				}
				kept, want := keepAliasesLight(loaded)
				loaded.Release()
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if live := liveSnapshotMappings.Load() - base; live > int64(test.capacity) {
					t.Fatalf("%d mappings live after releasing every read, want at most %d", live, test.capacity)
				}
				if kept != want {
					t.Fatalf("a released read's symbol path reads %q, want %q", kept, want)
				}
			}
			test.reset()
			if live := liveSnapshotMappings.Load() - base; live != 0 {
				t.Fatalf("%d mappings of deleted, released, evicted files are still mapped", live)
			}
		})
	}
}

// keepAliasesLight keeps one symbol path and its heap copy.
func keepAliasesLight(index *Index) (string, string) {
	for _, symbol := range index.Symbols {
		if symbol.Path != "" {
			return symbol.Path, strings.Clone(symbol.Path)
		}
	}
	return "", ""
}

// TestConcurrentSnapshotReadReleaseEvict reads, uses, and releases indexes
// from more distinct packs than the ring holds on several goroutines, so
// leases, retained hits, and evictions interleave. Run under -race; when every
// read is released and the ring is reset, nothing stays mapped
// (IDX-SNAP-V0-028).
func TestConcurrentSnapshotReadReleaseEvict(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	resetPackRetention()
	defer resetPackRetention()
	base := liveSnapshotMappings.Load()
	paths := snapshotCopies(t, receipt.PackPath, packCacheCapacity+2)
	loads := []snapshotLoad{loadFull, loadContext, loadEvent, loadEventDeferred, loadCompact}
	const workers, rounds = 8, 24
	failures := make([]error, workers)
	var pending sync.WaitGroup
	for worker := range workers {
		pending.Add(1)
		go func() {
			defer pending.Done()
			for round := range rounds {
				load := loads[(worker+round)%len(loads)]
				loaded, err := readPackSnapshot(paths[(worker*rounds+round)%len(paths)], identity, analyzerEngine(), load)
				if err != nil {
					failures[worker] = err
					return
				}
				for _, source := range loaded.Sources {
					if text, valid, _ := source.Text(); valid && len(text) > 0 && text[0] == 0 {
						failures[worker] = fmt.Errorf("source %s reads a NUL", source.Path)
					}
					break
				}
				loaded.Release()
			}
		}()
	}
	pending.Wait()
	if err := errors.Join(failures...); err != nil {
		t.Fatal(err)
	}
	resetPackRetention()
	if live := liveSnapshotMappings.Load() - base; live != 0 {
		t.Fatalf("%d mappings stay mapped after every read was released and the ring reset", live)
	}
}

// equalIgnoringLease compares decoded indexes without their lease. The lease
// is a per-read reference to the mapping, never decoded content, so a gob
// decode (no mapping) and a pack or sectioned decode of the same snapshot
// differ only there.
func equalIgnoringLease(want, got *Index) bool {
	if want == nil || got == nil {
		return reflect.DeepEqual(want, got)
	}
	wantLease, gotLease := want.lease, got.lease
	want.lease, got.lease = nil, nil
	defer func() { want.lease, got.lease = wantLease, gotLease }()
	return reflect.DeepEqual(want, got)
}

// TestAdoptedPackMappingUnmapsAfterLastRelease joins V1-0947 and V1-0983: a
// read that misses the ring while an unreleased Index keeps the file's mapping
// live adopts that mapping instead of mapping the file again, and once every
// Index and the ring release it, the mapping leaves the identity table and is
// unmapped (IDX-SNAP-V0-028, IDX-SNAP-V0-030).
func TestAdoptedPackMappingUnmapsAfterLastRelease(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	resetPackRetention()
	defer resetPackRetention()
	resetPackMappings()
	defer resetPackMappings()
	first, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull)
	if err != nil {
		t.Fatal(err)
	}
	owner := retainedOwner(t, receipt.PackPath)
	resetPackRetention()
	second, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull)
	if err != nil {
		t.Fatal(err)
	}
	if retainedOwner(t, receipt.PackPath) != owner {
		t.Fatal("a read of a live mapping's file mapped it again")
	}
	if refs := owner.refs.Load(); refs != 3 {
		t.Fatalf("the adopted mapping holds %d references, want the ring's and two reads'", refs)
	}
	first.Release()
	second.Release()
	resetPackRetention()
	if refs := owner.refs.Load(); refs != 0 {
		t.Fatalf("the released mapping holds %d references, want 0 (unmapped)", refs)
	}
	for key := range packMappings.mappings() {
		if key.path == receipt.PackPath {
			t.Fatal("an unmapped mapping stayed in the identity table")
		}
	}
}
