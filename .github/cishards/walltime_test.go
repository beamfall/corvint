package cishards

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// wallUnit is one placed unit of a fixture: a whole package (index -1) or one
// of its test slices.
type wallUnit struct {
	pkg   string
	index int
	cost  int64
}

// fixtureUnits lists the units a fixture's universe is placed as, in the
// AFP-V0-022 order: longest first, then package, then slice index.
func fixtureUnits(universe []string, costs map[string]int64, splits map[string]fixtureSplit) []wallUnit {
	units := []wallUnit{}
	for _, p := range universe {
		if s, ok := splits[p]; ok {
			for i, ms := range s.ms {
				units = append(units, wallUnit{p, i, ms})
			}
			continue
		}
		units = append(units, wallUnit{p, -1, costs[p]})
	}
	sort.Slice(units, func(i, j int) bool {
		a, b := units[i], units[j]
		if a.cost != b.cost {
			return a.cost > b.cost
		}
		if a.pkg != b.pkg {
			return a.pkg < b.pkg
		}
		return a.index < b.index
	})
	return units
}

// leastLoaded is the AFP-V0-022 placement AFP-V0-2181 amends, kept as an oracle:
// each unit into the least-loaded shard, lowest index on ties, a slice never
// beside a sibling. It returns the shard of each unit.
func leastLoaded(units []wallUnit, total int) map[wallUnit]int {
	loads := make([]int64, total)
	holds := make([]map[string]bool, total)
	at := map[wallUnit]int{}
	for _, u := range units {
		bin := -1
		for i := 0; i < total; i++ {
			if u.index >= 0 && holds[i][u.pkg] {
				continue
			}
			if bin < 0 || loads[i] < loads[bin] {
				bin = i
			}
		}
		if holds[bin] == nil {
			holds[bin] = map[string]bool{}
		}
		holds[bin][u.pkg] = true
		loads[bin] += u.cost
		at[u] = bin
	}
	return at
}

// placed returns the shard of each unit of a plan.
func placed(plan []Shard, costs map[string]int64) map[wallUnit]int {
	at := map[wallUnit]int{}
	for i, s := range plan {
		for _, p := range s.Packages {
			at[wallUnit{p, -1, costs[p]}] = i
		}
		for _, sl := range s.Slices {
			at[wallUnit{sl.Package, sl.Index, sl.Milliseconds}] = i
		}
	}
	return at
}

// walls returns each shard's predicted wall milliseconds at parallel: Wall over
// its units, divided back by parallel.
func walls(plan []Shard, costs map[string]int64, parallel int) []float64 {
	out := make([]float64, len(plan))
	for i, s := range plan {
		var sum, longest int64
		for _, p := range s.Packages {
			sum += costs[p]
			longest = max(longest, costs[p])
		}
		for _, sl := range s.Slices {
			sum += sl.Milliseconds
			longest = max(longest, sl.Milliseconds)
		}
		out[i] = float64(Wall(sum, longest, parallel)) / float64(parallel)
	}
	return out
}

func spread(w []float64) (hi, lo float64) {
	hi, lo = w[0], w[0]
	for _, v := range w {
		hi, lo = max(hi, v), min(lo, v)
	}
	return hi, lo
}

// lowerBound is the best wall any placement of units across total shards at
// parallel can reach: the longest unit, or the sum spread over every slot.
func lowerBound(units []wallUnit, total, parallel int) float64 {
	var sum, longest int64
	for _, u := range units {
		sum += u.cost
		longest = max(longest, u.cost)
	}
	return max(float64(longest), float64(sum)/float64(total*parallel))
}

func TestAFPV02181WallModel(t *testing.T) {
	t.Run("AFP-V0-2181", func(t *testing.T) {
		for _, c := range []struct {
			sum, longest int64
			parallel     int
			want         int64
		}{
			{0, 0, 1, 0},
			{500, 100, 1, 500}, // parallel 1: the summed load, as AFP-V0-022 balanced
			{500, 300, 2, 600}, // the long pole outlasts the rest
			{500, 100, 4, 500}, // the sum spread over four slots outlasts the pole
			{100, 100, 4, 400},
		} {
			if got := Wall(c.sum, c.longest, c.parallel); got != c.want {
				t.Fatalf("Wall(%d, %d, %d) = %d, want %d", c.sum, c.longest, c.parallel, got, c.want)
			}
		}
		universe := []string{"example.org/a", "example.org/b"}
		costs := costsFile(map[string]int64{"example.org/a": 10, "example.org/b": 20})
		for _, p := range []int{0, -1, MaxParallel + 1} {
			if _, err := PlanFromParallel(universe, 2, p, costs, nil, nil); err == nil {
				t.Fatalf("PlanFromParallel admitted parallel %d", p)
			}
			if _, err := SelectFromParallel(universe, universe[:1], 2, p, costs, nil, nil); err == nil {
				t.Fatalf("SelectFromParallel admitted parallel %d", p)
			}
		}
		if _, err := PlanFromParallel(universe, 2, MaxParallel, costs, nil, nil); err != nil {
			t.Fatal(err)
		}
		// Invalid costs keep the lexical fallback at every parallelism.
		for _, p := range []int{1, 4} {
			plan, err := PlanFromParallel(universe, 2, p, []byte("{}"), nil, nil)
			if err != nil || !reflect.DeepEqual(plan, []Shard{{Packages: universe[:1]}, {Packages: universe[1:]}}) {
				t.Fatalf("parallel %d fallback %v %v", p, plan, err)
			}
		}
	})
}

// randomWallFixture is a universe with tied costs and a few split packages.
func randomWallFixture(r *rand.Rand) (universe []string, ms map[string]int64, splits map[string]fixtureSplit, total int) {
	total = 2 + r.Intn(7)
	ms = map[string]int64{}
	splits = map[string]fixtureSplit{}
	for i, n := 0, 4+r.Intn(30); i < n; i++ {
		p := fmt.Sprintf("example.org/p%02d", i)
		universe = append(universe, p)
		ms[p] = int64(10 * (1 + r.Intn(12)))
		if r.Intn(5) == 0 {
			count := 2 + r.Intn(total-1)
			s := fixtureSplit{}
			for j := 0; j < count; j++ {
				if j < count-1 {
					s.named = append(s.named, []string{"TestS" + strconv.Itoa(j)})
				}
				s.ms = append(s.ms, int64(10*(1+r.Intn(12))))
			}
			splits[p] = s
		}
	}
	return universe, ms, splits, total
}

func TestAFPV02181ParallelOnePlacementIsLeastLoaded(t *testing.T) {
	t.Run("AFP-V0-2181", func(t *testing.T) {
		r := rand.New(rand.NewSource(2181))
		for trial := 0; trial < 300; trial++ {
			universe, ms, splits, total := randomWallFixture(r)
			allowed := []string{}
			for p := range splits {
				allowed = append(allowed, p)
			}
			costs, allow, slices := costsFile(ms), allowFile(allowed...), slicesFile(splits)
			selected := []string{}
			for _, p := range universe {
				if r.Intn(2) == 0 {
					selected = append(selected, p)
				}
			}
			if len(selected) == 0 {
				selected = universe[:1]
			}
			for _, sub := range [][]string{universe, selected} {
				var plans [][]Shard
				for _, f := range []func() ([]Shard, error){
					func() ([]Shard, error) { return PlanFromParallel(sub, total, 1, costs, allow, slices) },
					func() ([]Shard, error) { return PlanFrom(sub, total, costs, allow, slices) },
					func() ([]Shard, error) { return SelectFromParallel(universe, sub, total, 1, costs, allow, slices) },
					func() ([]Shard, error) { return SelectFrom(universe, sub, total, costs, allow, slices) },
				} {
					plan, err := f()
					if err != nil {
						t.Fatal(err)
					}
					requireComplete(t, sub, plan)
					plans = append(plans, plan)
				}
				for _, plan := range plans[1:] {
					if !reflect.DeepEqual(plan, plans[0]) {
						t.Fatalf("trial %d: parallel-1 plans differ: %v %v", trial, plan, plans[0])
					}
				}
				want := leastLoaded(fixtureUnits(sub, ms, splits), total)
				if got := placed(plans[0], ms); !reflect.DeepEqual(got, want) {
					t.Fatalf("trial %d: parallel 1 is not the AFP-V0-022 least-loaded placement:\n%v\n%v", trial, got, want)
				}
			}
		}
	})
}

func TestAFPV02181PlanAndSelectBalanceWallTime(t *testing.T) {
	t.Run("AFP-V0-2181", func(t *testing.T) {
		// Least-loaded puts 80+50+40 (170 ms) beside the 80 ms pole, a 85 ms wall at
		// -p 2; the wall rule fills the pole's free slot instead and reaches 80 ms.
		ms := map[string]int64{"example.org/a": 80, "example.org/b": 50, "example.org/c": 50, "example.org/d": 50, "example.org/e": 40, "example.org/f": 40}
		universe := []string{"example.org/a", "example.org/b", "example.org/c", "example.org/d", "example.org/e", "example.org/f"}
		costs := costsFile(ms)
		old, err := PlanFrom(universe, 2, costs, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if hi, _ := spread(walls(old, ms, 2)); hi != 85 {
			t.Fatalf("least-loaded wall %v, want 85", hi)
		}
		plan, err := PlanFromParallel(universe, 2, 2, costs, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		requireComplete(t, universe, plan)
		if hi, lo := spread(walls(plan, ms, 2)); hi != 80 || lo != 75 {
			t.Fatalf("wall placement %v..%v, want 75..80: %v", lo, hi, plan)
		}
		// Select places the selection exactly as Plan places it alone.
		wider := append([]string{"example.org/z"}, universe...)
		ms["example.org/z"] = 500
		costs = costsFile(ms)
		selected, err := SelectFromParallel(wider, universe, 2, 2, costs, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		alone, err := PlanFromParallel(universe, 2, 2, costs, nil, nil)
		if err != nil || !reflect.DeepEqual(selected, alone) {
			t.Fatalf("select %v, plan %v %v", selected, alone, err)
		}
		// A split package's slices still avoid each other at parallel above 1.
		splits := map[string]fixtureSplit{"example.org/a": {named: [][]string{{"TestA"}}, ms: []int64{40, 40}}}
		plan, err = PlanFromParallel(universe, 2, 4, costs, allowFile("example.org/a"), slicesFile(splits))
		if err != nil {
			t.Fatal(err)
		}
		if requireComplete(t, universe, plan)["example.org/a"] != 2 {
			t.Fatalf("split package not sliced: %v", plan)
		}
	})
}

const replayPrefix = "github.com/Beamfall/corvint/"

// replayFixture is the frozen per-unit milliseconds of run 41568676566: each
// package's measured time, and for each split package its committed slices.
func replayFixture(t *testing.T) (universe []string, ms map[string]int64, splits map[string]fixtureSplit) {
	t.Helper()
	ms = map[string]int64{}
	splits = map[string]fixtureSplit{}
	for _, entry := range strings.Split(run41568676566, "; ") {
		if entry == "" {
			continue
		}
		path, times, ok := strings.Cut(entry, " ")
		if !ok {
			t.Fatalf("fixture entry %q", entry)
		}
		p := replayPrefix + path
		universe = append(universe, p)
		s := fixtureSplit{}
		for i, f := range strings.Split(times, ",") {
			n, err := strconv.ParseInt(f, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			ms[p] += n
			s.ms = append(s.ms, n)
			s.named = append(s.named, []string{"TestS" + strconv.Itoa(i)})
		}
		if len(s.ms) > 1 {
			s.named = s.named[:len(s.ms)-1]
			splits[p] = s
		}
	}
	sort.Strings(universe)
	return universe, ms, splits
}

func TestAFPV02181Run41568676566Replay(t *testing.T) {
	t.Run("AFP-V0-2181", func(t *testing.T) {
		universe, ms, splits := replayFixture(t)
		if len(universe) != 426 || len(splits) != 3 {
			t.Fatalf("fixture has %d packages, %d split", len(universe), len(splits))
		}
		allowed := []string{}
		for p := range splits {
			allowed = append(allowed, p)
		}
		costs, allow, slices := costsFile(ms), allowFile(allowed...), slicesFile(splits)
		const shards = 16
		byCost := append([]string{}, universe...)
		sort.SliceStable(byCost, func(i, j int) bool { return ms[byCost[i]] > ms[byCost[j]] })
		heavy := byCost[:40] // a narrowed run of the 40 heaviest packages, every pole included
		sparse := append([]string{}, byCost[:3]...)
		for i := 3; i < len(byCost); i += 12 {
			sparse = append(sparse, byCost[i])
		}
		for _, c := range []struct {
			name     string
			selected []string
			parallel int
			ratio    bool // max/min, else max over the lower bound
		}{
			{"full", nil, 1, true},
			{"full", nil, 4, false},
			{"heavy40", heavy, 1, true},
			{"heavy40", heavy, 4, false},
			{"sparse", sparse, 1, false},
			{"sparse", sparse, 4, false},
		} {
			var plan []Shard
			var err error
			sub := universe
			if c.selected == nil {
				plan, err = PlanFromParallel(universe, shards, c.parallel, costs, allow, slices)
			} else {
				sub = c.selected
				plan, err = SelectFromParallel(universe, c.selected, shards, c.parallel, costs, allow, slices)
			}
			if err != nil {
				t.Fatal(err)
			}
			requireComplete(t, sub, plan)
			hi, lo := spread(walls(plan, ms, c.parallel))
			bound := lowerBound(fixtureUnits(sub, ms, splits), shards, c.parallel)
			old, err := SelectFrom(universe, sub, shards, costs, allow, slices)
			if err != nil {
				t.Fatal(err)
			}
			oldHi, _ := spread(walls(old, ms, c.parallel))
			t.Logf("%s -p %d: wall %.1f..%.1f s (max/min %.3f), lower bound %.1f s, least-loaded max %.1f s",
				c.name, c.parallel, lo/1000, hi/1000, hi/lo, bound/1000, oldHi/1000)
			if c.ratio && hi > 1.25*lo {
				t.Fatalf("%s -p %d: max/min %.3f above 1.25", c.name, c.parallel, hi/lo)
			}
			if hi > 1.25*bound {
				t.Fatalf("%s -p %d: max %.1f s above 1.25 x the %.1f s lower bound", c.name, c.parallel, hi/1000, bound/1000)
			}
			if hi > oldHi {
				t.Fatalf("%s -p %d: wall placement %.1f s worse than least-loaded %.1f s", c.name, c.parallel, hi/1000, oldHi/1000)
			}
		}
	})
}

// run41568676566 is generated from package-costs.json and test-slices.json as
// refreshed from https://github.com/beamfall/corvint/actions/runs/41568676566
// (source 32e93e84d7b2bfc4deeff935b6ffa9c2c84239fe): "path ms" per package,
// "path ms,ms,..." for a split package's slices, the catch-all last. It stays
// frozen when those tables are refreshed again.
const run41568676566 = "" + //corvint:fixture-path
	"benchmarks 8923; benchmarks/dogfood-measure 27156; benchmarks/dogfood-workers 1095; " + //corvint:fixture-path
	"benchmarks/runner 1146; benchmarks/selfuse-batch 78818; " + //corvint:fixture-path
	"cmd/corvint 308045,312587,313009,311137; cmd/corvint-analyzer-c-jni 1441; " + //corvint:fixture-path
	"cmd/corvint-analyzer-dotnet 1046; cmd/corvint-analyzer-go 4791; " + //corvint:fixture-path
	"cmd/corvint-analyzer-html-css 24373; cmd/corvint-analyzer-java 1464; " + //corvint:fixture-path
	"cmd/corvint-analyzer-js 13680; cmd/corvint-analyzer-kotlin-android 1143; " + //corvint:fixture-path
	"cmd/corvint-analyzer-objective-c 1446; cmd/corvint-analyzer-python 23527; " + //corvint:fixture-path
	"cmd/corvint-analyzer-ruby 1046; cmd/corvint-analyzer-rust 1210; " + //corvint:fixture-path
	"cmd/corvint-analyzer-shader 7104; cmd/corvint-analyzer-shell 16320; " + //corvint:fixture-path
	"cmd/corvint-analyzer-sql-native 2033; cmd/corvint-analyzer-structured-data 4049; " + //corvint:fixture-path
	"cmd/corvint-analyzer-swift 1354; cmd/corvint-behavior-falsify 1026; " + //corvint:fixture-path
	"cmd/corvint-cem-candidate 1146; cmd/corvint-cem-experiments 1017; " + //corvint:fixture-path
	"cmd/corvint-companion-release 1035; cmd/corvint-console 1233; cmd/corvint-corpus-http 6061; " + //corvint:fixture-path
	"cmd/corvint-corpus-mcp 54363; cmd/corvint-corpus-parity 1453; " + //corvint:fixture-path
	"cmd/corvint-corpus-republish 2336; cmd/corvint-dashboard-snapshot 3175; " + //corvint:fixture-path
	"cmd/corvint-docs-mcp 2239; cmd/corvint-gateway 1020; cmd/corvint-go-test-provider 78054; " + //corvint:fixture-path
	"cmd/corvint-go-toolchain-receipt 1016; cmd/corvint-intake 1016; " + //corvint:fixture-path
	"cmd/corvint-js-test-provider 12095; cmd/corvint-lsp 2700; cmd/corvint-mcp 1; " + //corvint:fixture-path
	"cmd/corvint-playwright-minimize 1047; cmd/corvint-postmerge-connect 2630; " + //corvint:fixture-path
	"cmd/corvint-postmerge-host-launcher 1334; cmd/corvint-postmerge-metrics 1843; " + //corvint:fixture-path
	"cmd/corvint-postmerge-workflow 1062; cmd/corvint-public-release-check 1051; " + //corvint:fixture-path
	"cmd/corvint-pulse 1373; cmd/corvint-readiness-record 1031; cmd/corvint-release-candidate 1; " + //corvint:fixture-path
	"cmd/corvint-release-gate 1013; cmd/corvint-release-install 1; " + //corvint:fixture-path
	"cmd/corvint-remote-provider 1015; cmd/corvint-ruby-test-provider 1162; cmd/corvint-tasks 1; " + //corvint:fixture-path
	"cmd/corvint-test 55756; cmd/corvint-test-runner 12789; cmd/corvint-test-validity-mcp 1892; " + //corvint:fixture-path
	"cmd/corvint-tests-accept 1041; cmd/corvint-update 1016; cmd/corvint-web-flows 1047; " + //corvint:fixture-path
	"cmd/corvint-work-queue 2414; conformance 1012; conformance/cli-errors-v0 29366; " + //corvint:fixture-path
	"conformance/cli-parity-v0 46871; conformance/dashboard-snapshot-v0 22418; " + //corvint:fixture-path
	"conformance/frontier-v0 1068; conformance/go-live-test-v0 54289; " + //corvint:fixture-path
	"conformance/harness-event-v0 1035; conformance/host-lifecycle-v1 3483; " + //corvint:fixture-path
	"conformance/human-documentation-compiler-v0 2496; conformance/mcp-2026-07-28 28438; " + //corvint:fixture-path
	"conformance/mcp-test-validity-v0 3909; conformance/ocm-v0 4171; conformance/perf-v0 1012; " + //corvint:fixture-path
	"conformance/pulse-snapshot-v0 66471; conformance/release-artifact-v0 25522; " + //corvint:fixture-path
	"conformance/release-artifact-v0/archive-verifier 1019; " + //corvint:fixture-path
	"conformance/release-artifact-v0/archivebuild 1042; " + //corvint:fixture-path
	"conformance/release-artifact-v0/archiveverify 1254; " + //corvint:fixture-path
	"conformance/release-artifact-v0/archivewire 1; conformance/use-cases-v0 1235; " + //corvint:fixture-path
	"conformance/work-queue-v0 14439; conformance/work-queue-v0/cmd/cli-adapter 1; " + //corvint:fixture-path
	"conformance/work-queue-v0/cmd/cli-prose 1; conformance/work-queue-v0/cmd/cli-replay 1; " + //corvint:fixture-path
	"conformance/work-queue-v0/cmd/cli-scope 1; examples/evidence-provider/v0 1013; " + //corvint:fixture-path
	"examples/evidence-provider/v0/check 1032; examples/evidence-provider/v0/conformance 54417; " + //corvint:fixture-path
	"experimental/analyzers/sqlnative 7091; integrations/testfixture 1; internal/analyzercap 5958; " + //corvint:fixture-path
	"internal/analyzerdotnet 1138; internal/analyzerexec 1581; internal/analyzergo 4132; " + //corvint:fixture-path
	"internal/analyzerhtmlcss 4400; internal/analyzerjs 4081; " + //corvint:fixture-path
	"internal/analyzerkotlinandroid 110965; internal/analyzernativebridge 67109; " + //corvint:fixture-path
	"internal/analyzerpython 2862; internal/analyzerruby 1444; internal/analyzerrust 1298; " + //corvint:fixture-path
	"internal/analyzershader 5486; internal/analyzershell 6520; internal/analyzerstructured 1610; " + //corvint:fixture-path
	"internal/analyzerswift 15832; internal/appflows 49623; internal/appmap 215716; " + //corvint:fixture-path
	"internal/archivepath 1017; internal/attest 1331; internal/authorityevent 1028; " + //corvint:fixture-path
	"internal/authoritystore 1793; internal/behaviorfalsify 111753; internal/behaviorfalsify/cli 1; " + //corvint:fixture-path
	"internal/betarung 1065; internal/breakagemap 15375; internal/capcatalog 2163; " + //corvint:fixture-path
	"internal/cem/cemcode 1017; internal/cem/cli 11331; internal/cem/corpusbinding 17843; " + //corvint:fixture-path
	"internal/cem/coverprofile 1; internal/cem/gitauth 9733; internal/cem/gitrun 4838; " + //corvint:fixture-path
	"internal/cem/mdreport 1; internal/cem/patch 1441; internal/cem/publish 1106; " + //corvint:fixture-path
	"internal/cem/sim 1033; internal/cem/stablefixture 1; internal/cem/verify 30594; " + //corvint:fixture-path
	"internal/cem/wire 2452; internal/cem/witnesspaths 1015; internal/cem/workflow 15825; " + //corvint:fixture-path
	"internal/cem2proto 17513; internal/cemcandidate 3444; internal/cemchain 129557; " + //corvint:fixture-path
	"internal/cemdiscriminate 1039; internal/changee2e 40430; internal/changewitness 1019; " + //corvint:fixture-path
	"internal/clitree 1031; internal/clitree/clitreetest 1; internal/compactionkernel 1050; " + //corvint:fixture-path
	"internal/companioncli 1014; internal/companionrelease 440240; internal/console 8674; " + //corvint:fixture-path
	"internal/contextindex 236652; internal/contextsummary 1; internal/corpusindex 70741; " + //corvint:fixture-path
	"internal/corpusrepublish 2538; internal/corpusserve 2009; internal/criterionexperiment 122118; " + //corvint:fixture-path
	"internal/dashboard/adapters 3337; internal/dashboard/authority 1014; " + //corvint:fixture-path
	"internal/dashboard/model 24757; internal/dashboard/repository 24218; " + //corvint:fixture-path
	"internal/dashboard/roadmap 1376; internal/dashboard/source 1802; internal/delta 11271; " + //corvint:fixture-path
	"internal/depsource 1922; internal/diagnostic 14959; internal/disagree 1033; " + //corvint:fixture-path
	"internal/doccitations 1054; internal/doccompiler 8742; internal/doccorpus 221273; " + //corvint:fixture-path
	"internal/doccorpus/selfcorpus 87433; internal/docmaintain 5666; internal/docviews 58492; " + //corvint:fixture-path
	"internal/dogfoodflow 41442; internal/dogfoodocm 1167; internal/dogfoodoperation 1016; " + //corvint:fixture-path
	"internal/evalrepo 1877; internal/evidencereach 1134; internal/execfile 4025; " + //corvint:fixture-path
	"internal/extevidence 193712; internal/extmutate 7916; internal/flowcoverage 6739; " + //corvint:fixture-path
	"internal/flowcoverage/testfixture 1; internal/flowdocs 22060; internal/frictionmetric 1133; " + //corvint:fixture-path
	"internal/frontier 1023; internal/frontiernext 1; internal/frontiernextrepo 1082; " + //corvint:fixture-path
	"internal/frontierrepo 2075; internal/gateagreement 1019; internal/gateselect 2445; " + //corvint:fixture-path
	"internal/genesis 1520; internal/gitancestry 1036; internal/gitancestry/ancestrytest 1; " + //corvint:fixture-path
	"internal/gitnotes 18161; internal/gitprobe 1022; internal/gitstatus 2778; " + //corvint:fixture-path
	"internal/gokernel 5011; internal/goplsclient 1691; internal/gotestrun 2171; " + //corvint:fixture-path
	"internal/groupreap 22435; internal/intake 1297; internal/jsonend 1015; " + //corvint:fixture-path
	"internal/jstestprovider 19769; internal/jstestprovider/externallive 1258; " + //corvint:fixture-path
	"internal/jstestprovider/loopbackhttp 1; internal/liveverify/affected 532432; " + //corvint:fixture-path
	"internal/liveverify/affected/cfamily 1070; internal/liveverify/affected/dotnet 1031; " + //corvint:fixture-path
	"internal/liveverify/affected/golang 44062; internal/liveverify/affected/java 1123; " + //corvint:fixture-path
	"internal/liveverify/affected/java/javalex 1; internal/liveverify/affected/kotlin 1056; " + //corvint:fixture-path
	"internal/liveverify/affected/languages 1025; internal/liveverify/affected/python 1155; " + //corvint:fixture-path
	"internal/liveverify/affected/ruby 1715; internal/liveverify/affected/rust 1058; " + //corvint:fixture-path
	"internal/liveverify/affected/shell 1436; internal/liveverify/affected/swift 1075; " + //corvint:fixture-path
	"internal/liveverify/affected/typescript 27395; internal/liveverify/godiscovery 2643; " + //corvint:fixture-path
	"internal/liveverify/gorunner 39004; internal/liveverify/gotest 1248; " + //corvint:fixture-path
	"internal/liveverify/jsresolve 1021; internal/liveverify/mutate 17071; " + //corvint:fixture-path
	"internal/liveverify/parentverify 1489; internal/liveverify/processidentity 1; " + //corvint:fixture-path
	"internal/liveverify/provider 77282; internal/liveverify/provider/interlock 1137; " + //corvint:fixture-path
	"internal/liveverify/pymutate 1030; internal/liveverify/pyresolve 1427; " + //corvint:fixture-path
	"internal/liveverify/rbmutate 3242; internal/liveverify/rbmutate/minirspec 1; " + //corvint:fixture-path
	"internal/liveverify/session 65910; internal/localauthority 1140; " + //corvint:fixture-path
	"internal/localcompletion 73391; internal/lrf 1014; internal/lrfrepo 174965; " + //corvint:fixture-path
	"internal/lspevidence 2092; internal/lspprovider 5490; internal/lspsnapshot 1037; " + //corvint:fixture-path
	"internal/lspstdio 33628; internal/mcp/bridge 68863; internal/mcp/cli 34231; " + //corvint:fixture-path
	"internal/mcp/corpusbridge 16472; internal/mcp/docsbridge 29997; internal/mcp/protocol 1022; " + //corvint:fixture-path
	"internal/mcp/rootbind 1; internal/mcp/server 2818; internal/mcp/tasksbridge 4293; " + //corvint:fixture-path
	"internal/mcp/tasksread 1013; internal/mcp/testvaliditybridge 1123; internal/mcp/toolgroup 1; " + //corvint:fixture-path
	"internal/migrationratchet 1136; internal/modelgateway 1077; internal/necessity 1333; " + //corvint:fixture-path
	"internal/obscorpus 1143; internal/observations 2808; internal/observedstale 1085; " + //corvint:fixture-path
	"internal/opencodequalification 75599; internal/outcomecal 1037; internal/perfbudget 112975; " + //corvint:fixture-path
	"internal/perfbudget/cmd/perf-budget 7595; internal/plansnapshot 1176; " + //corvint:fixture-path
	"internal/playwrightminimize 1316; internal/playwrightminimize/cli 1; " + //corvint:fixture-path
	"internal/playwrightruns 1753; internal/postmergeconnector 2758; internal/postmergehost 36680; " + //corvint:fixture-path
	"internal/postmergemetrics 3319; internal/postmergeproof 14317; " + //corvint:fixture-path
	"internal/postmergeproof/procfs 1190; internal/postmergeworkflow 105377; " + //corvint:fixture-path
	"internal/procgroup 105644; internal/procprofile 3107; internal/projectpath 1032; " + //corvint:fixture-path
	"internal/projectprofile 1016; internal/proofabstraction 1407; internal/pulse 1045; " + //corvint:fixture-path
	"internal/pythongrammar 1019; internal/pythonsyntax 1050; internal/receiptbundle 4080; " + //corvint:fixture-path
	"internal/releasecandidate 3423; internal/releasegate 48488; internal/remoteprovider 1286; " + //corvint:fixture-path
	"internal/repoenvelope 1016; internal/repro 1016; internal/requestpath 1093; " + //corvint:fixture-path
	"internal/revieworder 1075; internal/rootalias 1; internal/rubyautoload 1029; " + //corvint:fixture-path
	"internal/rubyprofile 2114; internal/rubytestprovider 5216; internal/runhygiene 1; " + //corvint:fixture-path
	"internal/runidentity 2473; internal/runtimeenv 1016; internal/scopelease 11291; " + //corvint:fixture-path
	"internal/secretscreen 107170; internal/semescalate 1486; internal/sessiontest 25155; " + //corvint:fixture-path
	"internal/sharedpath 7364; internal/skhproof 1086; internal/skillexport 1035; " + //corvint:fixture-path
	"internal/slotlearn 1358; internal/specindex 10445; internal/stepnegation 1231; " + //corvint:fixture-path
	"internal/stepverify 21514; internal/stepverify/safeopen 1021; internal/swiftimpact 1178; " + //corvint:fixture-path
	"internal/taskbarrier 1286; internal/taskman 111793; internal/taskprojection 1013; " + //corvint:fixture-path
	"internal/tasks 13840; internal/tasks/archive 26302; internal/tasks/authority 62851; " + //corvint:fixture-path
	"internal/tasks/barrierwarning 1031; internal/tasks/cli 245717,251315; " + //corvint:fixture-path
	"internal/tasks/criterionbinding 1079; internal/tasks/defects 1303; " + //corvint:fixture-path
	"internal/tasks/dispatch 144364; internal/tasks/evidencestore 1023; " + //corvint:fixture-path
	"internal/tasks/fixture 1027; internal/tasks/importer 1042; internal/tasks/intent 57739; " + //corvint:fixture-path
	"internal/tasks/journal 21162; internal/tasks/lifecycle 1015; internal/tasks/mutation 1669; " + //corvint:fixture-path
	"internal/tasks/obligation 1047; internal/tasks/release 1024; internal/tasks/safeopen 1018; " + //corvint:fixture-path
	"internal/tasks/scale 88702; internal/tasks/scopes 1157; internal/tasks/service 10623; " + //corvint:fixture-path
	"internal/tasks/similar 42474; internal/tasks/snapshot 7527; " + //corvint:fixture-path
	"internal/tasks/store 315139,318623,323662,287341,311661; internal/tasks/supervisor 26101; " + //corvint:fixture-path
	"internal/tasks/testhooks 1017; internal/tasks/ticket 1293; internal/tasks/transaction 90942; " + //corvint:fixture-path
	"internal/tasks/wire 1534; internal/tasks/worktrail 1016; internal/tcq 1514; " + //corvint:fixture-path
	"internal/testacceptance 73945; internal/testacceptance/cli 1; internal/testevidence 1114; " + //corvint:fixture-path
	"internal/testplan 6570; internal/testplatform 1052; internal/testrunner 64463; " + //corvint:fixture-path
	"internal/testrunner/cli 1; internal/testrunner/dynamic 14557; internal/testrunner/mobile 7176; " + //corvint:fixture-path
	"internal/testrunner/native 13688; internal/testrunner/platform 10253; " + //corvint:fixture-path
	"internal/testrunner/registry 1025; internal/testrunner/sql 1074; internal/testsource 18112; " + //corvint:fixture-path
	"internal/testsupport 3269; internal/testvalidity 1029; internal/testvaliditydoc 1482; " + //corvint:fixture-path
	"internal/testvaliditydoc/externalorigins 1064; internal/toolchainreceipt 1015; " + //corvint:fixture-path
	"internal/touchsurprise 1451; internal/trace 54687; internal/tracemigraterepo 7447; " + //corvint:fixture-path
	"internal/tracerecordrepo 47466; internal/tracerepopaths 1; internal/unplannedread 2474; " + //corvint:fixture-path
	"internal/untrackedallowance 1011; internal/update 22640; internal/upgradechanges 9268; " + //corvint:fixture-path
	"internal/versionline 1015; internal/witness 13481; internal/witnesscollapse 1952; " + //corvint:fixture-path
	"internal/witnesspaths/datareaders 1; internal/worklistadapter 1; internal/workqueue 3459; " + //corvint:fixture-path
	"internal/worksource 2477; internal/workspace 1033; internal/worktreeimpact 8025; " + //corvint:fixture-path
	"internal/wp3codec 1023; protocol/integrations 1028; script/spec-coverage-audit 1021; " + //corvint:fixture-path
	"tools/beamfall-shadow 25116; tools/beamfall-shadow/fdexec 1; " + //corvint:fixture-path
	"tools/beamfall-shadow/fixtures/matrix/go 1; tools/beamfall-shadow/wrapper 1489; " + //corvint:fixture-path
	"tools/capabilities-check 64387; tools/cem-citation-baseline 4811; " + //corvint:fixture-path
	"tools/cem-release-suites 3951; tools/cem-stable-regen 160059; " + //corvint:fixture-path
	"tools/cem-structural-closure 1161; tools/cem-trial 41936; tools/ci-flake-report 1425; " + //corvint:fixture-path
	"tools/ci-forward-row 34432; tools/ci-nightly-signal 1126; tools/ci-reuse-plan 7191; " + //corvint:fixture-path
	"tools/ci-shard-costs 1140; tools/ci-test-slices 2376; tools/compat-trial 21509; " + //corvint:fixture-path
	"tools/compat-trial/build-fixtures 8864; tools/compat-trial/fixtures/new 1; " + //corvint:fixture-path
	"tools/compat-trial/fixtures/old 1; tools/corvint-pr-tests 119588; tools/cw-trial 18782; " + //corvint:fixture-path
	"tools/declaration-compat-check 1075; tools/docs-blinded-task 1; tools/docs-ci-plan 4664; " + //corvint:fixture-path
	"tools/dogfood-guard-hunks 1; tools/dogfood-inherit-citations 1809; " + //corvint:fixture-path
	"tools/fixture-maintenance-check 3693; tools/flow-guide-check 5943; " + //corvint:fixture-path
	"tools/gate-affected-select 4231; tools/gate-ledger 32760; tools/gotest-evidence 1092; " + //corvint:fixture-path
	"tools/heading-nav-bench 1083; tools/native-hook-observer 1317; tools/paired-trial 1042; " + //corvint:fixture-path
	"tools/pi-tui-fixture 1020; tools/plan-delta 1018; tools/qualify-opencode 1; " + //corvint:fixture-path
	"tools/requirement-id-collisions 1469; tools/retrieval-bench 4429; " + //corvint:fixture-path
	"tools/selection-baseline 1097; tools/str-parity 1022; tools/tasks-profiles-check 1026; " + //corvint:fixture-path
	"tools/test-commands-check 1735; tools/test-commands-run 1035; " + //corvint:fixture-path
	"tools/test-exec-writes-check 3330; tools/test-read-scopes-check 3136; tools/test-shards 55821; " + //corvint:fixture-path
	"tools/tiers-replay 1218; tools/unbounded-readers 1143; " //corvint:fixture-path
