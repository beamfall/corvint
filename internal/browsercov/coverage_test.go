// SPDX-License-Identifier: Apache-2.0

package browsercov

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

func repoWith(t *testing.T, files map[string]string) *Repo {
	t.Helper()
	root := t.TempDir()
	r := NewRepo(root, map[string]string{})
	for p, body := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		r.OIDs[p] = r.blobOID([]byte(body))
	}
	return NewRepo(root, r.OIDs)
}

// at returns the UTF-16 offset of the first occurrence of sub in s.
func at(t *testing.T, s, sub string) int {
	t.Helper()
	i := strings.Index(s, sub)
	if i < 0 {
		t.Fatalf("%q not in source", sub)
	}
	return len([]rune(s[:i])) // fixtures are BMP-only below each probe
}

const app = `function add(a, b) {
  return a + b;
}
function neg(a) {
  if (a < 0) {
    return "neg";
  }
  return "pos";
}
`

// cov is V8 block coverage of app for a run that called add only, or neg
// with a non-negative argument.
func cov(t *testing.T, callAdd bool) []Function {
	n := len([]rune(app))
	addStart, addEnd := at(t, app, "function add"), at(t, app, "function neg")
	negStart := addEnd
	thenStart, thenEnd := at(t, app, "{\n    return \"neg\""), at(t, app, "\n  return \"pos\"")
	addCount, negCount := 0, 1
	if callAdd {
		addCount, negCount = 1, 0
	}
	fns := []Function{
		{Ranges: []Range{{0, n, 1}}},
		{FunctionName: "add", Ranges: []Range{{addStart, addEnd - 1, addCount}}},
		{FunctionName: "neg", Ranges: []Range{{negStart, n - 1, negCount}}},
	}
	if !callAdd {
		fns[2].Ranges = append(fns[2].Ranges, Range{thenStart, thenEnd, 0})
	}
	return fns
}

func TestPaintInnermostWins(t *testing.T) {
	u := paint(10, []Function{{Ranges: []Range{{0, 10, 1}, {2, 8, 0}}}, {Ranges: []Range{{4, 6, 3}}}})
	want := []int8{1, 1, 0, 0, 1, 1, 0, 0, 1, 1}
	if !bytes.Equal(int8s(u), int8s(want)) {
		t.Fatalf("paint = %v, want %v", u, want)
	}
}

func int8s(v []int8) []byte {
	b := make([]byte, len(v))
	for i, x := range v {
		b[i] = byte(x)
	}
	return b
}

// TestRecordsShareBlocksAcrossRuns proves two runs of one file emit the same
// block spans, so linecov.Build unions them, and that each run's executed
// set is its own: a line only one test executed selects only it, and a line
// no test executed selects none (LPCV-V0-950, LPCV-V0-952, LPCV-V0-973).
func TestRecordsShareBlocksAcrossRuns(t *testing.T) {
	r := repoWith(t, map[string]string{"public/app.js": app})
	var runs []Run
	for _, name := range []string{"adds", "negates"} {
		obs, reason := r.Attribute(Script{URL: "http://localhost:3000/app.js", Source: app, Functions: cov(t, name == "adds")})
		if reason != "" || len(obs) != 1 || obs[0].Path != "public/app.js" {
			t.Fatalf("Attribute = %+v, %q", obs, reason)
		}
		runs = append(runs, Run{Package: "tests/app.spec.js", Name: name, Obs: obs})
	}
	recs := Records("example.com/m", runs)
	spans := func(p []byte) string {
		var out []string
		for _, l := range strings.Split(string(p), "\n")[1:] {
			if i := strings.LastIndexByte(l, ' '); i > 0 {
				out = append(out, l[:i])
			}
		}
		return strings.Join(out, "\n")
	}
	if spans(recs[0].Profile) != spans(recs[1].Profile) {
		t.Fatalf("block spans differ:\n%s\n---\n%s", recs[0].Profile, recs[1].Profile)
	}
	ix, err := linecov.Build("example.com/m", "c0", "browser-test", r.OIDs, recs)
	if err != nil {
		t.Fatal(err)
	}
	tests := func(line int32) []string {
		var out []string
		seen := map[string]bool{}
		for _, b := range ix.Files[0].Blocks {
			if b.Start == line {
				for _, id := range b.Tests {
					if n := ix.Tests[id].Name; !seen[n] {
						seen[n] = true
						out = append(out, n)
					}
				}
			}
		}
		return out
	}
	for line, want := range map[int32]string{2: "adds", 5: "negates", 6: "", 8: "negates"} {
		if got := strings.Join(tests(line), ","); got != want {
			t.Errorf("line %d executed by %q, want %q", line, got, want)
		}
	}
}

// TestAttributeSourceMap binds a bundle's coverage to the original file
// through an inline map whose sourcesContent is the tracked blob.
func TestAttributeSourceMap(t *testing.T) {
	orig := "export function hello() {\n  return 1;\n}\nexport function bye() {\n  return 2;\n}\n"
	r := repoWith(t, map[string]string{"src/greet.ts": orig})
	// Generated line 0 maps to original line 1 ("return 1"); line 1 maps to
	// original line 4 ("return 2").
	bundle := "a();\nb();\n"
	mapJSON := `{"version":3,"sources":["webpack://app/./src/greet.ts"],"sourcesContent":[` + quote(orig) + `],"mappings":"AACA;AAGA"}`
	bundle += "//# sourceMappingURL=data:application/json;base64," + base64.StdEncoding.EncodeToString([]byte(mapJSON))
	fns := []Function{{Ranges: []Range{{0, len(bundle), 1}}}, {Ranges: []Range{{5, 9, 0}}}}
	obs, reason := r.Attribute(Script{URL: "http://localhost:3000/bundle.js", Source: bundle, Functions: fns})
	if reason != "" || len(obs) != 1 || obs[0].Path != "src/greet.ts" {
		t.Fatalf("Attribute = %+v, %q", obs, reason)
	}
	if !obs[0].Lines[1] || obs[0].Lines[4] {
		t.Fatalf("lines = %v, want 1 executed and 4 unexecuted", obs[0].Lines)
	}
}

// TestAttributeMapSourceRoot: a map's sourceRoot prefixes each source, and an
// absolute source names its file, which must be tracked (LPCV-V0-972).
func TestAttributeMapSourceRoot(t *testing.T) {
	orig := "export function hello() {\n  return 1;\n}\n"
	r := repoWith(t, map[string]string{"src/greet.ts": orig, "lib/greet.ts": orig})
	bundle := "a();\n"
	script := func(sourceRoot, source string) Script {
		m := `{"version":3,"sourceRoot":` + quote(sourceRoot) + `,"sources":[` + quote(source) + `],"mappings":"AACA"}`
		src := bundle + "//# sourceMappingURL=data:application/json;base64," + base64.StdEncoding.EncodeToString([]byte(m))
		return Script{URL: "http://localhost:3000/bundle.js", Source: src, Functions: []Function{{Ranges: []Range{{0, len(src), 1}}}}}
	}
	obs, reason := r.Attribute(script("lib", "greet.ts"))
	if reason != "" || len(obs) != 1 || obs[0].Path != "lib/greet.ts" {
		t.Fatalf("sourceRoot: Attribute = %+v, %q", obs, reason)
	}
	obs, reason = r.Attribute(script("", "file://"+filepath.ToSlash(filepath.Join(r.Root, "src/greet.ts"))))
	if reason != "" || len(obs) != 1 || obs[0].Path != "src/greet.ts" {
		t.Fatalf("absolute source: Attribute = %+v, %q", obs, reason)
	}
	if _, reason = r.Attribute(script("", "file://"+filepath.ToSlash(filepath.Join(r.Root, "src/new.ts")))); reason == "" {
		t.Fatal("an untracked in-repository source bound")
	}
	for _, c := range [][3]string{{"", "a.ts", "a.ts"}, {"src", "a.ts", "src/a.ts"}, {"src/", "a.ts", "src/a.ts"}, {"src", "/abs/a.ts", "/abs/a.ts"}, {"src", "webpack://x/a.ts", "webpack://x/a.ts"}} {
		if got := joinSourceRoot(c[0], c[1]); got != c[2] {
			t.Errorf("joinSourceRoot(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// TestAttributeFileBOM: Node's offsets start after a byte-order mark.
func TestAttributeFileBOM(t *testing.T) {
	body := "\uFEFFrun();\n"
	r := repoWith(t, map[string]string{"server.js": body})
	obs, reason := r.Attribute(Script{URL: "file://" + filepath.ToSlash(filepath.Join(r.Root, "server.js")), Functions: []Function{{Ranges: []Range{{0, 7, 1}}}}})
	if reason != "" || len(obs) != 1 {
		t.Fatalf("Attribute = %+v, %q", obs, reason)
	}
	if u := obs[0].Units; len(u) != 8 || u[0] != -1 || u[1] != 1 {
		t.Fatalf("units = %v, want the mark unpainted and run() executed", u)
	}
}

// TestRunnerConfig: only the runners' configuration files are exempt.
func TestRunnerConfig(t *testing.T) {
	for rel, want := range map[string]bool{"playwright.config.js": true, "e2e/cypress.config.mjs": true, "src/config.js": false} {
		if isRunnerConfig(rel) != want {
			t.Errorf("isRunnerConfig(%q) = %v", rel, !want)
		}
	}
}

func quote(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n")
	return "\"" + r.Replace(s) + "\""
}

// TestAttributeRefusals: unbindable local code makes the record incomplete;
// a third-party origin and the runners' own scripts do not (LPCV-V0-954).
func TestAttributeRefusals(t *testing.T) {
	r := repoWith(t, map[string]string{"public/app.js": app, "server.ts": "x"})
	r.Generated = []string{filepath.Join(r.Root, ".corvint-work")}
	for _, c := range []struct {
		s          Script
		incomplete bool
	}{
		{Script{URL: "http://localhost:3000/other.js", Source: "other()"}, true},
		{Script{URL: "https://cdn.example.com/lib.js", Source: "lib()"}, false},
		{Script{URL: "http://localhost:3000/__cypress/runner/cypress_runner.js", Source: "x"}, false},
		{Script{URL: "http://[::1]:3000/__cypress-cy-prompt/driver/cy-prompt.js", Source: "x"}, false},
		{Script{URL: "<embedded>", Source: "snapshot()"}, false},
		{Script{URL: "http://[::1]:3000/", Source: `(()=>{const n=window.Cypress=parent.Cypress;})()`}, false},
		{Script{URL: "file://" + filepath.ToSlash(filepath.Join(r.Root, "server.ts"))}, true},
		{Script{URL: "file://" + filepath.ToSlash(filepath.Join(r.Root, "node_modules/x/index.js"))}, false},
		{Script{URL: "http://localhost:3000/app.js", Source: app + "//# sourceMappingURL=app.js.map", SourceMap: &fetchedMap{URL: "x", Error: "HTTP 404"}}, true},
		{Script{URL: "file://" + filepath.ToSlash(filepath.Join(r.Root, "build/out.js"))}, true},
		{Script{URL: "file://" + filepath.ToSlash(filepath.Join(r.Root, ".corvint-work/hook.cjs"))}, false},
	} {
		_, reason := r.Attribute(c.s)
		if (reason != "") != c.incomplete {
			t.Errorf("%s: reason %q, want incomplete=%v", c.s.URL, reason, c.incomplete)
		}
	}
}

func TestAttributeInlineHTML(t *testing.T) {
	page := "<html><body><script>\nwindow.x = 1;\n</script></body></html>\n"
	r := repoWith(t, map[string]string{"public/index.html": page})
	src := "\nwindow.x = 1;\n"
	obs, reason := r.Attribute(Script{URL: "http://localhost:3000/", Source: src, Functions: []Function{{Ranges: []Range{{0, len(src), 1}}}}})
	if reason != "" || len(obs) != 1 || obs[0].Path != "public/index.html" {
		t.Fatalf("Attribute = %+v, %q", obs, reason)
	}
	if obs[0].Units[strings.Index(page, "window")] != 1 || obs[0].Units[0] != -1 {
		t.Fatal("inline script offsets not shifted into the page")
	}
}

// LPCV-V0-8301: anonymous eval and Function bodies cannot bind immutable code.
func TestAnonymousScriptIncomplete(t *testing.T) {
	r := repoWith(t, map[string]string{"app.js": app})
	for _, source := range []string{app, `return new Function("return 7")()`} {
		obs, reason := r.Attribute(Script{Source: source, Functions: []Function{{Ranges: []Range{{0, len(source), 1}}}}})
		if len(obs) != 0 || reason != "script without a URL cannot bind coverage" {
			t.Fatalf("anonymous attribution=%v reason=%q", obs, reason)
		}
	}
}

// LPCV-V0-8301: an external map on disk cannot legitimize untracked Node output.
func TestBuildOutputMapRemainsIncomplete(t *testing.T) {
	r := repoWith(t, map[string]string{"src/app.ts": "export const x:number=1;\n"})
	for _, f := range []struct{ name, text string }{
		{"bundle.js", "const x=1;\n//# sourceMappingURL=bundle.js.map\n"},
		{"bundle.js.map", `{"version":3,"sources":["src/app.ts"],"sourcesContent":["export const x:number=1;\n"],"mappings":"AAAA"}`},
	} {
		if err := os.WriteFile(filepath.Join(r.Root, f.name), []byte(f.text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	obs, reason := r.Attribute(Script{URL: "file://" + filepath.ToSlash(filepath.Join(r.Root, "bundle.js")), Functions: []Function{{Ranges: []Range{{0, 10, 1}}}}})
	if len(obs) != 0 || reason != "bundle.js runs but is not tracked at the index commit" {
		t.Fatalf("build attribution=%v reason=%q", obs, reason)
	}
}
