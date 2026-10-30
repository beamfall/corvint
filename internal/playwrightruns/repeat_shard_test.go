package playwrightruns

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// runsFixture reads one report under testdata/runs. The real ones were produced by
// @playwright/test 1.61.1 (see https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-11-runs-repeat-each-and-shards.md).
func runsFixture(t *testing.T, name string) *Run {
	t.Helper()
	run, err := ReadFile(filepath.Join("testdata", "runs", name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return run
}

func runsFixtures(t *testing.T, names ...string) []*Run {
	t.Helper()
	out := []*Run{}
	for _, n := range names {
		out = append(out, runsFixture(t, n))
	}
	return out
}

func loadedFixtures(t *testing.T, names ...string) []Loaded {
	t.Helper()
	out := []Loaded{}
	for _, run := range runsFixtures(t, names...) {
		out = append(out, Loaded{Run: run})
	}
	return out
}

func compact(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

// singleRepeatCases are unsharded single-repeat inputs whose documents must stay byte-identical to
// the output before PWR-V0-011..030 (goldens produced at 645304a2fb71065f9061d8d8d866600d703e84c6),
// apart from the served-digest statements PWR-V0-055 and PWR-V0-056 add (servedDigest per run or
// report, summary.servedDigestUnknownRuns and the SERVED_DIGEST_UNKNOWN unknown; GitHub #1873).
func singleRepeatCases(t *testing.T) map[string][]byte {
	declared := identity("app-1", "cfg-1", blobsA)
	ambiguous := reportJSON(t, "1.63.0", declared, cartTest(false, fail("Error: expect(page).toHaveURL(expected) failed", fixtureRoot+"/cart.spec.ts", 4)), cartTest(false, pass()))
	failed := reportJSON(t, "1.63.0", declared, cartTest(true, fail("Error: boom", fixtureRoot+"/pages/cart.ts", 7)))
	passed := reportJSON(t, "1.63.0", declared, cartTest(true, pass()))
	synthetic := []*Run{mustParse(t, ambiguous), mustParse(t, failed), mustParse(t, passed)}
	return map[string][]byte{
		"golden-stability-real.json":      compact(t, Stability(runsFixtures(t, "whole.json", "merged.json", "whole.json"), 3)),
		"golden-stability-synthetic.json": compact(t, Stability(synthetic, 2)),
		"golden-compare-real.json":        compact(t, Compare(loadedFixtures(t, "whole.json", "whole.json"), loadedFixtures(t, "merged.json"))),
		"golden-compare-synthetic.json": compact(t, Compare([]Loaded{{Run: mustParse(t, failed)}, {Run: mustParse(t, passed)}},
			[]Loaded{{Run: mustParse(t, ambiguous)}, {Run: mustParse(t, passed)}})),
	}
}

func TestSingleRepeatOutputByteIdentical(t *testing.T) {
	for name, got := range singleRepeatCases(t) {
		want, err := os.ReadFile(filepath.Join("testdata", "runs", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s changed:\n got %s\nwant %s", name, got, want)
		}
	}
}

// mutatedFixture parses a testdata/runs report after edit changes its decoded JSON.
func mutatedFixture(t *testing.T, name string, edit func(map[string]any)) *Run {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "runs", name))
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	run, err := Parse(raw)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return run
}

func byTitle(t *testing.T, tests []StabilityEntry) map[string]StabilityEntry {
	t.Helper()
	out := map[string]StabilityEntry{}
	for _, e := range tests {
		key := strings.Join(e.TitlePath, " > ")
		if _, dup := out[key]; dup {
			t.Fatalf("two entries for %q", key)
		}
		out[key] = e
	}
	return out
}

func compareRows(t *testing.T, tests []CompareEntry) map[string]CompareEntry {
	t.Helper()
	out := map[string]CompareEntry{}
	for _, e := range tests {
		out[strings.Join(e.TitlePath, " > ")] = e
	}
	return out
}

func eq(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want %#v", what, got, want)
	}
}

// One report run with --repeat-each 3 is three observations (PWR-V0-011, PWR-V0-013).
func TestRepeatEachIsObservations(t *testing.T) {
	run := runsFixture(t, "repeat3.json")
	eq(t, "repeatEach", run.RepeatEach, map[string]int{"node": 3})
	report := Stability([]*Run{run}, 3)
	eq(t, "runs[0].repeatEach", report.Runs[0].RepeatEach, map[string]int{"node": 3})
	tests := byTitle(t, report.Tests)
	if len(tests) != 4 {
		t.Fatalf("tests = %d, want 4", len(tests))
	}
	adds := tests["cart > adds item"]
	eq(t, "adds verdict", adds.Verdict, Intermittent)
	eq(t, "adds outcomes", adds.Outcomes, []string{"passed", "failed", "passed"})
	eq(t, "adds runs", adds.Runs, []int{1, 1, 1})
	eq(t, "adds repeats", adds.Repeats, []int{0, 1, 2})
	for _, name := range []string{"cart > removes item", "checkout", "search"} {
		e := tests[name]
		if e.Verdict != Deterministic || e.Reason != "" || len(e.Outcomes) != 3 {
			t.Errorf("%s: %s %s %v, want DETERMINISTIC over three repeats", name, e.Verdict, e.Reason, e.Outcomes)
		}
	}
	raw := compact(t, report)
	if !bytes.Contains(raw, []byte(`"repeatEach":{"node":3}`)) || !bytes.Contains(raw, []byte(`"repeats":[0,1,2]`)) {
		t.Errorf("document lacks repeatEach or repeats: %s", raw)
	}
}

// A duplicated identity whose count is not the declared repeatEach stays ambiguous (PWR-V0-012).
func TestRepeatCountMismatchAmbiguous(t *testing.T) {
	run := runsFixture(t, "repeat3-count-mismatch.json")
	tests := byTitle(t, Stability([]*Run{run}, 2).Tests)
	adds := tests["cart > adds item"]
	eq(t, "adds", []string{adds.Verdict, adds.Reason}, []string{Insufficient, ReasonAmbiguous})
	eq(t, "removes", tests["cart > removes item"].Verdict, Deterministic)
	cmp := Compare(loadedFixtures(t, "repeat3-count-mismatch.json"), loadedFixtures(t, "repeat3.json"))
	eq(t, "compare verdict", cmp.Verdict, Incomparable)
	eq(t, "adds row", compareRows(t, cmp.Tests)["cart > adds item"].Verdict, AmbiguousTest)
	// A repeated title at two spec locations is ambiguous even at the declared count.
	moved := mutatedFixture(t, "repeat3.json", func(doc map[string]any) {
		specs := doc["suites"].([]any)[0].(map[string]any)["suites"].([]any)[0].(map[string]any)["specs"].([]any)
		for _, s := range specs {
			if spec := s.(map[string]any); spec["title"] == "adds item" {
				spec["line"] = 99.0
				break
			}
		}
	})
	eq(t, "moved", byTitle(t, Stability([]*Run{moved}, 2).Tests)["cart > adds item"].Reason, ReasonAmbiguous)
	// Repeats without a reported spec location cannot be shown to be one spec.
	unlocated := mutatedFixture(t, "repeat3.json", func(doc map[string]any) {
		for _, s := range doc["suites"].([]any)[0].(map[string]any)["suites"].([]any)[0].(map[string]any)["specs"].([]any) {
			delete(s.(map[string]any), "line")
			delete(s.(map[string]any), "column")
		}
	})
	eq(t, "unlocated", byTitle(t, Stability([]*Run{unlocated}, 2).Tests)["cart > removes item"].Reason, ReasonAmbiguous)
}

// N repeats in one report give the same k/N as N single-repeat reports (PWR-V0-014).
func TestCompareRepeatsMatchSingleReports(t *testing.T) {
	singles := []string{"whole.json", "whole-adds-failed.json", "whole.json"}
	repeated := Compare(loadedFixtures(t, "repeat3.json"), loadedFixtures(t, "repeat3.json"))
	single := Compare(loadedFixtures(t, singles...), loadedFixtures(t, singles...))
	mixed := Compare(loadedFixtures(t, singles...), loadedFixtures(t, "repeat3.json"))
	for name, r := range map[string]CompareReport{"repeated": repeated, "single": single, "mixed": mixed} {
		eq(t, name+" verdict", r.Verdict, Parity)
		adds := compareRows(t, r.Tests)["cart > adds item"]
		eq(t, name+" baseline k/N", adds.Baseline.Stability, "2/3")
		eq(t, name+" candidate k/N", adds.Candidate.Stability, "2/3")
	}
	if a, b := compact(t, repeated.Tests), compact(t, single.Tests); !bytes.Equal(a, b) {
		t.Errorf("repeat rows differ from single-report rows:\n%s\n%s", a, b)
	}
}

// Three shards and merge-reports output each equal one unsharded run (PWR-V0-016, PWR-V0-019,
// PWR-V0-020, PWR-V0-022).
func TestCompleteShardSetMatchesWholeRun(t *testing.T) {
	shards := []string{"shard-3-of-3.json", "shard-1-of-3.json", "shard-2-of-3.json"}
	// Two complete sets at the same inputs are one group with every shard duplicated: a limit, refused.
	twice := Stability(runsFixtures(t, append(append([]string{}, shards...), shards...)...), 2)
	eq(t, "two sets", []any{len(twice.ShardSets), twice.ShardSets[0].Reason, twice.ShardSets[0].Duplicate}, []any{1, ReasonShardDuplicate, []int{1, 2, 3}})
	one := Stability(runsFixtures(t, shards...), 1)
	eq(t, "set", one.ShardSets, []ShardSet{{Run: 1, Total: 3, Reports: []int{1, 2, 3}, Shards: []int{3, 1, 2}, Status: ShardSetAdmitted,
		Missing: []int{}, Duplicate: []int{}}})
	wholeOne := Stability(runsFixtures(t, "whole.json"), 1)
	if a, b := compact(t, one.Tests), compact(t, wholeOne.Tests); !bytes.Equal(a, b) {
		t.Errorf("sharded stability rows differ:\n%s\n%s", a, b)
	}
	merged := Stability(runsFixtures(t, "merged.json"), 1)
	if a, b := compact(t, merged.Tests), compact(t, wholeOne.Tests); !bytes.Equal(a, b) {
		t.Errorf("merge-reports stability rows differ:\n%s\n%s", a, b)
	}
	byShards := Compare(loadedFixtures(t, "whole.json"), loadedFixtures(t, shards...))
	byWhole := Compare(loadedFixtures(t, "whole.json"), loadedFixtures(t, "whole.json"))
	byMerged := Compare(loadedFixtures(t, "whole.json"), loadedFixtures(t, "merged.json"))
	for name, r := range map[string]CompareReport{"shards": byShards, "merged": byMerged} {
		eq(t, name+" verdict", r.Verdict, byWhole.Verdict)
		if a, b := compact(t, r.Tests), compact(t, byWhole.Tests); !bytes.Equal(a, b) {
			t.Errorf("%s compare rows differ:\n%s\n%s", name, a, b)
		}
	}
	eq(t, "verdict", byWhole.Verdict, Parity)
	eq(t, "compare sets", len(byShards.ShardSets[SideCandidate]), 1)
}

// Incomplete, duplicated, mismatched and undeclared shard sets are refused, never PARITY or
// DETERMINISTIC (PWR-V0-016, PWR-V0-017, PWR-V0-019, PWR-V0-020).
func TestShardSetRefusals(t *testing.T) {
	undeclared := func(name string) *Run {
		return mutatedFixture(t, name, func(doc map[string]any) {
			delete(doc["config"].(map[string]any)["metadata"].(map[string]any), "corvintRunIdentity")
		})
	}
	otherBlob := mutatedFixture(t, "shard-3-of-3.json", func(doc map[string]any) {
		identity := doc["config"].(map[string]any)["metadata"].(map[string]any)["corvintRunIdentity"].(map[string]any)
		identity["testFileBlobs"].(map[string]any)["a.spec.js"] = "blob-a2"
	})
	cases := map[string]struct {
		runs    []*Run
		reasons []string
		sets    []ShardSet
	}{
		"missing shard": {runsFixtures(t, "shard-1-of-3.json", "shard-2-of-3.json"), []string{ReasonShardSetIncomplete},
			[]ShardSet{{Run: 1, Total: 3, Reports: []int{1, 2}, Shards: []int{1, 2}, Status: ShardSetRefused, Reason: ReasonShardSetIncomplete, Missing: []int{3}, Duplicate: []int{}}}},
		"duplicate shard": {runsFixtures(t, "shard-1-of-3.json", "shard-2-of-3.json", "shard-2-of-3-again.json", "shard-3-of-3.json"), []string{ReasonShardDuplicate},
			[]ShardSet{{Run: 1, Total: 3, Reports: []int{1, 2, 3, 4}, Shards: []int{1, 2, 2, 3}, Status: ShardSetRefused, Reason: ReasonShardDuplicate, Missing: []int{}, Duplicate: []int{2}}}},
		"mismatched identity": {runsFixtures(t, "shard-1-of-3.json", "shard-2-of-3.json", "shard-3-of-3-app-2.json"), []string{ReasonShardSetIncomplete},
			[]ShardSet{
				{Run: 1, Total: 3, Reports: []int{1, 2}, Shards: []int{1, 2}, Status: ShardSetRefused, Reason: ReasonShardSetIncomplete, Missing: []int{3}, Duplicate: []int{}},
				{Run: 3, Total: 3, Reports: []int{3}, Shards: []int{3}, Status: ShardSetRefused, Reason: ReasonShardSetIncomplete, Missing: []int{1, 2}, Duplicate: []int{}}}},
		"mismatched test-file blob": {[]*Run{runsFixture(t, "shard-1-of-3.json"), runsFixture(t, "shard-2-of-3.json"), otherBlob}, []string{ReasonShardSetIncomplete},
			[]ShardSet{
				{Run: 1, Total: 3, Reports: []int{1, 2}, Shards: []int{1, 2}, Status: ShardSetRefused, Reason: ReasonShardSetIncomplete, Missing: []int{3}, Duplicate: []int{}},
				{Run: 3, Total: 3, Reports: []int{3}, Shards: []int{3}, Status: ShardSetRefused, Reason: ReasonShardSetIncomplete, Missing: []int{1, 2}, Duplicate: []int{}}}},
		"undeclared inputs": {[]*Run{undeclared("shard-1-of-3.json"), undeclared("shard-2-of-3.json"), undeclared("shard-3-of-3.json")}, []string{ReasonShardInputsUndeclared},
			[]ShardSet{{Run: 1, Total: 3, Reports: []int{1, 2, 3}, Shards: []int{1, 2, 3}, Status: ShardSetRefused, Reason: ReasonShardInputsUndeclared, Missing: []int{}, Duplicate: []int{}}}},
	}
	for name, c := range cases {
		report := Stability(c.runs, 1)
		eq(t, name+" stability sets", report.ShardSets, c.sets)
		if report.Summary.Deterministic+report.Summary.Intermittent != 0 || len(report.Tests) == 0 {
			t.Errorf("%s: summary %+v over %d tests, want all INSUFFICIENT", name, report.Summary, len(report.Tests))
		}
		for _, e := range report.Tests {
			if e.Verdict != Insufficient || e.Reason != c.reasons[0] {
				t.Errorf("%s: %v %s %s", name, e.TitlePath, e.Verdict, e.Reason)
			}
		}
		loaded := []Loaded{}
		for _, run := range c.runs {
			loaded = append(loaded, Loaded{Run: run})
		}
		cmp := Compare(loadedFixtures(t, "whole.json"), loaded)
		eq(t, name+" compare verdict", cmp.Verdict, Incomparable)
		eq(t, name+" compare reasons", cmp.Reasons, c.reasons)
		eq(t, name+" compare sets", cmp.ShardSets[SideCandidate], c.sets)
		eq(t, name+" compare rows", len(cmp.Tests), 0)
		if len(cmp.Problems) != len(c.sets) || cmp.Problems[0].Side != SideCandidate || cmp.Problems[0].Reason != c.reasons[0] {
			t.Errorf("%s: problems %+v", name, cmp.Problems)
		}
	}
}

// The same test in two shards of one set is ambiguous (PWR-V0-018).
func TestShardOverlapAmbiguous(t *testing.T) {
	runs := runsFixtures(t, "shard-1-of-3.json", "shard-2-of-3.json", "shard-3-of-3-overlap.json")
	report := Stability(runs, 1)
	eq(t, "status", report.ShardSets[0].Status, ShardSetAdmitted)
	tests := byTitle(t, report.Tests)
	for _, name := range []string{"cart > adds item", "cart > removes item"} {
		eq(t, name, []string{tests[name].Verdict, tests[name].Reason}, []string{Insufficient, ReasonAmbiguous})
	}
	eq(t, "checkout", tests["checkout"].Verdict, Deterministic)
	loaded := []Loaded{}
	for _, run := range runs {
		loaded = append(loaded, Loaded{Run: run})
	}
	cmp := Compare(loadedFixtures(t, "whole.json"), loaded)
	eq(t, "compare verdict", cmp.Verdict, Incomparable)
	eq(t, "adds row", compareRows(t, cmp.Tests)["cart > adds item"].Verdict, AmbiguousTest)
}

// A sharded --repeat-each run resolves repeats over the merged set (PWR-V0-021).
func TestShardedRepeatEachComposes(t *testing.T) {
	runs := runsFixtures(t, "repeat2-shard-2-of-2.json", "repeat2-shard-1-of-2.json")
	report := Stability(runs, 2)
	eq(t, "status", report.ShardSets[0].Status, ShardSetAdmitted)
	adds := byTitle(t, report.Tests)["cart > adds item"]
	eq(t, "adds", []any{adds.Verdict, adds.Outcomes, adds.Runs, adds.Repeats}, []any{Intermittent, []string{"passed", "failed"}, []int{1, 1}, []int{0, 1}})
	for _, e := range report.Tests {
		if e.Verdict == Insufficient {
			t.Errorf("%v: %s %s", e.TitlePath, e.Verdict, e.Reason)
		}
	}
	cmp := Compare(loadedFixtures(t, "whole.json", "whole-adds-failed.json"), []Loaded{{Run: runs[0]}, {Run: runs[1]}})
	eq(t, "compare verdict", cmp.Verdict, Parity)
	row := compareRows(t, cmp.Tests)["cart > adds item"]
	eq(t, "k/N", []string{row.Baseline.Stability, row.Candidate.Stability}, []string{"1/2", "1/2"})
	// Only one shard of the repeated run is refused like any incomplete set.
	half := Stability(runs[:1], 1)
	eq(t, "half", half.ShardSets[0].Missing, []int{1})
}

// config.shard must name a shard within its total, and declared counts are bounded (PWR-V0-011,
// PWR-V0-016).
func TestShardDeclarationValidated(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "runs", "shard-1-of-3.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, shard := range []string{`{"current":0,"total":3}`, `{"current":4,"total":3}`, `{"current":1,"total":0}`} {
		bad := bytes.Replace(raw, []byte(`"shard": {`), []byte(`"shard": `+shard+`, "x": {`), 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("fixture shard key not found")
		}
		if _, err := Parse(bad); !errors.Is(err, ErrShape) {
			t.Errorf("%s: err = %v, want ErrShape", shard, err)
		}
	}
	// Declared counts are bounded before anything is laid out per shard or repeat.
	huge := bytes.Replace(raw, []byte(`"shard": {`), []byte(`"shard": {"current":1,"total":1025}, "x": {`), 1)
	if _, err := Parse(huge); !errors.Is(err, ErrTooLarge) {
		t.Errorf("shard total 1025: err = %v, want ErrTooLarge", err)
	}
	repeat, err := os.ReadFile(filepath.Join("testdata", "runs", "repeat3.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(repeat, []byte(`"repeatEach": 3`)) {
		t.Fatal("fixture repeatEach key not found")
	}
	if _, err := Parse(bytes.Replace(repeat, []byte(`"repeatEach": 3`), []byte(`"repeatEach": 1025`), 1)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("repeatEach 1025: err = %v, want ErrTooLarge", err)
	}
	if _, err := Parse(bytes.Replace(repeat, []byte(`"repeatEach": 3`), []byte(`"repeatEach": 1024`), 1)); err != nil {
		t.Errorf("repeatEach 1024: %v", err)
	}
}
