package contextindex

import (
	"context"
	"testing"
)

// TestNativeJvmAndStylesheetSuffixesAreIndexed_V2716 pins IDX-SNAP-V0-110:
// the C-family, JVM, Pony, AsciiDoc, stylesheet and template suffixes enter
// the source table as searchable text, AsciiDoc ranks as documentation, and a
// suffix outside the allowlist is still not admitted.
func TestNativeJvmAndStylesheetSuffixesAreIndexed_V2716(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testGit(t, root, "init", "-q", "-b", "main")
	files := map[string]string{
		"src/parse.c":          "int parse_header(void) { return 0; }\n",
		"src/parse.h":          "int parse_header(void);\n",
		"src/parse.cc":         "int parse_header() { return 0; }\n",
		"src/parse.cpp":        "int parse_header() { return 0; }\n",
		"src/parse.cxx":        "int parse_header() { return 0; }\n",
		"src/parse.hh":         "int parse_header();\n",
		"src/parse.hpp":        "int parse_header();\n",
		"src/parse.hxx":        "int parse_header();\n",
		"jvm/Parser.java":      "class Parser { void parseHeader() {} }\n",
		"jvm/Parser.scala":     "object Parser { def parseHeader(): Unit = () }\n",
		"pony/parser.pony":     "actor Parser\n  be parse_header() => None\n",
		"docs/parser.adoc":     "= Parser\n\nThe header parser.\n",
		"docs/parser.asciidoc": "= Parser\n\nThe header parser.\n",
		"web/parser.css":       ".parser-header { color: red; }\n",
		"web/parser.scss":      ".parser-header { color: red; }\n",
		"web/parser.less":      ".parser-header { color: red; }\n",
		"web/parser.pcss":      ".parser-header { color: red; }\n",
		"web/parser.tpl":       "{{ parser_header }}\n",
		"assets/parser.xyz":    "parser header\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-qm", "native sources")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for path := range files {
		_, admitted := index.Sources[path]
		if want := path != "assets/parser.xyz"; admitted != want {
			t.Errorf("%s admitted=%v want %v", path, admitted, want)
		}
	}
	if index.UnsupportedSuffixCount != 1 {
		t.Fatalf("UnsupportedSuffixCount = %d, want only assets/parser.xyz", index.UnsupportedSuffixCount)
	}
	for _, path := range []string{"docs/parser.adoc", "docs/parser.asciidoc"} {
		if !isDocumentationSuffix(path) {
			t.Errorf("%s is not documentation", path)
		}
	}
	if isDocumentationSuffix("src/parse.c") {
		t.Error("src/parse.c is documentation")
	}
	channel, err := TaskLexicalChannel(context.Background(), index, "Fix parse_header in the parser", 20)
	if err != nil {
		t.Fatal(err)
	}
	named := map[string]bool{}
	for _, candidate := range channel.Candidates {
		if len(candidate.Identifiers) > 0 {
			named[candidate.Path] = true
		}
	}
	for _, path := range []string{"src/parse.c", "src/parse.hpp", "pony/parser.pony"} {
		if !named[path] {
			t.Errorf("%s does not name parse_header in the lexical channel: %v", path, channel.Candidates)
		}
	}
}
