//go:build darwin || linux

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/.github/cishards"
)

// AFP-V0-048: the sharded driver places exactly what the protected full branch
// places, whole packages and AFP-V0-041 test slices, through cishards.Plan.

const sliceRevision = "0123456789abcdef0123456789abcdef01234567"
const sliceRunURL = "https://github.com/beamfall/corvint/actions/runs/1"

func costsJSON(ms map[string]int64) []byte {
	raw, _ := json.Marshal(map[string]any{
		"profile":      "corvint-ci-package-costs/0",
		"source":       map[string]string{"revision": sliceRevision, "runURL": sliceRunURL, "goVersion": "go1.27.2"},
		"milliseconds": ms,
	})
	return raw
}

func allowJSON(packages ...string) []byte {
	m := map[string]string{}
	for _, p := range packages {
		m[p] = "fixture: tests are order-independent"
	}
	raw, _ := json.Marshal(map[string]any{"profile": "corvint-ci-test-split-allow/0", "packages": m})
	return raw
}

type splitEntry struct {
	named [][]string
	rest  int64
}

func slicesJSON(split map[string]splitEntry) []byte {
	packages := map[string]any{}
	for p, e := range split {
		named := []any{}
		for i, tests := range e.named {
			named = append(named, map[string]any{"milliseconds": int64(1000 * (i + 2)), "tests": tests})
		}
		packages[p] = map[string]any{"named": named, "restMilliseconds": e.rest}
	}
	raw, _ := json.Marshal(map[string]any{
		"profile":  "corvint-ci-test-slices/0",
		"source":   map[string]string{"revision": sliceRevision, "runURL": sliceRunURL, "goVersion": "go1.27.2"},
		"packages": packages,
	})
	return raw
}

func usePlan(t *testing.T, plan func([]string, int) ([]cishards.Shard, error)) {
	t.Helper()
	old := planShards
	planShards = plan
	t.Cleanup(func() { planShards = old })
}

func usePlanFrom(t *testing.T, costs, allow, slices []byte) {
	t.Helper()
	usePlan(t, func(u []string, n int) ([]cishards.Shard, error) {
		return cishards.PlanFrom(u, n, costs, allow, slices)
	})
}

// placements counts, over every shard, each whole package and each slice
// (package#index) a selection runs.
func placements(t *testing.T, layout []cishards.Shard, universe, selected []string) (map[string]int, map[string]int) {
	t.Helper()
	whole, sliced := map[string]int{}, map[string]int{}
	for i := range layout {
		packages, slices, err := shardUnits(layout, universe, selected, i)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range packages {
			whole[p]++
		}
		for _, s := range slices {
			sliced[fmt.Sprintf("%s#%d/%d", s.Package, s.Index, s.Count)]++
		}
	}
	return whole, sliced
}

func TestAFPV0048DriverPlacesTheFullBranchUnits(t *testing.T) {
	const m = "example.org/m/"
	universe := []string{m + "a", m + "b", m + "c", m + "d", m + "e", m + "s", m + "t"}
	costs := costsJSON(map[string]int64{m + "a": 9000, m + "b": 4000, m + "c": 3000, m + "d": 2000, m + "e": 1000, m + "s": 20000, m + "t": 12000})
	allow := allowJSON(m+"s", m+"t")
	split := map[string]splitEntry{
		m + "s": {named: [][]string{{"TestA", "TestB"}, {"TestC"}}, rest: 5000},
		m + "t": {named: [][]string{{"TestX"}}, rest: 4000},
	}
	slices := slicesJSON(split)
	usePlanFrom(t, costs, allow, slices)
	for total := 2; total <= 8; total++ {
		layout, err := shardLayout(universe, total)
		if err != nil {
			t.Fatalf("shards=%d: %v", total, err)
		}
		want, err := cishards.PlanFrom(universe, total, costs, allow, slices)
		if err != nil {
			t.Fatal(err)
		}
		// FULL is, shard by shard, exactly the full branch's ci-shards output.
		for i := range layout {
			packages, got, err := shardUnits(layout, universe, []string{"./..."}, i)
			if err != nil || !equal(packages, want[i].Packages) || !equal(got, want[i].Slices) {
				t.Fatalf("shards=%d shard=%d differs from the full branch: %v %v %v", total, i, packages, got, err)
			}
		}
		// Every package runs whole once, or every one of its slices once and never whole.
		whole, sliced := placements(t, layout, universe, []string{"./..."})
		sCount := 3
		if total < sCount {
			sCount = 0 // more slices than shards keeps the package whole (AFP-V0-041 (1))
		}
		for _, p := range universe {
			count := map[string]int{m + "s": sCount, m + "t": 2}[p]
			if count == 0 {
				if whole[p] != 1 {
					t.Fatalf("shards=%d: %s whole %d times", total, p, whole[p])
				}
				continue
			}
			if whole[p] != 0 {
				t.Fatalf("shards=%d: split %s also runs whole", total, p)
			}
			for i := 0; i < count; i++ {
				if n := sliced[fmt.Sprintf("%s#%d/%d", p, i, count)]; n != 1 {
					t.Fatalf("shards=%d: %s slice %d runs %d times", total, p, i, n)
				}
			}
		}
		wantWholeCount := len(universe) - 2
		if sCount == 0 {
			wantWholeCount++
		}
		if len(whole) != wantWholeCount || len(sliced) != sCount+2 {
			t.Fatalf("shards=%d: unexpected units %v %v", total, whole, sliced)
		}
		// A narrowed selection runs a selected split package's every slice, on its own
		// shard, and nothing it did not select.
		whole, sliced = placements(t, layout, universe, []string{m + "s", m + "b"})
		wantSliced := map[string]int{}
		wantWhole := map[string]int{m + "b": 1}
		if sCount == 0 {
			wantWhole[m+"s"] = 1
		}
		for i := 0; i < sCount; i++ {
			wantSliced[fmt.Sprintf("%ss#%d/%d", m, i, sCount)] = 1
		}
		if !equal(whole, wantWhole) || !equal(sliced, wantSliced) {
			t.Fatalf("shards=%d: narrowed placement %v %v", total, whole, sliced)
		}
	}
	layout, err := shardLayout(universe, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, selected := range [][]string{{m + "gone"}, {m + "s", m + "s"}, {}} {
		if _, _, err := shardUnits(layout, universe, selected, 0); err == nil {
			t.Fatalf("selection %v admitted", selected)
		}
	}
	if _, _, err := shardUnits(layout, universe, []string{"./..."}, 4); err == nil {
		t.Fatal("shard outside the layout admitted")
	}
}

// Without a split package the driver keeps its earlier AFP-V0-022 placement.
func TestAFPV0048ZeroSlicesKeepTheIntersect(t *testing.T) {
	universe := []string{"example.org/fixture/a", "example.org/fixture/b", "example.org/fixture/c/d", "example.org/fixture/e"}
	for total := 2; total <= 8; total++ {
		layout, err := shardLayout(universe, total)
		if err != nil {
			t.Fatal(err)
		}
		for _, selected := range [][]string{{"./..."}, {"example.org/fixture/a", "example.org/fixture/e"}} {
			for i := 0; i < total; i++ {
				packages, slices, err := shardUnits(layout, universe, selected, i)
				want, wantErr := cishards.Intersect(universe, selected, i, total)
				if err != nil || wantErr != nil || len(slices) != 0 || !equal(packages, want) {
					t.Fatalf("shards=%d shard=%d %v: %v %v differs from Intersect %v", total, i, selected, packages, slices, want)
				}
			}
		}
	}
	const m = "example.org/m/"
	universe = []string{m + "a", m + "s"}
	usePlanFrom(t, costsJSON(map[string]int64{m + "a": 10, m + "s": 20}), allowJSON(m+"s"), slicesJSON(nil))
	layout, err := shardLayout(universe, 3)
	if err != nil {
		t.Fatal(err)
	}
	whole, sliced := placements(t, layout, universe, []string{"./..."})
	if len(sliced) != 0 || whole[m+"a"] != 1 || whole[m+"s"] != 1 {
		t.Fatalf("empty slice file split: %v %v", whole, sliced)
	}
}

// A slice file the helper does not admit for a package keeps it whole, in the
// driver as in the full branch; a valid file whose names are stale is checked by
// execution in TestAFPV0048ShardedRunExecutesSlices.
func TestAFPV0048MismatchedSliceFileMatchesTheFullBranch(t *testing.T) {
	const m = "example.org/m/"
	universe := []string{m + "a", m + "b", m + "s", m + "t"}
	costs := costsJSON(map[string]int64{m + "a": 9000, m + "b": 4000, m + "s": 20000, m + "t": 12000})
	good := map[string]splitEntry{m + "s": {named: [][]string{{"TestA"}, {"TestB"}}, rest: 100}, m + "t": {named: [][]string{{"TestX"}}, rest: 100}}
	for name, c := range map[string]struct {
		allow, slices []byte
		whole         []string
	}{
		"outside the universe": {allowJSON(m+"s", m+"t", m+"gone"), slicesJSON(map[string]splitEntry{m + "gone": {named: [][]string{{"TestA"}}, rest: 1}, m + "s": good[m+"s"]}), []string{m + "t"}},
		"not allowed":          {allowJSON(m + "t"), slicesJSON(good), []string{m + "s"}},
		"unsorted names":       {allowJSON(m+"s", m+"t"), slicesJSON(map[string]splitEntry{m + "s": {named: [][]string{{"TestB", "TestA"}}, rest: 1}, m + "t": good[m+"t"]}), []string{m + "s"}},
		"overlapping slices":   {allowJSON(m+"s", m+"t"), slicesJSON(map[string]splitEntry{m + "s": {named: [][]string{{"TestA"}, {"TestA"}}, rest: 1}, m + "t": good[m+"t"]}), []string{m + "s"}},
		"invalid name":         {allowJSON(m+"s", m+"t"), slicesJSON(map[string]splitEntry{m + "s": {named: [][]string{{"TestA|TestB"}}, rest: 1}, m + "t": good[m+"t"]}), []string{m + "s"}},
		"wrong profile":        {allowJSON(m+"s", m+"t"), []byte(strings.Replace(string(slicesJSON(good)), "corvint-ci-test-slices/0", "corvint-ci-test-slices/9", 1)), []string{m + "s", m + "t"}},
		"unknown member":       {allowJSON(m+"s", m+"t"), []byte(strings.Replace(string(slicesJSON(good)), `"profile"`, `"extra":1,"profile"`, 1)), []string{m + "s", m + "t"}},
	} {
		t.Run(name, func(t *testing.T) {
			usePlanFrom(t, costs, c.allow, c.slices)
			layout, err := shardLayout(universe, 4)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := cishards.PlanFrom(universe, 4, costs, c.allow, c.slices)
			if !equal(layout, want) {
				t.Fatal("driver layout differs from the full branch")
			}
			whole, sliced := placements(t, layout, universe, []string{"./..."})
			for _, p := range c.whole {
				if whole[p] != 1 {
					t.Fatalf("%s not whole exactly once: %v %v", p, whole, sliced)
				}
				for k := range sliced {
					if strings.HasPrefix(k, p+"#") {
						t.Fatalf("%s also sliced: %v", p, sliced)
					}
				}
			}
		})
	}
}

// The committed slice file places each slice of a split package exactly once.
func TestAFPV0048CommittedSlicesArePlacedOnce(t *testing.T) {
	store := "github.com/Beamfall/corvint/internal/tasks/store"
	universe := []string{"github.com/Beamfall/corvint/cmd/corvint", store, "github.com/Beamfall/corvint/internal/tasks/authority", "github.com/Beamfall/corvint/tools/corvint-pr-tests"}
	layout, err := shardLayout(universe, 8)
	if err != nil {
		t.Fatal(err)
	}
	want, err := cishards.Plan(universe, 8)
	if err != nil || !equal(layout, want) {
		t.Fatalf("driver layout differs from the full branch: %v", err)
	}
	if !cishards.Splits(universe, 8) {
		t.Fatal("the committed slice file no longer splits the store; this test needs a split package")
	}
	whole, sliced := placements(t, layout, universe, []string{store})
	if len(sliced) < 2 || whole[store] != 0 {
		t.Fatalf("committed split placement: %v %v", whole, sliced)
	}
	for k, n := range sliced {
		if n != 1 {
			t.Fatalf("%s placed %d times", k, n)
		}
	}
}

func cloneLayout(layout []cishards.Shard) []cishards.Shard {
	out := make([]cishards.Shard, len(layout))
	for i, s := range layout {
		out[i] = cishards.Shard{Packages: append([]string{}, s.Packages...), Slices: append([]cishards.Slice{}, s.Slices...)}
	}
	return out
}

func TestAFPV0048DriverRefusesAmbiguousPlacement(t *testing.T) {
	const m = "example.org/m/"
	universe := []string{m + "a", m + "b", m + "s"}
	costs := costsJSON(map[string]int64{m + "a": 9000, m + "b": 4000, m + "s": 20000})
	allow := allowJSON(m + "s")
	slices := slicesJSON(map[string]splitEntry{m + "s": {named: [][]string{{"TestA", "TestB"}, {"TestC"}}, rest: 100}})
	good, err := cishards.PlanFrom(universe, 4, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	find := func(l []cishards.Shard, index int) (int, int) {
		for i, s := range l {
			for j, slice := range s.Slices {
				if slice.Index == index {
					return i, j
				}
			}
		}
		t.Fatal("fixture slice missing")
		return 0, 0
	}
	wholeShard := func(l []cishards.Shard, p string) int {
		for i, s := range l {
			for _, q := range s.Packages {
				if q == p {
					return i
				}
			}
		}
		t.Fatal("fixture package missing")
		return 0
	}
	for name, mutate := range map[string]func(l []cishards.Shard) []cishards.Shard{
		"slice dropped": func(l []cishards.Shard) []cishards.Shard {
			i, j := find(l, 1)
			l[i].Slices = append(l[i].Slices[:j], l[i].Slices[j+1:]...)
			return l
		},
		"slice twice": func(l []cishards.Shard) []cishards.Shard {
			i, j := find(l, 0)
			l[(i+1)%len(l)].Slices = append(l[(i+1)%len(l)].Slices, l[i].Slices[j])
			return l
		},
		"split package also whole": func(l []cishards.Shard) []cishards.Shard {
			l[0].Packages = append(l[0].Packages, m+"s")
			return l
		},
		"whole package dropped": func(l []cishards.Shard) []cishards.Shard {
			i := wholeShard(l, m+"a")
			l[i].Packages = nil
			return l
		},
		"whole package twice": func(l []cishards.Shard) []cishards.Shard {
			i := wholeShard(l, m+"b")
			l[(i+1)%len(l)].Packages = append(l[(i+1)%len(l)].Packages, m+"b")
			return l
		},
		"package outside the universe": func(l []cishards.Shard) []cishards.Shard {
			l[0].Packages = append(l[0].Packages, m+"gone")
			return l
		},
		"slice outside the universe": func(l []cishards.Shard) []cishards.Shard {
			l[0].Slices = append(l[0].Slices, cishards.Slice{Package: m + "gone", Index: 0, Count: 2, Flag: "-run", Pattern: "^(TestA)$"}, cishards.Slice{Package: m + "gone", Index: 1, Count: 2, Flag: "-skip", Pattern: "^(TestA)$"})
			return l
		},
		"catch-all runs": func(l []cishards.Shard) []cishards.Shard {
			i, j := find(l, 2)
			l[i].Slices[j].Flag = "-run"
			return l
		},
		"option pattern": func(l []cishards.Shard) []cishards.Shard {
			i, j := find(l, 0)
			l[i].Slices[j].Pattern = "-test.v"
			return l
		},
		"unanchored pattern": func(l []cishards.Shard) []cishards.Shard {
			i, j := find(l, 0)
			l[i].Slices[j].Pattern = "^(TestA|TestB)$|."
			return l
		},
		"named slices overlap": func(l []cishards.Shard) []cishards.Shard {
			i, j := find(l, 1)
			l[i].Slices[j].Pattern = "^(TestA)$"
			return l
		},
		"catch-all runs a named test": func(l []cishards.Shard) []cishards.Shard {
			i, j := find(l, 2)
			l[i].Slices[j].Pattern = "^(TestA|TestB)$"
			return l
		},
		"catch-all skips an unnamed test": func(l []cishards.Shard) []cishards.Shard {
			i, j := find(l, 2)
			l[i].Slices[j].Pattern = "^(TestA|TestB|TestC|TestD)$"
			return l
		},
		"inconsistent count": func(l []cishards.Shard) []cishards.Shard {
			i, j := find(l, 0)
			l[i].Slices[j].Count = 4
			return l
		},
		"missing shard": func(l []cishards.Shard) []cishards.Shard { return l[:len(l)-1] },
	} {
		t.Run(name, func(t *testing.T) {
			usePlan(t, func([]string, int) ([]cishards.Shard, error) { return mutate(cloneLayout(good)), nil })
			if _, err := shardLayout(universe, 4); err == nil || !strings.Contains(err.Error(), "AFP-V0-048") {
				t.Fatalf("ambiguous placement admitted: %v", err)
			}
		})
	}
	usePlan(t, func([]string, int) ([]cishards.Shard, error) { return nil, errors.New("plan failed") })
	if _, err := shardLayout(universe, 4); err == nil {
		t.Fatal("failed plan admitted")
	}
	usePlan(t, func([]string, int) ([]cishards.Shard, error) { return cloneLayout(good), nil })
	if _, err := shardLayout(universe, 4); err != nil {
		t.Fatalf("valid placement refused: %v", err)
	}
}

type testEvent struct{ Action, Package, Test string }

func readEvents(t *testing.T, path string) []testEvent {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var events []testEvent
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 65536), 8<<20)
	for scan.Scan() {
		var e testEvent
		if err := json.Unmarshal(scan.Bytes(), &e); err != nil {
			t.Fatalf("malformed test event %q: %v", scan.Text(), err)
		}
		events = append(events, e)
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

// Real execution over the fixture repository: every top-level test of a split
// package runs exactly once over the shards, narrowed or FULL, and a stale name
// still leaves every present test in the catch-all.
func TestAFPV0048ShardedRunExecutesSlices(t *testing.T) {
	o, _ := fixture(t)
	ctx := context.Background()
	const a, b = "example.org/fixture/a", "example.org/fixture/b"
	costs := costsJSON(map[string]int64{a: 1000, b: 1000})
	o.shards, o.partitionDigest = 2, cishards.ProfileDigest()
	id, err := toolIdentity(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	narrowed := plan(ctx, o, id, true)
	if narrowed.Reason != "" || !equal(narrowed.Packages, []string{a}) {
		t.Fatalf("fixture selection: %+v", narrowed)
	}
	full := narrowed
	full.Reason, full.Packages = "fixture fallback", []string{"./..."}
	run := func(t *testing.T, s selection, names []string) (map[string]int, int) {
		t.Helper()
		usePlanFrom(t, costs, allowJSON(a), slicesJSON(map[string]splitEntry{a: {named: [][]string{names}, rest: 500}}))
		tests, failures := map[string]int{}, 0
		indices := map[int]int{}
		for shard := 0; shard < o.shards; shard++ {
			o.shard = shard
			code, err := execute(ctx, o, s)
			if err != nil {
				t.Fatalf("shard %d: %v", shard, err)
			}
			var e execution
			if err = readJSON(filepath.Join(o.out, "execution.json"), &e); err != nil {
				t.Fatal(err)
			}
			if len(e.Slices) != len(e.Selection.ShardSlices) || e.SkippedEmptyShard != (len(e.Selection.ShardPackages) == 0 && len(e.Slices) == 0) {
				t.Fatalf("shard %d audit: %+v", shard, e)
			}
			for i, r := range e.Slices {
				want := e.Selection.ShardSlices[i]
				indices[want.Index]++
				if r.Package != want.Package || r.Index != want.Index || !equal(r.Args, append(append([]string{}, testArgs...), want.Flag, want.Pattern, want.Package)) {
					t.Fatalf("slice argv differs from the audited placement: %+v %+v", r, want)
				}
			}
			ran := false
			for _, ev := range readEvents(t, filepath.Join(o.out, "go.json")) {
				if ev.Test != "" && strings.Contains("pass fail skip", ev.Action) {
					tests[ev.Package+"."+ev.Test]++
					ran = ran || ev.Test == "TestValue"
				}
			}
			if code != map[bool]int{true: 1}[ran] {
				t.Fatalf("shard %d exit %d, relevant failure ran=%v", shard, code, ran)
			}
			if code != 0 {
				failures++
			}
		}
		if indices[0] != 1 || indices[1] != 1 || len(indices) != 2 {
			t.Fatalf("slices of the split package not each placed once: %v", indices)
		}
		return tests, failures
	}
	t.Run("narrowed", func(t *testing.T) {
		tests, failures := run(t, narrowed, []string{"TestValue"})
		if !equal(tests, map[string]int{a + ".TestValue": 1}) || failures != 1 {
			t.Fatalf("narrowed split run: %v failures=%d", tests, failures)
		}
	})
	t.Run("full", func(t *testing.T) {
		tests, failures := run(t, full, []string{"TestValue"})
		if !equal(tests, map[string]int{a + ".TestValue": 1, b + ".TestOther": 1}) || failures != 1 {
			t.Fatalf("full split run: %v failures=%d", tests, failures)
		}
	})
	t.Run("stale names", func(t *testing.T) {
		tests, failures := run(t, narrowed, []string{"TestGone"})
		if !equal(tests, map[string]int{a + ".TestValue": 1}) || failures != 1 {
			t.Fatalf("stale slice file lost or repeated a test: %v failures=%d", tests, failures)
		}
	})
	t.Run("refused placement", func(t *testing.T) {
		usePlan(t, func(u []string, n int) ([]cishards.Shard, error) {
			l, err := cishards.PlanFrom(u, n, costs, allowJSON(a), slicesJSON(map[string]splitEntry{a: {named: [][]string{{"TestValue"}}, rest: 500}}))
			for i := range l {
				l[i].Slices = nil
			}
			return l, err
		})
		if err := os.Remove(filepath.Join(o.out, "execution.json")); err != nil {
			t.Fatal(err)
		}
		code, err := execute(ctx, o, full)
		if code != 2 || err == nil || !strings.Contains(err.Error(), "AFP-V0-048") {
			t.Fatalf("lost slices executed: %d %v", code, err)
		}
		if _, err := os.Stat(filepath.Join(o.out, "execution.json")); !os.IsNotExist(err) {
			t.Fatal("refused placement recorded an execution")
		}
	})
}
