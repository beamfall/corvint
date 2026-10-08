package contextindex

import (
	"strings"
	"testing"
)

// TestKHNV0008_SymbolExtentsReuseTheIndexExtractors: a Go source names
// functions, types and Receiver.Method with doc-comment-to-brace extents; a
// Python source is walked by the index's Python extractor; a path no
// extractor admits, or bytes that are not text, is refused rather than read
// as an empty symbol table.
func TestKHNV0008_SymbolExtentsReuseTheIndexExtractors(t *testing.T) {
	goSrc := "package p\n\n// F does f.\nfunc F() int {\n\treturn 1\n}\n\ntype T struct{}\n\nfunc (t *T) M() {}\n"
	got, ok := SymbolExtents("p.go", []byte(goSrc))
	if !ok {
		t.Fatal("Go source refused")
	}
	byName := map[string]SymbolExtent{}
	for _, e := range got {
		byName[e.Name] = e
	}
	if f := byName["F"]; f.Start != 3 || f.End != 6 || f.Content != "// F does f.\nfunc F() int {\n\treturn 1\n}" {
		t.Fatalf("F extent: %+v", f)
	}
	if m, ok := byName["T.M"]; !ok || m.Content != "func (t *T) M() {}" {
		t.Fatalf("method extent: %+v in %+v", m, got)
	}
	if _, ok := byName["M"]; ok {
		t.Fatalf("bare method name is ambiguous across receivers: %+v", got)
	}
	if _, ok := byName["T"]; !ok {
		t.Fatalf("type missing: %+v", got)
	}

	py, ok := SymbolExtents("m.py", []byte("def f():\n    return 1\n\n\ndef g():\n    return 2\n"))
	if !ok || len(py) < 2 || py[0].Name != "f" || !strings.HasPrefix(py[0].Content, "def f():") || strings.Contains(py[0].Content, "def g") {
		t.Fatalf("Python extents: %v %+v", ok, py)
	}

	for name, c := range map[string]struct {
		path string
		data []byte
	}{
		"no extractor": {"notes.txt", []byte("F\n")},
		"not text":     {"p.go", []byte{0xff, 0xfe, 0x00, 0x01}},
		"bad Go":       {"p.go", []byte("package p\nfunc (\n")},
	} {
		if _, ok := SymbolExtents(c.path, c.data); ok {
			t.Fatalf("%s admitted", name)
		}
	}
}
