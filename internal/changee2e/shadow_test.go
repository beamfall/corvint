package changee2e

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func registered() ShadowRegistration {
	return ShadowRegistration{Operators: []string{"text-literal-swap"}, Seed: "42", CatalogDigest: "sha256:" + strings.Repeat("a", 64),
		CompilerVersion: CompilerVersion}
}

func eligibleScenario(killed ...string) ShadowScenario {
	return ShadowScenario{Obligation: "obligation:f/o", Verdict: VerdictDiscriminates, OracleOrigin: OriginAdmittedAtBase, KilledMutants: killed}
}

// gateWindow builds a window that meets the frozen gate: 30 entries (10 bug-fix), hosted tests
// t1 and t2 run in every entry, and each catches perEntry seeded mutants per entry, all of which
// an eligible compiled scenario also catches.
func gateWindow(total int) ShadowWindow {
	w := ShadowWindow{Schema: ShadowWindowSchema, Registration: registered(), Gate: FrozenShadowGate,
		HostedTests: []HostedTest{{ID: "t1", AssertedOutcomes: []string{"outcome:f/o"}}, {ID: "t2", AssertedOutcomes: []string{"outcome:f/p"}}}}
	for i := range 30 {
		typ := EntryMerged
		if i < 10 {
			typ = EntryBugFix
		}
		e := ShadowEntry{ID: fmt.Sprintf("e%02d", i), Type: typ, Base: "b", Head: "h", TestRevision: "h", Obligations: map[string]int{"REACHED": 1},
			Hosted:   HostedSide{TestsRun: []string{"t1", "t2"}, MutantsCaught: map[string][]string{}, Cost: SideCost{WallMS: int64(600000 + i*1000), CPUMS: 900000, CostUSD: 0.5}},
			Compiled: SideCost{WallMS: int64(60000 + i*1000), CPUMS: 120000, CostUSD: 0.05}}
		w.Entries = append(w.Entries, e)
	}
	for n := range total {
		for _, test := range []string{"t1", "t2"} {
			e := &w.Entries[n%30]
			m := fmt.Sprintf("%s-m%03d", test, n)
			e.Mutants = append(e.Mutants, m)
			e.Hosted.MutantsCaught[test] = append(e.Hosted.MutantsCaught[test], m)
			if len(e.Scenarios) == 0 {
				e.Scenarios = []ShadowScenario{eligibleScenario()}
			}
			e.Scenarios[0].KilledMutants = append(e.Scenarios[0].KilledMutants, m)
		}
	}
	return w
}

type shadowDoc struct {
	Entries []struct {
		ID              string   `json:"id"`
		Misses          []string `json:"misses"`
		Escapes         []string `json:"escapes"`
		Useful          bool     `json:"useful"`
		CompiledMinutes float64  `json:"compiled_minutes"`
		HostedMinutes   float64  `json:"hosted_minutes"`
		Compiled        SideCost `json:"compiled"`
		Hosted          SideCost `json:"hosted"`
	} `json:"entries"`
	Totals struct {
		Usefulness    Rate `json:"usefulness"`
		SeededParity  Rate `json:"seeded_parity"`
		MissRate      Rate `json:"miss_rate"`
		Escapes       int  `json:"escapes"`
		CompiledExtra int  `json:"seeded_caught_compiled_only"`
		Compiled      struct {
			Minutes     float64 `json:"minutes"`
			CostUSD     float64 `json:"cost_usd"`
			WallMSP95   int64   `json:"wall_ms_p95"`
			WallMSMedia int64   `json:"wall_ms_median"`
		} `json:"compiled"`
		Hosted struct {
			Minutes float64 `json:"minutes"`
			CostUSD float64 `json:"cost_usd"`
		} `json:"hosted"`
	} `json:"totals"`
	Candidates []candidateReport `json:"candidates"`
}

func runShadow(t *testing.T, w ShadowWindow) shadowDoc {
	t.Helper()
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Shadow(data)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Shadow(data)
	if string(out) != string(again) {
		t.Fatal("shadow report is not deterministic")
	}
	var doc shadowDoc
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// CCE-V0-011: a window that does not pin the operator set, seed, catalog digest and compiler
// version refuses; so does a malformed or inconsistent one.
func TestCCEV0011ShadowRegistrationRefuses(t *testing.T) {
	for name, edit := range map[string]func(*ShadowWindow){
		"no operators":        func(w *ShadowWindow) { w.Registration.Operators = nil },
		"empty operator":      func(w *ShadowWindow) { w.Registration.Operators = []string{""} },
		"no seed":             func(w *ShadowWindow) { w.Registration.Seed = "" },
		"no catalog digest":   func(w *ShadowWindow) { w.Registration.CatalogDigest = "" },
		"no compiler version": func(w *ShadowWindow) { w.Registration.CompilerVersion = "" },
	} {
		w := gateWindow(1)
		edit(&w)
		data, _ := json.Marshal(w)
		if _, err := Shadow(data); refusalCode(err) != "e2e-shadow-unregistered" {
			t.Fatalf("%s: %v", name, err)
		}
	}
	w := gateWindow(1)
	w.Entries[1].Mutants = append(w.Entries[1].Mutants, w.Entries[0].Mutants[0])
	dup, _ := json.Marshal(w)
	for name, data := range map[string][]byte{
		"unknown member":    []byte(`{"schema":"e2e-shadow-window/0","extra":1}`),
		"wrong schema":      []byte(`{"schema":"e2e-shadow-window/9"}`),
		"duplicated mutant": dup,
	} {
		if _, err := Shadow(data); refusalCode(err) != "e2e-shadow-window-invalid" {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// CCE-V0-011: a mutant caught by one eligible scenario beside a broken one counts once as caught;
// a mutant caught only by non-scoring scenarios is one miss; in an UNKNOWN entry every hosted
// catch is a miss; a real failure without an eligible REGRESSION_DETECTED scenario is one miss
// and one escape.
func TestCCEV0011ShadowMissesAndEscapes(t *testing.T) {
	w := ShadowWindow{Schema: ShadowWindowSchema, Registration: registered(), Gate: FrozenShadowGate,
		HostedTests: []HostedTest{{ID: "t1", AssertedOutcomes: []string{"outcome:f/o"}}, {ID: "t2"}}}
	hosted := func(ms ...string) HostedSide {
		return HostedSide{TestsRun: []string{"t1", "t2"}, MutantsCaught: map[string][]string{"t1": ms}}
	}
	w.Entries = []ShadowEntry{
		{ID: "a", Type: EntryMerged, Base: "b", Head: "h", TestRevision: "h", Obligations: map[string]int{"REACHED": 2}, Mutants: []string{"m1", "m2", "m9"},
			Hosted: hosted("m1", "m2"), Scenarios: []ShadowScenario{
				eligibleScenario("m1", "m9"),
				{Obligation: "o2", Verdict: VerdictScenarioBroken, OracleOrigin: OriginAdmittedAtBase, KilledMutants: []string{"m1", "m2"}},
				{Obligation: "o3", Verdict: VerdictNewReach, OracleOrigin: OriginNewInDiff, KilledMutants: []string{"m2"}},
				{Obligation: "o4", Verdict: VerdictDiscriminates, OracleOrigin: OriginChangedInDiff, KilledMutants: []string{"m2"}},
			}},
		{ID: "u", Type: EntryMerged, Base: "b", Head: "h", TestRevision: "h", Obligations: map[string]int{"REACHED": 1, "UNKNOWN": 1}, Mutants: []string{"m3"},
			Hosted: hosted("m3"), Scenarios: []ShadowScenario{eligibleScenario("m3")}},
		{ID: "f", Type: EntryBugFix, Base: "b", Head: "h", TestRevision: "b", Obligations: map[string]int{"REACHED": 2},
			Hosted: HostedSide{TestsRun: []string{"t1", "t2"}, RealFailures: []string{"t1", "t2"}}, Scenarios: []ShadowScenario{
				{Obligation: "o1", Verdict: VerdictRegressionDetected, OracleOrigin: OriginAdmittedAtBase, RealFailures: []string{"t1"}},
				{Obligation: "o2", Verdict: VerdictRegressionDetected, OracleOrigin: OriginChangedInDiff, RealFailures: []string{"t2"}},
			}},
	}
	doc := runShadow(t, w)
	want := map[string]string{"a": "mutant:m2", "u": "mutant:m3", "f": "failure:f/t2"}
	for _, e := range doc.Entries {
		if strings.Join(e.Misses, ",") != want[e.ID] {
			t.Fatalf("entry %s misses %v, want %s", e.ID, e.Misses, want[e.ID])
		}
		if wantEscape := e.ID == "f"; (len(e.Escapes) == 1) != wantEscape || len(e.Escapes) > 1 {
			t.Fatalf("entry %s escapes %v", e.ID, e.Escapes)
		}
	}
	if doc.Totals.MissRate.Numerator != 2 || doc.Totals.MissRate.Denominator != 3 || doc.Totals.SeededParity.Numerator != 1 ||
		doc.Totals.Escapes != 1 || doc.Totals.CompiledExtra != 1 {
		t.Fatalf("totals %+v", doc.Totals)
	}
	if u := doc.Totals.Usefulness; u.Numerator != 2 || u.Denominator != 3 || u.Side != "lower" {
		t.Fatalf("usefulness %+v (entry u reads UNKNOWN and is not useful)", u)
	}
	// t2 asserts no outcome and caught no seeded regression: every zero-denominator criterion fails.
	for _, c := range doc.Candidates {
		if c.Test == "t2" {
			for _, cr := range c.Criteria {
				if cr.Held && cr.Name != "zero-escapes" {
					t.Fatalf("zero-denominator criterion held: %+v", cr)
				}
			}
			if c.Candidate || !strings.Contains(c.Criteria[4].Evidence, "denominator zero") {
				t.Fatalf("t2: %+v", c)
			}
		}
	}
}

// CCE-V0-011/012: the bounds and the Holm correction match the reference computation in Shadow
// evaluation and retirement: zero misses needs 59 caught mutants uncorrected (0.95^59 ~ 0.0485),
// and the first Holm step for two candidates needs 72 (0.95^72 ~ 0.0249 <= 0.025 < 0.95^71).
func TestCCEV0012ShadowBoundsAndHolm(t *testing.T) {
	near := func(got, want float64) bool { return math.Abs(got-want) < 1e-6 }
	for _, tc := range []struct {
		got, want float64
		name      string
	}{
		{binomialCDF(0, 59, 0.05), math.Pow(0.95, 59), "P(X=0|59)"},
		{binomialCDF(1, 10, 0.3), 0.149308, "P(X<=1|10,0.3)"},
		{binomialCDF(10, 10, 0.3), 1, "P(X<=n)"},
		{clopperPearsonUpper(0, 59), round6(1 - math.Pow(0.05, 1.0/59)), "upper 0/59"},
		{clopperPearsonUpper(1, 10), 0.394163, "upper 1/10"},
		{clopperPearsonLower(9, 10), 0.605837, "lower 9/10"},
		{clopperPearsonUpper(0, 0), 1, "upper with zero denominator"},
		{clopperPearsonLower(0, 0), 0, "lower with zero denominator"},
	} {
		if !near(tc.got, tc.want) {
			t.Fatalf("%s = %.6f, want %.6f", tc.name, tc.got, tc.want)
		}
	}
	if !(math.Pow(0.95, 59) <= 0.05 && math.Pow(0.95, 58) > 0.05 && math.Pow(0.95, 72) <= 0.025 && math.Pow(0.95, 71) > 0.025) {
		t.Fatal("reference thresholds")
	}
	for _, tc := range []struct {
		caught int
		want   bool
	}{{72, true}, {71, false}} {
		doc := runShadow(t, gateWindow(tc.caught))
		if len(doc.Candidates) != 2 {
			t.Fatalf("candidates %+v", doc.Candidates)
		}
		for _, c := range doc.Candidates {
			if c.Candidate != tc.want || c.Criteria[4].Held != tc.want || !near(c.PValue, round6(math.Pow(0.95, float64(tc.caught)))) {
				t.Fatalf("%d caught: %+v", tc.caught, c)
			}
			for _, cr := range c.Criteria {
				if cr.Name != "holm-bound" && !cr.Held {
					t.Fatalf("%d caught: criterion %+v failed", tc.caught, cr)
				}
			}
		}
	}
	// The second Holm step compares with alpha/1: one candidate at 0.95^59 and one at 0.95^72.
	w := gateWindow(72)
	for i := range w.Entries {
		e := &w.Entries[i]
		var keep []string
		for _, m := range e.Hosted.MutantsCaught["t2"] {
			if m < "t2-m059" {
				keep = append(keep, m)
			}
		}
		e.Hosted.MutantsCaught["t2"] = keep
	}
	doc := runShadow(t, w)
	if !doc.Candidates[0].Candidate || !doc.Candidates[1].Candidate || !strings.Contains(doc.Candidates[1].Criteria[4].Evidence, "rank 2 of 2") {
		t.Fatalf("Holm step-down: %+v", doc.Candidates)
	}
	// A declared gate other than the frozen one fails the window criterion.
	w = gateWindow(72)
	w.Gate.MinEntries = 3
	if c := runShadow(t, w).Candidates[0]; c.Candidate || c.Criteria[5].Held {
		t.Fatalf("tampered gate: %+v", c)
	}
	// One compiled miss of a seeded regression t1 caught fails parity for t1.
	w = gateWindow(72)
	w.Entries[0].Scenarios[0].KilledMutants = w.Entries[0].Scenarios[0].KilledMutants[1:]
	if c := runShadow(t, w).Candidates[0]; c.Candidate || c.Criteria[2].Held {
		t.Fatalf("parity: %+v", c)
	}
}

// CCE-V0-011: usefulness, minutes and cost appear per change and in aggregate, with median and p95
// wall-clock per side.
func TestCCEV0011ShadowUsefulnessMinutesCost(t *testing.T) {
	doc := runShadow(t, gateWindow(30))
	e := doc.Entries[0]
	if !e.Useful || e.CompiledMinutes != 1 || e.HostedMinutes != 10 || e.Compiled.CostUSD != 0.05 || e.Hosted.CostUSD != 0.5 {
		t.Fatalf("entry %+v", e)
	}
	tot := doc.Totals
	if tot.Usefulness.Numerator != 30 || tot.Usefulness.Denominator != 30 || tot.Compiled.WallMSMedia != 74000 || tot.Compiled.WallMSP95 != 88000 ||
		math.Abs(tot.Compiled.CostUSD-1.5) > 1e-9 || math.Abs(tot.Hosted.CostUSD-15) > 1e-9 || math.Abs(tot.Compiled.Minutes-37.25) > 1e-6 {
		t.Fatalf("totals %+v", tot)
	}
}

// CCE-V0-037 (V1-1713): a no-op diff whose admitted scenario passes at base and head reads
// PRESERVES; it never counts as useful, while the seeded mutants it kills still count as caught.
func TestCCEV0037NoOpPreservesNotUseful(t *testing.T) {
	w := gateWindow(30)
	w.Entries[0].Scenarios[0].Verdict = VerdictPreserves
	doc := runShadow(t, w)
	if e := doc.Entries[0]; e.Useful || len(e.Misses) != 0 {
		t.Fatalf("no-op entry %+v", e)
	}
	if u := doc.Totals.Usefulness; u.Numerator != 29 || u.Denominator != 30 {
		t.Fatalf("usefulness %+v", u)
	}
	w.Entries[0].Scenarios = append(w.Entries[0].Scenarios, eligibleScenario())
	if !runShadow(t, w).Entries[0].Useful {
		t.Fatal("a DISCRIMINATES scenario beside PRESERVES must keep the entry useful")
	}
}
