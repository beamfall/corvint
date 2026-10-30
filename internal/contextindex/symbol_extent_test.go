package contextindex

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

// TestKHNV0016_SymbolExtentsReuseTheIndexExtractors: a Go source names
// functions, types and Receiver.Method with doc-comment-to-brace extents; a
// Python source is walked by the index's Python extractor; a path no
// extractor admits, or bytes that are not text, is refused rather than read
// as an empty symbol table.
func TestKHNV0016_SymbolExtentsReuseTheIndexExtractors(t *testing.T) {
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

// referenceSymbolExtents is SymbolExtents as it was before V1-3015: one
// joined Content per extent and an all-pairs next-symbol search. It is the
// parity oracle for the shared-text implementation.
func referenceSymbolExtents(path string, data []byte) ([]SymbolExtent, bool) {
	if !textDecodable(data) {
		return nil, false
	}
	text := string(data)
	join := func(kind, name string, start, end int, lines []string) SymbolExtent {
		start = min(max(start, 1), len(lines))
		end = min(max(end, start), len(lines))
		for end > start && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		return SymbolExtent{Kind: kind, Name: name, Start: start, End: end, Content: strings.Join(lines[start-1:end], "\n")}
	}
	if strings.HasSuffix(path, ".go") {
		shared, ok := goSymbolExtents(text)
		if !ok {
			return nil, false
		}
		lines := strings.Split(text, "\n")
		out := make([]SymbolExtent, 0, len(shared))
		for _, e := range shared {
			out = append(out, join(e.Kind, e.Name, e.Start, e.End, lines))
		}
		return out, true
	}
	source := Source{Path: path}
	var symbols []Symbol
	if strings.HasSuffix(path, ".py") {
		found, _, refusal := pythonSourceFacts(source, text)
		if refusal != "" {
			return nil, false
		}
		symbols = found
	} else {
		text = normalizeLineEndings(text)
		found, notes, handled := languageSymbols(source, text)
		if !handled || len(notes) > 0 {
			return nil, false
		}
		symbols = found
	}
	lines := strings.Split(text, "\n")
	out := make([]SymbolExtent, 0, len(symbols))
	for _, s := range symbols {
		end := len(lines)
		if s.EndLine > 0 {
			end = s.EndLine
		} else {
			for _, next := range symbols {
				if next.Line > s.Line && next.Line-1 < end {
					end = next.Line - 1
				}
			}
		}
		out = append(out, join(s.Kind, s.Name, s.Line, end, lines))
	}
	return out, true
}

// V1-3015: the shared-text extents equal the joined ones, field for field,
// across the Go, Python and generic extractor paths.
func TestSymbolExtentsMatchTheJoinedReference(t *testing.T) {
	for path, source := range map[string]string{
		"p.go":   "package p\n\n// F does f.\nfunc F() int {\n\treturn 1\n}\n\ntype T struct{}\n\nfunc (t *T) M() {}\n\nvar (\n\t// A is a.\n\tA, B = 1,\n\t\t2\n\n\tC int\n)\n\nconst D, E = 3, 4\n",
		"m.py":   "def f():\n    return 1\n\n\ndef g():\n    return 2\n\nclass K:\n    def m(self):\n        pass\n",
		"lib.rb": "class Foo\n  def bar\n    1\n  end\n\n\n  def baz\n  end\nend\n\nmodule Qux\nend\n",
		"lib.rs": "fn a() {\r\n}\r\n\r\nstruct S;\r\n\r\nfn b() -> i32 {\r\n    2\r\n}\r\n",
		"App.kt": "class A {\n  fun x() = 1\n}\n\nfun y() {}\n",
	} {
		got, ok := SymbolExtents(path, []byte(source))
		want, wantOK := referenceSymbolExtents(path, []byte(source))
		if ok != wantOK || len(got) != len(want) {
			t.Fatalf("%s: got %v %d extents, want %v %d", path, ok, len(got), wantOK, len(want))
		}
		if len(got) == 0 {
			t.Fatalf("%s: fixture names no symbols", path)
		}
		for index := range got {
			if got[index] != want[index] {
				t.Fatalf("%s extent %d:\n got %+v\nwant %+v", path, index, got[index], want[index])
			}
		}
	}
}

// V1-3015: thousands of names declared by one multiline declaration share
// its text instead of retaining one copy per name.
func TestSymbolExtentsRetainOneDeclarationCopyPerSource(t *testing.T) {
	const names = 4000
	var source strings.Builder
	source.WriteString("package p\n\nvar (\n\t")
	for index := range names {
		fmt.Fprintf(&source, "name%04d,\n\t", index)
	}
	source.WriteString("last int\n)\n")
	data := []byte(source.String())

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	extents, ok := SymbolExtents("p.go", data)
	runtime.ReadMemStats(&after)
	if !ok || len(extents) != names+1 {
		t.Fatalf("extents: ok=%v count=%d", ok, len(extents))
	}
	// One joined copy per name would be names*len(data), about 230 MB here;
	// the parse itself allocates a small multiple of the source.
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > uint64(64*len(data)) {
		t.Fatalf("SymbolExtents allocated %d bytes for a %d-byte source", allocated, len(data))
	}
	shared := unsafe.StringData(extents[0].Content)
	for _, extent := range extents {
		if unsafe.StringData(extent.Content) != shared || len(extent.Content) != len(extents[0].Content) {
			t.Fatalf("%s does not share the declaration text", extent.Name)
		}
	}
}
