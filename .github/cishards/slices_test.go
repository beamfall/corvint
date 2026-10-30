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
	"regexp"
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
	if got := SplitPackages(universe, 8, defaultAllow, defaultSlices); len(got) != len(universe) {
		t.Fatalf("admitted %d of %d committed splits", len(got), len(universe))
	}
	if len(universe) != 0 && !Splits(universe, 8) {
		t.Fatal("Splits disagrees with the committed slices")
	}
	if Splits(universe, 1) {
		t.Fatal("one shard must not split")
	}
}

// TestAFPSourceGoVersionsAdmitted proves a cost table and a slice file are usable
// from any admitted source toolchain and refused from any other (decisions 0522, 0557).
func TestAFPSourceGoVersionsAdmitted(t *testing.T) {
	costs := costsFile(map[string]int64{"example.org/a": 1000})
	slices := slicesFile(map[string]fixtureSplit{"example.org/split": {named: [][]string{{"TestA"}}, ms: []int64{1, 1}}})
	for version, want := range map[string]bool{"go1.27.1": true, "go1.27.2": true, "go1.27.3": true, "go1.27.4": false, "go1.26.0": false} {
		c := bytes.Replace(costs, []byte(`"go1.27.1"`), []byte(`"`+version+`"`), 1)
		if _, ok := Costs(c); ok != want {
			t.Errorf("cost table from %s usable=%t, want %t", version, ok, want)
		}
		s := bytes.Replace(slices, []byte(`"go1.27.1"`), []byte(`"`+version+`"`), 1)
		if ok := SliceFileUsable(s); ok != want {
			t.Errorf("slice file from %s usable=%t, want %t", version, ok, want)
		}
	}
}

// AFP-V0-060 (V1-1096): the committed slice file splits both oversized allowed
// packages into slices of at most 400 s each, and over the committed cost
// table's universe the plan places every package or slice exactly once, each
// split package's slices in different shards. Every name the file records is
// selected by exactly one slice of its package, and a name added or renamed
// since generation is selected by the catch-all alone.
func TestAFPV0060CommittedSlicesBoundOversizedPackages(t *testing.T) {
	const bound = 400000
	var f sliceFile
	if !strict(defaultSlices, &f) {
		t.Fatal("committed slice file does not decode")
	}
	costs, ok := Costs(defaultCosts)
	if !ok {
		t.Fatal("committed cost table is unusable")
	}
	universe := make([]string, 0, len(costs))
	for p := range costs {
		universe = append(universe, p)
	}
	// AFP-V0-164 (V1-1752): internal/tasks/cli is split too; internal/tasks/authority
	// stays allowed but runs whole while its cost is within the bound (V1-0961).
	for _, p := range []string{
		"github.com/Beamfall/corvint/cmd/corvint",
		"github.com/Beamfall/corvint/internal/tasks/store",
		"github.com/Beamfall/corvint/internal/tasks/cli",
	} {
		if _, ok := f.Packages[p]; !ok {
			t.Fatalf("%s is not split", p)
		}
		if _, ok := costs[p]; !ok {
			t.Fatalf("%s is missing from the cost table", p)
		}
	}
	// AFP-V0-164: every allowed package the cost table puts above the bound is split.
	allowed, ok := Allowed(defaultAllow)
	if !ok {
		t.Fatal("committed allow-list is unusable")
	}
	for p := range allowed {
		if _, split := f.Packages[p]; costs[p] > bound && !split {
			t.Fatalf("%s costs %d ms, over %d, and is not split", p, costs[p], bound)
		}
	}
	split := SplitPackages(universe, 8, defaultAllow, defaultSlices)
	if len(split) != len(f.Packages) {
		t.Fatalf("admitted %d of %d committed splits", len(split), len(f.Packages))
	}
	plan, err := Plan(universe, 8)
	if err != nil {
		t.Fatal(err)
	}
	requireComplete(t, universe, plan)
	for p, slices := range split {
		shards := map[int]bool{}
		for i, s := range plan {
			for _, sl := range s.Slices {
				if sl.Package == p {
					if shards[i] {
						t.Fatalf("%s: two slices in shard %d", p, i)
					}
					shards[i] = true
				}
			}
		}
		patterns := make([]*regexp.Regexp, len(slices))
		for i, sl := range slices {
			if sl.Milliseconds > bound {
				t.Fatalf("%s#%d plans %d ms, over %d", p, sl.Index, sl.Milliseconds, bound)
			}
			patterns[i] = regexp.MustCompile(sl.Pattern)
		}
		selects := func(name string) int {
			n := 0
			for i, sl := range slices {
				if patterns[i].MatchString(name) == (sl.Flag == "-run") {
					n++
				}
			}
			return n
		}
		for _, named := range f.Packages[p].Named {
			for _, name := range named.Tests {
				if got := selects(name); got != 1 {
					t.Fatalf("%s: %s is selected by %d slices", p, name, got)
				}
				if got := selects(name + "Renamed"); got != 1 {
					t.Fatalf("%s: renamed %s is selected by %d slices", p, name, got)
				}
			}
		}
		if got := selects("TestAFPV0060AddedAfterGeneration"); got != 1 {
			t.Fatalf("%s: an added test is selected by %d slices", p, got)
		}
	}
}

// AFP-V0-590 fixture: two split packages, priced and unpriced whole packages.
func selectFixture() (universe []string, costs, allow, slices []byte) {
	universe = []string{"example.org/big", "example.org/huge", "example.org/a", "example.org/b", "example.org/c", "example.org/d", "example.org/e", "example.org/new"}
	costs = costsFile(map[string]int64{"example.org/big": 9000, "example.org/huge": 20000, "example.org/a": 3000, "example.org/b": 2500, "example.org/c": 2000, "example.org/d": 100, "example.org/e": 1500})
	allow = allowFile("example.org/big", "example.org/huge")
	slices = slicesFile(map[string]fixtureSplit{
		"example.org/big":  {named: [][]string{{"TestA", "TestB"}}, ms: []int64{4500, 4500}},
		"example.org/huge": {named: [][]string{{"TestA"}, {"ExampleB", "FuzzC", "TestC"}}, ms: []int64{7000, 7000, 6000}},
	})
	return
}

func TestAFPV0590SelectRunsEverySelectedPackageExactlyOnce(t *testing.T) {
	universe, costs, allow, slices := selectFixture()
	r := rand.New(rand.NewSource(590))
	for total := 1; total <= 8; total++ {
		for mask := 0; mask < 1<<len(universe); mask++ {
			selected := []string{}
			for i, p := range universe {
				if mask&(1<<i) != 0 {
					selected = append(selected, p)
				}
			}
			// Out-of-universe entries and duplicates never reach a shard.
			input := append(append([]string{"example.org/absent"}, selected...), selected...)
			r.Shuffle(len(input), func(a, b int) { input[a], input[b] = input[b], input[a] })
			plan, err := SelectFrom(universe, input, total, costs, allow, slices)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan) != total {
				t.Fatalf("%d shards for total %d", len(plan), total)
			}
			if len(selected) == 0 {
				if !reflect.DeepEqual(plan, make([]Shard, total)) {
					t.Fatalf("empty selection placed packages: %v", plan)
				}
				continue
			}
			sort.Strings(selected)
			requireComplete(t, selected, plan)
		}
	}
}

func TestAFPV0590SelectBalancesByMeasuredCost(t *testing.T) {
	universe, costs, allow, slices := selectFixture()
	ms := map[string]int64{"example.org/a": 3000, "example.org/b": 2500, "example.org/c": 2000, "example.org/d": 100, "example.org/e": 1500, "example.org/new": 2000}
	load := func(s Shard) int64 {
		var n int64
		for _, sl := range s.Slices {
			n += sl.Milliseconds
		}
		for _, p := range s.Packages {
			n += ms[p]
		}
		return n
	}
	full, err := PlanFrom(universe, 4, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	// Whole packages the intersection would run on one full shard spread across shards.
	selected := []string{}
	for _, s := range full {
		if len(s.Packages) > len(selected) {
			selected = append([]string(nil), s.Packages...)
		}
	}
	if len(selected) < 2 {
		t.Fatalf("fixture leaves no shard with two whole packages: %v", full)
	}
	plan, err := SelectFrom(universe, selected, 4, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	var sum, largest, unit int64
	used := 0
	for _, s := range plan {
		sum += load(s)
		largest = max(largest, load(s))
		if len(s.Packages)+len(s.Slices) > 0 {
			used++
		}
		for _, p := range s.Packages {
			unit = max(unit, ms[p])
		}
	}
	// List scheduling bound: no shard exceeds the mean plus the largest unit.
	if largest > sum/4+unit {
		t.Fatalf("largest shard %d exceeds the mean %d plus the largest unit %d", largest, sum/4, unit)
	}
	if used < 2 || largest >= sum {
		t.Fatalf("balanced placement uses %d shard(s), largest %d of %d", used, largest, sum)
	}
}

// The median of the whole cost table, not of the selection, prices an unpriced package.
func TestAFPV0590SelectUsesTheFullTableMedian(t *testing.T) {
	universe := []string{"example.org/a", "example.org/b", "example.org/c", "example.org/d", "example.org/x", "example.org/y", "example.org/z"}
	costs := costsFile(map[string]int64{"example.org/a": 100, "example.org/b": 100, "example.org/c": 100, "example.org/d": 3000})
	plan, err := SelectFrom(universe, []string{"example.org/z", "example.org/d", "example.org/x", "example.org/y"}, 2, costs, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []Shard{{Packages: []string{"example.org/d"}}, {Packages: []string{"example.org/x", "example.org/y", "example.org/z"}}}
	if !reflect.DeepEqual(plan, want) {
		t.Fatalf("unpriced packages not priced at the table median: %v", plan)
	}
}

func TestAFPV0590SelectIsDeterministicAndKeepsFullPlan(t *testing.T) {
	universe, costs, allow, slices := selectFixture()
	selected := []string{"example.org/huge", "example.org/new", "example.org/a", "example.org/d"}
	plan, err := SelectFrom(universe, selected, 8, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewSource(8))
	for i := 0; i < 20; i++ {
		u := append([]string(nil), universe...)
		s := append([]string(nil), selected...)
		r.Shuffle(len(u), func(a, b int) { u[a], u[b] = u[b], u[a] })
		r.Shuffle(len(s), func(a, b int) { s[a], s[b] = s[b], s[a] })
		again, err := SelectFrom(u, s, 8, costs, allow, slices)
		if err != nil || !reflect.DeepEqual(plan, again) {
			t.Fatalf("input order changed the selected plan: %v", err)
		}
	}
	// Selecting everything, or "./...", is the full partition, so the full fallback is unchanged.
	full, err := PlanFrom(universe, 8, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	for _, all := range [][]string{universe, {"./..."}} {
		got, err := SelectFrom(universe, all, 8, costs, allow, slices)
		if err != nil || !reflect.DeepEqual(got, full) {
			t.Fatalf("whole selection %v differs from the full plan: %v", all, err)
		}
	}
	if _, err := SelectFrom(nil, selected, 8, costs, allow, slices); err == nil {
		t.Fatal("empty universe admitted")
	}
	if _, err := SelectFrom(universe, selected, 0, costs, allow, slices); err == nil {
		t.Fatal("zero shards admitted")
	}
	if got, err := Select(universe, selected, 8); err != nil || len(got) != 8 {
		t.Fatalf("default-file Select: %v", err)
	}
}
