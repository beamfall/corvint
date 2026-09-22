package contextindex

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGoSymbolsMatchPythonUnicodeWordSemantics(t *testing.T) {
	source := Source{Path: "value.go", BlobHash: strings.Repeat("0", 40)}
	symbols, refusal := goSymbols(source, "package value\nfunc Fünf() {}\ntype T٣ struct{}\n")
	if refusal != "" {
		t.Fatalf("refusal = %q", refusal)
	}
	if len(symbols) != 2 || symbols[0].Name != "Fünf" || symbols[1].Name != "T٣" {
		t.Fatalf("symbols = %#v", symbols)
	}
}

func TestSourceImportsIncludesNonGoImporters(t *testing.T) {
	imports := sourceImports("web/caller.ts", "éimport \"ignored\"\nimport \"example.test/fixture/internal/token\"").imports
	if _, ok := imports["example.test/fixture/internal/token"]; !ok {
		t.Fatalf("imports = %v", imports)
	}
	if _, ok := imports["ignored"]; ok {
		t.Fatalf("Unicode word-boundary false positive: %v", imports)
	}
}

func TestSourceImportsIgnoresPythonStringsAndComments(t *testing.T) {
	text := "\"\"\"\nimport example.test/fixture/internal/token\n\"\"\"\n# import example.test/fixture/internal/token\n"
	if imports := sourceImports("caller.py", text).imports; len(imports) != 0 {
		t.Fatalf("Python pseudo-imports = %v", imports)
	}
}

func TestMarkerMatchesUsePythonUnicodeWordBoundaries(t *testing.T) {
	matches := markerMatches("éfeature:wrong feature:right scenario:wrong٣")
	if len(matches) != 1 || matches[0].kind != "feature" || matches[0].id != "right" {
		t.Fatalf("matches = %#v", matches)
	}
}

func TestScalarListPreservesQuotedCommas(t *testing.T) {
	value := scalar(`["0012,legacy",12,True]`)
	items, ok := value.([]any)
	if !ok || len(items) != 3 || items[0] != "0012,legacy" || items[1] != int64(12) || items[2] != true {
		t.Fatalf("scalar = %#v", value)
	}
	fallback := scalar(`[true,server]`).([]any)
	if fallback[0] != "true" || fallback[1] != "server" {
		t.Fatalf("fallback = %#v", fallback)
	}
	if got := scalar("+1"); got != "+1" {
		t.Fatalf("+1 = %#v", got)
	}
	if got := scalar("٤٢"); got != int64(42) {
		t.Fatalf("Unicode integer = %#v", got)
	}
	encoded, err := CanonicalJSON(map[string]any{"integer": scalar("999999999999999999999999999999999999")})
	if err != nil || string(encoded) != `{"integer":999999999999999999999999999999999999}` {
		t.Fatalf("large integer = %q, err=%v", encoded, err)
	}
}

func TestDocumentStatusIsAnchoredAndHeadingsTruncateByRune(t *testing.T) {
	heading := strings.Repeat("😀", 4_001)
	text := "---\n  status: rejected\n---\nstatus: accepted\n# Title\n## " + heading + "\n"
	source := Source{Path: "docs/specs/unicode.md", BlobHash: strings.Repeat("0", 40), Data: []byte(text)}
	body, valid, loaded := source.Text()
	if !loaded || !valid {
		t.Fatal("test source is not loaded valid text")
	}
	record, ok := documentRecord(source, body, map[string]Source{source.Path: source})
	if !ok {
		t.Fatal("document was not recognized")
	}
	if record.Fields["status"] != "accepted" {
		t.Fatalf("status = %q", record.Fields["status"])
	}
	if record.Fields["title"] != "Title" {
		t.Fatalf("title = %q", record.Fields["title"])
	}
	headings := record.Fields["headings"].(string)
	if !utf8.ValidString(headings) || utf8.RuneCountInString(headings) != 4_000 {
		t.Fatalf("heading bytes=%d runes=%d valid=%v", len(headings), utf8.RuneCountInString(headings), utf8.ValidString(headings))
	}
}

// The four shapes this repository writes a status field in, and the token capture
// that makes each comparable against the fixed status vocabulary. A status packed
// among other fields is admitted only when what precedes it is itself a `Key:
// value.` field run, so prose ending in a full stop cannot introduce one.
func TestDocumentStatusReadsEveryFieldShapeAndCapturesTheTokenAlone(t *testing.T) {
	for _, testCase := range []struct{ name, body, want string }{
		{"bare", "# T\nStatus: accepted\n", "accepted"},
		{"list item", "# T\n- Status: accepted and executed (2026-08-29)\n", "accepted"},
		{"qualified", "# T\nIntent status: accepted (owner instruction 2026-09-07)\nDelivery status: implemented\n", "accepted"},
		{"packed fields", "# T\nDate: 2026-09-05. Status: accepted. Authority: repository owner.\n", "accepted"},
		{"emphasised", "# T\n**Status**: proposed\n", "proposed"},
		{"hyphenated token", "# T\nIntent status: not-applicable (reading aid; decides nothing)\n", "not-applicable"},
		{"superseded is not accepted", "# T\nIntent status: superseded by accepted decision 0009\n", "superseded"},
		{"partial supersession keeps its argument", "# T\nStatus: partially-superseded-by:0031\n", "partially-superseded-by:0031"},
		{"partial supersession quoted with a spaced argument", "# T\nstatus: \"partially-superseded-by: 0042\"\n", "partially-superseded-by: 0042"},
		{"prose is not a field", "# T\nWe shipped it. Status was never recorded.\n", ""},
		{"indented key is not the document status", "# T\n  status: rejected\n", ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			source := Source{Path: "docs/specs/shape.md", BlobHash: strings.Repeat("0", 40), Data: []byte(testCase.body)}
			body, valid, loaded := source.Text()
			if !loaded || !valid {
				t.Fatal("test source is not loaded valid text")
			}
			record, ok := documentRecord(source, body, map[string]Source{source.Path: source})
			if !ok {
				t.Fatal("document was not recognized")
			}
			if record.Fields["status"] != testCase.want {
				t.Fatalf("status = %q, want %q", record.Fields["status"], testCase.want)
			}
		})
	}
}

func TestDocumentTitleUsesFirstH1WhenItEqualsStem(t *testing.T) {
	source := Source{Path: "docs/specs/token.md", Data: []byte("# token\n# wrong\n")}
	body, valid, loaded := source.Text()
	if !loaded || !valid {
		t.Fatal("test source is not loaded valid text")
	}
	record, ok := documentRecord(source, body, map[string]Source{source.Path: source})
	if !ok || record.Fields["title"] != "token" || record.Line != 1 {
		t.Fatalf("record = %#v, ok=%v", record, ok)
	}
}
