package linecov

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// The segment fixture is one product file of single-block functions, so a
// hunk on a function's first body line reaches exactly that block.
var segFuncs = []string{"login", "open", "checkA", "checkB", "ping", "search", "admin", "health"}

const segPath = "app/app.go"

func segLine(name string) int {
	for i, f := range segFuncs {
		if f == name {
			return 3 + 5*i
		}
	}
	panic(name)
}

func segSource() []byte {
	var b strings.Builder
	b.WriteString("package app\n\n")
	for i, f := range segFuncs {
		fmt.Fprintf(&b, "func %s() int {\n\tv := %d\n\treturn v\n}\n\n", strings.ToUpper(f[:1])+f[1:], i)
	}
	return []byte(b.String())
}

func segRead(string) ([]byte, error) { return segSource(), nil }

// segProfile lists every block of the fixture, executed when named in ran.
func segProfile(ran ...string) []byte {
	in := map[string]bool{}
	for _, r := range ran {
		in[r] = true
	}
	var b strings.Builder
	b.WriteString("mode: atomic\n")
	for _, f := range segFuncs {
		l := segLine(f)
		n := 0
		if in[f] {
			n = 1
		}
		fmt.Fprintf(&b, "example.com/m/%s:%d.%d,%d.2 2 %d\n", segPath, l, len("func "+f+"() int {")+1, l+3, n)
	}
	return []byte(b.String())
}

func segChange(names ...string) []FileDiff {
	d := FileDiff{Path: segPath, OldOID: "blob1", NewOID: "blob2"}
	for _, n := range names {
		l := segLine(n) + 1
		d.Hunks = append(d.Hunks, Hunk{Start: l, End: l})
	}
	return []FileDiff{d}
}

type segSpec struct {
	kind  SegmentKind
	id    string
	bound bool
	prior string
	units []string
	ran   []string
}

type testSpec struct {
	name       string
	elapsed    time.Duration
	level      string
	incomplete bool
	segs       []segSpec
	ran        []string // test-level blocks, inside or outside the segments
}

// step is a bound segment of the given kind reaching one function.
func step(kind SegmentKind, id string, ran ...string) segSpec {
	return segSpec{kind: kind, id: id, bound: true, ran: ran}
}

func segRecords(specs []testSpec, segmented bool) []Record {
	var recs []Record
	for _, s := range specs {
		r := Record{Package: "e2e", Name: s.name, Elapsed: s.elapsed, Level: s.level, Incomplete: s.incomplete}
		ran := append([]string(nil), s.ran...)
		for _, g := range s.segs {
			ran = append(ran, g.ran...)
			if segmented {
				r.Segments = append(r.Segments, SegmentRecord{Kind: g.kind, ID: g.id, Bound: g.bound, Prior: g.prior, Units: g.units, Elapsed: time.Second, Profile: segProfile(g.ran...)})
			}
		}
		r.Profile = segProfile(ran...)
		recs = append(recs, r)
	}
	return recs
}

func buildSeg(t *testing.T, specs ...testSpec) *Index {
	t.Helper()
	ix, err := Build("example.com/m", "c0", "go test", map[string]string{segPath: "blob1"}, segRecords(specs, true))
	if err != nil {
		t.Fatal(err)
	}
	return roundTrip(t, ix)
}

func roundTrip(t *testing.T, ix *Index) *Index {
	t.Helper()
	var b bytes.Buffer
	if err := ix.Encode(&b); err != nil {
		t.Fatal(err)
	}
	out, err := Decode(&b)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The cart fixture: A and B run the same setup, step and check; C shares the
// setup and step but checks something else afterwards. B is cheaper than A.
var cart = []testSpec{
	{name: "TestCartA", elapsed: 3 * time.Second, segs: []segSpec{step(KindSetup, "setup:signin", "login"), step(KindStep, "step:open", "open"), step(KindMethod, "Cart#checkA", "checkA")}},
	{name: "TestCartB", elapsed: 2 * time.Second, segs: []segSpec{step(KindSetup, "setup:signin", "login"), step(KindStep, "step:open", "open"), step(KindMethod, "Cart#checkA", "checkA")}},
	{name: "TestCartC", elapsed: 1 * time.Second, segs: []segSpec{step(KindSetup, "setup:signin", "login"), step(KindStep, "step:open", "open"), step(KindMethod, "Cart#checkB", "checkB")}},
}

func excluded(sh ShadowReport) map[string]string {
	out := map[string]string{}
	for _, ex := range sh.Exclusions {
		var reps []string
		for _, v := range ex.Via {
			reps = append(reps, v.Representative)
		}
		sort.Strings(reps)
		out[ex.Test] = strings.Join(reps, ",")
	}
	return out
}

func shadowOf(t *testing.T, ix *Index, change ...string) (Join, ShadowReport) {
	t.Helper()
	e := ix.Analyze(segChange(change...), segRead)
	if e.Fallback != "" {
		t.Fatalf("fallback %s", e.Fallback)
	}
	j := ix.Join(e)
	return j, ix.Shadow(j, nil)
}

// MCJ-V0-001: a segmented record keeps its segments, in order, through an
// encode and decode; blocks it ran outside every segment form one trailing
// unbound segment; and its test-level reach, hence selection, equals the
// same records without segments.
func TestSegmentedIndexRoundTripsAndSelectsAsBefore(t *testing.T) {
	specs := append(append([]testSpec(nil), cart...),
		testSpec{name: "TestExtra", segs: []segSpec{step(KindStep, "step:ping", "ping")}, ran: []string{"health"}},
		testSpec{name: "TestPlain", ran: []string{"admin", "open"}})
	ix := buildSeg(t, specs...)
	if ix.Format != Format || Format != "linecov-index/3" {
		t.Fatalf("format %q", ix.Format)
	}
	extra := ix.byName["e2e\x00TestExtra"]
	var got []string
	for _, s := range ix.TestSegments(extra) {
		seg := ix.Segment(s)
		got = append(got, fmt.Sprintf("%s:%s:%v", seg.Kind, seg.Label(), seg.Bound))
	}
	if want := []string{"step:step:ping:true", ":(outside every segment):false"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TestExtra segments %q, want %q", got, want)
	}
	plain := ix.byName["e2e\x00TestPlain"]
	if segs := ix.TestSegments(plain); len(segs) != 1 || ix.Segment(segs[0]).Kind != KindTest || ix.Segment(segs[0]).Bound {
		t.Fatalf("unsegmented record: %v", segs)
	}
	flat, err := Build("example.com/m", "c0", "go test", map[string]string{segPath: "blob1"}, segRecords(specs, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(flat.Segments) != 0 {
		t.Fatal("an index of unsegmented records stores segments")
	}
	for _, f := range segFuncs {
		a := ix.Analyze(segChange(f), segRead)
		b := flat.Analyze(segChange(f), segRead)
		if !reflect.DeepEqual(a.Tests, b.Tests) {
			t.Errorf("%s: segmented selects %v, unsegmented %v", f, a.Tests, b.Tests)
		}
	}
}

// Build refuses one test recorded twice: the second record would silently
// replace the first.
func TestBuildRefusesATestRecordedTwice(t *testing.T) {
	recs := segRecords([]testSpec{{name: "TestX", ran: []string{"open"}}}, false)
	if _, err := Build("example.com/m", "c0", "go test", map[string]string{segPath: "blob1"}, append(recs, recs...)); err == nil || !strings.Contains(err.Error(), "recorded twice") {
		t.Fatalf("err = %v", err)
	}
}

// v2Index is the linecov-index/2 wire shape, and v2Read its reader.
type (
	v2Test struct {
		Package, Name string
		Elapsed       time.Duration
	}
	v2Block struct {
		Start, StartCol, End, EndCol int32
		Tests                        []int32
	}
	v2File struct {
		Path, OID string
		Blocks    []v2Block
	}
	v2Index struct {
		Format, Module, Commit, Runner string
		Tests                          []v2Test
		Files                          []v2File
		Unordered                      []string
	}
)

func v2Read(data []byte) error {
	var ix v2Index
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&ix); err != nil {
		return err
	}
	if ix.Format != FormatV2 {
		return fmt.Errorf("index format %q, want %q", ix.Format, FormatV2)
	}
	return nil
}

// MCJ-V0-003: a /2 reader refuses /3, and /3 reads a /2 index as one
// whole-test segment per test that selects and narrows exactly as before.
func TestV2IndexReadsAsOneSegmentPerTest(t *testing.T) {
	specs := []testSpec{{name: "TestA", ran: []string{"login", "open"}}, {name: "TestB", ran: []string{"open"}}, {name: "TestC", ran: []string{"admin"}}}
	ix, err := Build("example.com/m", "c0", "go test", map[string]string{segPath: "blob1"}, segRecords(specs, false))
	if err != nil {
		t.Fatal(err)
	}
	v2 := v2Index{Format: FormatV2, Module: ix.Module, Commit: ix.Commit, Runner: ix.Runner, Unordered: []string{"z"}}
	for _, tt := range ix.Tests {
		v2.Tests = append(v2.Tests, v2Test{tt.Package, tt.Name, tt.Elapsed})
	}
	for _, f := range ix.Files {
		vf := v2File{Path: f.Path, OID: f.OID}
		for _, b := range f.Blocks {
			vf.Blocks = append(vf.Blocks, v2Block{b.Start, b.StartCol, b.End, b.EndCol, b.Tests})
		}
		v2.Files = append(v2.Files, vf)
	}
	var old bytes.Buffer
	if err := gob.NewEncoder(&old).Encode(v2); err != nil {
		t.Fatal(err)
	}
	if err := v2Read(old.Bytes()); err != nil {
		t.Fatalf("the /2 reader refuses its own index: %v", err)
	}
	read, err := Decode(bytes.NewReader(old.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for i := range read.Tests {
		segs := read.TestSegments(int32(i))
		if len(segs) != 1 || read.Segment(segs[0]).Test != int32(i) || read.Segment(segs[0]).Kind != KindTest {
			t.Fatalf("test %d segments %v", i, segs)
		}
	}
	heads := map[string][]string{"e2e": {"TestA", "TestB", "TestC"}, "app": nil}
	for _, f := range segFuncs {
		want, _ := ix.Narrow([]string{"e2e", "app"}, heads, Effect{}, ix.Analyze(segChange(f), segRead))
		got, _ := read.Narrow([]string{"e2e", "app"}, heads, Effect{}, read.Analyze(segChange(f), segRead))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: /2 plan %v, /3 plan %v", f, got, want)
		}
		a, b := ix.Analyze(segChange(f), segRead), read.Analyze(segChange(f), segRead)
		if !reflect.DeepEqual(a.Tests, b.Tests) {
			t.Errorf("%s: /2 selects %v, /3 %v", f, b.Tests, a.Tests)
		}
	}
	// The join of a /2 index names each test's whole segment.
	j := read.Join(read.Analyze(segChange("open"), segRead))
	if len(j.Tests) != 2 || !reflect.DeepEqual(j.Tests[0].Through, []string{"(whole test)"}) {
		t.Fatalf("join %+v", j.Tests)
	}
	var cur bytes.Buffer
	if err := buildSeg(t, cart...).Encode(&cur); err != nil {
		t.Fatal(err)
	}
	if err := v2Read(cur.Bytes()); err == nil {
		t.Fatal("a /2 reader accepted linecov-index/3")
	}
}

// A damaged segment table decides nothing.
func TestDecodeRefusesInconsistentSegments(t *testing.T) {
	for name, damage := range map[string]func(*Index){
		"segment of absent test": func(ix *Index) { ix.Segments[0].Test = 99 },
		"test without segment":   func(ix *Index) { ix.Segments = ix.Segments[:len(ix.Segments)-1] },
		"block names absent seg": func(ix *Index) { setFirstSegs(ix, []int32{999}) },
		"block segs disagree":    func(ix *Index) { setFirstSegs(ix, []int32{0}) },
		"v2 with segments":       func(ix *Index) { ix.Format = FormatV2 },
		"divergent without key":  func(ix *Index) { ix.Divergent = []Divergence{{}} },
	} {
		ix := buildSeg(t, cart...)
		damage(ix)
		var b bytes.Buffer
		if err := ix.Encode(&b); err != nil {
			t.Fatal(err)
		}
		if _, err := Decode(&b); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
}

// setFirstSegs replaces the segments of the first block several tests ran.
func setFirstSegs(ix *Index, segs []int32) {
	for fi := range ix.Files {
		for bi := range ix.Files[fi].Blocks {
			if len(ix.Files[fi].Blocks[bi].Tests) > 1 {
				ix.Files[fi].Blocks[bi].Segs = segs
				return
			}
		}
	}
}

// MCJ-V0-004: the join maps each changed block to the segments whose reach
// holds it and each segment to its test and units; an unbound segment joins
// no unit; a block no test ran is reported uncovered; a test whose own
// source changed is direct.
func TestJoinMapsChangedBlocksToSegmentsTestsAndUnits(t *testing.T) {
	open := step(KindStep, "step:open", "open")
	open.units = []string{"screen:cart"}
	unbound := segSpec{kind: KindStep, id: "step:open?", units: []string{"screen:ghost"}, ran: []string{"open"}}
	ix := buildSeg(t,
		testSpec{name: "TestA", segs: []segSpec{step(KindSetup, "setup:signin", "login"), open}},
		testSpec{name: "TestU", segs: []segSpec{unbound}},
	)
	e := ix.Analyze(segChange("open", "health"), segRead)
	j := ix.Join(e)
	var got []string
	for _, r := range j.Tests {
		got = append(got, fmt.Sprintf("%s %v %v %s", r.ID, r.Through, r.Units, r.Level))
	}
	want := []string{"e2e.TestA [step:open] [screen:cart] unknown", "e2e.TestU [step:open?] [] unknown"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("join\n got %q\nwant %q", got, want)
	}
	if !reflect.DeepEqual(j.Units, []string{"screen:cart"}) {
		t.Fatalf("units %v", j.Units)
	}
	if len(j.Uncovered) != 1 || ix.Files[j.Uncovered[0].File].Blocks[j.Uncovered[0].Block].Start != int32(segLine("health")) {
		t.Fatalf("uncovered %v", j.Uncovered)
	}
	if len(j.Changed) != 2 {
		t.Fatalf("changed %v", j.Changed)
	}
	refs, ok := ix.BlocksAt(segPath, "blob1", segLine("open")+1, segLine("open")+1)
	if !ok || len(refs) != 1 || len(ix.BlockSegments(refs[0])) != 2 {
		t.Fatalf("BlocksAt %v %v", refs, ok)
	}
	if _, ok := ix.BlocksAt(segPath, "other-blob", 1, 1); ok {
		t.Fatal("BlocksAt answered for another blob")
	}
	// A fallback joins nothing.
	if j := ix.Join(Effect{Fallback: "x"}); j.Fallback != "x" || j.Tests != nil {
		t.Fatalf("fallback join %+v", j)
	}
	// A test-file edit is a direct reach.
	if j := ix.Join(Effect{Tests: map[int32]bool{0: true}, Direct: map[int32]bool{0: true}}); len(j.Tests) != 1 || !j.Tests[0].Direct {
		t.Fatalf("direct join %+v", j.Tests)
	}
}

// MCJ-V0-005 and MCJ-V0-006: only bound, equal identities over an equal
// block set, from an equal prior state and with equal later identities, are
// equivalent; the cheapest selected member represents its class, ties by
// test identity.
func TestSharedStepEquivalence(t *testing.T) {
	t.Run("equivalent steps exclude all but the cheapest", func(t *testing.T) {
		ix := buildSeg(t, cart...)
		j, sh := shadowOf(t, ix, "open")
		if len(j.Tests) != 3 {
			t.Fatalf("reached %v", j.Tests)
		}
		if got := excluded(sh); !reflect.DeepEqual(got, map[string]string{"e2e.TestCartA": "e2e.TestCartB"}) {
			t.Fatalf("excluded %v", got)
		}
		ex := sh.Exclusions[0]
		if len(ex.Via) != 1 || ex.Via[0].Segment != "step:open" || ex.Via[0].Blocks != 1 || len(ex.Via[0].Reach) != 64 {
			t.Fatalf("exclusion evidence %+v", ex)
		}
	})
	t.Run("same step from the same setup, different later checks: both kept", func(t *testing.T) {
		// Condition 4: TestCartC asserts checkB after the shared step, so
		// TestCartB cannot stand in for it, nor it for TestCartB.
		ix := buildSeg(t, cart[1], cart[2])
		if _, sh := shadowOf(t, ix, "open"); len(sh.Exclusions) != 0 {
			t.Fatalf("excluded %v", excluded(sh))
		}
		if _, sh := shadowOf(t, ix, "login"); len(sh.Exclusions) != 0 {
			t.Fatalf("setup change excluded %v", excluded(sh))
		}
	})
	t.Run("ties go to the test identity", func(t *testing.T) {
		a, b := cart[0], cart[1]
		a.elapsed, b.elapsed = time.Second, time.Second
		ix := buildSeg(t, a, b)
		if _, sh := shadowOf(t, ix, "open"); !reflect.DeepEqual(excluded(sh), map[string]string{"e2e.TestCartB": "e2e.TestCartA"}) {
			t.Fatalf("excluded %v", excluded(sh))
		}
	})
	t.Run("data-dependent step: different block sets", func(t *testing.T) {
		a := testSpec{name: "TestA", segs: []segSpec{step(KindStep, "step:search", "search")}}
		b := testSpec{name: "TestB", elapsed: time.Second, segs: []segSpec{step(KindStep, "step:search", "search", "ping")}}
		if _, sh := shadowOf(t, buildSeg(t, a, b), "search"); len(sh.Exclusions) != 0 {
			t.Fatalf("excluded %v", excluded(sh))
		}
	})
	t.Run("different prior state", func(t *testing.T) {
		a := testSpec{name: "TestA", segs: []segSpec{step(KindSetup, "setup:signin", "login"), step(KindStep, "step:open", "open")}}
		b := testSpec{name: "TestB", segs: []segSpec{step(KindSetup, "setup:admin", "admin"), step(KindStep, "step:open", "open")}}
		if _, sh := shadowOf(t, buildSeg(t, a, b), "open"); len(sh.Exclusions) != 0 {
			t.Fatalf("excluded %v", excluded(sh))
		}
		// The same bound setup record as the prior state stands in for
		// equal earlier identities.
		pa := step(KindStep, "step:open", "open")
		pa.prior = "setup:signin"
		c := testSpec{name: "TestC", segs: []segSpec{step(KindStep, "step:x", "login"), pa}}
		d := testSpec{name: "TestD", elapsed: time.Second, segs: []segSpec{step(KindStep, "step:y", "admin"), pa}}
		if _, sh := shadowOf(t, buildSeg(t, c, d), "open"); !reflect.DeepEqual(excluded(sh), map[string]string{"e2e.TestC": "e2e.TestD"}) {
			t.Fatalf("excluded %v", excluded(sh))
		}
	})
	t.Run("unbound, incomplete, residual and direct keep the test", func(t *testing.T) {
		unbound := []segSpec{{kind: KindStep, id: "step:open", ran: []string{"open"}}}
		for name, specs := range map[string][]testSpec{
			"unbound":    {{name: "TestA", segs: unbound}, {name: "TestB", segs: unbound}},
			"incomplete": {{name: "TestA", incomplete: true, segs: cart[0].segs}, cart[1]},
			// A trailing block outside every segment is an unbound later
			// segment: condition 4 cannot hold.
			"residual": {{name: "TestA", segs: cart[0].segs, ran: []string{"health"}}, cart[1]},
		} {
			if _, sh := shadowOf(t, buildSeg(t, specs...), "open"); len(sh.Exclusions) != 0 {
				t.Errorf("%s: excluded %v", name, excluded(sh))
			}
		}
		ix := buildSeg(t, cart...)
		e := ix.Analyze(segChange("open"), segRead)
		e.Direct[ix.byName["e2e\x00TestCartA"]] = true
		if sh := ix.Shadow(ix.Join(e), nil); len(sh.Exclusions) != 0 {
			t.Fatalf("direct: excluded %v", excluded(sh))
		}
	})
	t.Run("a reached segment without an equivalent keeps the test", func(t *testing.T) {
		a := testSpec{name: "TestA", elapsed: 3 * time.Second, segs: []segSpec{step(KindStep, "step:open", "open"), step(KindStep, "step:ping", "ping")}}
		b := testSpec{name: "TestB", elapsed: 1 * time.Second, segs: []segSpec{step(KindStep, "step:open", "open"), step(KindStep, "step:ping", "ping")}}
		c := testSpec{name: "TestC", elapsed: 2 * time.Second, segs: []segSpec{step(KindStep, "step:open", "open"), step(KindStep, "step:ping", "ping", "health")}}
		_, sh := shadowOf(t, buildSeg(t, a, b, c), "open", "ping")
		if !reflect.DeepEqual(excluded(sh), map[string]string{"e2e.TestA": "e2e.TestB,e2e.TestB"}) {
			t.Fatalf("excluded %v", excluded(sh))
		}
	})
	t.Run("a representative that is not selected excludes nothing", func(t *testing.T) {
		ix := buildSeg(t, cart[0], cart[1])
		e := ix.Analyze(segChange("open"), segRead)
		b := ix.byName["e2e\x00TestCartB"]
		sh := ix.Shadow(ix.Join(e), func(t int32) bool { return t != b })
		if len(sh.Exclusions) != 0 {
			t.Fatalf("excluded %v", excluded(sh))
		}
	})
}

// MCJ-V0-016 and MCJ-V0-017: the report lists each would-skip test with its
// representative and evidence and counts by level and reason, while the
// plan's selection stays exactly what Narrow decided.
func TestShadowReportsAndNeverChangesSelection(t *testing.T) {
	specs := append([]testSpec(nil), cart...)
	for i := range specs {
		specs[i].level = "e2e"
	}
	specs = append(specs, testSpec{name: "TestUnit", ran: []string{"admin"}})
	ix := buildSeg(t, specs...)
	heads := map[string][]string{"e2e": {"TestCartA", "TestCartB", "TestCartC", "TestUnit", "TestNew"}}
	e := ix.Analyze(segChange("open"), segRead)
	plans, reason := ix.Narrow([]string{"e2e"}, heads, Effect{}, e)
	before := fmt.Sprint(plans)
	j := ix.Join(e)
	sh := ix.Shadow(j, nil)
	rep := ix.Report(plans, heads, j, sh)
	if reason != "" || fmt.Sprint(plans) != before {
		t.Fatalf("plans changed: %s", reason)
	}
	if sh.Mode != ShadowMode {
		t.Fatalf("mode %q", sh.Mode)
	}
	if got := plans["e2e"].Tests; !reflect.DeepEqual(got, []string{"TestCartA", "TestCartB", "TestCartC", "TestNew"}) {
		t.Fatalf("selected %v", got)
	}
	want := map[string]map[string]int{
		"e2e":     {"selected": 3},
		"unknown": {"unrecorded": 1, ReasonNotReached: 1},
	}
	if !reflect.DeepEqual(rep.Counts, want) {
		t.Fatalf("counts %v", rep.Counts)
	}
	if !reflect.DeepEqual(rep.ShadowCounts, map[string]int{"e2e": 1}) {
		t.Fatalf("shadow counts %v", rep.ShadowCounts)
	}
	for _, pt := range rep.Tests {
		if pt.Name == "TestCartA" {
			if pt.Reason != "selected" || pt.WouldSkip == nil || pt.WouldSkip.Via[0].Representative != "e2e.TestCartB" || !reflect.DeepEqual(pt.Through, []string{"step:open"}) {
				t.Fatalf("TestCartA entry %+v", pt)
			}
		} else if pt.WouldSkip != nil {
			t.Fatalf("%s would skip", pt.Name)
		}
	}
}

// faultFixture holds one shared and one non-shared case per segment kind
// (MCJ-V0-014). G and H are bound whole-test records; J is an unbound
// whole-test record as the Go collector writes.
var faultFixture = append(append([]testSpec(nil), cart...),
	testSpec{name: "TestD", elapsed: time.Second, segs: []segSpec{step(KindStep, "step:search", "search")}},
	testSpec{name: "TestF", elapsed: time.Second, segs: []segSpec{step(KindSetup, "setup:admin", "admin")}},
	testSpec{name: "TestG", elapsed: 5 * time.Second, segs: []segSpec{step(KindTest, "rec:health-ping", "ping")}},
	testSpec{name: "TestH", elapsed: 5 * time.Second, segs: []segSpec{step(KindTest, "rec:health-ping", "ping")}},
	testSpec{name: "TestI", elapsed: time.Second, segs: []segSpec{step(KindTest, "rec:other", "checkA")}},
	testSpec{name: "TestJ", elapsed: time.Second, ran: []string{"health"}},
)

var faultCases = []FaultCase{
	{Name: "shared setup", Kind: KindSetup, Diffs: segChange("login"), Keep: []string{"e2e.TestCartC"}, Representative: "e2e.TestCartB", Excluded: []string{"e2e.TestCartA"}},
	{Name: "own setup", Kind: KindSetup, Diffs: segChange("admin"), Keep: []string{"e2e.TestF"}},
	{Name: "shared step", Kind: KindStep, Diffs: segChange("open"), Keep: []string{"e2e.TestCartC"}, Representative: "e2e.TestCartB", Excluded: []string{"e2e.TestCartA"}},
	{Name: "own step", Kind: KindStep, Diffs: segChange("search"), Keep: []string{"e2e.TestD"}},
	{Name: "shared method", Kind: KindMethod, Diffs: segChange("checkA"), Keep: []string{"e2e.TestI"}, Representative: "e2e.TestCartB", Excluded: []string{"e2e.TestCartA"}},
	{Name: "own method", Kind: KindMethod, Diffs: segChange("checkB"), Keep: []string{"e2e.TestCartC"}},
	{Name: "shared whole test", Kind: KindTest, Diffs: segChange("ping"), Representative: "e2e.TestG", Excluded: []string{"e2e.TestH"}},
	{Name: "own whole test", Kind: KindTest, Diffs: segChange("health"), Keep: []string{"e2e.TestJ"}},
}

// MCJ-V0-014: per segment kind, a change reaching a non-shared segment keeps
// its test and a change reaching only an equivalent segment excludes all but
// the representative; a wrong expectation fails its kind.
func TestFaultInjectionPerSegmentKind(t *testing.T) {
	ix := buildSeg(t, faultFixture...)
	var results []FaultResult
	for _, c := range faultCases {
		r := ix.Fault(c, segRead)
		if !r.Pass {
			t.Errorf("%s: %s", c.Name, r.Detail)
		}
		results = append(results, r)
	}
	matrix := FaultMatrix(faultCases, results)
	for _, k := range SegmentKinds {
		if !matrix[k] {
			t.Errorf("kind %s not qualified", k)
		}
	}
	// Negative controls: a fixture that drops condition 4 or a wrong
	// representative must fail.
	bad := faultCases[2]
	bad.Representative = "e2e.TestCartC"
	if r := ix.Fault(bad, segRead); r.Pass {
		t.Fatal("a wrong representative passed")
	}
	bad = faultCases[3]
	bad.Keep = []string{"e2e.TestCartA"}
	if r := ix.Fault(bad, segRead); r.Pass {
		t.Fatal("an unreached keep passed")
	}
	if FaultMatrix(faultCases[:1], results[:1])[KindSetup] {
		t.Fatal("a kind without a keep case qualified")
	}
}

// MCJ-V0-018: a miss names both tests and the change, drafts a bugs ticket,
// marks its class divergent in the next index while its reach blobs hold, so
// the class excludes nothing, and blocks promotion.
func TestMissDetectorMarksClassDivergent(t *testing.T) {
	ix := buildSeg(t, cart...)
	_, sh := shadowOf(t, ix, "open")
	out := DetectMisses(sh, "base..head", map[string]bool{"e2e.TestCartA": false, "e2e.TestCartB": true, "e2e.TestCartC": true})
	if len(out.Misses) != 1 || out.Misses[0].Excluded != "e2e.TestCartA" || out.Misses[0].Representative != "e2e.TestCartB" || len(out.Misses[0].Files) != 1 {
		t.Fatalf("misses %+v", out)
	}
	ticket := out.Misses[0].Ticket()
	if !reflect.DeepEqual(ticket.Labels, []string{"agent-memory", "bugs"}) || !strings.Contains(ticket.Body, "e2e.TestCartA") || !strings.Contains(ticket.Body, "base..head") {
		t.Fatalf("ticket %+v", ticket)
	}
	if o := DetectMisses(sh, "c", map[string]bool{"e2e.TestCartA": false, "e2e.TestCartB": false}); len(o.Misses) != 0 {
		t.Fatalf("both failing is no miss: %+v", o)
	}
	if o := DetectMisses(sh, "c", map[string]bool{"e2e.TestCartB": true}); !reflect.DeepEqual(o.Unverified, []string{"e2e.TestCartA"}) {
		t.Fatalf("an excluded test that did not run is unverified: %+v", o)
	}

	next := buildSeg(t, cart...)
	if err := CarryDivergent(next, nil, out.Misses); err != nil {
		t.Fatal(err)
	}
	next = roundTrip(t, next)
	if len(next.Divergent) != 1 {
		t.Fatalf("divergent %v", next.Divergent)
	}
	if _, sh := shadowOf(t, next, "open"); len(sh.Exclusions) != 0 || !sh.Classes[0].Divergent && !sh.Classes[1].Divergent {
		t.Fatalf("divergent class excluded %v", excluded(sh))
	}
	// Another class is unaffected.
	if _, sh := shadowOf(t, next, "checkA"); len(sh.Exclusions) != 1 {
		t.Fatalf("checkA exclusions %v", excluded(sh))
	}
	// Carried while the blob holds, dropped once it changes.
	again := buildSeg(t, cart...)
	if err := CarryDivergent(again, next.Divergent, nil); err != nil {
		t.Fatal(err)
	}
	if len(again.Divergent) != 1 {
		t.Fatal("divergence dropped with an unchanged reach")
	}
	moved, err := Build("example.com/m", "c1", "go test", map[string]string{segPath: "blob2"}, segRecords(cart, true))
	if err != nil {
		t.Fatal(err)
	}
	if err := CarryDivergent(moved, next.Divergent, nil); err != nil {
		t.Fatal(err)
	}
	if len(moved.Divergent) != 0 {
		t.Fatal("divergence kept after its reach changed")
	}
	// A miss naming no reach file could never be released, nor checked:
	// refused rather than dropped.
	noFiles := out.Misses[0]
	noFiles.Files = nil
	if err := CarryDivergent(buildSeg(t, cart...), nil, []Miss{noFiles}); err == nil {
		t.Fatal("a miss without reach files was accepted")
	}

	q := Qualification{Changes: 1200, Faults: map[SegmentKind]bool{KindStep: true, KindMethod: true, KindSetup: true, KindTest: true}}
	if ok, why := q.Promotable(); !ok {
		t.Fatalf("qualified evidence refused: %s", why)
	}
	for name, mod := range map[string]func(*Qualification){
		"miss":       func(q *Qualification) { q.Misses = 1 },
		"divergent":  func(q *Qualification) { q.Divergent = 1 },
		"unverified": func(q *Qualification) { q.Unverified = 1 },
		"few":        func(q *Qualification) { q.Changes = 999 },
		"fault":      func(q *Qualification) { q.Faults = map[SegmentKind]bool{KindStep: true} },
	} {
		qq := q
		mod(&qq)
		if ok, _ := qq.Promotable(); ok {
			t.Errorf("%s: promotable", name)
		}
	}
}

// Review findings on the shadow path, each a fail-open case:
// code a test runs outside its segments as well as inside them; a changed
// test helper or an order-sensitive package; and an incomplete record's
// units.
func TestShadowKeepsWhatSegmentsCannotSee(t *testing.T) {
	t.Run("a block run again after the last segment is a later check", func(t *testing.T) {
		r := testSpec{name: "TestR", elapsed: time.Second, segs: []segSpec{step(KindStep, "step:open", "open")}}
		tt := testSpec{name: "TestT", elapsed: 2 * time.Second, segs: []segSpec{step(KindStep, "step:open", "open")}}
		recs := segRecords([]testSpec{r, tt}, true)
		// TestT's whole run executed Open twice, its segment once.
		openLine := fmt.Sprintf(":%d.", segLine("open"))
		var b strings.Builder
		for _, l := range strings.SplitAfter(string(recs[1].Profile), "\n") {
			if strings.Contains(l, openLine) {
				l = strings.TrimSuffix(l, "1\n") + "2\n"
			}
			b.WriteString(l)
		}
		recs[1].Profile = []byte(b.String())
		ix, err := Build("example.com/m", "c0", "go test", map[string]string{segPath: "blob1"}, recs)
		if err != nil {
			t.Fatal(err)
		}
		ix = roundTrip(t, ix)
		if segs := ix.TestSegments(ix.byName["e2e\x00TestT"]); len(segs) != 2 || ix.Segment(segs[1]).Bound {
			t.Fatalf("TestT segments %v", segs)
		}
		if _, sh := shadowOf(t, ix, "open"); len(sh.Exclusions) != 0 {
			t.Fatalf("excluded %v", excluded(sh))
		}
	})
	t.Run("a set-mode segmented record is incomplete", func(t *testing.T) {
		recs := segRecords(cart[:2], true)
		recs[0].Profile = bytes.Replace(recs[0].Profile, []byte("mode: atomic"), []byte("mode: set"), 1)
		ix, err := Build("example.com/m", "c0", "go test", map[string]string{segPath: "blob1"}, recs)
		if err != nil {
			t.Fatal(err)
		}
		if !ix.Tests[ix.byName["e2e\x00TestCartA"]].Incomplete {
			t.Fatal("set-mode record complete")
		}
		if _, sh := shadowOf(t, ix, "open"); len(sh.Exclusions) != 0 {
			t.Fatalf("excluded %v", excluded(sh))
		}
	})
	t.Run("a changed test helper or an unordered package keeps every test", func(t *testing.T) {
		ix := buildSeg(t, cart...)
		e := ix.Analyze(segChange("open"), segRead)
		e.Whole["e2e"] = true // as a hunk in a helper of e2e/helpers_test.go sets it
		j := ix.Join(e)
		if !reflect.DeepEqual(j.Whole, []string{"e2e"}) {
			t.Fatalf("join whole %v", j.Whole)
		}
		if sh := ix.Shadow(j, nil); len(sh.Exclusions) != 0 {
			t.Fatalf("helper change: excluded %v", excluded(sh))
		}
		ix.Unordered = []string{"e2e"}
		if _, sh := shadowOf(t, ix, "open"); len(sh.Exclusions) != 0 {
			t.Fatalf("unordered: excluded %v", excluded(sh))
		}
	})
	t.Run("an incomplete record joins no unit", func(t *testing.T) {
		g := step(KindStep, "step:open", "open")
		g.units = []string{"screen:cart"}
		ix := buildSeg(t, testSpec{name: "TestA", incomplete: true, segs: []segSpec{g}})
		if j := ix.Join(ix.Analyze(segChange("open"), segRead)); len(j.Tests) != 1 || j.Units != nil || j.Tests[0].Units != nil {
			t.Fatalf("join %+v", j)
		}
	})
}
