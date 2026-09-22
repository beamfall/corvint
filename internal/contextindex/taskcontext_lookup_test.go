package contextindex

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"testing"
)

// nameTokensOracle is the regex form nameTokens replaced (TCP-V0-015's
// camel-split token rule), kept as the differential reference.
func nameTokensOracle(name string) []string {
	spaced := contextCamel.ReplaceAllString(name, "$1 $2")
	tokens := contextToken.FindAllString(spaced, -1)
	for position := range tokens {
		tokens[position] = strings.ToLower(tokens[position])
	}
	return tokens
}

// TestNameTokensMatchesRegexOracle: the byte scanner and the regex agree on
// every explicit shape (empty, punctuation only, digits, acronyms, adjacent
// boundaries, non-ASCII and invalid UTF-8 separators) and on deterministic
// random byte strings, including whether an empty result is nil.
func TestNameTokensMatchesRegexOracle(t *testing.T) {
	t.Run("TCP-V0-015", checkNameTokensMatchesRegexOracle)
}

func checkNameTokensMatchesRegexOracle(t *testing.T) {
	explicit := []string{
		"", "_", "___", "-.-", " ", "a", "A", "Ab", "aB", "aBcD", "aBC", "ABCdef", "HTTPServer2Go",
		"parseJSONValue", "snake_case_name", "a1B2", "x9", "9x", "Test", "TestDrainQueueTwice",
		"describe 'Foo bar-Baz'", "ünicode Über", "日本語Text", "naïveCase", "\xff\xfeAbc", "a\xffB",
		"a\x00B", "Ab\x80cD", "\xc3", "é", "aé", "éA", "trailingUpperA", "lowerUPPERlower",
	}
	for _, name := range explicit {
		assertNameTokens(t, name)
	}
	random := rand.New(rand.NewSource(20260906))
	alphabet := []byte("abcxyzABCXYZ0189_ -./\x00\x7f\x80\x9c\xa9\xc3\xe2\xff")
	for run := 0; run < 5000; run++ {
		buffer := make([]byte, random.Intn(25))
		for position := range buffer {
			buffer[position] = alphabet[random.Intn(len(alphabet))]
		}
		assertNameTokens(t, string(buffer))
	}
}

func assertNameTokens(t *testing.T, name string) {
	t.Helper()
	got, want := nameTokens(name), nameTokensOracle(name)
	if !slices.Equal(got, want) || (got == nil) != (want == nil) {
		t.Fatalf("nameTokens(%q) = %#v, regex oracle %#v", name, got, want)
	}
}

// TestCreditMirroredMatchesFullScan: the stem-indexed counterpart read credits
// exactly the paths a full pairRelation scan of the tracked tree credits,
// with the same relation, over every path convention pairRelation names
// (same directory, mirrored directory, elsewhere in the tree, the JVM suffix,
// a non-source tracked path) and the two it must not credit (the module
// directory member, and a same-stem path in the same role).
func TestCreditMirroredMatchesFullScan(t *testing.T) {
	t.Run("TCP-V0-015", checkCreditMirroredMatchesFullScan)
}

func checkCreditMirroredMatchesFullScan(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                          "module example.test/mirror\n\ngo 1.27.0\n",
		"src/render.go":                   "package src\n\nfunc Render() {}\n",
		"src/render_test.go":              "package src\n\nfunc TestRender() {}\n",
		"src/parser.ts":                   "export function parse() {}\n",
		"__tests__/parser.test.ts":        "import { parse } from '../src/parser';\n",
		"lib/codec.py":                    "def encode():\n    pass\n",
		"tests/test_codec.py":             "def test_encode():\n    pass\n",
		"app/Store.java":                  "class Store {}\n",
		"check/StoreTest.java":            "class StoreTest {}\n",
		"core/queue.go":                   "package core\n\nfunc Queue() {}\n",
		"core/queue/queue_impl.go":        "package queue\n\nfunc Impl() {}\n",
		"core/queue/queue_helper_test.go": "package queue\n\nfunc TestHelper() {}\n",
		"core/queue_spec.rb":              "describe 'queue' do end\n",
		"docs/render.md":                  "# render\n",
		"other/unrelated.go":              "package other\n\nfunc Elsewhere() {}\n",
		"other/unrelated_helper.go":       "package other\n\nfunc Helper() {}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	compiler := newTaskContextCompiler(index, "mirror", "")
	linker := compiler.newTestLinker()
	anchors := keys(compiler.trackedPaths())
	sort.Strings(anchors)
	relations := map[string]int{}
	for _, anchor := range anchors {
		anchorIsTest := contextIsTest(anchor)
		want := map[string]string{}
		for candidate := range compiler.trackedPaths() {
			relation := pairRelation(anchor, candidate, contextStem(anchor), anchorIsTest)
			if relation == "" || relation == "module directory member" || candidate == anchor || contextIsTest(candidate) == anchorIsTest {
				continue
			}
			want[candidate] = relation
		}
		got := map[string]string{}
		entries := map[string]*testCandidate{}
		linker.creditMirrored(anchor, anchorIsTest, func(path string) *testCandidate {
			if path == anchor || contextIsTest(path) == anchorIsTest {
				return nil
			}
			if entries[path] == nil {
				entries[path] = &testCandidate{path: path, anchor: anchor}
			}
			return entries[path]
		})
		for path, entry := range entries {
			got[path] = entry.mirrored
			if entry.weight != mirroredWeight(entry.mirrored) {
				t.Fatalf("%s -> %s weight %v, want %v", anchor, path, entry.weight, mirroredWeight(entry.mirrored))
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("anchor %s: indexed %v, full scan %v", anchor, got, want)
		}
		for _, relation := range want {
			relations[relation]++
		}
	}
	for _, relation := range []string{"test counterpart", "source counterpart", "test counterpart in the mirrored directory", "source counterpart elsewhere in the tree"} {
		if relations[relation] == 0 {
			t.Fatalf("fixture never produced %q: %v", relation, relations)
		}
	}
}

// TestSymbolRowsDefinerCountsExact: the definer count that gates the
// definition slot is every symbol of the name, not every path, so a name at
// exactly contextMaxDefiners symbols is admitted and one over is not, a
// duplicate definition inside one path counts toward the cut while emitting
// one row, and a task naming no eligible identifier yields no row.
func TestSymbolRowsDefinerCountsExact(t *testing.T) {
	t.Run("TCP-V0-010", checkSymbolRowsDefinerCountsExact)
}

func checkSymbolRowsDefinerCountsExact(t *testing.T) {
	index := &Index{Sources: map[string]Source{}, Symbols: []Symbol{}}
	define := func(name string, paths int, duplicated bool) []string {
		defined := make([]string, 0, paths)
		for position := 0; position < paths; position++ {
			path := fmt.Sprintf("pkg/%s/file%02d.go", strings.ToLower(name), position)
			index.Sources[path] = Source{}
			index.Symbols = append(index.Symbols, Symbol{Kind: "func", Name: name, Path: path, Line: 1})
			defined = append(defined, path)
		}
		if duplicated {
			index.Symbols = append(index.Symbols, Symbol{Kind: "func", Name: name, Path: defined[0], Line: 9})
		}
		return defined
	}
	boundary := define("boundaryName", contextMaxDefiners, false)
	define("surplusName", contextMaxDefiners+1, false)
	twice := define("twiceName", contextMaxDefiners-1, true)
	define("overName", contextMaxDefiners, true)
	rowsOf := func(task string) []string {
		found := make([]string, 0)
		for _, row := range newTaskContextCompiler(index, task, "").symbolRows() {
			found = append(found, row.path)
		}
		return found
	}
	cases := []struct {
		task string
		want []string
	}{
		{"`boundaryName` `surplusName`", boundary},
		{"`twiceName` `overName`", twice},
		{"the boundary name and the surplus name", []string{}},
	}
	for _, item := range cases {
		if got := rowsOf(item.task); !slices.Equal(got, item.want) {
			t.Fatalf("%q rows = %v, want %v", item.task, got, item.want)
		}
	}
	for _, row := range newTaskContextCompiler(index, "`twiceName`", "").symbolRows() {
		if row.line != 1 {
			t.Fatalf("duplicate definition chose line %d, want the first symbol at line 1", row.line)
		}
	}
}
