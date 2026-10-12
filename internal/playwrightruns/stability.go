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

// RunSummary names one admitted report by its 1-based argument position. Shard and RepeatEach
// appear only when the report declares a shard or a repeatEach above 1 (PWR-V0-013, PWR-V0-019).
type RunSummary struct {
	Run               int            `json:"run"`
	Sha256            string         `json:"sha256"`
	PlaywrightVersion string         `json:"playwrightVersion"`
	IdentityInputs    string         `json:"identityInputs"` // DECLARED | NOT_DECLARED
	Shard             *Shard         `json:"shard,omitempty"`
	RepeatEach        map[string]int `json:"repeatEach,omitempty"`
}

// presentedRepeatEach is a run's repeatEach declaration as emitted, secret-screened; nil when empty.
func presentedRepeatEach(run *Run) map[string]int {
	if len(run.RepeatEach) == 0 {
		return nil
	}
	out := map[string]int{}
	for p, n := range run.RepeatEach {
		out[screenText(p)] = n
	}
	return out
}

// StabilityEntry is one test under one set of identity inputs, or the observations of one test
// that could not be pooled for one reason (Inputs is then null). Runs names each observation's
// logical run by its first report's position; Repeats, present only when an observation comes from
// a run that declares repeatEach above 1, is each observation's 0-based repeat (PWR-V0-013).
type StabilityEntry struct {
	Identity
	Inputs      *PoolInputs `json:"inputs"`
	Runs        []int       `json:"runs"`
	Repeats     []int       `json:"repeats,omitempty"`
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
	Profile string       `json:"profile"`
	MinRuns int          `json:"minRuns"`
	Runs    []RunSummary `json:"runs"`
	// ShardSets lists every shard set, admitted or refused; absent when no report is a shard.
	ShardSets []ShardSet       `json:"shardSets,omitempty"`
	Tests     []StabilityEntry `json:"tests"`
	Summary   StabilitySummary `json:"summary"`
}

type observation struct {
	run          int
	repeat       int
	repeated     bool // the observation's run declares repeatEach above 1 for its project
	outcome      string
	firstError   string
	expectedFail bool
}

// Stability pools each test's observations across runs whose identity inputs match and classifies
// every pool (PWR-V0-003): differing outcomes, or a flaky run, are INTERMITTENT at any run count;
// identical outcomes are DETERMINISTIC only at minRuns or more observations, otherwise
// INSUFFICIENT. Each repeat of a repeatEach run is one observation (PWR-V0-011), and the shard
// reports of one logical run count as one run; a refused shard set's tests never pool (PWR-V0-019).
func Stability(runs []*Run, minRuns int) StabilityReport {
	report := StabilityReport{Profile: StabilityProfile, MinRuns: minRuns, Runs: []RunSummary{}, Tests: []StabilityEntry{}}
	type bucket struct {
		entry *StabilityEntry
		obs   []observation
	}
	buckets := map[string]*bucket{}
	positions := make([]int, len(runs))
	for i, run := range runs {
		positions[i] = i + 1
		declared := "NOT_DECLARED"
		if run.InputsDeclared {
			declared = "DECLARED"
		}
		report.Runs = append(report.Runs, RunSummary{Run: i + 1, Sha256: run.Sha256, PlaywrightVersion: screenText(run.Version), IdentityInputs: declared,
			Shard: run.Shard, RepeatEach: presentedRepeatEach(run)})
	}
	logical, sets := logicalRuns(runs, positions)
	if len(sets) > 0 {
		report.ShardSets = sets
	}
	for _, run := range logical {
		for _, t := range run.Tests {
			o := observation{run: run.index, repeat: t.Repeat, repeated: run.repeats(t.Project) > 1, outcome: outcomeOf(t), expectedFail: t.ExpectedFail}
			for _, r := range t.Results {
				if r.FirstError != "" {
					o.firstError = r.FirstError
					break
				}
			}
			inputs, reason := poolInputs(run.Run, t)
			if run.refusal != "" {
				inputs, reason = nil, run.refusal
			}
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
	expectedFail, repeated := false, false
	repeats := []int{}
	for _, o := range obs {
		out.Runs = append(out.Runs, o.run)
		repeats = append(repeats, o.repeat)
		repeated = repeated || o.repeated
		out.Outcomes = append(out.Outcomes, o.outcome)
		distinct[o.outcome] = true
		expectedFail = expectedFail || o.expectedFail
		if o.firstError != "" && !errs[o.firstError] {
			errs[o.firstError] = true
			out.FirstErrors = append(out.FirstErrors, o.firstError)
		}
	}
	if repeated {
		out.Repeats = repeats
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
