package playwrightruns

import (
	"encoding/json"
	"slices"
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

	// Served-digest statements of a run (PWR-V0-055).
	ServedDigestDeclared  = "DECLARED"
	ServedDigestUnknown   = "UNKNOWN"
	ServedDigestDisagrees = "DISAGREES"

	// IdentityRemedy is the one per-run instruction for a report that declares no identity inputs
	// (PWR-V0-051).
	IdentityRemedy = "declare config.metadata.corvintRunIdentity: embed the corvintRunIdentity member printed by corvint runs identity --config FILE (PWR-V0-046)"
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
	// ConfigDigest and TestFileBlob are absent under --pool-by closure, which keys on
	// TestClosureRule and TestClosureDigest instead (PWR-V0-069).
	ConfigDigest string `json:"configDigest,omitempty"`
	TestFileBlob string `json:"testFileBlob,omitempty"`
	// ServedArtifactDigest is omitted when the run declares none (PWR-V0-055).
	ServedArtifactDigest string `json:"servedArtifactDigest,omitempty"`
	TestClosureRule      string `json:"testClosureRule,omitempty"`
	TestClosureDigest    string `json:"testClosureDigest,omitempty"`
}

// presented is the inputs as emitted, secret-screened; nil stays nil.
func (in *PoolInputs) presented() *PoolInputs {
	if in == nil {
		return nil
	}
	return &PoolInputs{PlaywrightVersion: screenText(in.PlaywrightVersion), ApplicationRevision: screenText(in.ApplicationRevision),
		ConfigDigest: screenText(in.ConfigDigest), TestFileBlob: screenText(in.TestFileBlob), ServedArtifactDigest: screenText(in.ServedArtifactDigest),
		TestClosureRule: in.TestClosureRule, TestClosureDigest: in.TestClosureDigest}
}

// RunSummary names one admitted report by its 1-based argument position. Shard and RepeatEach
// appear only when the report declares a shard or a repeatEach above 1 (PWR-V0-013, PWR-V0-019).
// UnpooledTests and Remedy appear only on an unsharded report that declares no identity inputs:
// its observations are counted here once instead of one IDENTITY_INPUT_MISSING entry per test
// (PWR-V0-051).
type RunSummary struct {
	Run               int            `json:"run"`
	Sha256            string         `json:"sha256"`
	PlaywrightVersion string         `json:"playwrightVersion"`
	IdentityInputs    string         `json:"identityInputs"` // DECLARED | NOT_DECLARED
	Shard             *Shard         `json:"shard,omitempty"`
	RepeatEach        map[string]int `json:"repeatEach,omitempty"`
	UnpooledTests     int            `json:"unpooledTests,omitempty"`
	Remedy            string         `json:"remedy,omitempty"`
	// ServedDigest is DECLARED, UNKNOWN or DISAGREES (PWR-V0-055).
	ServedDigest string `json:"servedDigest"`
	// Interrupted is true when any result of the report has status interrupted: the run was cut
	// short, and its unfinished tests are excluded observations (PWR-V0-058).
	Interrupted bool `json:"interrupted,omitempty"`
	// OutsideWindow is true when --last N left the report's logical run out of the window
	// (PWR-V0-063).
	OutsideWindow bool `json:"outsideWindow,omitempty"`
	// IdentitySource is EMBEDDED, SIDECAR or NOT_DECLARED, present only when the caller gave a
	// sidecar option (PWR-V0-067).
	IdentitySource string `json:"identitySource,omitempty"`
	// ClosureSource is DECLARED, COMPUTED (rule corvint-test-closure/0) or NOT_DECLARED, present
	// only under --pool-by closure (PWR-V0-069, PWR-V0-074).
	ClosureSource string `json:"closureSource,omitempty"`
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
	Inputs   *PoolInputs `json:"inputs"`
	Runs     []int       `json:"runs"`
	Repeats  []int       `json:"repeats,omitempty"`
	Outcomes []string    `json:"outcomes"`
	Verdict  string      `json:"verdict"`
	Reason   string      `json:"reason"`
	// MissingInputs names the undeclared identity inputs of an IDENTITY_INPUT_MISSING entry
	// across its observations (PWR-V0-053).
	MissingInputs []string `json:"missingInputs,omitempty"`
	// Excluded lists the observations of this test, at the entry's inputs, that did not complete
	// (RUN_INTERRUPTED or NOT_RUN): they are named here and never counted (PWR-V0-058).
	Excluded    []ExcludedObservation `json:"excluded,omitempty"`
	Flags       []string              `json:"flags"`
	FirstErrors []string              `json:"firstErrors"`
	Suggestions []string              `json:"suggestions"`
}

// ExcludedObservation is one observation a pooled entry names but does not count (PWR-V0-058).
// Repeat is present only when the observation's run declares repeatEach above 1.
type ExcludedObservation struct {
	Run    int    `json:"run"`
	Repeat *int   `json:"repeat,omitempty"`
	Reason string `json:"reason"`
}

// StabilitySummary counts entries by verdict and flag.
type StabilitySummary struct {
	Deterministic                int `json:"deterministic"`
	Intermittent                 int `json:"intermittent"`
	Insufficient                 int `json:"insufficient"`
	ExpectedFailNotDeterministic int `json:"expectedFailNotDeterministic"`
	// IdentityNotDeclaredRuns counts the reports whose tests were not pooled because the report
	// declares no identity inputs (PWR-V0-051).
	IdentityNotDeclaredRuns int `json:"identityNotDeclaredRuns,omitempty"`
	// ServedDigestUnknownRuns counts the reports that declare no servedArtifactDigest: their
	// revision is trusted without evidence of the build served (PWR-V0-055).
	ServedDigestUnknownRuns int `json:"servedDigestUnknownRuns,omitempty"`
	// ServedDigestDisagreeingRuns counts the reports whose served build differs from another
	// report's at the same applicationRevision (PWR-V0-055).
	ServedDigestDisagreeingRuns int `json:"servedDigestDisagreeingRuns,omitempty"`
	// InterruptedRuns counts the reports marked interrupted, and ExcludedObservations the
	// observations folded into a pooled entry as excluded (PWR-V0-058).
	InterruptedRuns      int `json:"interruptedRuns,omitempty"`
	ExcludedObservations int `json:"excludedObservations,omitempty"`
	// ClosureNotDeclaredRuns counts, under --pool-by closure, the reports that declare no closure
	// rule (PWR-V0-069).
	ClosureNotDeclaredRuns int `json:"closureNotDeclaredRuns,omitempty"`
}

// StabilityReport is the `corvint runs stability` document.
type StabilityReport struct {
	Profile string `json:"profile"`
	MinRuns int    `json:"minRuns"`
	// Last is the --last N window over logical runs by stats.startTime; 0 (absent) uses every run
	// (PWR-V0-063).
	Last int `json:"last,omitempty"`
	// Window names the rule applied to Last: "all" or "completed" (PWR-V0-175); absent without Last.
	Window string `json:"window,omitempty"`
	// WindowInterruptedRuns lists the 1-based reports of interrupted runs that took a slot of a
	// truncating "all" window and so displaced a completed run (PWR-V0-175).
	WindowInterruptedRuns []int `json:"windowInterruptedRuns,omitempty"`
	// PoolBy is "closure" under --pool-by closure, else absent (PWR-V0-069).
	PoolBy string       `json:"poolBy,omitempty"`
	Runs   []RunSummary `json:"runs"`
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
// An observation that did not complete (RUN_INTERRUPTED, NOT_RUN) at declared inputs is listed as
// excluded on the entry pooled at those inputs, so a test keeps one row; without such an entry it
// keeps its own INSUFFICIENT row (PWR-V0-058).
func Stability(runs []*Run, minRuns int) StabilityReport {
	return StabilityLast(runs, minRuns, 0)
}

// StabilityLast is Stability over the last logical runs by start time (PWR-V0-063): with last
// above 0, only the last logical runs ordered by stats.startTime (ties by argument position) pool,
// and the others' reports are marked outsideWindow. The caller must refuse reports without a start
// time before asking for a window; such a run would order first.
func StabilityLast(runs []*Run, minRuns, last int) StabilityReport {
	return StabilityPooled(runs, minRuns, last, PoolByInputs)
}

// Pool modes of runs stability (PWR-V0-069).
const (
	PoolByInputs  = "inputs"
	PoolByClosure = "closure"
)

// StabilityPooled is StabilityLast pooling by poolBy: PoolByInputs is the PWR-V0-002 key, and
// PoolByClosure keys on the declared per-test closure instead of the config digest and test-file
// blob (PWR-V0-069). The caller must refuse a contradicted closure first (ClosureContradiction).
func StabilityPooled(runs []*Run, minRuns, last int, poolBy string) StabilityReport {
	return StabilityWindowed(runs, minRuns, last, poolBy, WindowAll)
}

// Window rules of --last N (PWR-V0-175). WindowAll is the PWR-V0-063 window over every logical run;
// WindowCompleted skips interrupted logical runs before taking the last N.
const (
	WindowAll       = "all"
	WindowCompleted = "completed"
)

// StabilityWindowed is StabilityPooled with the --last window rule named: under WindowCompleted an
// interrupted logical run never takes a window slot and is marked outsideWindow, so a gate over the
// last N completed runs is expressible; under WindowAll the interrupted runs that kept a slot are
// listed in windowInterruptedRuns when the window truncated.
func StabilityWindowed(runs []*Run, minRuns, last int, poolBy, window string) StabilityReport {
	closure := poolBy == PoolByClosure
	report := StabilityReport{Profile: StabilityProfile, MinRuns: minRuns, Last: last, Runs: []RunSummary{}, Tests: []StabilityEntry{}}
	if closure {
		report.PoolBy = PoolByClosure
	}
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
			Shard: run.Shard, RepeatEach: presentedRepeatEach(run), ServedDigest: servedDigestState(run, runs), Interrupted: interrupted(run),
			IdentitySource: run.IdentitySource})
		if closure {
			report.Runs[i].ClosureSource = "NOT_DECLARED"
			switch {
			case !run.InputsDeclared || run.Inputs.TestClosureRule == "":
				report.Summary.ClosureNotDeclaredRuns++
			case run.Inputs.TestClosureRule == ClosureRuleComputed:
				report.Runs[i].ClosureSource = "COMPUTED"
			default:
				report.Runs[i].ClosureSource = run.Inputs.TestClosureRule
			}
		}
		if report.Runs[i].Interrupted {
			report.Summary.InterruptedRuns++
		}
		switch report.Runs[i].ServedDigest {
		case ServedDigestUnknown:
			report.Summary.ServedDigestUnknownRuns++
		case ServedDigestDisagrees:
			report.Summary.ServedDigestDisagreeingRuns++
		}
	}
	logical, sets := logicalRuns(runs, positions)
	if len(sets) > 0 {
		report.ShardSets = sets
	}
	if last > 0 {
		report.Window = window
		if report.Window == "" {
			report.Window = WindowAll
		}
	}
	if last > 0 && (len(logical) > last || window == WindowCompleted) {
		logical = lastWindow(logical, sets, last, report.Runs, window == WindowCompleted)
		if window != WindowCompleted {
			for _, run := range logical {
				if interrupted(run.Run) {
					report.WindowInterruptedRuns = append(report.WindowInterruptedRuns, run.index)
				}
			}
		}
	}
	// pending holds observations that did not complete at declared inputs until every pooled
	// entry exists (PWR-V0-058).
	type pendingObs struct {
		poolKey, ownKey string
		obs             observation
		identity        Identity
		reason          string
	}
	pending := []pendingObs{}
	for _, run := range logical {
		if run.refusal == "" && !run.Run.InputsDeclared && run.Shard == nil {
			// One statement for the whole report instead of one abstention per test (PWR-V0-051).
			summary := &report.Runs[run.index-1]
			summary.UnpooledTests, summary.Remedy = len(run.Tests), IdentityRemedy
			report.Summary.IdentityNotDeclaredRuns++
			continue
		}
		for _, t := range run.Tests {
			o := observation{run: run.index, repeat: t.Repeat, repeated: run.repeats(t.Project) > 1, outcome: outcomeOf(t), expectedFail: t.ExpectedFail}
			for _, r := range t.Results {
				if r.FirstError != "" {
					o.firstError = r.FirstError
					break
				}
			}
			inputs, reason := poolInputs(run.Run, t, closure)
			if run.refusal != "" {
				inputs, reason = nil, run.refusal
			}
			raw, _ := json.Marshal(inputs)
			key := t.Key() + "\x00" + reason + "\x00" + string(raw)
			if declared := declaredPoolInputs(run.Run, t, closure); run.refusal == "" && (reason == ReasonInterrupted || reason == ReasonNotRun) {
				// A test with no declared inputs (no closure digest) pools nowhere, so its
				// observations go to its INPUTS_MISSING row (PWR-V0-174).
				pooled, _ := json.Marshal(declared)
				poolKey := t.Key() + "\x00\x00" + string(pooled)
				if declared == nil {
					poolKey = t.Key() + "\x00" + ReasonInputsMissing + "\x00null"
				}
				pending = append(pending, pendingObs{poolKey: poolKey, ownKey: key, obs: o, identity: t.Identity, reason: reason})
				continue
			}
			b := buckets[key]
			if b == nil {
				b = &bucket{entry: &StabilityEntry{Identity: t.Identity.presented(), Inputs: inputs.presented(), Reason: reason}}
				buckets[key] = b
			}
			b.obs = append(b.obs, o)
			if reason == ReasonInputsMissing {
				for _, name := range missingInputs(run.Run, t, closure) {
					if !slices.Contains(b.entry.MissingInputs, name) {
						b.entry.MissingInputs = append(b.entry.MissingInputs, name)
					}
				}
				sort.Strings(b.entry.MissingInputs)
			}
		}
	}
	for _, p := range pending {
		if b := buckets[p.poolKey]; b != nil {
			ex := ExcludedObservation{Run: p.obs.run, Reason: p.reason}
			if p.obs.repeated {
				repeat := p.obs.repeat
				ex.Repeat = &repeat
			}
			b.entry.Excluded = append(b.entry.Excluded, ex)
			report.Summary.ExcludedObservations++
			continue
		}
		b := buckets[p.ownKey]
		if b == nil {
			b = &bucket{entry: &StabilityEntry{Identity: p.identity.presented(), Reason: p.reason}}
			buckets[p.ownKey] = b
		}
		b.obs = append(b.obs, p.obs)
		sort.SliceStable(b.obs, func(i, j int) bool { return b.obs[i].run < b.obs[j].run })
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
func poolInputs(run *Run, t Test, closure bool) (*PoolInputs, string) {
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
	if in := declaredPoolInputs(run, t, closure); in != nil {
		return in, ""
	}
	return nil, ReasonInputsMissing
}

// declaredPoolInputs is the inputs the run declares for the test, nil when any is missing. Under
// closure pooling the key is the declared closure in place of the config digest and test-file blob
// (PWR-V0-069); there is no fallback to the other key.
func declaredPoolInputs(run *Run, t Test, closure bool) *PoolInputs {
	if closure {
		in := &PoolInputs{PlaywrightVersion: run.Version, ApplicationRevision: run.Inputs.ApplicationRevision,
			ServedArtifactDigest: run.Inputs.ServedArtifactDigest, TestClosureRule: run.Inputs.TestClosureRule,
			TestClosureDigest: run.Inputs.TestClosureDigests[t.File]}
		if !run.InputsDeclared || in.ApplicationRevision == "" || in.TestClosureRule == "" || in.TestClosureDigest == "" {
			return nil
		}
		return in
	}
	in := &PoolInputs{PlaywrightVersion: run.Version, ApplicationRevision: run.Inputs.ApplicationRevision,
		ConfigDigest: run.Inputs.ConfigDigest, TestFileBlob: run.Inputs.TestFileBlobs[t.File], ServedArtifactDigest: run.Inputs.ServedArtifactDigest}
	if !run.InputsDeclared || in.ApplicationRevision == "" || in.ConfigDigest == "" || in.TestFileBlob == "" {
		return nil
	}
	return in
}

// ClosureContradiction finds two reports that declare the same closure rule and digest for one
// file but different non-empty test-file blobs for it (PWR-V0-070): a closure contains its test
// file, so one of the claims is false. run and conflictsWith are 1-based argument positions, the
// later report first; ok is false when there is none.
func ClosureContradiction(runs []*Run) (run, conflictsWith int, ok bool) {
	type claim struct {
		blob string
		run  int
	}
	seen := map[string]claim{}
	for i, r := range runs {
		if !r.InputsDeclared || r.Inputs.TestClosureRule == "" {
			continue
		}
		files := make([]string, 0, len(r.Inputs.TestClosureDigests))
		for file := range r.Inputs.TestClosureDigests {
			files = append(files, file)
		}
		sort.Strings(files)
		for _, file := range files {
			blob := r.Inputs.TestFileBlobs[file]
			if blob == "" {
				continue
			}
			key := r.Inputs.TestClosureRule + "\x00" + file + "\x00" + r.Inputs.TestClosureDigests[file]
			if prior, found := seen[key]; !found {
				seen[key] = claim{blob, i + 1}
			} else if prior.blob != blob {
				return i + 1, prior.run, true
			}
		}
	}
	return 0, 0, false
}

// interrupted reports whether any result of the report has status interrupted (PWR-V0-058).
func interrupted(run *Run) bool {
	for _, t := range run.Tests {
		for _, r := range t.Results {
			if r.Status == "interrupted" {
				return true
			}
		}
	}
	return false
}

// lastWindow keeps the last logical runs by start time, ties by argument position, in their
// original order, and marks every report of the others outsideWindow (PWR-V0-063).
func lastWindow(logical []logicalRun, sets []ShardSet, last int, summaries []RunSummary, completedOnly bool) []logicalRun {
	ordered := []logicalRun{}
	for _, run := range logical {
		if !completedOnly || !interrupted(run.Run) {
			ordered = append(ordered, run)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if !a.StartTime.Equal(b.StartTime) {
			return a.StartTime.Before(b.StartTime)
		}
		return a.index < b.index
	})
	keep := map[int]bool{}
	for _, run := range ordered[max(0, len(ordered)-last):] {
		keep[run.index] = true
	}
	kept := []logicalRun{}
	for _, run := range logical {
		if keep[run.index] {
			kept = append(kept, run)
			continue
		}
		reports := []int{run.index}
		if run.Shard != nil {
			for _, set := range sets {
				if set.Run == run.index {
					reports = set.Reports
				}
			}
		}
		for _, pos := range reports {
			summaries[pos-1].OutsideWindow = true
		}
	}
	return kept
}

// servedDigestState is a report's served-digest statement (PWR-V0-055): UNKNOWN without a
// declared servedArtifactDigest, DISAGREES when another report in the set declares a different one
// at the same applicationRevision (so at least one of them served a build other than the revision's),
// else DECLARED.
func servedDigestState(run *Run, all []*Run) string {
	if !run.InputsDeclared || run.Inputs.ServedArtifactDigest == "" {
		return ServedDigestUnknown
	}
	for _, o := range all {
		if o.InputsDeclared && o.Inputs.ServedArtifactDigest != "" && o.Inputs.ApplicationRevision == run.Inputs.ApplicationRevision &&
			o.Inputs.ServedArtifactDigest != run.Inputs.ServedArtifactDigest {
			return ServedDigestDisagrees
		}
	}
	return ServedDigestDeclared
}

// missingInputs names which declared identity inputs a test lacks, so an adopter can see why it
// did not pool: a declared report without testFileBlobs[file] usually names a file under a
// project testDir that `runs identity` was not told about (PWR-V0-053).
func missingInputs(run *Run, t Test, closure bool) []string {
	missing := []string{}
	if !run.InputsDeclared {
		return append(missing, "corvintRunIdentity")
	}
	if run.Inputs.ApplicationRevision == "" {
		missing = append(missing, "applicationRevision")
	}
	if closure {
		if run.Inputs.TestClosureRule == "" {
			missing = append(missing, "testClosureRule")
		}
		if run.Inputs.TestClosureDigests[t.File] == "" {
			missing = append(missing, "testClosureDigests["+screenText(t.File)+"]")
		}
		return missing
	}
	if run.Inputs.ConfigDigest == "" {
		missing = append(missing, "configDigest")
	}
	if run.Inputs.TestFileBlobs[t.File] == "" {
		missing = append(missing, "testFileBlobs["+screenText(t.File)+"]")
	}
	return missing
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
