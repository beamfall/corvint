package linecov

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// decideFixture is the collected fixture module of linecov_test.go, its
// records, and the commit they were collected at.
type decideFixture struct {
	dir, base string
	records   []Record
	oids      map[string]string
}

var fixtureDirs = []string{"calc", "cmd/tool", "e2e", "ord"}

// fixtureReach is the static plan of a fixture path's directory: calc is
// imported by cmd/tool, which e2e builds and executes.
var fixtureReach = map[string][]string{
	"calc":     {"calc", "cmd/tool", "e2e"},
	"cmd/tool": {"cmd/tool", "e2e"},
	"e2e":      {"e2e"},
	"ord":      {"ord"},
}

func collectFixture(t *testing.T) decideFixture {
	t.Helper()
	requireGo(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, fixture)
	git(t, dir, "init", "-q")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "base")
	base := git(t, dir, "rev-parse", "HEAD")
	outcomes, err := Collect(ctx, dir, CollectOptions{Module: "example.com/m", Packages: fixtureDirs, Work: t.TempDir(), Parallel: 4, Timeout: 5 * time.Minute, SuiteTimeout: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	var records []Record
	for _, o := range outcomes {
		if o.Record == nil {
			t.Fatalf("%s.%s diverged alone: %s", o.Package, o.Name, o.Divergent)
		}
		records = append(records, *o.Record)
	}
	oids, err := OIDs(ctx, dir, base)
	if err != nil {
		t.Fatal(err)
	}
	return decideFixture{dir: dir, base: base, records: records, oids: oids}
}

// index builds the fixture index without the records drop names, writes it
// as linecov-index/4 and returns its path.
func (f decideFixture) index(t *testing.T, mutate func(*Index), drop ...string) string {
	t.Helper()
	var records []Record
	for _, r := range f.records {
		if !strings.Contains(" "+strings.Join(drop, " ")+" ", " "+r.Package+"."+r.Name+" ") {
			records = append(records, r)
		}
	}
	ix, err := Build("example.com/m", f.base, "fixture-runner", f.oids, records)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(ix)
	}
	file := filepath.Join(t.TempDir(), "linecov.idx")
	var b bytes.Buffer
	if err := ix.Encode(&b); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// blind empties each in-order record's reach, keeping the record: the index
// of a run in order that executed none of the changed blocks, which leaves
// the per-test rules to decide (GLS-V0-008). A collected in-order run
// executes every block an isolated test does, so a collected index keeps the
// package whole for order instead.
func blind(also ...func(*Index)) func(*Index) {
	return func(ix *Index) {
		for i := range ix.Files {
			for j := range ix.Files[i].Blocks {
				b := &ix.Files[i].Blocks[j]
				var kept []int32
				for _, id := range b.Tests {
					if ix.Tests[id].Name != InOrder {
						kept = append(kept, id)
					}
				}
				b.Tests = kept
			}
		}
		for _, f := range also {
			f(ix)
		}
	}
}

// commit checks out from, applies each old->new replacement and commits.
func (f decideFixture) commit(t *testing.T, from string, edits ...[3]string) string {
	t.Helper()
	git(t, f.dir, "checkout", "-q", "--detach", from)
	for _, e := range edits {
		full := filepath.Join(f.dir, filepath.FromSlash(e[0]))
		src, err := os.ReadFile(full)
		if err != nil && e[1] != "" {
			t.Fatal(err)
		}
		if e[1] == "" {
			src = append(src, e[2]...)
		} else {
			if bytes.Count(src, []byte(e[1])) != 1 {
				t.Fatalf("edit site %q of %s not unique", e[1], e[0])
			}
			src = bytes.Replace(src, []byte(e[1]), []byte(e[2]), 1)
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, f.dir, "add", "-A")
	git(t, f.dir, "commit", "-qm", "edit")
	return git(t, f.dir, "rev-parse", "HEAD")
}

// plan opens file lazily and plans base..head with every fixture package as
// the static plan and fixtureReach as the static walk.
func (f decideFixture) plan(t *testing.T, file, base, head string, unknown bool) Selection {
	t.Helper()
	ctx := context.Background()
	ix, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	drift, err := Diff(ctx, f.dir, ix.Commit, base)
	if err != nil {
		t.Fatal(err)
	}
	change, err := Diff(ctx, f.dir, base, head)
	if err != nil {
		t.Fatal(err)
	}
	readDrift, err := BlobReader(ctx, f.dir, drift)
	if err != nil {
		t.Fatal(err)
	}
	readChange, err := BlobReader(ctx, f.dir, change)
	if err != nil {
		t.Fatal(err)
	}
	// The static plan: every unit a changed path's directory reaches.
	var units []Unit
	seen := map[string]bool{}
	for _, c := range change {
		for _, d := range fixtureReach[path.Dir(c.Path)] {
			if !seen[d] {
				seen[d] = true
				units = append(units, Unit{ID: "go:example.com/m/" + d, Dir: d, Witness: c.Path})
			}
		}
	}
	git(t, f.dir, "checkout", "-q", "--detach", head)
	return ix.Plan(PlanInput{
		Base: base, Head: head, Drift: drift, Change: change, ReadDrift: readDrift, ReadChange: readChange, Units: units, Environ: os.Environ(),
		Heads: func(dirs []string) (map[string][]string, error) { return HeadTestNames(f.dir, dirs) },
		StaticOf: func(paths []string) (bool, map[string]string, error) {
			reach := map[string]string{}
			for _, p := range paths {
				for _, d := range fixtureReach[path.Dir(p)] {
					if _, ok := reach[d]; !ok {
						reach[d] = p
					}
				}
			}
			return unknown, reach, nil
		},
	})
}

// modes renders each unit as dir=mode[:kept,kept] for comparison.
func modes(s Selection) []string {
	var out []string
	for _, u := range s.Units {
		entry := strings.TrimPrefix(u.Unit, "go:example.com/m/") + "=" + string(u.Mode)
		if u.Mode == Whole && u.Category != "" {
			entry += "(" + u.Category + ")"
		}
		if u.Mode == Tests {
			var kept []string
			for _, k := range u.Kept {
				kept = append(kept, k.Name+"/"+k.Why)
			}
			entry += ":" + strings.Join(kept, ",")
		}
		out = append(out, entry)
	}
	sort.Strings(out)
	return out
}

// GLS-V0-011..016 on a collected fixture: each path class, the drift, the
// unrecorded and unordered rules, and the exclusion entry.
func TestDecide(t *testing.T) {
	f := collectFixture(t)
	full := f.index(t, nil)
	cases := []struct {
		name    string
		file    string // index; empty is full
		drift   [][3]string
		change  [][3]string
		unknown bool
		want    []string
		state   string
		classes []string // side:class of every analysed path, sorted
	}{
		{name: "a line one test executed selects that test alone",
			file:    "blind",
			change:  [][3]string{{"calc/calc.go", `return "zero"`, `return "nil"`}},
			want:    []string{"calc=tests:TestZero/selected", "cmd/tool=exclude", "e2e=exclude"},
			classes: []string{"change:body"}},
		{name: "a line the in-order run executed keeps its package whole",
			change:  [][3]string{{"calc/calc.go", `return "zero"`, `return "nil"`}},
			want:    []string{"calc=whole(order-sensitive)", "cmd/tool=exclude", "e2e=exclude"},
			classes: []string{"change:body"}},
		{name: "an unexecuted line selects no test",
			change:  [][3]string{{"calc/calc.go", "return 1", "return 2"}},
			want:    []string{"calc=compile", "cmd/tool=exclude", "e2e=exclude"},
			classes: []string{"change:body"}},
		{name: "a body reached only through a started binary keeps the E2E test",
			file:    "blind",
			change:  [][3]string{{"calc/calc.go", "return n * 2", "return n * 3"}},
			want:    []string{"calc=tests:TestDouble/selected", "cmd/tool=exclude", "e2e=whole"},
			classes: []string{"change:body"}},
		{name: "a declaration edit keeps its static reach whole",
			change:  [][3]string{{"calc/calc.go", "var Base = 10", "var Base = 11"}},
			want:    []string{"calc=whole(declaration)", "cmd/tool=whole(declaration)", "e2e=whole(declaration)"},
			classes: []string{"change:declaration"}},
		{name: "a declaration edit beside a body edit keeps only the declaration's reach whole",
			change:  [][3]string{{"calc/calc.go", "var Base = 10", "var Base = 11"}, {"ord/ord.go", "calls++", "calls += 1"}},
			want:    []string{"calc=whole(declaration)", "cmd/tool=whole(declaration)", "e2e=whole(declaration)", "ord=whole(order-sensitive)"},
			classes: []string{"change:body", "change:declaration"}},
		{name: "a test-function edit keeps its package whole for the in-order run",
			file:    "blind",
			change:  [][3]string{{"calc/calc_test.go", `Classify(0), "zero"`, `Classify(0), "pos"`}},
			want:    []string{"calc=whole(order-sensitive)", "cmd/tool=exclude", "e2e=exclude"},
			classes: []string{"change:test-function"}},
		{name: "a test helper edit keeps its package whole",
			change:  [][3]string{{"calc/calc_test.go", "if got != exp {", "if got == exp {"}},
			want:    []string{"calc=whole(test-helper)", "cmd/tool=whole(test-helper)", "e2e=whole(test-helper)"},
			classes: []string{"change:test-helper"}},
		{name: "a test file added at the head keeps its package whole",
			change:  [][3]string{{"calc/calc.go", "return n * 2", "return n * 3"}, {"calc/new_test.go", "", "package calc\n\nimport \"testing\"\n\nfunc TestFresh(t *testing.T) {}\n"}},
			want:    []string{"calc=whole(added)", "cmd/tool=whole(added)", "e2e=whole(added)"},
			classes: []string{"change:added", "change:body"}},
		{name: "a test whose blocks only the in-order run holds keeps its package whole",
			file:    "drop",
			change:  [][3]string{{"calc/calc.go", "return n * 2", "return n * 3"}},
			want:    []string{"calc=whole(order-sensitive)", "cmd/tool=exclude", "e2e=whole(order-sensitive)"},
			classes: []string{"change:body"}},
		{name: "a package with records but no in-order record stays whole",
			file:    "drop-both",
			change:  [][3]string{{"calc/calc.go", "return n * 2", "return n * 3"}},
			want:    []string{"calc=whole(unordered)", "cmd/tool=exclude", "e2e=whole(order-sensitive)"},
			classes: []string{"change:body"}},
		{name: "a test with no record is kept as unrecorded",
			file:    "drop-blind",
			change:  [][3]string{{"calc/calc.go", "return n * 2", "return n * 3"}},
			want:    []string{"calc=tests:TestDouble/selected,TestNeg/unrecorded", "cmd/tool=exclude", "e2e=whole"},
			classes: []string{"change:body"}},
		{name: "an incomplete record is kept as unrecorded",
			file:    "incomplete",
			change:  [][3]string{{"calc/calc.go", "return n * 2", "return n * 3"}},
			want:    []string{"calc=tests:TestDouble/selected,TestZero/unrecorded", "cmd/tool=exclude", "e2e=whole"},
			classes: []string{"change:body"}},
		{name: "a drift test-function edit keeps its package whole for the in-order run",
			file:    "blind",
			drift:   [][3]string{{"calc/calc_test.go", `Classify(-1), "neg"`, `Classify(-2), "neg"`}},
			change:  [][3]string{{"calc/calc.go", "return n * 2", "return n * 3"}},
			want:    []string{"calc=whole(order-sensitive)", "cmd/tool=exclude", "e2e=whole"},
			classes: []string{"change:body", "drift:test-function"}},
		// The stale rule in tests mode is TestNarrowKeepsUnrecordedAndStale:
		// any test-function edit here reaches calc's in-order record.
		{name: "a drift body edit beside a test-function edit keeps the package whole",
			file:    "blind",
			drift:   [][3]string{{"calc/calc.go", "return n * 2", "return n * 4"}},
			change:  [][3]string{{"calc/calc_test.go", `Classify(0), "zero"`, `Classify(0), "pos"`}},
			want:    []string{"calc=whole(order-sensitive)", "cmd/tool=exclude", "e2e=whole"},
			classes: []string{"change:test-function", "drift:body"}},
		{name: "a drift declaration edit outside a unit's reach leaves it to its records",
			drift:   [][3]string{{"calc/calc.go", "var Base = 10", "var Base = 12"}},
			change:  [][3]string{{"ord/ord.go", "calls++", "calls += 1"}},
			want:    []string{"ord=whole(order-sensitive)"},
			classes: []string{"change:body", "drift:declaration"}},
		{name: "a drift declaration edit keeps its reach whole",
			drift:   [][3]string{{"calc/calc.go", "var Base = 10", "var Base = 12"}},
			change:  [][3]string{{"e2e/e2e_test.go", `!= "20"`, `!= "20" && false`}},
			want:    []string{"e2e=whole(declaration)"},
			classes: []string{"change:test-function", "drift:declaration"}},
		{name: "a path the drift changed is a blob mismatch",
			drift:   [][3]string{{"calc/calc.go", `return "pos"`, `return "positive"`}},
			change:  [][3]string{{"calc/calc.go", `return "zero"`, `return "nil"`}},
			want:    []string{"calc=whole(blob-mismatch)", "cmd/tool=whole(blob-mismatch)", "e2e=whole(blob-mismatch)"},
			classes: []string{"change:blob-mismatch", "drift:body"}},
		{name: "an unordered package stays whole",
			file:    "unordered",
			change:  [][3]string{{"calc/calc.go", `return "zero"`, `return "nil"`}, {"ord/ord.go", `return "first"`, `return "first!"`}},
			want:    []string{"calc=tests:TestZero/selected", "cmd/tool=exclude", "e2e=exclude", "ord=whole(unordered)"},
			classes: []string{"change:body", "change:body"}},
		{name: "a non-Go input keeps its static reach whole",
			file:    "blind",
			change:  [][3]string{{"calc/calc.go", `return "zero"`, `return "nil"`}, {"cmd/tool/notes.txt", "", "x\n"}},
			want:    []string{"calc=tests:TestZero/selected", "cmd/tool=whole(non-go)", "e2e=whole(non-go)"},
			classes: []string{"change:body", "change:non-go"}},
		{name: "an unknown static plan of the fallback paths falls back",
			change:  [][3]string{{"calc/calc.go", `return "zero"`, `return "nil"`}, {"cmd/tool/notes.txt", "", "x\n"}},
			unknown: true, state: StateFallback,
			classes: []string{"change:body", "change:non-go"}},
		{name: "every changed path a fallback keeps every unit whole without a walk",
			change:  [][3]string{{"calc/calc.go", "var Base = 10", "var Base = 11"}},
			unknown: true,
			want:    []string{"calc=whole(declaration)", "cmd/tool=whole(declaration)", "e2e=whole(declaration)"},
			classes: []string{"change:declaration"}},
	}
	files := map[string]string{
		"":           full,
		"blind":      f.index(t, blind()),
		"drop":       f.index(t, nil, "calc.TestNeg"),
		"drop-both":  f.index(t, nil, "calc.TestNeg", "calc."+InOrder),
		"drop-blind": f.index(t, blind(), "calc.TestNeg"),
		"incomplete": f.index(t, blind(func(ix *Index) { ix.Tests[ix.byName["calc\x00TestZero"]].Incomplete = true })),
		"unordered":  f.index(t, blind(func(ix *Index) { ix.Unordered = []string{"ord"} })),
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := f.base
			if c.drift != nil {
				base = f.commit(t, f.base, c.drift...)
			}
			head := f.commit(t, base, c.change...)
			sel := f.plan(t, files[c.file], base, head, c.unknown)
			var classes []string
			for _, p := range sel.Paths {
				classes = append(classes, p.Side+":"+p.Class)
			}
			sort.Strings(classes)
			if !reflect.DeepEqual(classes, c.classes) {
				t.Errorf("classes %v, want %v", classes, c.classes)
			}
			want := c.state
			if want == "" {
				want = StateNarrowed
			}
			if sel.State != want {
				t.Fatalf("state %s (%s %s), want %s", sel.State, sel.Reason, sel.Detail, want)
			}
			if want == StateFallback {
				if sel.Reason != ReasonFallbackScopeUnknown || len(sel.Units) != 0 {
					t.Fatalf("fallback %s decided %d units", sel.Reason, len(sel.Units))
				}
				return
			}
			if got := modes(sel); !reflect.DeepEqual(got, c.want) {
				t.Errorf("units %v, want %v", got, c.want)
			}
			excluded := 0
			for _, u := range sel.Units {
				excluded += u.Excluded
				if u.Mode == Tests && u.Run == "" || u.Mode == Compile && u.Run != "^$" {
					t.Errorf("%s: mode %s run %q", u.Unit, u.Mode, u.Run)
				}
			}
			if (excluded > 0) != (sel.Exclusion != nil) {
				t.Fatalf("%d excluded tests, exclusion entry %+v", excluded, sel.Exclusion)
			}
			if sel.Exclusion != nil && (sel.Exclusion.Record != sel.Index.Digest || !strings.HasPrefix(sel.Exclusion.Record, "sha256:") ||
				sel.Exclusion.Invalidation != Invalidation || len(sel.Exclusion.Changed) == 0) {
				t.Errorf("exclusion entry %+v", sel.Exclusion)
			}
			if sel.Counts.TestsExcluded != excluded || sel.Counts.Units != len(sel.Units) {
				t.Errorf("counts %+v", sel.Counts)
			}
		})
	}
}

// GLS-V0-001, GLS-V0-002: a /4 index round-trips, and a lazy open answers
// as a whole decode does while reading only the sections a change touches.
func TestCompactRoundTrip(t *testing.T) {
	f := collectFixture(t)
	file := f.index(t, nil)
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	whole, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Build("example.com/m", f.base, "fixture-runner", f.oids, f.records)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(whole.Tests, ix.Tests) || !reflect.DeepEqual(whole.Files, ix.Files) || whole.Digest() == "" {
		t.Fatal("round trip changed the index")
	}
	lazy, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer lazy.Close()
	if lazy.Digest() != whole.Digest() {
		t.Fatalf("digests %s and %s", lazy.Digest(), whole.Digest())
	}
	for _, loaded := range lazy.lazy.loaded {
		if loaded {
			t.Fatal("open decoded a section")
		}
	}
	head := f.commit(t, f.base, [3]string{"calc/calc.go", `return "zero"`, `return "nil"`})
	ctx := context.Background()
	diffs, err := Diff(ctx, f.dir, f.base, head)
	if err != nil {
		t.Fatal(err)
	}
	read, err := BlobReader(ctx, f.dir, diffs)
	if err != nil {
		t.Fatal(err)
	}
	a, b := whole.Analyze(diffs, read), lazy.Analyze(diffs, read)
	// TestZero and calc's in-order record, which executed the line.
	if !reflect.DeepEqual(a.Tests, b.Tests) || len(a.Tests) != 2 {
		t.Fatalf("whole selects %v, lazy %v", a.Tests, b.Tests)
	}
	for i, loaded := range lazy.lazy.loaded {
		if loaded != (lazy.Files[i].Path == "calc/calc.go") {
			t.Errorf("section %s loaded=%v", lazy.Files[i].Path, loaded)
		}
	}
	// An older gob index still reads, and re-encodes as /4.
	ix.Format = FormatV3
	var old bytes.Buffer
	if err := ix.Encode(&old); err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(old.Bytes(), []byte(magicV4)) {
		t.Fatal("a FormatV3 index encoded as /4")
	}
	back, err := Decode(&old)
	if err != nil || back.Format != Format || !reflect.DeepEqual(back.Files, whole.Files) {
		t.Fatalf("gob /3: %v", err)
	}
}

// GLS-V0-002: a damaged index decides nothing.
func TestCompactDamage(t *testing.T) {
	f := collectFixture(t)
	file := f.index(t, nil)
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	h, base, err := splitHeader(data)
	if err != nil {
		t.Fatal(err)
	}
	write := func(t *testing.T, b []byte) string {
		p := filepath.Join(t.TempDir(), "x.idx")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	flip := func(at int) []byte {
		b := append([]byte(nil), data...)
		b[at] ^= 0xff
		return b
	}
	headerAt := len(magicV4) + 8
	for name, b := range map[string][]byte{
		"bad magic":          append([]byte("linecov-index/9\n"), data[len(magicV4):]...),
		"truncated header":   data[:headerAt+len(h)/2],
		"header length huge": append(append(append([]byte(nil), data[:len(magicV4)]...), binary.LittleEndian.AppendUint64(nil, 1<<40)...), data[headerAt:]...),
		"header byte":        flip(headerAt + 3),
		"section cut":        data[:len(data)-1],
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(bytes.NewReader(b)); err == nil {
				t.Error("Decode accepted it")
			}
			if ix, err := Open(write(t, b)); err == nil {
				// A flipped header byte may still parse; its sections then
				// cannot all verify, so no analysis may succeed silently.
				if err := ix.loadAll(); err == nil {
					t.Error("Open accepted it and every section verified")
				}
				ix.Close()
			}
		})
	}
	// A damaged section: Open succeeds, the path touching it falls back.
	ix, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	calc := ix.byPath["calc/calc.go"]
	_, rows, err := parseHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	bad := write(t, flip(int(base+rows[calc].offset)))
	lazy, err := Open(bad)
	if err != nil {
		t.Fatal(err)
	}
	defer lazy.Close()
	head := f.commit(t, f.base, [3]string{"calc/calc.go", `return "zero"`, `return "nil"`})
	ctx := context.Background()
	diffs, _ := Diff(ctx, f.dir, f.base, head)
	read, _ := BlobReader(ctx, f.dir, diffs)
	results, _ := lazy.AnalyzePaths(diffs, read)
	if len(results) != 1 || results[0].Class != ClassSectionInvalid {
		t.Fatalf("results %+v", results)
	}
	if e := lazy.Analyze(diffs, read); e.Fallback == "" {
		t.Fatal("Analyze mapped a damaged section")
	}
}

// GLS-V0-016: head test names under every build constraint.
func TestHeadTestNames(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"p/a_test.go": "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\nfunc Test_b(t *testing.T) {}\nfunc Testhelper(t *testing.T) {}\nfunc TestMain(m *testing.M) {}\nfunc ExampleA() {}\nfunc FuzzA(f *testing.F) {}\n" +
			"type s struct{}\n\nfunc (s) TestMethod(t *testing.T) {}\n\nvar src = `\nfunc TestInString(t *testing.T) {}\n`\n\n// func TestInComment(t *testing.T) {}\nfunc helper() { func() {}() }\n",
		"p/w_test.go":      "//go:build windows\n\npackage p\n\nimport \"testing\"\n\nfunc TestWindowsOnly(t *testing.T) {}\n",
		"p/broken_test.go": "package p\n\nfunc TestBroken(t *testing.T) { if {\n",
		"p/p.go":           "package p\n\nfunc TestNotATestFile() {}\n",
		"m/main_test.go":   "package m\n\nimport \"testing\"\n\nfunc TestMain(m *testing.M) { m.Run() }\n",
		"s/s.go":           "package s\n",
	})
	got, err := HeadTestNames(dir, []string{"p", "missing", "m", "s"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ExampleA", "FuzzA", "TestA", "TestBroken", "TestWindowsOnly", "Test_b"}
	if !reflect.DeepEqual(got["p"], want) || len(got["missing"]) != 0 {
		t.Fatalf("names %v, want %v", got, want)
	}
	// A TestMain-only binary still runs: an empty, non-nil list; a directory
	// with no test file, or none at all, lists nothing (GLS-V0-008).
	if m, ok := got["m"]; !ok || m == nil || len(m) != 0 {
		t.Fatalf("TestMain-only names %#v (%v)", m, ok)
	}
	if got["s"] != nil || got["missing"] != nil {
		t.Fatalf("test-less names %#v %#v", got["s"], got["missing"])
	}
}

// GLS-V0-009: -p bounds parallelism only and leaves the identity.
func TestRunnerIdentityDropsParallelism(t *testing.T) {
	for in, want := range map[string]string{
		"":                        "",
		"-p=4":                    "",
		"-race -p=4":              "-race",
		"-p 8 -tags=x":            "-tags=x",
		"--p=2 -race -mod=mod":    "-race -mod=mod",
		"-race -pgo=off":          "-race -pgo=off",
		"-tags=p -parallel=3 -p4": "-tags=p -parallel=3 -p4",
	} {
		if got := identityFlags(in); got != want {
			t.Errorf("identityFlags(%q) = %q, want %q", in, got, want)
		}
	}
	test := strings.Join(TestRuntime(DefaultTestTimeout), " ")
	a := RunnerIdentity("go1", "linux", "amd64", "1", "-race -p=4", "", "GOAMD64=v1", "", "", test)
	b := RunnerIdentity("go1", "linux", "amd64", "1", "-race", "", "GOAMD64=v1", "", "", test)
	if a != b || a == RunnerIdentity("go1", "linux", "amd64", "1", "", "", "GOAMD64=v1", "", "", test) || a == RunnerIdentity("go1", "linux", "amd64", "1", "-race", "", "GOAMD64=v2", "", "", test) {
		t.Fatalf("identities %q %q", a, b)
	}
	if got := ArchFeatures(map[string]string{"GOAMD64": "v3", "GOFIPS140": "off", "GOARM": ""}); got != "GOAMD64=v3 GOFIPS140=off" {
		t.Fatalf("ArchFeatures %q", got)
	}
}

// GLS-V0-020: the test runtime go test -json passes, deadline included, is
// part of the identity, RunnerTestTimeout reads the deadline back, and a
// GOFLAGS test flag, which collection would not apply, is refused.
func TestRunnerIDBindsTestRuntime(t *testing.T) {
	requireGo(t)
	ctx := context.Background()
	dir := t.TempDir()
	env := append(os.Environ(), "GOFLAGS=-race -p=4 -count=1")
	ids := map[time.Duration]string{}
	for _, timeout := range []time.Duration{DefaultTestTimeout, 50 * time.Minute, 0} {
		id, _, err := RunnerID(ctx, dir, env, timeout, "")
		if err != nil {
			t.Fatal(err)
		}
		if again, _, err := RunnerID(ctx, dir, env, timeout, ""); err != nil || again != id {
			t.Fatalf("timeout %v: identity %q then %q (%v)", timeout, id, again, err)
		}
		if got, ok := RunnerTestTimeout(id); !ok || got != timeout {
			t.Errorf("RunnerTestTimeout(%q) = %v %v, want %v", id, got, ok, timeout)
		}
		ids[timeout] = id
	}
	if !strings.Contains(ids[DefaultTestTimeout], `test="-test.paniconexit0 -test.v=test2json -test.timeout=10m0s"`) || ids[DefaultTestTimeout] == ids[50*time.Minute] || ids[DefaultTestTimeout] == ids[0] {
		t.Fatalf("identities %q", ids)
	}
	if _, ok := RunnerTestTimeout(`go1 linux amd64 arch="" cgo=1 goflags="-race" goexperiment="" atomic`); ok {
		t.Error("an identity without a test runtime named one")
	}
	for _, flags := range []string{"-v", "-v=false", "-json", "-json=false", "-test.v=false", "-short", "-timeout=1m", "-run=TestA", "-count=2", "-cover", "-failfast"} {
		if id, _, err := RunnerID(ctx, dir, append(os.Environ(), "GOFLAGS=-race "+flags), DefaultTestTimeout, ""); err == nil || !strings.Contains(err.Error(), flags) {
			t.Errorf("GOFLAGS -race %s: %q %v, want a refusal naming it", flags, id, err)
		}
	}
}

// GLS-V0-009: an architecture feature setting is part of the identity, so
// an index collected under GOAMD64=v1 decides nothing for a v2 runner.
func TestRunnerIDBindsArchFeatures(t *testing.T) {
	requireGo(t)
	ctx := context.Background()
	id := func(env ...string) string {
		t.Helper()
		got, _, err := RunnerID(ctx, t.TempDir(), append(os.Environ(), env...), DefaultTestTimeout, "")
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	v1, v2 := id("GOAMD64=v1"), id("GOAMD64=v2")
	if v1 == v2 || !strings.Contains(v2, "GOAMD64=v2") {
		t.Fatalf("GOAMD64 v1 and v2 identities %q and %q", v1, v2)
	}
	if again := id("GOAMD64=v1"); again != v1 {
		t.Fatalf("one setting, two identities %q and %q", v1, again)
	}
}

// GLS-V0-008: a package collected in parts merges into the index one
// collection would write, and a skipped in-order run no part ran leaves the
// package unordered.
func TestMergeResolvesInOrderParts(t *testing.T) {
	prof := func(blocks ...string) []byte {
		var b strings.Builder
		b.WriteString("mode: atomic\n")
		for _, x := range blocks {
			b.WriteString("example.com/m/p/p.go:" + x + "\n")
		}
		return []byte(b.String())
	}
	oids := map[string]string{"p/p.go": "blob"}
	isolated := []Record{
		{Package: "p", Name: "TestA", Profile: prof("3.1,4.2 1 1", "6.1,7.2 1 0")},
		{Package: "p", Name: "TestB", Profile: prof("3.1,4.2 1 0", "6.1,7.2 1 1")},
	}
	partA, err := Build("example.com/m", "c", "r", oids, isolated)
	if err != nil {
		t.Fatal(err)
	}
	partA.InOrderSkipped = []string{"p"}
	inOrder := func(blocks ...string) *Index {
		ix, err := Build("example.com/m", "c", "r", oids, []Record{{Package: "p", Name: InOrder, Profile: prof(blocks...)}})
		if err != nil {
			t.Fatal(err)
		}
		ix.InOrderRan = []string{"p"}
		return ix
	}
	merged, err := Merge(partA, inOrder("3.1,4.2 1 1", "6.1,7.2 1 1"))
	if err != nil {
		t.Fatal(err)
	}
	// Isolated records holding every block the run executed do not show the
	// tests behave in order as alone: the in-order record stays.
	if len(merged.Tests) != 3 || merged.Tests[0].Name != InOrder || len(merged.Unordered) != 0 || merged.unorderedSet()["p"] {
		t.Fatalf("covered in-order record dropped: %+v unordered %v", merged.Tests, merged.Unordered)
	}
	merged, err = Merge(partA, inOrder("3.1,4.2 1 1", "9.1,9.9 1 1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Tests) != 3 || merged.Tests[0].Name != InOrder {
		t.Fatalf("order-only block dropped: %+v", merged.Tests)
	}
	alone, err := Merge(partA)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(alone.Unordered, []string{"p"}) || !partA.unorderedSet()["p"] {
		t.Fatalf("a skipped in-order run no part ran is ordered: %v", alone.Unordered)
	}
	// An index with records of p and no in-order record of it, as Build
	// wrote before it kept every in-order record, has unknown order effects.
	bare, err := Build("example.com/m", "c", "r", oids, isolated)
	if err != nil {
		t.Fatal(err)
	}
	if !bare.unorderedSet()["p"] {
		t.Fatal("a package with no in-order record is ordered")
	}
}

// GLS-V0-008: -tests and -inorder bound what one collection runs.
func TestCollectParts(t *testing.T) {
	requireGo(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, fixture)
	git(t, dir, "init", "-q")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "base")
	names := func(o CollectOptions) []string {
		o.Module, o.Packages, o.Work, o.Parallel, o.Timeout, o.SuiteTimeout = "example.com/m", []string{"calc"}, t.TempDir(), 2, time.Minute, time.Minute
		outcomes, err := Collect(ctx, dir, o)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, x := range outcomes {
			out = append(out, x.Name)
		}
		entries, _ := os.ReadDir(o.Work)
		if len(entries) != 0 {
			t.Errorf("scratch keeps %d entries after the package ran", len(entries))
		}
		return out
	}
	if got := names(CollectOptions{InOrder: InOrderOnly}); !reflect.DeepEqual(got, []string{InOrder}) {
		t.Errorf("only: %v", got)
	}
	if got := names(CollectOptions{InOrder: InOrderSkip, Tests: regexpMust(t, "^TestZ")}); !reflect.DeepEqual(got, []string{"TestZero"}) {
		t.Errorf("skip with filter: %v", got)
	}
	if got := names(CollectOptions{}); len(got) != 4 {
		t.Errorf("run: %v", got)
	}
	_ = runtime.GOOS
}

func regexpMust(t *testing.T, re string) *regexp.Regexp {
	t.Helper()
	r, err := regexp.Compile(re)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// GLS-V0-008: a stripped record keeps the mode line and every executed
// block, so memory grows with executed blocks, not tests times the module.
func TestCompactProfile(t *testing.T) {
	in := "mode: atomic\nm/p.go:1.1,2.2 1 0\nm/p.go:3.1,4.2 1 10\nm/p.go:5.1,6.2 2 1\nm/q.go:1.1,1.9 1 0"
	seen := map[string]bool{}
	if got := string(compactProfile([]byte(in), seen)); got != in {
		t.Fatalf("first profile lost instrumentation: %q", got)
	}
	want := "mode: atomic\nm/p.go:3.1,4.2 1 10\nm/p.go:5.1,6.2 2 1\n"
	if got := string(compactProfile([]byte(in), seen)); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	child := "mode: atomic\nm/child.go:1.1,2.2 1 0\nm/p.go:1.1,2.2 1 1\nmalformed\n"
	if got := string(compactProfile([]byte(child), seen)); got != child {
		t.Fatalf("new child instrumentation, later hit or malformed input lost: %q", got)
	}
}

// GLS-V0-009: a setting that changes what a test executes but that the
// identity does not carry (GODEBUG, a C toolchain or cgo flags, a go.work
// workspace) is refused rather than modelled, so selection is
// runner-unavailable; GOWORK=off and an unchanged environment are read.
func TestRunnerIDRefusesUncarriedEnvironment(t *testing.T) {
	requireGo(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"go.mod": "module example.com/m\n\ngo 1.22\n"})
	var base []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if name != "GOFLAGS" && name != "GOENV" && name != "GOWORK" && name != "GODEBUG" && name != "CC" && name != "CXX" && !strings.HasPrefix(name, "CGO_") {
			base = append(base, kv)
		}
	}
	base = append(base, "GOENV=off")
	for _, kv := range []string{"", "GOWORK=off"} {
		if _, _, err := RunnerID(ctx, dir, append(base, kv), DefaultTestTimeout, ""); err != nil {
			t.Errorf("%q: %v, want an identity", kv, err)
		}
	}
	work := filepath.Join(t.TempDir(), "go.work")
	if err := os.WriteFile(work, []byte("go 1.22\n\nuse "+dir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, kv := range []string{"GODEBUG=http2client=0", "CC=cc-other", "CXX=c++-other", "CGO_CFLAGS=-O0", "CGO_LDFLAGS=-lm", "GOWORK=" + work} {
		name, _, _ := strings.Cut(kv, "=")
		if id, _, err := RunnerID(ctx, dir, append(base, kv), DefaultTestTimeout, ""); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: %q %v, want a refusal naming %s", kv, id, err, name)
		}
	}
}
