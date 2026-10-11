package playwrightruns

import (
	"encoding/json"
	"sort"
)

// Stability profile, bounds and vocabulary (PWR-V0-003, PWR-V0-004).
const (
	StabilityProfile = "corvint-runs-stability/0"
	DefaultMinRuns   = 3
	MinMinRuns       = 2
	MaxRuns          = 64

	Deterministic = "DETERMINISTIC"
	Intermittent  = "INTERMITTENT"
	Insufficient  = "INSUFFICIENT"

	ReasonBelowThreshold             = "BELOW_THRESHOLD"
	ReasonInputsMissing              = "IDENTITY_INPUT_MISSING"
	ReasonAmbiguous                  = "IDENTITY_AMBIGUOUS"
	ReasonInterrupted                = "RUN_INTERRUPTED"
	ReasonNotRun                     = "NOT_RUN"
	FlagExpectedFailNotDeterministic = "EXPECTED_FAIL_NOT_DETERMINISTIC"
)

// expectedFailSuggestions are the alternatives to an expected-fail annotation that does not
// reproduce every time (PWR-V0-004).
var expectedFailSuggestions = []string{
	"replace test.fail() with test.fixme() or test.skip() naming the defect id",
	"fix the setup that makes the outcome vary at identical inputs",
}

// PoolInputs are the identity inputs that must match for runs to be pooled (PWR-V0-002).
type PoolInputs struct {
	PlaywrightVersion   string `json:"playwrightVersion"`
	ApplicationRevision string `json:"applicationRevision"`
	ConfigDigest        string `json:"configDigest"`
	TestFileBlob        string `json:"testFileBlob"`
}

// presented is the inputs as emitted, secret-screened; nil stays nil.
func (in *PoolInputs) presented() *PoolInputs {
	if in == nil {
		return nil
	}
	return &PoolInputs{PlaywrightVersion: screenText(in.PlaywrightVersion), ApplicationRevision: screenText(in.ApplicationRevision),
		ConfigDigest: screenText(in.ConfigDigest), TestFileBlob: screenText(in.TestFileBlob)}
}

// RunSummary names one admitted report by its 1-based argument position.
type RunSummary struct {
	Run               int    `json:"run"`
	Sha256            string `json:"sha256"`
	PlaywrightVersion string `json:"playwrightVersion"`
	IdentityInputs    string `json:"identityInputs"` // DECLARED | NOT_DECLARED
}

// StabilityEntry is one test under one set of identity inputs, or the observations of one test
// that could not be pooled for one reason (Inputs is then null).
type StabilityEntry struct {
	Identity
	Inputs      *PoolInputs `json:"inputs"`
	Runs        []int       `json:"runs"`
	Outcomes    []string    `json:"outcomes"`
	Verdict     string      `json:"verdict"`
	Reason      string      `json:"reason"`
	Flags       []string    `json:"flags"`
	FirstErrors []string    `json:"firstErrors"`
	Suggestions []string    `json:"suggestions"`
}

// StabilitySummary counts entries by verdict and flag.
type StabilitySummary struct {
	Deterministic                int `json:"deterministic"`
	Intermittent                 int `json:"intermittent"`
	Insufficient                 int `json:"insufficient"`
	ExpectedFailNotDeterministic int `json:"expectedFailNotDeterministic"`
}

// StabilityReport is the `corvint runs stability` document.
type StabilityReport struct {
	Profile string           `json:"profile"`
	MinRuns int              `json:"minRuns"`
	Runs    []RunSummary     `json:"runs"`
	Tests   []StabilityEntry `json:"tests"`
	Summary StabilitySummary `json:"summary"`
}

type observation struct {
	run          int
	outcome      string
	firstError   string
	expectedFail bool
}

// Stability pools each test's observations across runs whose identity inputs match and classifies
// every pool (PWR-V0-003): differing outcomes, or a flaky run, are INTERMITTENT at any run count;
// identical outcomes are DETERMINISTIC only at minRuns or more runs, otherwise INSUFFICIENT.
func Stability(runs []*Run, minRuns int) StabilityReport {
	report := StabilityReport{Profile: StabilityProfile, MinRuns: minRuns, Runs: []RunSummary{}, Tests: []StabilityEntry{}}
	type bucket struct {
		entry *StabilityEntry
		obs   []observation
	}
	buckets := map[string]*bucket{}
	for i, run := range runs {
		index := i + 1
		declared := "NOT_DECLARED"
		if run.InputsDeclared {
			declared = "DECLARED"
		}
		report.Runs = append(report.Runs, RunSummary{Run: index, Sha256: run.Sha256, PlaywrightVersion: screenText(run.Version), IdentityInputs: declared})
		for _, t := range run.Tests {
			o := observation{run: index, outcome: outcomeOf(t), expectedFail: t.ExpectedFail}
			for _, r := range t.Results {
				if r.FirstError != "" {
					o.firstError = r.FirstError
					break
				}
			}
			inputs, reason := poolInputs(run, t)
			raw, _ := json.Marshal(inputs)
			key := t.Key() + "\x00" + reason + "\x00" + string(raw)
			b := buckets[key]
			if b == nil {
				b = &bucket{entry: &StabilityEntry{Identity: t.Identity.presented(), Inputs: inputs.presented(), Reason: reason}}
				buckets[key] = b
			}
			b.obs = append(b.obs, o)
		}
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		e := classify(buckets[k].entry, buckets[k].obs, minRuns)
		switch e.Verdict {
		case Deterministic:
			report.Summary.Deterministic++
		case Intermittent:
			report.Summary.Intermittent++
		default:
			report.Summary.Insufficient++
		}
		if len(e.Flags) > 0 {
			report.Summary.ExpectedFailNotDeterministic++
		}
		report.Tests = append(report.Tests, e)
	}
	return report
}

// poolInputs returns the inputs a test's observation pools under, or the reason it cannot pool.
func poolInputs(run *Run, t Test) (*PoolInputs, string) {
	switch {
	case run.Ambiguous[t.Key()]:
		return nil, ReasonAmbiguous
	case !t.Executed:
		return nil, ReasonNotRun
	}
	for _, r := range t.Results {
		if r.Status == "interrupted" {
			return nil, ReasonInterrupted
		}
	}
	in := &PoolInputs{PlaywrightVersion: run.Version, ApplicationRevision: run.Inputs.ApplicationRevision,
		ConfigDigest: run.Inputs.ConfigDigest, TestFileBlob: run.Inputs.TestFileBlobs[t.File]}
	if !run.InputsDeclared || in.ApplicationRevision == "" || in.ConfigDigest == "" || in.TestFileBlob == "" {
		return nil, ReasonInputsMissing
	}
	return in, ""
}

// outcomeOf is one run's outcome for a test: "flaky" when Playwright saw it fail then pass within
// the run, else its last result's status.
func outcomeOf(t Test) string {
	if t.Status == "flaky" {
		return "flaky"
	}
	if !t.Executed {
		return "notRun"
	}
	return t.Results[len(t.Results)-1].Status
}

func classify(e *StabilityEntry, obs []observation, minRuns int) StabilityEntry {
	out := *e
	out.Runs, out.Outcomes, out.Flags, out.FirstErrors, out.Suggestions = []int{}, []string{}, []string{}, []string{}, []string{}
	distinct, errs := map[string]bool{}, map[string]bool{}
	expectedFail := false
	for _, o := range obs {
		out.Runs = append(out.Runs, o.run)
		out.Outcomes = append(out.Outcomes, o.outcome)
		distinct[o.outcome] = true
		expectedFail = expectedFail || o.expectedFail
		if o.firstError != "" && !errs[o.firstError] {
			errs[o.firstError] = true
			out.FirstErrors = append(out.FirstErrors, o.firstError)
		}
	}
	switch {
	case out.Reason != "":
		out.Verdict = Insufficient
	case len(distinct) > 1 || distinct["flaky"]:
		out.Verdict = Intermittent
	case len(obs) >= minRuns:
		out.Verdict = Deterministic
	default:
		out.Verdict, out.Reason = Insufficient, ReasonBelowThreshold
	}
	if out.Verdict == Intermittent && expectedFail {
		out.Flags = []string{FlagExpectedFailNotDeterministic}
		out.Suggestions = append(out.Suggestions, expectedFailSuggestions...)
	}
	return out
}
