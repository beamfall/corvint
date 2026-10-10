package cishards

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const testRevision = "0123456789abcdef0123456789abcdef01234567"
const testRunURL = "https://github.com/beamfall/corvint/actions/runs/1"

type fixtureSplit struct {
	named [][]string
	ms    []int64 // one per named slice, then the catch-all
}

func costsFile(ms map[string]int64) []byte {
	raw, _ := json.Marshal(map[string]any{
		"profile":      "corvint-ci-package-costs/0",
		"source":       map[string]string{"revision": testRevision, "runURL": testRunURL, "goVersion": "go1.27.1"},
		"milliseconds": ms,
	})
	return raw
}

func allowFile(packages ...string) []byte {
	m := map[string]string{}
	for _, p := range packages {
		m[p] = "fixture: tests are order-independent"
	}
	raw, _ := json.Marshal(map[string]any{"profile": "corvint-ci-test-split-allow/0", "packages": m})
	return raw
}

func slicesFile(splits map[string]fixtureSplit) []byte {
	packages := map[string]any{}
	for p, s := range splits {
		named := []any{}
		for i, tests := range s.named {
			named = append(named, map[string]any{"milliseconds": s.ms[i], "tests": tests})
		}
		packages[p] = map[string]any{"named": named, "restMilliseconds": s.ms[len(s.ms)-1]}
	}
	raw, _ := json.Marshal(map[string]any{
		"profile":  "corvint-ci-test-slices/0",
		"source":   map[string]string{"revision": testRevision, "runURL": testRunURL, "goVersion": "go1.27.1"},
		"packages": packages,
	})
	return raw
}

// requireComplete checks that every package runs either whole in exactly one
// shard or as each of its slices exactly once, siblings in distinct shards.
func requireComplete(t *testing.T, universe []string, plan []Shard) map[string]int {
	t.Helper()
	whole := map[string]int{}
	slices := map[string]map[int]int{}
	for i, s := range plan {
		for _, p := range s.Packages {
			whole[p]++
		}
		for _, sl := range s.Slices {
			if slices[sl.Package] == nil {
				slices[sl.Package] = map[int]int{}
			}
			if _, dup := slices[sl.Package][sl.Index]; dup {
				t.Fatalf("slice %s#%d placed twice", sl.Package, sl.Index)
			}
			slices[sl.Package][sl.Index] = i
		}
	}
	counts := map[string]int{}
	for _, p := range universe {
		switch {
		case whole[p] == 1 && slices[p] == nil:
			counts[p] = 1
		case whole[p] == 0 && slices[p] != nil:
			shards := map[int]bool{}
			for _, s := range plan {
				for _, sl := range s.Slices {
					if sl.Package == p {
						if len(slices[p]) != sl.Count {
							t.Fatalf("%s: %d of %d slices placed", p, len(slices[p]), sl.Count)
						}
						if shards[slices[p][sl.Index]] {
							t.Fatalf("%s: sibling slices share a shard", p)
						}
						shards[slices[p][sl.Index]] = true
					}
				}
			}
			counts[p] = len(slices[p])
		default:
			t.Fatalf("%s: whole=%d slices=%v", p, whole[p], slices[p])
		}
	}
	if len(whole)+len(slices) != len(universe) {
		t.Fatalf("plan names packages outside the universe: %v %v", whole, slices)
	}
	return counts
}

func TestAFPV0041PlanEqualsPartitionWithoutSplits(t *testing.T) {
	r := rand.New(rand.NewSource(41))
	for round := 0; round < 200; round++ {
		universe := []string{}
		ms := map[string]int64{}
		for i, n := 0, 1+r.Intn(40); i < n; i++ {
			p := fmt.Sprintf("example.org/p%02d", i)
			universe = append(universe, p)
			if r.Intn(4) != 0 {
				ms[p] = 1 + r.Int63n(5000)
			}
		}
		if len(ms) == 0 {
			ms["example.org/p00"] = 1
		}
		total := 1 + r.Intn(8)
		for _, costs := range [][]byte{costsFile(ms), nil} {
			want, err := partition(universe, total, costs)
			if err != nil {
				t.Fatal(err)
			}
			for _, files := range [][2][]byte{{nil, nil}, {allowFile(), slicesFile(nil)}, {defaultAllow, defaultSlices}} {
				plan, err := PlanFrom(universe, total, costs, files[0], files[1])
				if err != nil {
					t.Fatal(err)
				}
				for i, s := range plan {
					if len(s.Slices) != 0 || !reflect.DeepEqual(append([]string{}, s.Packages...), append([]string{}, want[i]...)) {
						t.Fatalf("round %d shard %d: plan %v differs from partition %v", round, i, s, want[i])
					}
				}
			}
		}
	}
}

func TestAFPV0041SlicedPlanIsCompleteAndDeterministic(t *testing.T) {
	universe := []string{"example.org/big", "example.org/huge", "example.org/a", "example.org/b", "example.org/c", "example.org/d"}
	costs := costsFile(map[string]int64{"example.org/big": 9000, "example.org/huge": 20000, "example.org/a": 3000, "example.org/b": 2500, "example.org/c": 2000, "example.org/d": 100})
	allow := allowFile("example.org/big", "example.org/huge")
	slices := slicesFile(map[string]fixtureSplit{
		"example.org/big":  {named: [][]string{{"TestA", "TestB"}}, ms: []int64{4500, 4500}},
		"example.org/huge": {named: [][]string{{"TestA"}, {"ExampleB", "FuzzC", "TestC"}}, ms: []int64{7000, 7000, 6000}},
	})
	plan, err := PlanFrom(universe, 4, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	counts := requireComplete(t, universe, plan)
	if counts["example.org/big"] != 2 || counts["example.org/huge"] != 3 || counts["example.org/a"] != 1 {
		t.Fatalf("splits not applied: %v", counts)
	}
	var largest int64
	for _, s := range plan {
		var load int64
		for _, sl := range s.Slices {
			load += sl.Milliseconds
		}
		for _, p := range s.Packages {
			load += map[string]int64{"example.org/a": 3000, "example.org/b": 2500, "example.org/c": 2000, "example.org/d": 100}[p]
		}
		largest = max(largest, load)
	}
	if largest >= 20000 {
		t.Fatalf("splitting did not lower the largest shard: %d", largest)
	}
	want := map[string][2]string{
		"example.org/huge#0": {"-run", "^(TestA)$"},
		"example.org/huge#1": {"-run", "^(ExampleB|FuzzC|TestC)$"},
		"example.org/huge#2": {"-skip", "^(ExampleB|FuzzC|TestA|TestC)$"},
		"example.org/big#1":  {"-skip", "^(TestA|TestB)$"},
	}
	for _, s := range plan {
		for _, sl := range s.Slices {
			if w, ok := want[fmt.Sprintf("%s#%d", sl.Package, sl.Index)]; ok && (w[0] != sl.Flag || w[1] != sl.Pattern) {
				t.Fatalf("%s#%d: %s %s", sl.Package, sl.Index, sl.Flag, sl.Pattern)
			}
		}
	}
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 20; i++ {
		shuffled := append([]string(nil), universe...)
		r.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		again, err := PlanFrom(shuffled, 4, costs, allow, slices)
		if err != nil || !reflect.DeepEqual(plan, again) {
			t.Fatalf("input order changed the plan: %v", err)
		}
	}
}

func TestAFPV0041FallbackKeepsPackagesWhole(t *testing.T) {
	universe := []string{"example.org/a", "example.org/b", "example.org/c", "example.org/split"}
	costs := costsFile(map[string]int64{"example.org/a": 1000, "example.org/b": 1000, "example.org/c": 1000, "example.org/split": 8000})
	allow := allowFile("example.org/split", "example.org/a")
	good := fixtureSplit{named: [][]string{{"TestA"}}, ms: []int64{4000, 4000}}
	long := []string{}
	for i := 0; len(strings.Join(long, "|")) <= MaxPatternBytes; i++ {
		long = append(long, fmt.Sprintf("TestLongName%06d", i))
	}
	sort.Strings(long)
	tooMany := fixtureSplit{}
	for i := 0; i < MaxSlices; i++ {
		tooMany.named = append(tooMany.named, []string{fmt.Sprintf("Test%02d", i)})
		tooMany.ms = append(tooMany.ms, 100)
	}
	tooMany.ms = append(tooMany.ms, 100)
	entry := func(s fixtureSplit) []byte { return slicesFile(map[string]fixtureSplit{"example.org/split": s}) }
	whole := map[string]struct {
		total        int
		costs, a, s  []byte
		splitAnother bool
	}{
		"allow-list unreadable":        {4, costs, []byte(`{"profile":"corvint-ci-test-split-allow/0","packages":{"example.org/split":"x"},"extra":1}`), entry(good), false},
		"allow-list reason empty":      {4, costs, []byte(`{"profile":"corvint-ci-test-split-allow/0","packages":{"example.org/split":" "}}`), entry(good), false},
		"allow-list profile":           {4, costs, []byte(`{"profile":"corvint-ci-test-split-allow/9","packages":{"example.org/split":"x"}}`), entry(good), false},
		"package not allowed":          {4, costs, allowFile("example.org/a"), entry(good), false},
		"slice file profile":           {4, costs, allow, bytes.Replace(entry(good), []byte("test-slices/0"), []byte("test-slices/1"), 1), false},
		"slice file revision":          {4, costs, allow, bytes.Replace(entry(good), []byte(testRevision), []byte("HEAD"), 1), false},
		"slice file go version":        {4, costs, allow, bytes.Replace(entry(good), []byte("go1.27.1"), []byte("go1.26.0"), 1), false},
		"slice file run":               {4, costs, allow, bytes.Replace(entry(good), []byte("beamfall/corvint"), []byte("other/corvint"), 1), false},
		"slice file trailing data":     {4, costs, allow, append(entry(good), []byte(` {}`)...), false},
		"slice file unknown field":     {4, costs, allow, bytes.Replace(entry(good), []byte(`"named"`), []byte(`"extra":1,"named"`), 1), false},
		"costs invalid":                {4, []byte(`{}`), allow, entry(good), false},
		"one shard":                    {1, costs, allow, entry(good), false},
		"more slices than shards":      {2, costs, allow, entry(fixtureSplit{named: [][]string{{"TestA"}, {"TestB"}}, ms: []int64{1, 1, 1}}), false},
		"more slices than bound":       {MaxShards, costs, allow, entry(tooMany), false},
		"empty named slice":            {4, costs, allow, entry(fixtureSplit{named: [][]string{{}}, ms: []int64{1, 1}}), false},
		"name with separator":          {4, costs, allow, entry(fixtureSplit{named: [][]string{{"TestA/b"}}, ms: []int64{1, 1}}), false},
		"name with alternation":        {4, costs, allow, entry(fixtureSplit{named: [][]string{{"TestA|TestB"}}, ms: []int64{1, 1}}), false},
		"name with regexp syntax":      {4, costs, allow, entry(fixtureSplit{named: [][]string{{"Test.*"}}, ms: []int64{1, 1}}), false},
		"name not ASCII":               {4, costs, allow, entry(fixtureSplit{named: [][]string{{"TestÄ"}}, ms: []int64{1, 1}}), false},
		"benchmark name":               {4, costs, allow, entry(fixtureSplit{named: [][]string{{"BenchmarkA"}}, ms: []int64{1, 1}}), false},
		"option-like name":             {4, costs, allow, entry(fixtureSplit{named: [][]string{{"-test.v"}}, ms: []int64{1, 1}}), false},
		"unsorted names":               {4, costs, allow, entry(fixtureSplit{named: [][]string{{"TestB", "TestA"}}, ms: []int64{1, 1}}), false},
		"name in two slices":           {4, costs, allow, entry(fixtureSplit{named: [][]string{{"TestA"}, {"TestA"}}, ms: []int64{1, 1, 1}}), false},
		"zero cost":                    {4, costs, allow, entry(fixtureSplit{named: [][]string{{"TestA"}}, ms: []int64{0, 1}}), false},
		"cost over bound":              {4, costs, allow, entry(fixtureSplit{named: [][]string{{"TestA"}}, ms: []int64{1, 3000001}}), false},
		"pattern over bound":           {4, costs, allow, entry(fixtureSplit{named: [][]string{long}, ms: []int64{1, 1}}), false},
		"one bad package, one good":    {4, costs, allow, slicesFile(map[string]fixtureSplit{"example.org/split": {named: [][]string{{"Test("}}, ms: []int64{1, 1}}, "example.org/a": good}), true},
		"package outside the universe": {4, costs, allow, slicesFile(map[string]fixtureSplit{"example.org/gone": good}), false},
	}
	if plan, err := PlanFrom(universe, 4, costs, allow, entry(good)); err != nil || requireComplete(t, universe, plan)["example.org/split"] != 2 {
		t.Fatalf("control fixture did not split: %v", err)
	}
	for name, c := range whole {
		t.Run(name, func(t *testing.T) {
			plan, err := PlanFrom(universe, c.total, c.costs, c.a, c.s)
			if err != nil {
				t.Fatal(err)
			}
			counts := requireComplete(t, universe, plan)
			if counts["example.org/split"] != 1 {
				t.Fatalf("package split despite %s", name)
			}
			if c.splitAnother != (counts["example.org/a"] == 2) {
				t.Fatalf("other package split=%v", counts["example.org/a"] == 2)
			}
		})
	}
}

// The catch-all keeps exactly-once execution for the tests `go test` finds at run
// time, not the ones the slice file names: here one named test was removed, one
// renamed, and one added since listing. Anchoring keeps TestA from also running
// TestAB, which another slice names.
func TestAFPV0041SlicesRunEveryTestExactlyOnce(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.org/sliced\n\ngo 1.27.1\n",
		"sliced.go": `package sliced

func Double(n int) int { return 2 * n }
`,
		"sliced_test.go": `package sliced

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(m.Run()) }

func TestA(t *testing.T)       {}
func TestAB(t *testing.T)      {}
func TestB(t *testing.T)       { t.Run("x", func(t *testing.T) {}); t.Run("y/z", func(t *testing.T) {}) }
func TestC(t *testing.T)       { t.Parallel() }
func TestRenamed(t *testing.T) {}
func TestAdded(t *testing.T)   {}
func BenchmarkA(b *testing.B)  {}

func FuzzD(f *testing.F) {
	f.Add(1)
	f.Fuzz(func(t *testing.T, n int) {
		if Double(n) != n+n {
			t.Fatal(n)
		}
	})
}

func ExampleDouble() {
	fmt.Println(Double(2))
	// Output: 4
}
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pkg := "example.org/sliced"
	universe := []string{pkg, "example.org/other"}
	costs := costsFile(map[string]int64{pkg: 9000, "example.org/other": 1000})
	slices := slicesFile(map[string]fixtureSplit{pkg: {
		named: [][]string{{"TestA", "TestGone"}, {"ExampleDouble", "FuzzD", "TestOld"}, {"TestAB", "TestB"}},
		ms:    []int64{2000, 2000, 2000, 3000},
	}})
	plan, err := PlanFrom(universe, 4, costs, allowFile(pkg), slices)
	if err != nil {
		t.Fatal(err)
	}
	if requireComplete(t, universe, plan)[pkg] != 4 {
		t.Fatal("fixture package not split")
	}
	ran := map[string]int{}
	for _, s := range plan {
		for _, sl := range s.Slices {
			for name := range goTest(t, dir, sl.Flag, sl.Pattern) {
				ran[name]++
			}
		}
	}
	want := goTest(t, dir)
	for _, name := range []string{"TestA", "TestAB", "TestAdded", "TestRenamed", "TestB/x", "TestB/y/z", "TestC", "ExampleDouble", "FuzzD"} {
		if !want[name] {
			t.Fatalf("unfiltered run lacks %s: %v", name, want)
		}
	}
	for name := range want {
		if ran[name] != 1 {
			t.Fatalf("%s ran %d times across slices: %v", name, ran[name], ran)
		}
	}
	if len(ran) != len(want) {
		t.Fatalf("slices ran tests the package does not have: %v", ran)
	}
}

// goTest returns every test, subtest, example and fuzz seed run that passed.
func goTest(t *testing.T, dir string, args ...string) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", append(append([]string{"test", "-json", "-count=1"}, args...), ".")...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local", "GOPROXY=off")
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("go test %v: %v\n%s", args, err, raw)
	}
	passed := map[string]bool{}
	s := bufio.NewScanner(bytes.NewReader(raw))
	for s.Scan() {
		var e struct{ Action, Test string }
		if json.Unmarshal(s.Bytes(), &e) == nil && e.Test != "" && e.Action == "pass" {
			if passed[e.Test] {
				t.Fatalf("%s passed twice in one invocation", e.Test)
			}
			passed[e.Test] = true
		}
	}
	return passed
}

// A committed slice file the helper would silently ignore is a configuration
// error, not a fallback: every entry must be admitted.
func TestAFPV0041CommittedSlicesAreAdmitted(t *testing.T) {
	allowed, ok := Allowed(defaultAllow)
	if !ok {
		t.Fatal("committed allow-list is unusable")
	}
	var f struct {
		Packages map[string]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(defaultSlices, &f); err != nil {
		t.Fatal(err)
	}
	universe := []string{}
	for p := range f.Packages {
		if allowed[p] == "" {
			t.Fatalf("%s is split but not allowed", p)
		}
		universe = append(universe, p)
	}
	if got := SplitPackages(universe, 6, defaultAllow, defaultSlices); len(got) != len(universe) {
		t.Fatalf("admitted %d of %d committed splits", len(got), len(universe))
	}
	if len(universe) != 0 && !Splits(universe, 6) {
		t.Fatal("Splits disagrees with the committed slices")
	}
	if Splits(universe, 1) {
		t.Fatal("one shard must not split")
	}
}
