package main

import (
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestTSPV0785ParsedFilesAreReusedAndEqual pins TSP-V0-785: a second slice of
// the same package bytes reuses every parse (the same parsed files) and
// yields the slice a fresh parse yields, and one changed byte misses for that file alone.
func TestTSPV0785ParsedFilesAreReusedAndEqual(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"p/p.go":      "package p\n\nimport \"strings\"\n\nfunc Upper(s string) string { return strings.ToUpper(s) }\n\nfunc Lower(s string) string { return strings.ToLower(s) }\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestUpper(t *testing.T) { _ = Upper(\"a\") }\n\nfunc TestLower(t *testing.T) { _ = Lower(\"A\") }\n",
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := map[string]testBaseBlob{"p/p.go": {content: []byte("package p\n\nimport \"strings\"\n\nfunc Upper(s string) string { return strings.ToTitle(s) }\n\nfunc Lower(s string) string { return strings.ToLower(s) }\n")}}
	candidate := testSourceCandidate{pkg: "example.test/m/p", fallback: "whole", own: []string{"p/p.go"}}
	entries := func() int {
		testSourceParsedCache.Lock()
		defer testSourceParsedCache.Unlock()
		return len(testSourceParsedCache.entries)
	}
	parses := func() map[*ast.File]bool {
		testSourceParsedCache.Lock()
		defer testSourceParsedCache.Unlock()
		seen := make(map[*ast.File]bool)
		for _, parsed := range testSourceParsedCache.entries {
			seen[parsed.file] = true
		}
		return seen
	}
	empty := func() {
		testSourceParsedCache.Lock()
		clear(testSourceParsedCache.entries)
		testSourceParsedCache.Unlock()
	}
	empty()
	cold := testSourceSlice(root, candidate, base)
	if cold.Pattern != "^(TestUpper)$" {
		t.Fatalf("cold slice %+v, want TestUpper alone", cold)
	}
	parsed := entries()
	if parsed != 2 {
		t.Fatalf("cold slice cached %d files, want 2", parsed)
	}
	first := parses()
	if warm := testSourceSlice(root, candidate, base); !reflect.DeepEqual(warm, cold) || !reflect.DeepEqual(parses(), first) {
		t.Fatalf("warm slice %+v reparsed (%d entries), want %+v from the %d cached parses", warm, entries(), cold, parsed)
	}
	empty()
	if fresh := testSourceSlice(root, candidate, base); !reflect.DeepEqual(fresh, cold) {
		t.Fatalf("slice after emptying the cache %+v, want %+v", fresh, cold)
	}
	edited := files["p/p_test.go"] + "\nfunc TestBoth(t *testing.T) { _ = Upper(Lower(\"b\")) }\n"
	if err := os.WriteFile(filepath.Join(root, "p/p_test.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	before := entries()
	if changed := testSourceSlice(root, candidate, base); changed.Pattern != "^(TestBoth|TestUpper)$" || entries() != before+1 {
		t.Fatalf("slice after a test file edit %+v with %d entries, want TestBoth|TestUpper with %d", changed, entries(), before+1)
	}
}
