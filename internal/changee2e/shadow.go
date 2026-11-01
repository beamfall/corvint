package changee2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/Beamfall/corvint/internal/jsonend"
)

// Shadow schemas (CCE-V0-011).
const (
	ShadowWindowSchema = "e2e-shadow-window/0"
	ShadowSchema       = "e2e-shadow/0"
)

// Entry types (Shadow evaluation and retirement).
const (
	EntryMerged = "merged"
	EntryBugFix = "bug-fix"
	EntryRedCI  = "red-ci"
)

// ShadowGate is the frozen V0 gate. Only an owner decision changes these values; a window that
// declares other values fails the window criterion (CCE-V0-012).
type ShadowGate struct {
	MinEntries          int     `json:"min_entries"`
	MinBugFix           int     `json:"min_bug_fix"`
	CandidateMinChanges int     `json:"candidate_min_changes"`
	CandidateMinCaught  int     `json:"candidate_min_caught"`
	MissRateBound       float64 `json:"miss_rate_bound"`
	Alpha               float64 `json:"alpha"`
	TotalParity         float64 `json:"total_parity"`
}

// FrozenShadowGate holds the gate values of Shadow evaluation and retirement.
var FrozenShadowGate = ShadowGate{MinEntries: 30, MinBugFix: 10, CandidateMinChanges: 5, CandidateMinCaught: 1,
	MissRateBound: 0.05, Alpha: 0.05, TotalParity: 0.95}

// ShadowWindow is the frozen, digest-pinned input of `corvint-test e2e shadow`. It carries recorded
// results; shadow computes counts and criteria and never runs either side.
type ShadowWindow struct {
	Schema       string             `json:"schema"`
	Registration ShadowRegistration `json:"registration"`
	Gate         ShadowGate         `json:"gate"`
	HostedTests  []HostedTest       `json:"hosted_tests"`
	Entries      []ShadowEntry      `json:"entries"`
	// Truncated is set when the window was assembled from a capped document; shadow refuses it
	// with `e2e-input-truncated` (CCE-V0-014).
	Truncated int `json:"truncated,omitempty"`
}

// ShadowRegistration is pinned before the first run (CCE-V0-011).
type ShadowRegistration struct {
	Operators       []string `json:"operators"`
	Seed            string   `json:"seed"`
	CatalogDigest   string   `json:"catalog_digest"`
	CompilerVersion string   `json:"compiler_version"`
}

// HostedTest names a hosted test and the outcome blocks it asserts.
type HostedTest struct {
	ID                 string   `json:"id"`
	AssertedOutcomes   []string `json:"asserted_outcomes"`
	UnadmittedOutcomes []string `json:"unadmitted_outcomes"`
}

// ShadowEntry is one window change with both sides' recorded results.
type ShadowEntry struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Base         string           `json:"base"`
	Head         string           `json:"head"`
	TestRevision string           `json:"test_revision"`
	Obligations  map[string]int   `json:"obligations"`
	Mutants      []string         `json:"mutants"`
	Scenarios    []ShadowScenario `json:"scenarios"`
	Hosted       HostedSide       `json:"hosted"`
	Compiled     SideCost         `json:"compiled_cost"`
}

// ShadowScenario is one compiled scenario's unmutated verdict and the targets it caught: the
// mutants it passed without and failed with at its outcome assertion in every repeat, and the real
// failures it read REGRESSION_DETECTED for.
type ShadowScenario struct {
	Obligation    string   `json:"obligation"`
	Verdict       string   `json:"verdict"`
	OracleOrigin  string   `json:"oracle_origin"`
	KilledMutants []string `json:"killed_mutants"`
	RealFailures  []string `json:"real_failures"`
}

// HostedSide is the hosted affected selection's recorded result for one entry.
type HostedSide struct {
	TestsRun      []string            `json:"tests_run"`
	MutantsCaught map[string][]string `json:"mutants_caught"`
	RealFailures  []string            `json:"real_failures"`
	Cost          SideCost            `json:"cost"`
}

// SideCost is one side's measured time and cost for one entry, on the same host.
type SideCost struct {
	WallMS  int64   `json:"wall_ms"`
	CPUMS   int64   `json:"cpu_ms"`
	CostUSD float64 `json:"cost_usd"`
}

// Rate prints its numerator, denominator and one-sided 95% Clopper-Pearson bound (CCE-V0-011).
type Rate struct {
	Numerator   int     `json:"numerator"`
	Denominator int     `json:"denominator"`
	Bound       float64 `json:"bound"`
	Side        string  `json:"side"`
}

type sideTotals struct {
	WallMSMedian int64   `json:"wall_ms_median"`
	WallMSP95    int64   `json:"wall_ms_p95"`
	CPUMSMedian  int64   `json:"cpu_ms_median"`
	CPUMSP95     int64   `json:"cpu_ms_p95"`
	Minutes      float64 `json:"minutes"`
	CostUSD      float64 `json:"cost_usd"`
}

type entryReport struct {
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	Obligations     map[string]int `json:"obligations"`
	Verdicts        map[string]int `json:"scenarios_by_verdict"`
	Origins         map[string]int `json:"scenarios_by_oracle_origin"`
	HostedTestsRun  int            `json:"hosted_tests_run"`
	Mutants         int            `json:"seeded_regressions"`
	CompiledCaught  int            `json:"seeded_caught_compiled"`
	HostedCaught    int            `json:"seeded_caught_hosted"`
	RealFailures    int            `json:"real_failures"`
	Misses          []string       `json:"misses"`
	Escapes         []string       `json:"escapes"`
	Useful          bool           `json:"useful"`
	Compiled        SideCost       `json:"compiled"`
	Hosted          SideCost       `json:"hosted"`
	CompiledMinutes float64        `json:"compiled_minutes"`
	HostedMinutes   float64        `json:"hosted_minutes"`
}

type criterion struct {
	Name     string `json:"name"`
	Held     bool   `json:"held"`
	Evidence string `json:"evidence"`
}

type candidateReport struct {
	Test      string      `json:"test"`
	Candidate bool        `json:"candidate"`
	PValue    float64     `json:"p_value"`
	Criteria  []criterion `json:"criteria"`
}

type shadowReport struct {
	Schema       string             `json:"schema"`
	Window       string             `json:"window_digest"`
	Registration ShadowRegistration `json:"registration"`
	Gate         ShadowGate         `json:"frozen_gate"`
	Entries      []entryReport      `json:"entries"`
	Totals       shadowTotals       `json:"totals"`
	Candidates   []candidateReport  `json:"candidates"`
	Limitations  []string           `json:"limitations"`
	Truncated    int                `json:"truncated,omitempty"` // entries dropped by the byte cap (CCE-V0-014)
}

type shadowTotals struct {
	Entries        map[string]int `json:"entries_by_type"`
	Obligations    map[string]int `json:"obligations"`
	Verdicts       map[string]int `json:"scenarios_by_verdict"`
	Origins        map[string]int `json:"scenarios_by_oracle_origin"`
	HostedTestsRun int            `json:"hosted_tests_run"`
	Usefulness     Rate           `json:"usefulness"`
	SeededParity   Rate           `json:"seeded_parity"`
	MissRate       Rate           `json:"miss_rate"`
	EntryMissRate  Rate           `json:"entry_miss_rate"`
	Escapes        int            `json:"escapes"`
	RealFailures   int            `json:"real_failures"`
	CompiledExtra  int            `json:"seeded_caught_compiled_only"`
	Compiled       sideTotals     `json:"compiled"`
	Hosted         sideTotals     `json:"hosted"`
}

var shadowLimitations = []string{
	"shadow reads recorded results from the window; it does not re-run either side or re-verify the proofs behind each scenario's killed mutants",
	"every red-ci commit in the period is assumed included; the window cannot prove completeness",
	"mutants of one entry are not independent; entry_miss_rate gives the bound with entries as the unit",
}

// Shadow reads a frozen window and emits the CCE-V0-011/012 report. A window without the pinned
// operator set, seed, catalog digest and compiler version refuses `e2e-shadow-unregistered`.
func Shadow(window []byte) ([]byte, error) {
	var w ShadowWindow
	dec := json.NewDecoder(bytes.NewReader(window))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil || !jsonend.Done(dec) {
		return nil, refuse("e2e-shadow-window-invalid", "decode: %v", err)
	}
	if w.Truncated != 0 {
		return nil, refuse("e2e-input-truncated", "window dropped %d entries", w.Truncated)
	}
	if w.Schema != ShadowWindowSchema {
		return nil, refuse("e2e-shadow-window-invalid", "schema %q", w.Schema)
	}
	r := w.Registration
	if len(r.Operators) == 0 || slices.Contains(r.Operators, "") || r.Seed == "" || r.CatalogDigest == "" || r.CompilerVersion == "" {
		return nil, refuse("e2e-shadow-unregistered", "registration must pin operators, seed, catalog_digest and compiler_version")
	}
	if err := w.validate(); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(window)
	rep := shadowReport{Schema: ShadowSchema, Window: "sha256:" + hex.EncodeToString(sum[:]), Registration: r, Gate: FrozenShadowGate,
		Limitations: shadowLimitations}
	rep.Totals = shadowTotals{Entries: map[string]int{}, Obligations: map[string]int{}, Verdicts: map[string]int{}, Origins: map[string]int{}}
	t := &rep.Totals
	var cw, cc, hw, hc []int64
	useful, hostedCaught, compiledOfHosted, entriesWithHosted, entriesMissed := 0, 0, 0, 0, 0
	for _, e := range w.Entries {
		er := e.report()
		rep.Entries = append(rep.Entries, er)
		t.Entries[e.Type]++
		addCounts(t.Obligations, er.Obligations)
		addCounts(t.Verdicts, er.Verdicts)
		addCounts(t.Origins, er.Origins)
		t.HostedTestsRun += er.HostedTestsRun
		t.Escapes += len(er.Escapes)
		t.RealFailures += er.RealFailures
		if er.Useful {
			useful++
		}
		hostedSet := e.hostedMutants()
		caught := e.compiledCaught()
		for m := range caught {
			if !hostedSet[m] {
				t.CompiledExtra++
			}
		}
		hostedCaught += len(hostedSet)
		for m := range hostedSet {
			if caught[m] && !e.unknown() {
				compiledOfHosted++
			}
		}
		if len(hostedSet)+er.RealFailures > 0 {
			entriesWithHosted++
			if len(er.Misses) > 0 {
				entriesMissed++
			}
		}
		cw, cc = append(cw, e.Compiled.WallMS), append(cc, e.Compiled.CPUMS)
		hw, hc = append(hw, e.Hosted.Cost.WallMS), append(hc, e.Hosted.Cost.CPUMS)
		t.Compiled.Minutes += er.CompiledMinutes
		t.Hosted.Minutes += er.HostedMinutes
		t.Compiled.CostUSD += e.Compiled.CostUSD
		t.Hosted.CostUSD += e.Hosted.Cost.CostUSD
	}
	t.Usefulness = lowerRate(useful, len(w.Entries))
	t.SeededParity = lowerRate(compiledOfHosted, hostedCaught)
	t.MissRate = upperRate(hostedCaught-compiledOfHosted, hostedCaught)
	t.EntryMissRate = upperRate(entriesMissed, entriesWithHosted)
	t.Compiled.WallMSMedian, t.Compiled.WallMSP95, t.Compiled.CPUMSMedian, t.Compiled.CPUMSP95 = quantile(cw, 50), quantile(cw, 95), quantile(cc, 50), quantile(cc, 95)
	t.Hosted.WallMSMedian, t.Hosted.WallMSP95, t.Hosted.CPUMSMedian, t.Hosted.CPUMSP95 = quantile(hw, 50), quantile(hw, 95), quantile(hc, 50), quantile(hc, 95)
	rep.Candidates = w.candidates(t)
	return capList(&rep.Entries, &rep.Truncated, func() ([]byte, error) { return encode(rep) })
}

func (w *ShadowWindow) validate() error {
	bad := func(format string, args ...any) error { return refuse("e2e-shadow-window-invalid", format, args...) }
	tests := map[string]bool{}
	for _, h := range w.HostedTests {
		if h.ID == "" || tests[h.ID] {
			return bad("hosted test id %q empty or duplicated", h.ID)
		}
		tests[h.ID] = true
	}
	entries, mutants := map[string]bool{}, map[string]bool{}
	for _, e := range w.Entries {
		if e.ID == "" || entries[e.ID] {
			return bad("entry id %q empty or duplicated", e.ID)
		}
		entries[e.ID] = true
		if e.Type != EntryMerged && e.Type != EntryBugFix && e.Type != EntryRedCI {
			return bad("entry %s: type %q", e.ID, e.Type)
		}
		if e.Base == "" || e.Head == "" || e.TestRevision == "" {
			return bad("entry %s: base, head and test_revision are required", e.ID)
		}
		own := map[string]bool{}
		for _, m := range e.Mutants {
			if m == "" || mutants[m] {
				return bad("entry %s: mutant id %q empty or duplicated in the window", e.ID, m)
			}
			mutants[m], own[m] = true, true
		}
		ran := map[string]bool{}
		for _, id := range e.Hosted.TestsRun {
			if !tests[id] {
				return bad("entry %s: hosted test %q is not declared", e.ID, id)
			}
			ran[id] = true
		}
		for id, ms := range e.Hosted.MutantsCaught {
			if !ran[id] {
				return bad("entry %s: hosted test %q caught mutants without running", e.ID, id)
			}
			for _, m := range ms {
				if !own[m] {
					return bad("entry %s: mutant %q is not seeded in this entry", e.ID, m)
				}
			}
		}
		for _, id := range e.Hosted.RealFailures {
			if !ran[id] {
				return bad("entry %s: real failure %q did not run", e.ID, id)
			}
		}
		for _, s := range e.Scenarios {
			for _, m := range s.KilledMutants {
				if !own[m] {
					return bad("entry %s: scenario %s killed unseeded mutant %q", e.ID, s.Obligation, m)
				}
			}
			for _, id := range s.RealFailures {
				if !slices.Contains(e.Hosted.RealFailures, id) {
					return bad("entry %s: scenario %s names %q, not a real failure of the entry", e.ID, s.Obligation, id)
				}
			}
		}
	}
	return nil
}

// eligible reports whether a scenario may score a catch (CCE-V0-011).
func (s ShadowScenario) eligible() bool {
	switch s.Verdict {
	case VerdictDiscriminates, VerdictNewReach, VerdictRegressionDetected, VerdictPreserves:
		return s.OracleOrigin == OriginAdmittedAtBase
	}
	return false
}

func (e ShadowEntry) unknown() bool { return e.Obligations["UNKNOWN"] > 0 }

// compiledCaught is the set of mutants an eligible scenario caught; an UNKNOWN entry catches none.
func (e ShadowEntry) compiledCaught() map[string]bool {
	out := map[string]bool{}
	if e.unknown() {
		return out
	}
	for _, s := range e.Scenarios {
		if s.eligible() {
			for _, m := range s.KilledMutants {
				out[m] = true
			}
		}
	}
	return out
}

func (e ShadowEntry) compiledRealCaught() map[string]bool {
	out := map[string]bool{}
	if e.unknown() {
		return out
	}
	for _, s := range e.Scenarios {
		if s.eligible() && s.Verdict == VerdictRegressionDetected {
			for _, id := range s.RealFailures {
				out[id] = true
			}
		}
	}
	return out
}

func (e ShadowEntry) hostedMutants() map[string]bool {
	out := map[string]bool{}
	for _, ms := range e.Hosted.MutantsCaught {
		for _, m := range ms {
			out[m] = true
		}
	}
	return out
}

func (e ShadowEntry) report() entryReport {
	er := entryReport{ID: e.ID, Type: e.Type, Obligations: map[string]int{}, Verdicts: map[string]int{}, Origins: map[string]int{},
		HostedTestsRun: len(e.Hosted.TestsRun), Mutants: len(e.Mutants), RealFailures: len(e.Hosted.RealFailures),
		Compiled: e.Compiled, Hosted: e.Hosted.Cost, CompiledMinutes: minutes(e.Compiled.WallMS), HostedMinutes: minutes(e.Hosted.Cost.WallMS),
		Misses: []string{}, Escapes: []string{}}
	addCounts(er.Obligations, e.Obligations)
	for _, s := range e.Scenarios {
		er.Verdicts[s.Verdict]++
		er.Origins[s.OracleOrigin]++
		// PRESERVES passes at base and head, so it cannot tell a refactor from a no-op diff and is
		// never usefulness evidence; it still scores the mutants it kills (CCE-V0-037).
		er.Useful = er.Useful || (s.eligible() && s.Verdict != VerdictPreserves)
	}
	er.Useful = er.Useful && !e.unknown()
	caught, hosted := e.compiledCaught(), e.hostedMutants()
	er.CompiledCaught, er.HostedCaught = len(caught), len(hosted)
	for m := range hosted {
		if !caught[m] {
			er.Misses = append(er.Misses, "mutant:"+m)
		}
	}
	realCaught := e.compiledRealCaught()
	for _, id := range e.Hosted.RealFailures {
		if !realCaught[id] {
			target := "failure:" + e.ID + "/" + id
			er.Misses = append(er.Misses, target)
			er.Escapes = append(er.Escapes, target)
		}
	}
	sort.Strings(er.Misses)
	sort.Strings(er.Escapes)
	return er
}

// candidates evaluates CCE-V0-012 for every hosted test. The hypothesis family is every hosted
// test that caught at least one seeded regression in the window.
func (w *ShadowWindow) candidates(t *shadowTotals) []candidateReport {
	g := FrozenShadowGate
	windowOK := w.Gate == g && len(w.Entries) >= g.MinEntries && t.Entries[EntryBugFix] >= g.MinBugFix && t.SeededParity.Denominator > 0 &&
		float64(t.SeededParity.Numerator) >= g.TotalParity*float64(t.SeededParity.Denominator)
	windowEvidence := fmt.Sprintf("entries %d (min %d), bug-fix %d (min %d), total seeded parity %d/%d (min %.2f), declared gate equals frozen gate: %v",
		len(w.Entries), g.MinEntries, t.Entries[EntryBugFix], g.MinBugFix, t.SeededParity.Numerator, t.SeededParity.Denominator, g.TotalParity, w.Gate == g)
	var out []candidateReport
	var family []int
	for _, h := range w.HostedTests {
		ran, caught, missed, riskEntries, escapes := 0, 0, 0, 0, 0
		for _, e := range w.Entries {
			if !slices.Contains(e.Hosted.TestsRun, h.ID) {
				continue
			}
			ran++
			if e.Type != EntryMerged {
				riskEntries++
			}
			compiled := e.compiledCaught()
			for _, m := range e.Hosted.MutantsCaught[h.ID] {
				caught++
				if !compiled[m] {
					missed++
				}
			}
			realCaught := e.compiledRealCaught()
			for _, id := range e.Hosted.RealFailures {
				if id == h.ID && !realCaught[id] {
					escapes++
				}
			}
		}
		p := binomialCDF(missed, caught, g.MissRateBound)
		c := candidateReport{Test: h.ID, PValue: round6(p), Criteria: []criterion{
			{"outcomes-admitted", len(h.AssertedOutcomes) > 0 && len(h.UnadmittedOutcomes) == 0,
				fmt.Sprintf("%d asserted outcome blocks, %d unadmitted", len(h.AssertedOutcomes), len(h.UnadmittedOutcomes))},
			{"candidate-evidence", ran >= g.CandidateMinChanges && caught >= g.CandidateMinCaught,
				fmt.Sprintf("ran in %d window changes (min %d), caught %d seeded regressions (min %d)", ran, g.CandidateMinChanges, caught, g.CandidateMinCaught)},
			{"seeded-parity", caught > 0 && missed == 0, fmt.Sprintf("compiled caught %d/%d of the seeded regressions it caught", caught-missed, caught)},
			{"zero-escapes", riskEntries > 0 && escapes == 0, fmt.Sprintf("%d escapes over %d bug-fix or red-ci entries it ran in", escapes, riskEntries)},
			{"holm-bound", false, ""},
			{"window-gate", windowOK, windowEvidence},
		}}
		if caught > 0 {
			family = append(family, len(out))
		} else {
			c.Criteria[4].Evidence = "no seeded regressions caught: denominator zero"
		}
		out = append(out, c)
	}
	// Holm: order the family by p-value and compare the k-th smallest with alpha/(m-k+1); stop at
	// the first hypothesis not rejected.
	sort.SliceStable(family, func(i, j int) bool { return out[family[i]].PValue < out[family[j]].PValue })
	m, stopped := len(family), false
	for k, i := range family {
		threshold := g.Alpha / float64(m-k)
		rejected := !stopped && out[i].PValue <= threshold
		stopped = stopped || !rejected
		out[i].Criteria[4] = criterion{"holm-bound", rejected,
			fmt.Sprintf("p=%.6f against %.6f (rank %d of %d), one-sided exact binomial H0 miss rate > %.2f", out[i].PValue, threshold, k+1, m, g.MissRateBound)}
	}
	for i := range out {
		out[i].Candidate = true
		for _, c := range out[i].Criteria {
			out[i].Candidate = out[i].Candidate && c.Held
		}
	}
	return out
}

func addCounts(dst, src map[string]int) {
	for k, v := range src {
		dst[k] += v
	}
}

func minutes(ms int64) float64 { return round6(float64(ms) / 60000) }

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

// quantile is the nearest-rank percentile of xs; 0 for an empty list.
func quantile(xs []int64, pct int) int64 {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	rank := int(math.Ceil(float64(pct) / 100 * float64(len(s))))
	return s[max(rank, 1)-1]
}

// binomialCDF is P(X <= x) for X ~ Binomial(n, p); 1 when n is 0.
func binomialCDF(x, n int, p float64) float64 {
	if x >= n {
		return 1
	}
	sum := 0.0
	for k := 0; k <= x; k++ {
		lg := lchoose(n, k) + float64(k)*math.Log(p) + float64(n-k)*math.Log1p(-p)
		sum += math.Exp(lg)
	}
	return math.Min(sum, 1)
}

func lchoose(n, k int) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}

// clopperPearsonUpper is the one-sided 95% upper bound on a rate with x of n: the p where
// P(X <= x | n, p) = 0.05. A zero denominator reads 1, never a vacuous 0.
func clopperPearsonUpper(x, n int) float64 {
	if n == 0 || x >= n {
		return 1
	}
	lo, hi := 0.0, 1.0
	for range 100 {
		mid := (lo + hi) / 2
		if binomialCDF(x, n, mid) > 0.05 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return round6(hi)
}

// clopperPearsonLower is the one-sided 95% lower bound; a zero denominator reads 0.
func clopperPearsonLower(x, n int) float64 {
	if n == 0 || x == 0 {
		return 0
	}
	return round6(1 - clopperPearsonUpper(n-x, n))
}

func upperRate(x, n int) Rate { return Rate{x, n, clopperPearsonUpper(x, n), "upper"} }
func lowerRate(x, n int) Rate { return Rate{x, n, clopperPearsonLower(x, n), "lower"} }
