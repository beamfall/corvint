package contextindex

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

// packFixture is taskContextFixture with a second commit touching the
// subject, so the co-change slot has history to count over, indexed under
// the pack opt-in.
func packFixture(t *testing.T) (*Index, SnapshotReceipt) {
	t.Helper()
	t.Setenv(snapshotFormatEnv, packFormatValue)
	root := taskContextFixture(t).Root
	demux := filepath.Join(root, "cache", "demux.go")
	if err := os.WriteFile(demux, []byte("package cache\n\nfunc Split(key string) string { return key + \"\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "-A")
	testGit(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "touch demux")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.PackPath == "" || receipt.PackBytes <= packHeaderSlot {
		t.Fatalf("pack receipt = %+v", receipt)
	}
	return index, receipt
}

func TestPackSnapshotDecodesEverySectionToTheGobValues(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	for _, load := range []snapshotLoad{loadFull, loadEvent, loadCompact} {
		gob := gobDecoded(t, receipt, identity, load)
		packed, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), load)
		if err != nil {
			t.Fatalf("load %d: %v", load, err)
		}
		if !reflect.DeepEqual(gob, packed) {
			t.Fatalf("load %d: pack decode differs from the gob decode\n gob: %+v\npack: %+v", load, gob, packed)
		}
		history, ok := packHistory(packed)
		if ok != (load == loadFull) {
			t.Fatalf("load %d: history registered = %v", load, ok)
		}
		if !ok {
			continue
		}
		spawned, err := readCoChangeHistory(context.Background(), index)
		if err != nil {
			t.Fatal(err)
		}
		if len(spawned) < 2 || !reflect.DeepEqual(spawned, history) {
			t.Fatalf("cochange section differs from git log:\n git: %+v\npack: %+v", spawned, history)
		}
	}
}

func TestPackTermViewFindMatchesTermPostings(t *testing.T) {
	index, _ := packFixture(t)
	for name, postings := range map[string]termPostings{"terms": index.Vocabulary.Terms, "words": index.Vocabulary.Words, "pathterms": index.Vocabulary.PathTerms} {
		var encoded bytes.Buffer
		if err := termTableSection(name, postings).write(&encoded); err != nil {
			t.Fatal(err)
		}
		view, err := viewTermTable(encoded.Bytes())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := view.check(len(index.Vocabulary.Paths), name == "terms"); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(view.postings(), postings) {
			t.Fatalf("%s: view differs from the table", name)
		}
		for key := 0; key < postings.keyCount(); key++ {
			wantLow, wantHigh, wantOK := postings.find(postings.key(key))
			low, high, ok := view.find(postings.key(key))
			if low != wantLow || high != wantHigh || ok != wantOK {
				t.Fatalf("%s %q: view find = %d %d %v, table find = %d %d %v", name, postings.key(key), low, high, ok, wantLow, wantHigh, wantOK)
			}
		}
		if _, _, ok := view.find("\x00no such key"); ok {
			t.Fatalf("%s: found a missing key", name)
		}
	}
}

func copiedPostings(postings termPostings) termPostings {
	copied := postings
	copied.KeyOffsets = append([]uint32(nil), postings.KeyOffsets...)
	copied.Offsets = append([]uint32(nil), postings.Offsets...)
	copied.Sources = append([]uint32(nil), postings.Sources...)
	copied.Counts = append([]uint32(nil), postings.Counts...)
	return copied
}

// packWithVocabulary writes a fresh pack for index whose vocabulary is the
// fixture's with mutate applied, so every block digest, digest table and
// header covers the malformed table: the reader has to refuse the table
// itself rather than a checksum.
func packWithVocabulary(t *testing.T, index *Index, mutate func(*TermTable)) string {
	t.Helper()
	vocabulary := &TermTable{
		Paths:     index.Vocabulary.Paths,
		Terms:     copiedPostings(index.Vocabulary.Terms),
		Words:     copiedPostings(index.Vocabulary.Words),
		PathTerms: copiedPostings(index.Vocabulary.PathTerms),
	}
	mutate(vocabulary)
	malformed := *index
	malformed.Vocabulary = vocabulary
	path := filepath.Join(t.TempDir(), "vocabulary"+packExtension)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = encodePackSnapshot(file, &malformed, analyzerEngine(), nil, "")
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// TestPackRefusesMalformedTermTablesUnderValidChecksums covers the structural
// faults a consistently signed pack can still carry: a posting naming a
// source outside Paths, which the ranking indexes into Paths and the document
// lengths, and a counted Terms table without the Counts the same ranking
// reads beside Sources. Words and PathTerms carry no counts by construction.
func TestPackRefusesMalformedTermTablesUnderValidChecksums(t *testing.T) {
	index, _ := packFixture(t)
	identity := fixtureIdentity(index)
	read := func(path string) error {
		loaded, err := readPackSnapshot(path, identity, analyzerEngine(), loadFull)
		if err == nil {
			forgetPackHistory(loaded)
		}
		return err
	}
	if err := read(packWithVocabulary(t, index, func(*TermTable) {})); err != nil {
		t.Fatalf("the rewritten pack must load unmutated: %v", err)
	}
	malformed := []struct {
		name   string
		mutate func(*TermTable)
	}{
		{"terms source one past the paths", func(table *TermTable) {
			table.Terms.Sources[0] = uint32(len(table.Paths))
		}},
		{"words source far outside the paths", func(table *TermTable) {
			table.Words.Sources[len(table.Words.Sources)-1] = ^uint32(0)
		}},
		{"pathterms source one past the paths", func(table *TermTable) {
			table.PathTerms.Sources[0] = uint32(len(table.Paths))
		}},
		{"counted terms without counts", func(table *TermTable) {
			table.Terms.Counts = nil
		}},
		{"counted terms with short counts", func(table *TermTable) {
			table.Terms.Counts = table.Terms.Counts[:len(table.Terms.Counts)-1]
		}},
	}
	for _, item := range malformed {
		if err := read(packWithVocabulary(t, index, item.mutate)); err == nil {
			t.Fatalf("%s: the pack was not refused", item.name)
		}
	}
}

func rewritePack(t *testing.T, path string, mutate func(content []byte, header packHeader) []byte) {
	t.Helper()
	file, err := openPackFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header, err := file.header()
	file.close()
	file.release()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, mutate(content, header), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackSnapshotRefusesCorruptionTruncationAndOutOfRangeAsAMiss(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	good, err := os.ReadFile(receipt.PackPath)
	if err != nil {
		t.Fatal(err)
	}
	restore := func() {
		if err := os.WriteFile(receipt.PackPath, good, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	refuse := func(name string) {
		t.Helper()
		if _, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull); err == nil {
			t.Fatalf("%s: the pack was not refused", name)
		}
	}
	// One flipped byte in every block of every section.
	var header packHeader
	rewritePack(t, receipt.PackPath, func(content []byte, found packHeader) []byte { header = found; return content })
	for _, entry := range header.Sections {
		for block := uint64(0); block < packBlockCount(entry.Length); block++ {
			restore()
			rewritePack(t, receipt.PackPath, func(content []byte, _ packHeader) []byte {
				content[entry.Offset+block*packBlockSize] ^= 0xff
				return content
			})
			refuse(entry.Name + " block")
		}
		restore()
		rewritePack(t, receipt.PackPath, func(content []byte, _ packHeader) []byte {
			content[entry.DigestsOffset] ^= 0xff
			return content
		})
		refuse(entry.Name + " digest table")
	}
	// The header slot, a truncated file, and a section table naming bytes
	// outside the file.
	restore()
	rewritePack(t, receipt.PackPath, func(content []byte, _ packHeader) []byte { content[len(packMagic)+8] ^= 0xff; return content })
	refuse("header table")
	restore()
	rewritePack(t, receipt.PackPath, func(content []byte, _ packHeader) []byte { return content[:len(content)/2] })
	refuse("truncation")
	restore()
	rewritePack(t, receipt.PackPath, func(content []byte, found packHeader) []byte {
		found.Sections[3].Offset = uint64(len(content)) + 1
		reheaded, err := encodePackHeader(found)
		if err != nil {
			t.Fatal(err)
		}
		copy(content, reheaded)
		return content
	})
	refuse("out-of-range section offset")
	// A refused pack is the gob hit: the same index, no error.
	loaded, hit, err := LoadSnapshot(context.Background(), index.Root)
	if err != nil || !hit {
		t.Fatalf("refused pack must fall back to the gob: hit=%v err=%v", hit, err)
	}
	if _, ok := packHistory(loaded); ok {
		t.Fatal("the gob fallback registered a pack history")
	}
	if !reflect.DeepEqual(loaded.Vocabulary, index.Vocabulary) || len(loaded.Sources) != len(index.Sources) {
		t.Fatal("the gob fallback decoded a different index")
	}
	// With the gob gone the refusal is the snapshot's existing miss.
	if err := os.Remove(receipt.Path); err != nil {
		t.Fatal(err)
	}
	loaded, hit, err = LoadSnapshot(context.Background(), index.Root)
	if err != nil || hit || loaded != nil {
		t.Fatalf("corrupt pack without a gob must miss: hit=%v err=%v", hit, err)
	}
}

// TestPackSnapshotRefusesDuplicateOrOverlappingSections: a forged section
// table whose entries repeat a name or alias another section's bytes keeps
// every digest in agreement, so only the table check refuses it; a compact
// read, which touches one section, is refused too.
func TestPackSnapshotRefusesDuplicateOrOverlappingSections(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	good, err := os.ReadFile(receipt.PackPath)
	if err != nil {
		t.Fatal(err)
	}
	forgeries := map[string]func(sections []packSectionEntry) []packSectionEntry{
		"duplicate name": func(sections []packSectionEntry) []packSectionEntry { return append(sections, sections[0]) },
		"aliased bytes": func(sections []packSectionEntry) []packSectionEntry {
			last := &sections[len(sections)-1]
			last.Offset, last.Length, last.DigestsOffset, last.DigestsSHA256 = sections[0].Offset, sections[0].Length, sections[0].DigestsOffset, sections[0].DigestsSHA256
			return sections
		},
	}
	for name, forge := range forgeries {
		if err := os.WriteFile(receipt.PackPath, good, 0o644); err != nil {
			t.Fatal(err)
		}
		rewritePack(t, receipt.PackPath, func(content []byte, found packHeader) []byte {
			found.Sections = forge(found.Sections)
			reheaded, err := encodePackHeader(found)
			if err != nil {
				t.Fatal(err)
			}
			copy(content, reheaded)
			return content
		})
		if _, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadCompact); err == nil {
			t.Fatalf("%s: the forged section table was served", name)
		}
	}
}

func TestPackCochangeServesTheSubjectHitAndFallsBackAfterAnAmend(t *testing.T) {
	index, receipt := packFixture(t)
	ctx := context.Background()
	packet := func(t *testing.T, format string, wantHistory bool) map[string]any {
		t.Helper()
		t.Setenv(snapshotFormatEnv, format)
		loaded, hit, err := LoadSnapshot(ctx, index.Root)
		if err != nil || !hit {
			t.Fatalf("%q: hit=%v err=%v", format, hit, err)
		}
		if _, ok := packHistory(loaded); ok != wantHistory {
			t.Fatalf("%q: pack history registered = %v", format, ok)
		}
		result, err := TaskContext(ctx, loaded, "does Split keep empty keys", "cache/demux.go", 20)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	viaPack, viaGob := packet(t, packFormatValue, true), packet(t, "", false)
	if !reflect.DeepEqual(viaPack, viaGob) {
		t.Fatalf("pack packet differs from the gob packet:\npack: %v\n gob: %v", viaPack, viaGob)
	}
	if rows := contextRowsByKind(t, viaPack)["cochange"]; len(rows) == 0 {
		t.Fatalf("fixture produced no cochange rows: %v", viaPack)
	}
	// An amend keeps the tree and moves the commit: the pack still hits, its
	// cochange section is ignored, and the slot spawns as before.
	testGit(t, index.Root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-q", "--amend", "--allow-empty", "-m", "amended")
	if strings.TrimSpace(testGit(t, index.Root, "rev-parse", "HEAD^{tree}")) != index.Revision {
		t.Fatal("amend changed the tree")
	}
	afterPack, afterGob := packet(t, packFormatValue, false), packet(t, "", false)
	if !reflect.DeepEqual(afterPack, afterGob) {
		t.Fatalf("after the amend the pack packet differs from the gob packet:\npack: %v\n gob: %v", afterPack, afterGob)
	}
	if _, err := os.Stat(receipt.PackPath); err != nil {
		t.Fatal(err)
	}
}

// TestPackCochangeMatchesTheSpawnAfterAShallowCloneDeepens: unshallowing
// keeps HEAD and the tree but changes what `git log HEAD` returns, so a pack
// written in the shallow clone must not answer the co-change slot afterwards.
func TestPackCochangeMatchesTheSpawnAfterAShallowCloneDeepens(t *testing.T) {
	source, _ := packFixture(t)
	root := filepath.Join(t.TempDir(), "clone")
	testGit(t, source.Root, "clone", "-q", "--depth", "1", "file://"+source.Root, root)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "fetch", "-q", "--unshallow")
	packet := func(format string) map[string]any {
		t.Setenv(snapshotFormatEnv, format)
		loaded, hit, err := LoadSnapshot(context.Background(), root)
		if err != nil || !hit {
			t.Fatalf("%q: hit=%v err=%v", format, hit, err)
		}
		result, err := TaskContext(context.Background(), loaded, "does Split keep empty keys", "cache/demux.go", 20)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	viaPack, viaGob := packet(packFormatValue), packet("")
	if len(contextRowsByKind(t, viaGob)["cochange"]) == 0 {
		t.Fatalf("deepened clone produced no cochange rows: %v", viaGob)
	}
	if !reflect.DeepEqual(viaPack, viaGob) {
		t.Fatalf("pack packet differs from the gob packet after unshallow:\npack: %v\n gob: %v", viaPack, viaGob)
	}
}

// TestPackCochangeMatchesTheSpawnAfterTheHistoryIsCutAtTheSameHead: a pack
// written in a complete clone must not answer the co-change slot once a
// shallow fetch or a grafts file cuts what `git log HEAD` returns at the same
// HEAD.
func TestPackCochangeMatchesTheSpawnAfterTheHistoryIsCutAtTheSameHead(t *testing.T) {
	source, _ := packFixture(t)
	cuts := map[string]func(t *testing.T, root string){
		"shallow fetch": func(t *testing.T, root string) { testGit(t, root, "fetch", "-q", "--depth", "1", "origin") },
		"grafts file": func(t *testing.T, root string) {
			grafts := filepath.Join(root, ".git", "info", "grafts")
			if err := os.WriteFile(grafts, []byte(source.CommitRevision+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, cut := range cuts {
		t.Run(name, func(t *testing.T) {
			t.Setenv(snapshotFormatEnv, packFormatValue)
			root := filepath.Join(t.TempDir(), "clone")
			testGit(t, source.Root, "clone", "-q", "file://"+source.Root, root)
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := WriteSnapshot(index); err != nil {
				t.Fatal(err)
			}
			cut(t, root)
			if strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")) != index.CommitRevision {
				t.Fatal("the cut moved HEAD")
			}
			packet := func(format string) map[string]any {
				t.Setenv(snapshotFormatEnv, format)
				loaded, hit, err := LoadSnapshot(context.Background(), root)
				if err != nil || !hit {
					t.Fatalf("%q: hit=%v err=%v", format, hit, err)
				}
				result, err := TaskContext(context.Background(), loaded, "does Split keep empty keys", "cache/demux.go", 20)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			viaPack, viaGob := packet(packFormatValue), packet("")
			if !reflect.DeepEqual(viaPack, viaGob) {
				t.Fatalf("pack packet differs from the gob packet after the cut:\npack: %v\n gob: %v", viaPack, viaGob)
			}
		})
	}
}

// retainedPackPaths counts the retained mappings by pack path.
func retainedPackPaths() map[string]int {
	packCache.Lock()
	defer packCache.Unlock()
	paths := make(map[string]int, len(packCache.mapped))
	for key := range packCache.mapped {
		paths[key.path]++
	}
	return paths
}

func registeredPackHistories() int {
	packHistories.mutex.Lock()
	defer packHistories.mutex.Unlock()
	return len(packHistories.entries)
}

// TestForgetPackHistoryReleasesRingSlot: a losing first read forgets the
// index it never handed out. The ring slot kept that index reachable until a
// later registration reused the slot (IDX-SNAP-V0-015).
func TestForgetPackHistoryReleasesRingSlot(t *testing.T) {
	resetPackRetention()
	defer resetPackRetention()
	loser := &Index{}
	registerPackHistory(loser, nil)
	forgetPackHistory(loser)
	if slices.Contains(packHistories.ring[:], loser) {
		t.Fatal("the forgotten index is still held by the history ring")
	}
}

// resetPackRetention drops what earlier tests retained so a count belongs to
// one test. Dropping the reference is what eviction does; the mapping stays
// valid for the indexes that alias it.
func resetPackRetention() {
	packCache.Lock()
	packCache.mapped, packCache.ring, packCache.next = map[packKey]*packFile{}, [packCacheCapacity]packKey{}, 0
	packCache.Unlock()
	packHistories.mutex.Lock()
	packHistories.entries, packHistories.ring, packHistories.next = map[*Index][]historyEntry{}, [packHistoryCapacity]*Index{}, 0
	packHistories.mutex.Unlock()
}

// TestPackRepeatedOpensRetainBoundedMappingsAndHistories bounds what a
// long-lived reader keeps. Before the bound, 200 opens of the 22,976-byte
// fixture pack retained 200 mappings (4.38 MiB) and 200 history
// registrations, and nothing released either.
func TestPackRepeatedOpensRetainBoundedMappingsAndHistories(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	resetPackRetention()
	const opens = 64
	loaded := make([]*Index, 0, opens)
	for open := 0; open < opens; open++ {
		packed, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull)
		if err != nil {
			t.Fatalf("open %d: %v", open, err)
		}
		loaded = append(loaded, packed)
	}
	if retained := retainedPackPaths(); len(retained) != 1 || retained[receipt.PackPath] != 1 {
		t.Fatalf("%d opens of one pack retained %v, want one mapping of %s", opens, retained, receipt.PackPath)
	}
	if !reflect.DeepEqual(loaded[0], loaded[opens-1]) {
		t.Fatal("the reused mapping decoded a different index")
	}
	newest, ok := packHistory(loaded[opens-1])
	if !ok || len(newest) < 2 {
		t.Fatalf("the newest load lost its history: registered=%v entries=%d", ok, len(newest))
	}
	if oldest, registered := packHistory(loaded[0]); registered {
		t.Fatalf("the oldest of %d registrations was retained with %d entries", opens, len(oldest))
	}
	if count := registeredPackHistories(); count != packHistoryCapacity {
		t.Fatalf("%d opens retained %d histories, want %d", opens, count, packHistoryCapacity)
	}
	// Distinct packs fill the ring and the oldest retention is evicted first.
	resetPackRetention()
	paths := make([]string, 0, packCacheCapacity+1)
	for open := 0; open <= packCacheCapacity; open++ {
		path := packWithVocabulary(t, index, func(*TermTable) {})
		if _, err := readPackSnapshot(path, identity, analyzerEngine(), loadFull); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		paths = append(paths, path)
	}
	retained := retainedPackPaths()
	if len(retained) != packCacheCapacity {
		t.Fatalf("%d distinct packs retained %d mappings, want %d", len(paths), len(retained), packCacheCapacity)
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

// aliasesMapping reports whether every non-empty source body of index points
// into mapping.
func aliasesMapping(index *Index, mapping []byte) bool {
	low := uintptr(unsafe.Pointer(unsafe.SliceData(mapping)))
	for _, source := range index.Sources {
		if len(source.Data) == 0 {
			continue
		}
		at := uintptr(unsafe.Pointer(unsafe.SliceData(source.Data)))
		if at < low || at >= low+uintptr(len(mapping)) {
			return false
		}
	}
	return true
}

// TestPackConcurrentFirstReadsShareOneMapping races first reads of one pack.
// Before the fix each racer that lost the retention kept its own mapping,
// aliased by the index it returned and never unmapped.
func TestPackConcurrentFirstReadsShareOneMapping(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)
	resetPackRetention()
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
			loaded[racer], failures[racer] = readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull)
		}()
	}
	close(start)
	pending.Wait()
	if err := errors.Join(failures...); err != nil {
		t.Fatal(err)
	}
	packCache.Lock()
	retained := make([]*packFile, 0, 1)
	for _, file := range packCache.mapped {
		retained = append(retained, file)
	}
	packCache.Unlock()
	if len(retained) != 1 {
		t.Fatalf("%d racers retained %d mappings, want 1", racers, len(retained))
	}
	for racer, packed := range loaded {
		if !aliasesMapping(packed, retained[0].mapping) {
			t.Fatalf("racer %d returned an index outside the retained mapping", racer)
		}
	}
}

// TestPackContextLoadVerifiesOnlyTheBodiesAPacketReads pins the context load
// of IDX-SNAP-V0-015: bodies are verified on first read, a corrupt body no
// read touches leaves the packet unchanged, and a corrupt body a read touches
// is served unloaded and withholds the packet as ErrSnapshotRefused.
func TestPackContextLoadVerifiesOnlyTheBodiesAPacketReads(t *testing.T) {
	index, receipt := packFixture(t)
	large := "docs/large.txt"
	if err := os.WriteFile(filepath.Join(index.Root, large), bytes.Repeat([]byte("filler prose line\n"), 3*packBlockSize/18+1), 0o644); err != nil {
		t.Fatal(err)
	}
	testGit(t, index.Root, "add", "-A")
	testGit(t, index.Root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "large")
	index, err := Build(context.Background(), index.Root)
	if err != nil {
		t.Fatal(err)
	}
	if receipt, err = WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	identity, ctx, task := fixtureIdentity(index), context.Background(), "does Split keep empty keys"
	read := func() *Index {
		t.Helper()
		loaded, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadContext)
		if err != nil {
			t.Fatal(err)
		}
		return loaded
	}
	deferred := read()
	for path, source := range index.Sources {
		want, wantValid, wantLoaded := source.Text()
		got, valid, loaded := deferred.Sources[path].Text()
		if got != want || valid != wantValid || loaded != wantLoaded {
			t.Fatalf("%s: deferred Text differs from the build", path)
		}
	}
	built, err := TaskContext(ctx, index, task, "cache/demux.go", 20)
	if err != nil {
		t.Fatal(err)
	}
	body := read().Sources[large].body
	if body.length < 3*packBlockSize {
		t.Fatalf("%s is not a deferred body spanning three blocks: %+v", large, body)
	}
	rewritePack(t, receipt.PackPath, func(content []byte, found packHeader) []byte {
		bodies, _ := found.entry(packSectionBodies)
		content[bodies.Offset+body.offset+body.length/2] ^= 0xff
		return content
	})
	if _, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadFull); err == nil {
		t.Fatal("the full load did not refuse the corrupt body")
	}
	corrupt := read()
	packet, err := TaskContext(ctx, corrupt, task, "cache/demux.go", 20)
	if err != nil || !reflect.DeepEqual(packet, built) {
		t.Fatalf("an unread corrupt body changed the packet: err=%v\n%v\n%v", err, packet, built)
	}
	if _, valid, loaded := corrupt.Sources[large].Text(); valid || loaded {
		t.Fatal("a corrupt body was served")
	}
	if _, err := TaskContext(ctx, corrupt, task, "cache/demux.go", 20); !errors.Is(err, ErrSnapshotRefused) {
		t.Fatalf("a packet over a refused body = %v, want ErrSnapshotRefused", err)
	}
}

// TestPackDeferredBodyRefusalIsComputedOnceAcrossLaterLoads: after a deferred
// read finds a corrupt body, the retained mapping is refused, so a later
// request in the same process loads the gob snapshot and computes once
// instead of computing over the pack, discarding it and rereading.
func TestPackDeferredBodyRefusalIsComputedOnceAcrossLaterLoads(t *testing.T) {
	index, _ := packFixture(t)
	large := "docs/large.txt"
	if err := os.WriteFile(filepath.Join(index.Root, large), bytes.Repeat([]byte("filler prose line\n"), 3*packBlockSize/18+1), 0o644); err != nil {
		t.Fatal(err)
	}
	testGit(t, index.Root, "add", "-A")
	testGit(t, index.Root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "large")
	index, err := Build(context.Background(), index.Root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	ctx, want := context.Background(), index.Sources[large]
	wantText, _, _ := want.Text()
	rewritePack(t, receipt.PackPath, func(content []byte, found packHeader) []byte {
		bodies, _ := found.entry(packSectionBodies)
		content[bodies.Offset+want.body.offset+packBlockSize] ^= 0xff
		return content
	})
	computes := make([]int, 3)
	for request := range computes {
		loaded, hit, err := LoadSnapshotDeferred(ctx, index.Root)
		if err != nil || !hit {
			t.Fatalf("request %d: deferred load hit=%v err=%v", request, hit, err)
		}
		text, _, _ := loaded.Sources[large].Text()
		computes[request]++
		if loaded.SnapshotRefusal() != nil {
			if loaded, hit, err = LoadSnapshot(ctx, index.Root); err != nil || !hit {
				t.Fatalf("request %d: eager reload hit=%v err=%v", request, hit, err)
			}
			text, _, _ = loaded.Sources[large].Text()
			computes[request]++
		}
		if text != wantText {
			t.Fatalf("request %d served a body that differs from the build", request)
		}
	}
	if !slices.Equal(computes, []int{2, 1, 1}) {
		t.Fatalf("computes per request = %v, want [2 1 1]", computes)
	}
}

// TestPackQueryAndEventLoadsVerifyABodyOnlyWhenItIsRead pins the deferred
// query and event loads of IDX-SNAP-V0-015: opening verifies no body block, a
// read verifies only the blocks of the body it touches, and a read of a
// corrupt body is served unloaded with SnapshotRefusal reporting
// ErrSnapshotRefused.
func TestPackQueryAndEventLoadsVerifyABodyOnlyWhenItIsRead(t *testing.T) {
	index, _ := packFixture(t)
	large := "docs/large.txt"
	if err := os.WriteFile(filepath.Join(index.Root, large), bytes.Repeat([]byte("filler prose line\n"), 3*packBlockSize/18+1), 0o644); err != nil {
		t.Fatal(err)
	}
	testGit(t, index.Root, "add", "-A")
	testGit(t, index.Root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "large")
	index, err := Build(context.Background(), index.Root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	identity, small := fixtureIdentity(index), "cache/demux.go"
	opened, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadContext)
	if err != nil {
		t.Fatal(err)
	}
	body := opened.Sources[large].body
	middle := int((body.offset + body.length/2) / packBlockSize)
	rewritePack(t, receipt.PackPath, func(content []byte, found packHeader) []byte {
		bodies, _ := found.entry(packSectionBodies)
		content[bodies.Offset+uint64(middle)*packBlockSize] ^= 0xff
		return content
	})
	corrupt, err := os.ReadFile(receipt.PackPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, load := range []snapshotLoad{loadContext, loadEventDeferred} {
		// A refusal marks the retained mapping refused, so each load reads
		// its own copy of the corrupt pack.
		path := filepath.Join(t.TempDir(), "corrupt"+packExtension)
		if err := os.WriteFile(path, corrupt, 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := readPackSnapshot(path, identity, analyzerEngine(), load)
		if err != nil {
			t.Fatalf("load %d refused a pack whose only corruption is an unread body: %v", load, err)
		}
		for block := range loaded.bodies.verified {
			if loaded.bodies.verified[block].Load() {
				t.Fatalf("load %d verified body block %d at open", load, block)
			}
		}
		want, _, _ := index.Sources[small].Text()
		if got, valid, ok := loaded.Sources[small].Text(); got != want || !valid || !ok || loaded.SnapshotRefusal() != nil {
			t.Fatalf("load %d: an intact body read differs from the build or refused", load)
		}
		if loaded.bodies.verified[middle].Load() {
			t.Fatalf("load %d verified the unread body's block", load)
		}
		if _, valid, ok := loaded.Sources[large].Text(); valid || ok {
			t.Fatalf("load %d served a corrupt body", load)
		}
		if err := loaded.SnapshotRefusal(); !errors.Is(err, ErrSnapshotRefused) {
			t.Fatalf("load %d refusal after a corrupt read = %v, want ErrSnapshotRefused", load, err)
		}
	}
}
