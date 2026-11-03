package v8

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

// fnRange is the V8 function range of the declaration starting at decl in
// src: from the keyword to its closing brace at the start of a line.
func fnRange(t *testing.T, src, decl string, count int) Range {
	t.Helper()
	i := strings.Index(src, decl)
	if i < 0 {
		t.Fatalf("no %q", decl)
	}
	j := strings.Index(src[i:], "\n}")
	return Range{Start: i, End: i + j + 2, Count: count}
}

func span(t *testing.T, src, text string, count int) Range {
	t.Helper()
	i := strings.Index(src, text)
	if i < 0 {
		t.Fatalf("no %q", text)
	}
	return Range{Start: i, End: i + len(text), Count: count}
}

// TestAddV8SourceMapped covers LPCV-V0-965: a V8 report of an esbuild bundle
// counts against the TypeScript source its map names, an uncalled function
// and an untaken branch stay unexecuted, and the result is a profile the
// shared linecov index builds from.
func TestAddV8SourceMapped(t *testing.T) {
	root, err := filepath.Abs("testdata/ts")
	if err != nil {
		t.Fatal(err)
	}
	js, err := os.ReadFile(filepath.Join(root, "out/lib.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	report := Report{Result: []Script{
		{URL: "node:internal/x", Functions: []Function{{Ranges: []Range{{0, 9, 1}}}}},
		{URL: "file://" + filepath.ToSlash(filepath.Join(root, "out/lib.js")), Functions: []Function{
			{Ranges: []Range{{0, len(src), 1}}},
			{Ranges: []Range{fnRange(t, src, "function add(", 1), span(t, src, "{\n    return 100;\n  }", 0)}},
			{Ranges: []Range{fnRange(t, src, "function mul(", 0)}},
		}},
	}}
	data, _ := json.Marshal(report)
	l := Lines{}
	keep := func(rel string) bool { return rel == "lib.ts" }
	if err := l.AddV8(data, Options{Scope: RepoScope(root, keep)}); err != nil {
		t.Fatal(err)
	}
	got := l["lib.ts"]
	want := map[int]bool{2: true, 3: true, 4: true, 5: false, 6: false, 7: true, 8: true, 9: false, 10: false, 11: false}
	for line, ran := range want {
		if r, ok := got[line]; !ok || r != ran {
			t.Errorf("lib.ts:%d = %v (instrumented %v), want %v; all %v", line, r, ok, ran, got)
		}
	}
	if _, ok := l["out/lib.js"]; ok {
		t.Errorf("generated file counted beside its source: %v", l)
	}
	ix, err := linecov.Build("fixture", "c", "node", map[string]string{"lib.ts": "oid"},
		[]linecov.Record{{Package: "lib.test.js", Name: "adds", Profile: l.Profile("fixture")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Files) != 1 || ix.Files[0].Path != "lib.ts" {
		t.Fatalf("index files %+v", ix.Files)
	}
	for _, b := range ix.Files[0].Blocks {
		if b.End != b.Start+1 || b.StartCol != 1 || b.EndCol != 1 {
			t.Errorf("block %+v is not one line", b)
		}
		if ran := len(b.Tests) > 0; ran != want[int(b.Start)] && b.Start != 1 {
			t.Errorf("block line %d ran %v", b.Start, ran)
		}
	}
}

// TestAddV8CachedMap covers LPCV-V0-965: Node's source-map-cache, keyed by
// script URL with line lengths, maps without reading the generated script.
func TestAddV8CachedMap(t *testing.T) {
	root, _ := filepath.Abs("testdata/ts")
	js, _ := os.ReadFile(filepath.Join(root, "out/lib.js"))
	raw, _ := os.ReadFile(filepath.Join(root, "out/lib.js.map"))
	var sm SourceMap
	if err := json.Unmarshal(raw, &sm); err != nil {
		t.Fatal(err)
	}
	sm.Sources = []string{"file://" + filepath.ToSlash(filepath.Join(root, "lib.ts"))}
	var lengths []int
	for _, line := range strings.Split(string(js), "\n") {
		lengths = append(lengths, len(line))
	}
	src := string(js)
	url := "file:///gone/lib.js"
	report := Report{
		Result: []Script{{URL: url, Functions: []Function{
			{Ranges: []Range{{0, len(src), 1}}},
			{Ranges: []Range{fnRange(t, src, "function add(", 0)}},
			{Ranges: []Range{fnRange(t, src, "function mul(", 1)}},
		}}},
		SourceMapCache: map[string]CachedMap{url: {LineLengths: lengths, Data: sm}},
	}
	data, _ := json.Marshal(report)
	l := Lines{}
	read := func(string) ([]byte, error) { t.Fatal("read the generated script"); return nil, nil }
	if err := l.AddV8(data, Options{Scope: RepoScope(root, nil), Read: read}); err != nil {
		t.Fatal(err)
	}
	if l["lib.ts"][3] || !l["lib.ts"][10] {
		t.Fatalf("lines %v", l["lib.ts"])
	}
}

// TestAddV8Unmapped covers LPCV-V0-965: a script without a map counts its own
// non-blank lines, with UTF-16 offsets past a non-BMP character.
func TestAddV8Unmapped(t *testing.T) {
	root := t.TempDir()
	src := "const s = '😀';\n\nfunction f() {\n  return 1;\n}\nfunction g() {\n  return 2;\n}\n"
	file := filepath.Join(root, "a.js")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	units := func(s string) int { return len([]rune(s)) + strings.Count(s, "😀") }
	at := func(text string) int { return units(src[:strings.Index(src, text)]) }
	report := Report{Result: []Script{{URL: "file://" + filepath.ToSlash(file), Functions: []Function{
		{Ranges: []Range{{0, units(src), 1}}},
		{Ranges: []Range{{at("function f"), at("function g"), 1}}},
		{Ranges: []Range{{at("function g"), units(src), 0}}},
	}}}}
	data, _ := json.Marshal(report)
	l := Lines{}
	if err := l.AddV8(data, Options{Scope: RepoScope(root, nil)}); err != nil {
		t.Fatal(err)
	}
	want := map[int]bool{1: true, 3: true, 4: true, 5: true, 6: false, 7: false, 8: false}
	if !reflect.DeepEqual(l["a.js"], want) {
		t.Fatalf("lines %v, want %v", l["a.js"], want)
	}
}

// TestRepoScope covers LPCV-V0-965: only tracked files inside the
// repository and outside node_modules are kept.
func TestRepoScope(t *testing.T) {
	s := RepoScope("/r", func(rel string) bool { return rel != "untracked.js" })
	for abs, want := range map[string]string{
		"/r/src/a.js":            "src/a.js",
		"/r/node_modules/x/a.js": "",
		"/r/a/node_modules/b.js": "",
		"/other/a.js":            "",
		"/r/untracked.js":        "",
	} {
		got, ok := s(abs)
		if got != want || ok != (want != "") {
			t.Errorf("%s = %q %v, want %q", abs, got, ok, want)
		}
	}
}

func TestDecodeMappings(t *testing.T) {
	type seg struct{ gl, gc, s, sl int }
	var got []seg
	// "AAAA;AACA,IAAI" : (0,0)->(0,0); (1,0)->(1,0), (1,4)->(1,0)
	err := decodeMappings("AAAA;AACA,IAAI;;A,gBAAgB", func(gl, gc, s, sl int) { got = append(got, seg{gl, gc, s, sl}) })
	if err != nil {
		t.Fatal(err)
	}
	want := []seg{{0, 0, 0, 0}, {1, 0, 0, 1}, {1, 4, 0, 1}, {3, 16, 0, 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if decodeMappings("AA", func(int, int, int, int) {}) == nil {
		t.Fatal("a two-field segment decoded")
	}
	if decodeMappings("A!", func(int, int, int, int) {}) == nil {
		t.Fatal("a non-base64 byte decoded")
	}
}

// TestReadSourceMapInline covers an inline data URL and a dangling file
// reference, which counts the script itself.
func TestReadSourceMapInline(t *testing.T) {
	m := `{"version":3,"sources":["a.ts"],"mappings":"AAAA"}`
	b64 := encode([]byte(m))
	sm, _, err := ReadSourceMap("/r/a.js", []byte("x\n//# sourceMappingURL=data:application/json;base64,"+b64+"\n"))
	if err != nil || sm == nil || sm.Sources[0] != "a.ts" {
		t.Fatalf("inline map %+v %v", sm, err)
	}
	sm, _, err = ReadSourceMap(filepath.Join(t.TempDir(), "a.js"), []byte("//# sourceMappingURL=a.js.map"))
	if err != nil || sm != nil {
		t.Fatalf("dangling map %+v %v", sm, err)
	}
}

func encode(b []byte) string {
	var out strings.Builder
	for i := 0; i < len(b); i += 3 {
		var n uint
		k := min(3, len(b)-i)
		for j := 0; j < 3; j++ {
			n <<= 8
			if j < k {
				n |= uint(b[i+j])
			}
		}
		for j := 0; j <= k; j++ {
			out.WriteByte(vlqChars[n>>(18-6*j)&63])
		}
	}
	return out.String()
}

// TestAddIstanbul covers LPCV-V0-965: the statement declaring an uncalled
// function ran, but its body did not.
func TestAddIstanbul(t *testing.T) {
	data := `{"/r/a.ts":{"path":"/r/a.ts",
	 "statementMap":{"0":{"start":{"line":1},"end":{"line":3}},"1":{"start":{"line":2},"end":{"line":2}},"2":{"start":{"line":5},"end":{"line":5}}},
	 "s":{"0":1,"1":0,"2":4},
	 "fnMap":{"0":{"loc":{"start":{"line":1},"end":{"line":3}}}},"f":{"0":0}},
	 "/elsewhere/b.ts":{"path":"/elsewhere/b.ts","statementMap":{"0":{"start":{"line":1},"end":{"line":1}}},"s":{"0":1},"fnMap":{},"f":{}}}`
	l := Lines{}
	if err := l.AddIstanbul([]byte(data), RepoScope("/r", nil)); err != nil {
		t.Fatal(err)
	}
	want := Lines{"a.ts": {1: false, 2: false, 3: false, 5: true}}
	if !reflect.DeepEqual(l, want) {
		t.Fatalf("got %v, want %v", l, want)
	}
}

// TestAddLCOV covers LPCV-V0-965 for bun and deno tracefiles.
func TestAddLCOV(t *testing.T) {
	data := "TN:\nSF:src/a.ts\nDA:1,1\nDA:2,0\nend_of_record\nSF:file:///r/b.js\nDA:3,2\nend_of_record\nSF:/x/c.js\nDA:1,1\nend_of_record\n"
	l := Lines{}
	if err := l.AddLCOV([]byte(data), "/r", RepoScope("/r", nil)); err != nil {
		t.Fatal(err)
	}
	want := Lines{"src/a.ts": {1: true, 2: false}, "b.js": {3: true}}
	if !reflect.DeepEqual(l, want) {
		t.Fatalf("got %v, want %v", l, want)
	}
	if err := l.AddLCOV([]byte("SF:/r/a.js\nDA:x\n"), "/r", RepoScope("/r", nil)); err == nil {
		t.Fatal("malformed DA accepted")
	}
}

// TestAddLCOVSourceMap covers LPCV-V0-965: bun reports the generated script
// it ran, which counts against its TypeScript original through its map.
func TestAddLCOVSourceMap(t *testing.T) {
	root, _ := filepath.Abs("testdata/ts")
	l := Lines{}
	data := "SF:out/lib.js\nDA:24,1\nDA:25,1\nDA:26,1\nDA:27,0\nDA:29,1\nDA:31,1\nDA:32,0\nend_of_record\n"
	if err := l.AddLCOV([]byte(data), root, RepoScope(root, func(rel string) bool { return rel == "lib.ts" })); err != nil {
		t.Fatal(err)
	}
	ts := l["lib.ts"]
	if len(l) != 1 || !ts[3] || !ts[4] || ts[5] || !ts[7] || ts[10] {
		t.Fatalf("got %v", l)
	}
}

func TestProfileMergesAcrossProcesses(t *testing.T) {
	a, b := Lines{}, Lines{}
	a.Add("x.js", 2, false)
	b.Add("x.js", 2, true)
	b.Add("x.js", 1, false)
	a.Merge(b)
	got := string(a.Profile("m"))
	want := "mode: set\nm/x.js:1.1,2.1 1 0\nm/x.js:2.1,3.1 1 1\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
