package contextindex

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// referenceTermCounts is the regex definition of a term (lexicalTerms) with
// occurrences counted.
func referenceTermCounts(text string) map[string]uint32 {
	counts := map[string]uint32{}
	spaced := contextCamel.ReplaceAllString(text, "$1 $2")
	for _, token := range contextToken.FindAllString(spaced, -1) {
		if len(token) >= 2 {
			counts[strings.ToLower(token)]++
		}
	}
	return counts
}

func firstLine(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		return text[:index]
	}
	return text
}

func termTableFixture() *Index {
	sources := map[string]Source{
		"cache/demux.go":  {Path: "cache/demux.go", Data: []byte("package cache\n// Demux splits keys.\nfunc Demux(keyRing KeyRing) {}\n")},
		"cache/notes.md":  {Path: "cache/notes.md", Data: []byte("demuxes the key ring twice: demux demux\n")},
		"docs/other.md":   {Path: "docs/other.md", Data: []byte("nothing here\n")},
		"cache/binary.go": {Path: "cache/binary.go", Data: []byte("\x00\x01binary")},
	}
	return &Index{Sources: sources}
}

func TestTermTableIsFlatSortedAndSkipsUnreadableSources(t *testing.T) {
	table := buildTermTable(termTableFixture().Sources)
	if !reflect.DeepEqual(table.Paths, []string{"cache/binary.go", "cache/demux.go", "cache/notes.md", "docs/other.md"}) {
		t.Fatalf("paths = %v", table.Paths)
	}
	for index := 1; index < table.Terms.keyCount(); index++ {
		if table.Terms.key(index-1) >= table.Terms.key(index) {
			t.Fatalf("keys are not sorted at %d: %q >= %q", index, table.Terms.key(index-1), table.Terms.key(index))
		}
	}
	if _, _, ok := table.Terms.find("binary"); ok {
		t.Fatal("a binary source must have no term postings")
	}
	if low, high, ok := table.PathTerms.find("binary"); !ok || high-low != 1 || table.Paths[table.PathTerms.Sources[low]] != "cache/binary.go" {
		t.Fatal("a binary source still has its path terms")
	}
	if len(table.Words.Counts) != 0 || len(table.PathTerms.Counts) != 0 {
		t.Fatal("presence tables carry no counts")
	}
}

func TestTermTableSurvivesGobThroughItsBinaryLayout(t *testing.T) {
	table := buildTermTable(termTableFixture().Sources)
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(table); err != nil {
		t.Fatal(err)
	}
	var decoded TermTable
	if err := gob.NewDecoder(&buffer).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&decoded, table) {
		t.Fatalf("decoded table differs:\n%+v\n%+v", decoded, *table)
	}
	var truncated termPostings
	if err := truncated.UnmarshalBinary([]byte{9, 0, 0, 0, 1}); err == nil {
		t.Fatal("a truncated table must be refused")
	}
}

func TestParallelInversionMatchesSingleWorker(t *testing.T) {
	perSource := make([]sourceTerms, 257)
	for source := range perSource {
		perSource[source] = sourceTerms{
			terms: map[string]uint32{
				"common":                             uint32(source + 1),
				"group" + string(rune('a'+source%7)): uint32(source%5 + 1),
			},
			words: map[string]struct{}{
				"Common":                             {},
				"Word" + string(rune('A'+source%11)): {},
			},
			pathTerms: []string{"path", "part" + string(rune('a'+source%13))},
		}
	}

	if got, want := invertCountsWithWorkers(perSource, 4), invertCountsWithWorkers(perSource, 1); !reflect.DeepEqual(got, want) {
		t.Fatal("parallel count inversion differs from single-worker inversion")
	}
	for name, pick := range map[string]func(sourceTerms) map[string]struct{}{
		"words": func(entry sourceTerms) map[string]struct{} { return entry.words },
		"paths": func(entry sourceTerms) map[string]struct{} { return stringSetOf(entry.pathTerms) },
	} {
		t.Run(name, func(t *testing.T) {
			got := invertPresenceWithWorkers(perSource, 4, pick)
			want := invertPresenceWithWorkers(perSource, 1, pick)
			if !reflect.DeepEqual(got, want) {
				t.Fatal("parallel presence inversion differs from single-worker inversion")
			}
		})
	}
}

// A snapshot's term table is self-attested, so decode refuses one whose
// offsets or source ids leave their slices instead of leaving key, find or
// documentLengths to panic on the first query (IDX-SNAP-V0-003).
func TestSnapshotRefusesTermTableOffsetsOutsideTheirSlices(t *testing.T) {
	cases := map[string]func(*termPostings, int){
		"key offset past the keys":       func(p *termPostings, _ int) { p.KeyOffsets[len(p.KeyOffsets)-1] = uint32(len(p.KeyBytes)) + 9 },
		"key offsets descending":         func(p *termPostings, _ int) { p.KeyOffsets[1] = p.KeyOffsets[2] + 1 },
		"posting offset past the source": func(p *termPostings, _ int) { p.Offsets[len(p.Offsets)-1] = uint32(len(p.Sources)) + 9 },
		"posting offsets descending":     func(p *termPostings, _ int) { p.Offsets[1] = p.Offsets[2] + 1 },
		"fewer posting than key offsets": func(p *termPostings, _ int) { p.Offsets = p.Offsets[:len(p.Offsets)-1] },
		"source outside the paths":       func(p *termPostings, paths int) { p.Sources[0] = uint32(paths) },
		"counts shorter than sources":    func(p *termPostings, _ int) { p.Counts = p.Counts[:len(p.Counts)-1] },
	}
	identity := repositoryIdentity{objectFormat: "sha1", treeRevision: "tree"}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			table := buildTermTable(termTableFixture().Sources)
			mutate(&table.Terms, len(table.Paths))
			file, err := os.Create(filepath.Join(t.TempDir(), "snapshot.gob"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			index := &Index{ObjectFormat: identity.objectFormat, Revision: identity.treeRevision, Vocabulary: table}
			if _, err := encodeSnapshot(file, index, "engine"); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeSnapshot(file, identity, "engine")
			if err == nil {
				walkTermTable(decoded.Vocabulary)
				t.Fatal("a term table with out-of-range offsets must be refused")
			}
		})
	}
}
