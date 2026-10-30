package playwrightruns

import (
	"fmt"
	"reflect"
	"sort"
)

// Compare profile and vocabulary (PWR-V0-006 to PWR-V0-009).
const (
	CompareProfile = "corvint-runs-compare/0"

	Parity       = "PARITY"
	Diverged     = "DIVERGED"
	Incomparable = "INCOMPARABLE"
	// Undeployed means the served builds contradict the declared revisions (PWR-V0-056).
	Undeployed = "UNDEPLOYED"

	Same    = "SAME"
	Changed = "CHANGED"
	Missing = "MISSING"
	New     = "NEW"
	// AmbiguousTest withholds a test whose identity occurs twice in one report (PWR-V0-006).
	AmbiguousTest = "AMBIGUOUS"

	SideBaseline  = "baseline"
	SideCandidate = "candidate"

	// Overall reasons. The INCOMPARABLE ones come first.
	ReasonReportUnreadable   = "REPORT_UNREADABLE"
	ReasonVersionDiffers     = "PLAYWRIGHT_VERSION_DIFFERS"
	ReasonProjectsDiffer     = "PROJECTS_DIFFER"
	ReasonConfigDiffers      = "CONFIG_DIFFERS"
	ReasonInputsDifferInSide = "INPUTS_DIFFER_WITHIN_SIDE"
	ReasonReportHasErrors    = "REPORT_HAS_ERRORS"
	ReasonNoTests            = "NO_TESTS"
	ReasonTestsChanged       = "TESTS_CHANGED"
	ReasonTestsMissing       = "TESTS_MISSING"
	ReasonTestsNew           = "TESTS_NEW"
	ReasonSupportDiffers     = "SUPPORT_ASSERTIONS_DIFFER"
	UnknownInputsNotDeclared = "IDENTITY_INPUTS_NOT_DECLARED"
	// UNDEPLOYED reasons and the served-digest unknown (PWR-V0-056).
	ReasonServedDigestUnchanged    = "SERVED_DIGEST_UNCHANGED"
	ReasonServedDigestDiffersAtRev = "SERVED_DIGEST_DIFFERS_AT_REVISION"
	UnknownServedDigest            = "SERVED_DIGEST_UNKNOWN"
	NotObserved                    = "NOT_OBSERVED"
	outcomeAbsent                  = "absent"
	DifferenceOutcome              = "OUTCOME"
	DifferenceRetries              = "RETRIES"
	DifferenceTestFile             = "TEST_FILE_ASSERTIONS"
	DifferenceSupport              = "SUPPORT_ASSERTIONS"
	DifferenceUnknownOrigin        = "UNKNOWN_ORIGIN_ASSERTIONS"
)

// Loaded is one report argument: the admitted run, or the reason it was not admitted.
type Loaded struct {
	Run *Run
	Err error
}

// ReportFacts are one report's identity and the resource facts the json reporter carries
// (PWR-V0-009). Fixture durations and cleanup outcomes are not serialized by the reporter.
type ReportFacts struct {
	Report             int      `json:"report"`
	Sha256             string   `json:"sha256"`
	PlaywrightVersion  string   `json:"playwrightVersion"`
	Projects           []string `json:"projects"`
	ConfiguredProjects []string `json:"configuredProjects"`
	IdentityInputs     string   `json:"identityInputs"` // DECLARED | NOT_DECLARED
	// IdentitySource is present only when the caller gave a sidecar option (PWR-V0-067).
	IdentitySource   string `json:"identitySource,omitempty"`
	ServedDigest     string `json:"servedDigest"` // DECLARED | UNKNOWN (PWR-V0-056)
	Workers          int    `json:"workers"`
	WorkerIndexes    []int  `json:"workerIndexes"`
	ResultDurationMS int64  `json:"resultDurationMS"`
	FixtureDurations string `json:"fixtureDurations"`
	Cleanup          string `json:"cleanup"`
	// Shard and RepeatEach appear only when the report declares a shard or a repeatEach above 1
	// (PWR-V0-014, PWR-V0-020).
	Shard      *Shard         `json:"shard,omitempty"`
	RepeatEach map[string]int `json:"repeatEach,omitempty"`
}

// Problem names one report that could not be admitted.
type Problem struct {
	Side   string `json:"side"`
	Report int    `json:"report"`
	Reason string `json:"reason"`
}

// OutcomeCount is how many of a side's reports gave one outcome.
type OutcomeCount struct {
	Outcome string `json:"outcome"`
	Count   int    `json:"count"`
}

// SideTest is one test's evidence on one side over that side's N observations: one per logical run,
// or repeatEach per run that declares it (PWR-V0-006, PWR-V0-009, PWR-V0-014).
type SideTest struct {
	Reports          int            `json:"reports"`
	Outcomes         []OutcomeCount `json:"outcomes"`
	Stability        string         `json:"stability"` // k/N: reports with the most frequent outcome
	MaxRetry         int            `json:"maxRetry"`
	FailedAssertions []Assertion    `json:"failedAssertions"`
}

// CompareEntry is one test matched by identity across the two sides.
type CompareEntry struct {
	Identity
	Verdict           string    `json:"verdict"`
	Differences       []string  `json:"differences"`
	SupportDifference bool      `json:"supportDifference"`
	Baseline          *SideTest `json:"baseline"`
	Candidate         *SideTest `json:"candidate"`
}

// CompareSummary counts entries by verdict, plus SAME entries whose only difference is in
// support (page-object, helper or fixture) assertions.
type CompareSummary struct {
	Same               int `json:"same"`
	Changed            int `json:"changed"`
	Missing            int `json:"missing"`
	New                int `json:"new"`
	Ambiguous          int `json:"ambiguous"`
	SupportDifferences int `json:"supportDifferences"`
}

// CompareReport is the `corvint runs compare` document.
type CompareReport struct {
	Profile  string                   `json:"profile"`
	Verdict  string                   `json:"verdict"`
	Reasons  []string                 `json:"reasons"`
	Unknowns []string                 `json:"unknowns"`
	Problems []Problem                `json:"problems"`
	Reports  map[string][]ReportFacts `json:"reports"`
	// ShardSets lists each side's shard sets; absent when no admitted report is a shard.
	ShardSets map[string][]ShardSet `json:"shardSets,omitempty"`
	Tests     []CompareEntry        `json:"tests"`
	Summary   CompareSummary        `json:"summary"`
}

// Compare matches tests by identity across baseline and candidate report sets and returns one
// verdict (PWR-V0-008): INCOMPARABLE with every applicable reason when the sets cannot be shown to
// be comparable, otherwise DIVERGED when any test is CHANGED, MISSING or NEW, otherwise PARITY.
// Per-test entries are computed whenever every report was admitted and every shard set is complete.
// Each side's shard reports are grouped into logical runs first (PWR-V0-016 to PWR-V0-020).
func Compare(baseline, candidate []Loaded) CompareReport {
	report := CompareReport{Profile: CompareProfile, Reasons: []string{}, Unknowns: []string{}, Problems: []Problem{},
		Reports: map[string][]ReportFacts{SideBaseline: {}, SideCandidate: {}}, Tests: []CompareEntry{}}
	reasons := map[string]bool{}
	sides := map[string][]*Run{}
	positions := map[string][]int{}
	for _, side := range []struct {
		name   string
		loaded []Loaded
	}{{SideBaseline, baseline}, {SideCandidate, candidate}} {
		for i, l := range side.loaded {
			if l.Err != nil || l.Run == nil {
				reason := ReasonReportUnreadable
				if l.Err != nil {
					reason = l.Err.Error()
				}
				report.Problems = append(report.Problems, Problem{Side: side.name, Report: i + 1, Reason: reason})
				reasons[ReasonReportUnreadable] = true
				continue
			}
			sides[side.name] = append(sides[side.name], l.Run)
			positions[side.name] = append(positions[side.name], i+1)
			report.Reports[side.name] = append(report.Reports[side.name], facts(i+1, l.Run))
		}
		if len(side.loaded) == 0 {
			report.Problems = append(report.Problems, Problem{Side: side.name, Report: 0, Reason: ReasonNoTests})
			reasons[ReasonNoTests] = true
		}
	}
	refused := false
	for _, side := range []string{SideBaseline, SideCandidate} {
		logical, sets := logicalRuns(sides[side], positions[side])
		sides[side] = nil
		for _, run := range logical {
			if run.refusal == "" {
				sides[side] = append(sides[side], run.Run)
			}
		}
		for _, set := range sets {
			if report.ShardSets == nil {
				report.ShardSets = map[string][]ShardSet{SideBaseline: {}, SideCandidate: {}}
			}
			report.ShardSets[side] = append(report.ShardSets[side], set)
			if set.Reason != "" {
				refused = true
				reasons[set.Reason] = true
				report.Problems = append(report.Problems, Problem{Side: side, Report: set.Run, Reason: set.Reason})
			}
		}
	}
	if len(sides[SideBaseline])+len(sides[SideCandidate]) > 0 {
		comparability(sides, reasons, &report)
	}
	// Test rows are withheld unless every report was admitted, and every shard set complete, on
	// both sides.
	if reasons[ReasonReportUnreadable] || refused || len(sides[SideBaseline]) == 0 || len(sides[SideCandidate]) == 0 {
		return finish(report, reasons)
	}
	entries(sides, &report)
	if !hasIncomparable(reasons) {
		s := report.Summary
		if s.Changed > 0 {
			reasons[ReasonTestsChanged] = true
		}
		if s.Missing > 0 {
			reasons[ReasonTestsMissing] = true
		}
		if s.New > 0 {
			reasons[ReasonTestsNew] = true
		}
		if s.SupportDifferences > 0 {
			reasons[ReasonSupportDiffers] = true
		}
	}
	return finish(report, reasons)
}

var incomparableReasons = []string{ReasonReportUnreadable, ReasonShardInputsUndeclared, ReasonShardSetIncomplete, ReasonShardDuplicate, ReasonVersionDiffers, ReasonProjectsDiffer, ReasonConfigDiffers,
	ReasonInputsDifferInSide, ReasonReportHasErrors, ReasonNoTests, ReasonAmbiguous, ReasonInterrupted}

func hasIncomparable(reasons map[string]bool) bool {
	for _, r := range incomparableReasons {
		if reasons[r] {
			return true
		}
	}
	return false
}

func finish(report CompareReport, reasons map[string]bool) CompareReport {
	switch {
	case hasIncomparable(reasons):
		report.Verdict = Incomparable
		for _, r := range incomparableReasons {
			if reasons[r] {
				report.Reasons = append(report.Reasons, r)
			}
		}
		return report
	case reasons[ReasonServedDigestUnchanged] || reasons[ReasonServedDigestDiffersAtRev]:
		report.Verdict = Undeployed
	case reasons[ReasonTestsChanged] || reasons[ReasonTestsMissing] || reasons[ReasonTestsNew]:
		report.Verdict = Diverged
	default:
		report.Verdict = Parity
	}
	for _, r := range []string{ReasonServedDigestUnchanged, ReasonServedDigestDiffersAtRev, ReasonTestsChanged, ReasonTestsMissing, ReasonTestsNew, ReasonSupportDiffers} {
		if reasons[r] {
			report.Reasons = append(report.Reasons, r)
		}
	}
	return report
}

func facts(index int, run *Run) ReportFacts {
	f := ReportFacts{Report: index, Sha256: run.Sha256, PlaywrightVersion: screenText(run.Version), Projects: []string{}, ConfiguredProjects: []string{}, IdentitySource: run.IdentitySource,
		IdentityInputs: "NOT_DECLARED", ServedDigest: ServedDigestUnknown, Workers: run.Workers, WorkerIndexes: []int{}, FixtureDurations: NotObserved, Cleanup: NotObserved}
	if run.InputsDeclared {
		f.IdentityInputs = "DECLARED"
		if run.Inputs.ServedArtifactDigest != "" {
			f.ServedDigest = ServedDigestDeclared
		}
	}
	for _, p := range run.Projects {
		f.Projects = append(f.Projects, screenText(p))
	}
	for _, p := range run.ConfiguredProjects {
		f.ConfiguredProjects = append(f.ConfiguredProjects, screenText(p))
	}
	seen := map[int]bool{}
	for _, t := range run.Tests {
		for _, r := range t.Results {
			f.ResultDurationMS += r.DurationMS
			if r.WorkerIndex >= 0 && !seen[r.WorkerIndex] {
				seen[r.WorkerIndex] = true
				f.WorkerIndexes = append(f.WorkerIndexes, r.WorkerIndex)
			}
		}
	}
	sort.Ints(f.WorkerIndexes)
	f.Shard, f.RepeatEach = run.Shard, presentedRepeatEach(run)
	return f
}

// comparability records every reason the two report sets cannot be compared (PWR-V0-008).
func comparability(sides map[string][]*Run, reasons map[string]bool, report *CompareReport) {
	all := append(append([]*Run{}, sides[SideBaseline]...), sides[SideCandidate]...)
	first := all[0]
	configs := map[string]bool{}
	declaredAll := true
	for _, run := range all {
		if run.Version != first.Version {
			reasons[ReasonVersionDiffers] = true
		}
		if !reflect.DeepEqual(run.Projects, first.Projects) || !reflect.DeepEqual(run.ConfiguredProjects, first.ConfiguredProjects) {
			reasons[ReasonProjectsDiffer] = true
		}
		if run.ReportErrors > 0 {
			reasons[ReasonReportHasErrors] = true
		}
		if len(run.Ambiguous) > 0 {
			reasons[ReasonAmbiguous] = true
		}
		for _, t := range run.Tests {
			for _, r := range t.Results {
				if r.Status == "interrupted" {
					reasons[ReasonInterrupted] = true
				}
			}
		}
		if !run.InputsDeclared || run.Inputs.ConfigDigest == "" {
			declaredAll = false
		} else {
			configs[run.Inputs.ConfigDigest] = true
		}
	}
	if declaredAll && len(configs) > 1 {
		reasons[ReasonConfigDiffers] = true
	}
	for _, side := range []string{SideBaseline, SideCandidate} {
		var declared *Inputs
		for _, run := range sides[side] {
			if run.InputsDeclared {
				if declared != nil && !reflect.DeepEqual(run.Inputs, *declared) {
					reasons[ReasonInputsDifferInSide] = true
				}
				if declared == nil {
					declared = &run.Inputs
				}
			}
			if len(run.Tests) == 0 {
				reasons[ReasonNoTests] = true
			}
		}
	}
	if !declaredAll {
		report.Unknowns = append(report.Unknowns, UnknownInputsNotDeclared)
	}
	servedDeployment(sides, reasons, report)
}

// servedDeployment checks the served builds against the declared revisions (PWR-V0-056). When
// every run declares a servedArtifactDigest and each side has one revision and one digest, a
// candidate that served the baseline's build at another revision is SERVED_DIGEST_UNCHANGED, and
// one that served another build at the same revision SERVED_DIGEST_DIFFERS_AT_REVISION; both make
// the verdict UNDEPLOYED. Any run without a digest adds the unknown SERVED_DIGEST_UNKNOWN instead.
func servedDeployment(sides map[string][]*Run, reasons map[string]bool, report *CompareReport) {
	type pair struct{ revision, digest string }
	seen := map[string]map[pair]bool{}
	for _, side := range []string{SideBaseline, SideCandidate} {
		seen[side] = map[pair]bool{}
		for _, run := range sides[side] {
			if !run.InputsDeclared || run.Inputs.ServedArtifactDigest == "" {
				report.Unknowns = append(report.Unknowns, UnknownServedDigest)
				return
			}
			seen[side][pair{run.Inputs.ApplicationRevision, run.Inputs.ServedArtifactDigest}] = true
		}
	}
	if len(seen[SideBaseline]) != 1 || len(seen[SideCandidate]) != 1 {
		return // empty side, or a within-side difference already reported
	}
	var base, cand pair
	for p := range seen[SideBaseline] {
		base = p
	}
	for p := range seen[SideCandidate] {
		cand = p
	}
	switch {
	case base.revision != cand.revision && base.digest == cand.digest:
		reasons[ReasonServedDigestUnchanged] = true
	case base.revision == cand.revision && base.digest != cand.digest:
		reasons[ReasonServedDigestDiffersAtRev] = true
	}
}

// entries matches tests by identity and classifies each (PWR-V0-006, PWR-V0-007). Each logical
// run contributes one observation slot per declared repeat of the test's project, filled in repeat
// order, so N repeats in one report count as N single-repeat reports (PWR-V0-014).
func entries(sides map[string][]*Run, report *CompareReport) {
	ids := map[string]Identity{}
	ambiguous := map[string]bool{}
	per := map[string]map[string][][]*Test{SideBaseline: {}, SideCandidate: {}}
	for side, runs := range sides {
		for i, run := range runs {
			for key := range run.Ambiguous {
				ambiguous[key] = true
			}
			for j := range run.Tests {
				t := &run.Tests[j]
				key := t.Key()
				ids[key] = t.Identity
				if per[side][key] == nil {
					per[side][key] = make([][]*Test, len(runs))
				}
				per[side][key][i] = append(per[side][key][i], t)
			}
		}
	}
	keys := make([]string, 0, len(ids))
	for k := range ids {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		e := CompareEntry{Identity: ids[k].presented(), Differences: []string{}}
		if ambiguous[k] {
			// Which observation is "the" test is unknown, so no side statistics or verdict.
			e.Verdict = AmbiguousTest
			report.Summary.Ambiguous++
			report.Tests = append(report.Tests, e)
			continue
		}
		base, cand := per[SideBaseline][k], per[SideCandidate][k]
		e.Baseline = sideTest(slots(sides[SideBaseline], base, ids[k].Project))
		e.Candidate = sideTest(slots(sides[SideCandidate], cand, ids[k].Project))
		switch {
		case cand == nil:
			e.Verdict = Missing
			report.Summary.Missing++
		case base == nil:
			e.Verdict = New
			report.Summary.New++
		default:
			e.Differences = differences(e.Baseline, e.Candidate)
			e.Verdict = Same
			for _, d := range e.Differences {
				if d == DifferenceSupport {
					e.SupportDifference = true
				} else {
					e.Verdict = Changed
				}
			}
			if e.Verdict == Changed {
				report.Summary.Changed++
			} else {
				report.Summary.Same++
				if e.SupportDifference {
					report.Summary.SupportDifferences++
				}
			}
		}
		screenAssertions(e.Baseline)
		screenAssertions(e.Candidate)
		report.Tests = append(report.Tests, e)
	}
}

// screenAssertions secret-screens emitted assertion files after the raw ones were compared, so
// redaction never hides a difference.
func screenAssertions(st *SideTest) {
	for i := range st.FailedAssertions {
		st.FailedAssertions[i].File = screenText(st.FailedAssertions[i].File)
	}
}

// slots lays one test's occurrences out over a side's observation slots: per logical run, as many
// slots as the run declares repeats for the project. It returns the occurrences and the slot count;
// the unfilled slots are absent and are counted, never materialized. An ambiguous identity never
// reaches here, so a run never holds more occurrences than slots.
func slots(runs []*Run, occurrences [][]*Test, project string) ([]*Test, int) {
	present, total := []*Test{}, 0
	for i, run := range runs {
		n := run.repeats(project)
		total += n
		if occurrences != nil {
			got := occurrences[i]
			if len(got) > n {
				got = got[:n]
			}
			present = append(present, got...)
		}
	}
	return present, total
}

// sideTest summarizes one test over one side's observation slots; a slot without the test counts as
// the outcome "absent", and a test no report has is all absent.
func sideTest(tests []*Test, total int) *SideTest {
	st := &SideTest{Reports: total, Outcomes: []OutcomeCount{}, FailedAssertions: []Assertion{}}
	counts := map[string]int{}
	seen := map[Assertion]bool{}
	if absent := total - len(tests); absent > 0 {
		counts[outcomeAbsent] = absent
	}
	for _, t := range tests {
		counts[outcomeOf(*t)]++
		for _, r := range t.Results {
			if r.Retry > st.MaxRetry {
				st.MaxRetry = r.Retry
			}
			for _, a := range r.Failures {
				if !seen[a] {
					seen[a] = true
					st.FailedAssertions = append(st.FailedAssertions, a)
				}
			}
		}
	}
	top := 0
	for outcome, n := range counts {
		st.Outcomes = append(st.Outcomes, OutcomeCount{Outcome: outcome, Count: n})
		if n > top {
			top = n
		}
	}
	sort.Slice(st.Outcomes, func(i, j int) bool { return st.Outcomes[i].Outcome < st.Outcomes[j].Outcome })
	sort.Slice(st.FailedAssertions, func(i, j int) bool { return assertionLess(st.FailedAssertions[i], st.FailedAssertions[j]) })
	st.Stability = fmt.Sprintf("%d/%d", top, total)
	return st
}

func assertionLess(a, b Assertion) bool {
	if a.Origin != b.Origin {
		return a.Origin < b.Origin
	}
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Matcher < b.Matcher
}

// differences lists what differs between the two sides of one test, in a fixed order. Outcome
// histograms compare as proportions only when both sides have the same report count.
func differences(base, cand *SideTest) []string {
	out := []string{}
	if !sameOutcomes(base, cand) {
		out = append(out, DifferenceOutcome)
	}
	if base.MaxRetry != cand.MaxRetry {
		out = append(out, DifferenceRetries)
	}
	for _, d := range []struct{ origin, name string }{
		{OriginTestFile, DifferenceTestFile}, {OriginSupport, DifferenceSupport}, {OriginUnknown, DifferenceUnknownOrigin}} {
		if !reflect.DeepEqual(byOrigin(base, d.origin), byOrigin(cand, d.origin)) {
			out = append(out, d.name)
		}
	}
	return out
}

// sameOutcomes is true when both sides saw the same set of outcomes in the same proportions.
func sameOutcomes(base, cand *SideTest) bool {
	if len(base.Outcomes) != len(cand.Outcomes) {
		return false
	}
	for i := range base.Outcomes {
		b, c := base.Outcomes[i], cand.Outcomes[i]
		if b.Outcome != c.Outcome || b.Count*cand.Reports != c.Count*base.Reports {
			return false
		}
	}
	return true
}

func byOrigin(st *SideTest, origin string) []Assertion {
	out := []Assertion{}
	for _, a := range st.FailedAssertions {
		if a.Origin == origin {
			out = append(out, a)
		}
	}
	return out
}
