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

func TestCountTermsMatchesTheRegexTokeniser(t *testing.T) {
	texts := []string{
		"", "a", "ab", "fooBarBaz FOOBar fooBAR 9abc abc9 x9Y",
		"café naïve Ünïcode straße 日本語 mixedÜpper",
		"a1B2c3 __init__ HTTPServer parseJSONResponse v2Beta",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		data, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, string(data))
	}
	for _, text := range texts {
		got, want := countTerms(text), referenceTermCounts(text)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("countTerms differs from the regexes on %q", firstLine(text))
		}
	}
}

func firstLine(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		return text[:index]
	}
	return text
}

func TestScanWordsKeepsWholeWordsAsWritten(t *testing.T) {
	got := scanWords("fooBar_9 ab _x9 café naïve 9abc a_b (Demux) Demuxes") // "caf": the ASCII run before é, as contextIdentifier reads a task
	want := map[string]struct{}{"fooBar_9": {}, "_x9": {}, "caf": {}, "9abc": {}, "a_b": {}, "Demux": {}, "Demuxes": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("words = %v, want %v", got, want)
	}
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

func TestLexicalRowsMatchWholeTokensFromTheTable(t *testing.T) {
	index := termTableFixture()
	compiler := newTaskContextCompiler(index, "demux the keyRing", "")
	rows := compiler.lexicalRows(0)
	got := map[string]string{}
	for _, row := range rows {
		got[row.path] = strings.SplitN(row.reason, ";", 2)[0]
	}
	want := map[string]string{
		// "demux" as a path term and a body token (twice), "key" and "ring" twice
		// each, and the whole identifier `keyRing` as written; "the" is a stop word.
		"cache/demux.go": "5 distinct task terms, 6 occurrences",
		// "demuxes" is not the token "demux"; the two bare "demux" tokens count.
		"cache/notes.md": "documentation: 3 distinct task terms, 4 occurrences",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lexical rows = %v, want %v", got, want)
	}
	if rows[0].path != "cache/demux.go" {
		t.Fatalf("more distinct terms rank first: %v", rows)
	}
}

func TestIdentifierEvidenceCountsWholeWordsByWeight(t *testing.T) {
	index := termTableFixture()
	compiler := newTaskContextCompiler(index, "`Demux` and KeyRing", "")
	if got := compiler.identifierEvidence("cache/demux.go"); got != 4 {
		t.Fatalf("demux.go evidence = %d, want 4 (backticked Demux 3 + KeyRing 1)", got)
	}
	if got := compiler.identifierEvidence("cache/notes.md"); got != 0 {
		t.Fatalf("notes.md evidence = %d, want 0: Demuxes is not the word Demux", got)
	}
	if got := compiler.identifierEvidence("absent.go"); got != 0 {
		t.Fatalf("an absent path scores %d, want 0", got)
	}
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

// walkTermTable reads every key, posting range and document length the way
// ranking does, so a table that should have been refused panics here.
func walkTermTable(table *TermTable) {
	for _, postings := range []*termPostings{&table.Terms, &table.Words, &table.PathTerms} {
		for index := 0; index < postings.keyCount(); index++ {
			_ = postingSources(postings, postings.key(index))
		}
	}
	table.documentLengths()
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

// TestLexicalRowsOrderByBM25AndAnswerWholeIdentifiers is TCP-V0-014: a rare
// whole identifier outranks many common-term occurrences, a question word is
// not a term, and the reason names the rarest term with its frequency.
func TestLexicalRowsOrderByBM25AndAnswerWholeIdentifiers(t *testing.T) {
	t.Run("TCP-V0-014", func(t *testing.T) {
		filler := strings.Repeat("option output behave default value ", 40)
		sources := map[string]Source{
			"lib/strip.js":   {Path: "lib/strip.js", Data: []byte("export const stripFinalNewline = (value) => value\n")},
			"lib/options.js": {Path: "lib/options.js", Data: []byte(filler + "\n")},
			"lib/output.js":  {Path: "lib/output.js", Data: []byte(filler + "output default\n")},
			"docs/api.md":    {Path: "docs/api.md", Data: []byte("## stripFinalNewline\nThe option's default value.\n")},
		}
		compiler := newTaskContextCompiler(&Index{Sources: sources}, "How does the `stripFinalNewline` option behave by default?", "")
		for _, term := range compiler.terms {
			if term == "how" || term == "does" || term == "the" || term == "by" {
				t.Fatalf("stop word %q survived into the task terms %v", term, compiler.terms)
			}
		}
		rows := compiler.lexicalRows(0)
		if len(rows) < 2 || rows[0].path != "lib/strip.js" {
			t.Fatalf("rare whole identifier should rank first, got %v", rows)
		}
		if !strings.Contains(rows[0].reason, "rarest `") || !strings.Contains(rows[0].reason, "(idf ") || !strings.Contains(rows[0].reason, "bm25 ") {
			t.Fatalf("reason should name the rarest term, its frequency and the score: %q", rows[0].reason)
		}
		// The whole identifier is a term of its own: strip.js carries it as written.
		if !strings.HasPrefix(rows[0].reason, "5 distinct task terms") {
			t.Fatalf("whole identifier should count as a distinct term: %q", rows[0].reason)
		}
		if rows[len(rows)-1].kind != "documentation" {
			t.Fatalf("documentation keeps its class: %v", rows)
		}
	})
}

// TestTaskCompoundTermMatchesOnlyUnsplitRuns is V1-0214: the lowered whole
// identifier taskLexicalTerms adds reaches a body or path posting only where
// the source writes it as one unsplit run, because both tables split camel
// case; the camel-case spelling is answered by the Words field alone.
func TestTaskCompoundTermMatchesOnlyUnsplitRuns(t *testing.T) {
	sources := map[string]Source{
		"lib/strip.js":              {Path: "lib/strip.js", Data: []byte("export const stripFinalNewline = (value) => value\n")},
		"config/flags.txt":          {Path: "config/flags.txt", Data: []byte("stripfinalnewline = true\n")},
		"docs/stripfinalnewline.md": {Path: "docs/stripfinalnewline.md", Data: []byte("Options.\n")},
	}
	terms := stringSetOf(taskLexicalTerms("How does stripFinalNewline behave?"))
	if _, ok := terms["stripfinalnewline"]; !ok {
		t.Fatalf("task terms %v lack the lowered whole identifier", terms)
	}
	table := buildTermTable(sources)
	postings := func(field termPostings, key string) []string {
		low, high, _ := field.find(key)
		paths := []string{}
		for index := low; index < high; index++ {
			paths = append(paths, table.Paths[field.Sources[index]])
		}
		return paths
	}
	for name, check := range map[string]struct {
		got, want []string
	}{
		"body":  {postings(table.Terms, "stripfinalnewline"), []string{"config/flags.txt"}},
		"path":  {postings(table.PathTerms, "stripfinalnewline"), []string{"docs/stripfinalnewline.md"}},
		"words": {postings(table.Words, "stripFinalNewline"), []string{"lib/strip.js"}},
		"split": {postings(table.Terms, "newline"), []string{"lib/strip.js"}},
	} {
		if !reflect.DeepEqual(check.got, check.want) {
			t.Fatalf("%s postings = %v, want %v", name, check.got, check.want)
		}
	}
}
