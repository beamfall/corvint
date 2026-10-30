package contextindex

import "testing"

// TestUnparsedSeparatesDroppedFromEmpty is the regression for the silent
// whole-file symbol drop. Before this table a grammar-rejected source and a
// source that genuinely defines nothing produced identical evidence -- both
// stayed counted in Sources with zero symbols -- so no caller could tell that
// an entire file's definitions had been discarded.
func TestUnparsedSeparatesDroppedFromEmpty(t *testing.T) {
	// `x = ?` is rejected by the closed grammar and by CPython alike, so the
	// drop under test is a real refusal rather than a second false positive.
	rejected := "def dropped_definition():\n    x = ?\n"
	empty := "CONSTANT = 1\n"
	parsed := "def kept_definition():\n    return 1\n"

	index := &Index{Sources: map[string]Source{
		"a_rejected.py": {Path: "a_rejected.py", BlobHash: "h1", Data: []byte(rejected)},
		"b_empty.py":    {Path: "b_empty.py", BlobHash: "h2", Data: []byte(empty)},
		"c_parsed.py":   {Path: "c_parsed.py", BlobHash: "h3", Data: []byte(parsed)},
		"d_binary.bin":  {Path: "d_binary.bin", BlobHash: "h4", Data: []byte{0x00, 0xff, 0xfe}},
	}}
	var chunk compiledSources
	chunk.collect(index, []string{"a_rejected.py", "b_empty.py", "c_parsed.py", "d_binary.bin"}, nil)

	// The rejected file really did lose its definition: this is the loss the
	// table has to account for, not a hypothetical one.
	for _, symbol := range chunk.symbols {
		if symbol.Path == "a_rejected.py" {
			t.Fatalf("rejected source contributed symbol %#v; fixture no longer exercises the drop", symbol)
		}
	}
	if len(chunk.symbols) != 1 || chunk.symbols[0].Name != "kept_definition" {
		t.Fatalf("symbols = %#v, want only kept_definition", chunk.symbols)
	}

	reasons := map[string]string{}
	details := map[string]string{}
	for _, item := range chunk.unparsed {
		reasons[item.Path] = item.Reason
		details[item.Path] = item.Detail
	}
	if got := reasons["a_rejected.py"]; got != UnparsedPythonGrammar {
		t.Errorf("rejected source reason = %q, want %q", got, UnparsedPythonGrammar)
	}
	if details["a_rejected.py"] == "" {
		t.Errorf("rejected source carries no extractor detail")
	}
	if got := reasons["d_binary.bin"]; got != UnparsedNotText {
		t.Errorf("binary source reason = %q, want %q", got, UnparsedNotText)
	}
	// The whole point: a file that defines nothing is NOT reported as dropped.
	if _, reported := reasons["b_empty.py"]; reported {
		t.Errorf("source with no definitions was reported as dropped")
	}
	if _, reported := reasons["c_parsed.py"]; reported {
		t.Errorf("successfully parsed source was reported as dropped")
	}

	// BlobHash rides along so a caller can bind the loss to exact bytes.
	for _, item := range chunk.unparsed {
		if item.BlobHash == "" {
			t.Errorf("unparsed entry %q carries no blob hash", item.Path)
		}
	}
}

// TestUnparsedPathsIsQueryable pins the membership view callers branch on.
func TestUnparsedPathsIsQueryable(t *testing.T) {
	index := &Index{Unparsed: []Unparsed{
		{Path: "one.py", BlobHash: "h1", Reason: UnparsedPythonGrammar, Detail: "MALFORMED_INPUT"},
	}}
	paths := index.UnparsedPaths()
	if paths["one.py"] != UnparsedPythonGrammar {
		t.Fatalf("UnparsedPaths = %#v", paths)
	}
	if _, ok := paths["absent.py"]; ok {
		t.Fatalf("UnparsedPaths reported an absent path")
	}
}
