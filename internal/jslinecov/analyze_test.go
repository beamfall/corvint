package jslinecov

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

// TestBodies covers LPCV-V0-966: function, arrow, method and TypeScript
// return-typed bodies are found; statement blocks, object literals, object
// return types and braces in strings, templates, comments and regular
// expressions are not.
func TestBodies(t *testing.T) {
	src := `// { not code
function a(x) {
  if (x) {
    return "}";
  }
  return ` + "`${x}{`" + `;
}
const b = (y) => {
  const o = { k: /[}]/.test(y) };
  return o;
};
class C {
  m(z) {
    for (;;) { break; }
  }
  n(): { v: number } {
    return { v: 1 };
  }
}
function d(q): Promise<string> {
  return q ? (q) : { r: 1 };
}
const e = async (s: string): Promise<void> => {
  /* } */
};
if ((x ? f(1) : 2)) {
  g();
}
`
	got, err := bodies([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []body{{2, 7}, {8, 11}, {13, 15}, {16, 18}, {20, 22}, {23, 25}}
	if !reflect.DeepEqual(sorted(got), want) {
		t.Fatalf("bodies %v, want %v", sorted(got), want)
	}
}

func sorted(bs []body) []body {
	out := append([]body{}, bs...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j].open < out[i].open {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// TestBodiesRefuse covers LPCV-V0-966: input the scanner cannot read is an
// error, never a guessed body.
func TestBodiesRefuse(t *testing.T) {
	for name, src := range map[string]string{
		"unbalanced": "function f() {\n",
		"extra":      "}\n",
		"string":     "const s = 'a\nb';\n",
		"comment":    "/* x",
		"regex":      "const r = /x\n/;\n",
		"template":   "const t = `${a`;\n",
	} {
		if _, err := bodies([]byte(src)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := bodies([]byte("const v = () => {\n  return <div>{x}</div>;\n};\n")); !errors.Is(err, errJSX) {
		t.Fatalf("JSX: %v", err)
	}
	if _, err := bodies([]byte("const lt = a < b;\nconst q = x <y;\n")); err != nil {
		t.Fatalf("comparison read as JSX: %v", err)
	}
}

// fixture is lib.js at blob "o1" with two functions, and an index whose test
// "adds" ran add's then-branch, "big" its else-branch, and "muls" ran mul.
const fixtureSrc = `function add(a, b) {
  const s = a + b;

  if (s > 100) {
    return 100;
  }
  return s;
}
function mul(a, b) {
  return a * b;
}
module.exports = { add, mul };
`

func fixtureIndex(t *testing.T) *linecov.Index {
	t.Helper()
	prof := func(ran ...int) []byte {
		hit := map[int]bool{}
		for _, l := range ran {
			hit[l] = true
		}
		out := "mode: set\n"
		for _, l := range []int{1, 2, 4, 5, 7, 9, 10, 12} {
			n := "0"
			if hit[l] {
				n = "1"
			}
			out += "m/lib.js:" + itoa(l) + ".1," + itoa(l+1) + ".1 1 " + n + "\n"
		}
		return []byte(out)
	}
	ix, err := linecov.Build("m", "c", "v8-lines/1 node-test", map[string]string{"lib.js": "o1", "lib.test.js": "t1"}, []linecov.Record{
		{Package: "lib.test.js", Name: "adds", Profile: prof(1, 2, 4, 7, 9, 12)},
		{Package: "lib.test.js", Name: "big", Profile: prof(1, 2, 4, 5, 9, 12)},
		{Package: "mul.test.js", Name: "muls", Profile: prof(1, 9, 10, 12)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func names(ix *linecov.Index, e linecov.Effect) []string {
	var out []string
	for i, tt := range ix.Tests {
		if e.Tests[int32(i)] {
			out = append(out, tt.Name)
		}
	}
	return out
}

// TestAnalyze covers LPCV-V0-966: line edits select the tests that ran them,
// an edit on an uninstrumented line selects the body's tests, and edits the
// index cannot bound fall back or keep the test file whole.
func TestAnalyze(t *testing.T) {
	ix := fixtureIndex(t)
	read := func(p string) ([]byte, error) {
		if p == "lib.js" {
			return []byte(fixtureSrc), nil
		}
		return nil, errors.New("absent")
	}
	edit := func(hunks ...linecov.Hunk) []linecov.FileDiff {
		return []linecov.FileDiff{{Path: "lib.js", OldOID: "o1", NewOID: "o2", Hunks: hunks}}
	}
	for name, c := range map[string]struct {
		diffs    []linecov.FileDiff
		tests    []string
		fallback bool
	}{
		"then branch":    {diffs: edit(linecov.Hunk{Start: 5, End: 5}), tests: []string{"big"}},
		"common line":    {diffs: edit(linecov.Hunk{Start: 7, End: 7}), tests: []string{"adds"}},
		"mul body":       {diffs: edit(linecov.Hunk{Start: 10, End: 10}), tests: []string{"muls"}},
		"blank line":     {diffs: edit(linecov.Hunk{Start: 3, End: 3}), tests: []string{"adds", "big", "muls"}},
		"insert after 4": {diffs: edit(linecov.Hunk{Start: 4, End: 3}), tests: []string{"adds", "big"}},
		"insert after 2": {diffs: edit(linecov.Hunk{Start: 2, End: 1}), tests: []string{"adds", "big", "muls"}},
		"signature":      {diffs: edit(linecov.Hunk{Start: 1, End: 1}), fallback: true},
		"top level":      {diffs: edit(linecov.Hunk{Start: 12, End: 12}), fallback: true},
		"other blob":     {diffs: []linecov.FileDiff{{Path: "lib.js", OldOID: "x", NewOID: "o2", Hunks: []linecov.Hunk{{Start: 5, End: 5}}}}, fallback: true},
		"uninstrumented": {diffs: []linecov.FileDiff{{Path: "util.js", OldOID: "u", NewOID: "v", Hunks: []linecov.Hunk{{Start: 1, End: 1}}}}, fallback: true},
		"non-JS":         {diffs: []linecov.FileDiff{{Path: "package.json", OldOID: "p", NewOID: "q"}}, fallback: true},
		"deleted":        {diffs: []linecov.FileDiff{{Path: "lib.js", OldOID: "o1"}}, fallback: true},
		"test helper":    {diffs: []linecov.FileDiff{{Path: "helper.test.js", OldOID: "h", NewOID: "i"}}, fallback: true},
	} {
		e := Analyze(ix, c.diffs, read)
		if (e.Fallback != "") != c.fallback {
			t.Errorf("%s: fallback %q", name, e.Fallback)
			continue
		}
		if got := names(ix, e); !c.fallback && !reflect.DeepEqual(got, c.tests) {
			t.Errorf("%s: tests %v, want %v", name, got, c.tests)
		}
	}
	e := Analyze(ix, []linecov.FileDiff{
		{Path: "lib.test.js", OldOID: "t1", NewOID: "t2", Hunks: []linecov.Hunk{{Start: 3, End: 3}}},
		{Path: "new.spec.ts", NewOID: "n"},
	}, read)
	if e.Fallback != "" || !e.Whole["lib.test.js"] || !e.Whole["new.spec.ts"] || len(e.Tests) != 0 {
		t.Fatalf("test file edits %+v", e)
	}
}

// TestAnalyzeInOrder covers LPCV-V0-966 with LPCV-V0-962: a line only the
// in-order run reached keeps its test file whole.
func TestAnalyzeInOrder(t *testing.T) {
	ix, err := linecov.Build("m", "c", "r", map[string]string{"lib.js": "o1"}, []linecov.Record{
		{Package: "a.test.js", Name: "one", Profile: []byte("mode: set\nm/lib.js:2.1,3.1 1 1\nm/lib.js:5.1,6.1 1 0\n")},
		{Package: "a.test.js", Name: linecov.InOrder, Profile: []byte("mode: set\nm/lib.js:2.1,3.1 1 1\nm/lib.js:5.1,6.1 1 1\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	e := Analyze(ix, []linecov.FileDiff{{Path: "lib.js", OldOID: "o1", NewOID: "o2", Hunks: []linecov.Hunk{{Start: 5, End: 5}}}},
		func(string) ([]byte, error) { return []byte(fixtureSrc), nil })
	if e.Fallback != "" || !e.Whole["a.test.js"] {
		t.Fatalf("effect %+v", e)
	}
}
