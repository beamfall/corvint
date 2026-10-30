package playwrightruns

import (
	"encoding/json"
	"sort"
)

// Reuse profile and vocabulary (PWR-V0-033 to PWR-V0-036).
const (
	ReusableProfile = "corvint-runs-reusable/0"

	DecisionReusable = "REUSABLE"
	DecisionMustRun  = "MUST_RUN"

	ReuseExactPass = "EXACT_PASS"
	ReuseFreshPass = "FRESH_PASS"

	ReasonNoPriorResult       = "NO_PRIOR_RESULT"
	ReasonPriorNotPassed      = "PRIOR_NOT_PASSED"
	ReasonPriorNotQualified   = "PRIOR_NOT_QUALIFIED"
	ReasonFingerprintChanged  = "FINGERPRINT_CHANGED"
	ReasonStale               = "STALE"
	ReasonFreshnessUnresolved = "FRESHNESS_UNRESOLVED"

	FormFull    = "full"
	FormSummary = "summary"

	// MaxRowPaths caps the deciding paths one reuse row names (PWR-V0-036).
	MaxRowPaths = 16
)

// Fingerprint field names, as emitted in a row's fields list.
const (
	FieldPlaywrightVersion   = "playwrightVersion"
	FieldApplicationRevision = "applicationRevision"
	FieldTestRevision        = "testRevision"
	FieldTestFileBlob        = "testFileBlob"
	FieldConfigDigest        = "configDigest"
	FieldProject             = "project"
	// FieldServedArtifactDigest differs when the two runs declare different served builds, one
	// of them none (PWR-V0-057).
	FieldServedArtifactDigest = "servedArtifactDigest"
)

// Fingerprint is what a retained result was produced at (PWR-V0-033): the Playwright version, the
// declared application and test-suite revisions, the declared blob of the test's file, the declared
// config digest and the test's project. The project is part of the test identity and may be empty
// (Playwright's unnamed default project); every other input is required.
type Fingerprint struct {
	PlaywrightVersion   string `json:"playwrightVersion"`
	ApplicationRevision string `json:"applicationRevision"`
	TestRevision        string `json:"testRevision"`
	TestFileBlob        string `json:"testFileBlob"`
	ConfigDigest        string `json:"configDigest"`
	Project             string `json:"project"`
	// ServedArtifactDigest is omitted when undeclared, so fingerprints without it are unchanged
	// (PWR-V0-057).
	ServedArtifactDigest string `json:"servedArtifactDigest,omitempty"`
}

func fingerprintOf(run *Run, t Test) Fingerprint {
	fp := Fingerprint{PlaywrightVersion: run.Version, Project: t.Project}
	if run.InputsDeclared {
		fp.ApplicationRevision, fp.ConfigDigest = run.Inputs.ApplicationRevision, run.Inputs.ConfigDigest
		fp.TestFileBlob, fp.TestRevision = run.Inputs.TestFileBlobs[t.File], run.TestRevision
		fp.ServedArtifactDigest = run.Inputs.ServedArtifactDigest
	}
	return fp
}

// missing names the required inputs that are not declared.
func (f Fingerprint) missing() []string {
	out := []string{}
	for _, field := range []struct{ name, value string }{
		{FieldPlaywrightVersion, f.PlaywrightVersion}, {FieldApplicationRevision, f.ApplicationRevision},
		{FieldTestRevision, f.TestRevision}, {FieldTestFileBlob, f.TestFileBlob}, {FieldConfigDigest, f.ConfigDigest},
	} {
		if field.value == "" {
			out = append(out, field.name)
		}
	}
	return out
}

// differs names the inputs that differ between two fingerprints, in a fixed order.
func (f Fingerprint) differs(o Fingerprint) []string {
	out := []string{}
	for _, field := range []struct {
		name string
		a, b string
	}{
		{FieldPlaywrightVersion, f.PlaywrightVersion, o.PlaywrightVersion}, {FieldApplicationRevision, f.ApplicationRevision, o.ApplicationRevision},
		{FieldTestRevision, f.TestRevision, o.TestRevision}, {FieldTestFileBlob, f.TestFileBlob, o.TestFileBlob},
		{FieldConfigDigest, f.ConfigDigest, o.ConfigDigest}, {FieldProject, f.Project, o.Project},
		{FieldServedArtifactDigest, f.ServedArtifactDigest, o.ServedArtifactDigest},
	} {
		if field.a != field.b {
			out = append(out, field.name)
		}
	}
	return out
}

// onlyRevisions reports whether every differing input is a revision, so freshness can decide.
func onlyRevisions(fields []string) bool {
	for _, f := range fields {
		if f != FieldApplicationRevision && f != FieldTestRevision {
			return false
		}
	}
	return len(fields) > 0
}

// priorObservation is one retained result of one test in one logical run.
type priorObservation struct {
	run       int
	fp        Fingerprint
	complete  bool
	outcome   string
	qualifier string // "" when the result qualifies as evidence; else why it does not
	passed    bool   // a qualified, expected pass
	durationM int64
}

// observe reads one test of one logical run as reuse evidence (PWR-V0-033). A result qualifies only
// from an admitted run without report errors, unambiguous, executed and not interrupted; it is a
// qualified pass only when Playwright also judged it expected and its outcome is passed (not flaky).
func observe(run logicalRun, t Test) priorObservation {
	fp := fingerprintOf(run.Run, t)
	o := priorObservation{run: run.index, fp: fp, complete: len(fp.missing()) == 0, outcome: outcomeOf(t)}
	for _, r := range t.Results {
		o.durationM += r.DurationMS
	}
	_, reason := poolInputs(run.Run, t, false)
	switch {
	case run.refusal != "":
		o.qualifier = run.refusal
	case reason != "" && reason != ReasonInputsMissing:
		o.qualifier = reason
	case run.ReportErrors > 0:
		o.qualifier = ReasonReportHasErrors
	}
	o.passed = o.qualifier == "" && o.outcome == "passed" && t.Status == "expected"
	return o
}

// FreshVerdict is the freshness of evidence produced at one revision for a test file, read at
// another (PWR-V0-036). Verdict is FRESH, STALE or UNRESOLVED (with Reason).
type FreshVerdict struct {
	Verdict   string
	Reason    string
	Paths     []string
	Truncated int
}

// FreshnessCheck answers the freshness of a test file's evidence between an old and a new revision.
type FreshnessCheck func(oldRevision, newRevision, testFile string) FreshVerdict

// ReuseEntry is one selected test's decision.
type ReuseEntry struct {
	Identity
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
	// Detail is the qualifier of a PRIOR_NOT_QUALIFIED row, the outcome of a PRIOR_NOT_PASSED row or
	// the unresolved reason of a FRESHNESS_UNRESOLVED row.
	Detail         string   `json:"detail,omitempty"`
	PriorRuns      []int    `json:"priorRuns,omitempty"`
	Fields         []string `json:"fields,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	PathsTruncated int      `json:"pathsTruncated,omitempty"`
}

// ReuseSummary counts the selected tests by decision and reason.
type ReuseSummary struct {
	Selected  int            `json:"selected"`
	Reusable  int            `json:"reusable"`
	MustRun   int            `json:"mustRun"`
	ByReason  map[string]int `json:"byReason"`
	Freshness string         `json:"freshness"` // ENABLED | DISABLED
}

// ReuseReport is the `corvint runs reusable` document.
type ReuseReport struct {
	Profile   string       `json:"profile"`
	Form      string       `json:"form"` // full | summary; the summary form omits tests, priors and shardSets
	Selection RunSummary   `json:"selection"`
	Priors    []RunSummary `json:"priors,omitempty"`
	ShardSets []ShardSet   `json:"shardSets,omitempty"`
	Tests     []ReuseEntry `json:"tests,omitempty"`
	Summary   ReuseSummary `json:"summary"`
}

// Compact is the summary form: the report without its per-test and per-run rows (PWR-V0-040).
func (r ReuseReport) Compact() ReuseReport {
	r.Form, r.Tests, r.Priors, r.ShardSets = FormSummary, nil, nil, nil
	return r
}

// runSummary states each run's servedDigest DECLARED or UNKNOWN; a differing build is already a
// FINGERPRINT_CHANGED row (PWR-V0-057), so reuse never states DISAGREES.
func runSummary(position int, run *Run) RunSummary {
	declared := "NOT_DECLARED"
	if run.InputsDeclared {
		declared = "DECLARED"
	}
	return RunSummary{Run: position, Sha256: run.Sha256, PlaywrightVersion: screenText(run.Version), IdentityInputs: declared,
		Shard: run.Shard, RepeatEach: presentedRepeatEach(run), ServedDigest: servedDigestState(run, nil), IdentitySource: run.IdentitySource}
}

// Reusable decides, for each test the selection report lists, whether a retained passing result
// from the prior reports can stand in for running it at the selection's declared fingerprint
// (PWR-V0-034 to PWR-V0-036). Missing or unqualified evidence is always MUST_RUN. fresh, when not
// nil, admits a qualified pass whose fingerprint differs only in its revisions when every
// differing revision pair is FRESH for the test's file.
func Reusable(selection *Run, priors []*Run, fresh FreshnessCheck) ReuseReport {
	report := ReuseReport{Profile: ReusableProfile, Form: FormFull, Selection: runSummary(0, selection), Priors: []RunSummary{}, Tests: []ReuseEntry{}}
	report.Summary.ByReason = map[string]int{}
	report.Summary.Freshness = "DISABLED"
	if fresh != nil {
		report.Summary.Freshness = "ENABLED"
	}
	positions := make([]int, len(priors))
	for i, run := range priors {
		positions[i] = i + 1
		report.Priors = append(report.Priors, runSummary(i+1, run))
	}
	logical, sets := logicalRuns(priors, positions)
	if len(sets) > 0 {
		report.ShardSets = sets
	}
	history := map[string][]priorObservation{}
	for _, run := range logical {
		for _, t := range run.Tests {
			history[t.Key()] = append(history[t.Key()], observe(run, t))
		}
	}
	seen := map[string]bool{}
	for _, t := range selection.Tests {
		key := t.Key()
		if seen[key] {
			continue
		}
		seen[key] = true
		e := decide(selection, t, history[key], fresh)
		e.Identity = t.Identity.presented()
		report.Tests = append(report.Tests, e)
		report.Summary.Selected++
		report.Summary.ByReason[e.Reason]++
		if e.Decision == DecisionReusable {
			report.Summary.Reusable++
		} else {
			report.Summary.MustRun++
		}
	}
	return report
}

func mustRun(reason string) ReuseEntry { return ReuseEntry{Decision: DecisionMustRun, Reason: reason} }

func decide(selection *Run, t Test, prior []priorObservation, fresh FreshnessCheck) ReuseEntry {
	if selection.Ambiguous[t.Key()] {
		return mustRun(ReasonAmbiguous)
	}
	want := fingerprintOf(selection, t)
	if missing := want.missing(); len(missing) > 0 {
		e := mustRun(ReasonInputsMissing)
		e.Fields = missing
		return e
	}
	if len(prior) == 0 {
		return mustRun(ReasonNoPriorResult)
	}
	exact, candidates, nearest := []priorObservation{}, map[string][]priorObservation{}, []string(nil)
	for _, o := range prior {
		if !o.complete {
			continue
		}
		diff := want.differs(o.fp)
		switch {
		case len(diff) == 0:
			exact = append(exact, o)
		case fresh != nil && onlyRevisions(diff):
			raw, _ := json.Marshal([]string{o.fp.ApplicationRevision, o.fp.TestRevision})
			candidates[string(raw)] = append(candidates[string(raw)], o)
		}
		if nearest == nil || len(diff) < len(nearest) {
			nearest = diff
		}
	}
	if len(exact) > 0 {
		if e, ok := allPassed(exact); !ok {
			return e
		}
		return ReuseEntry{Decision: DecisionReusable, Reason: ReuseExactPass, PriorRuns: priorRunsOf(exact)}
	}
	if len(candidates) > 0 {
		return decideFresh(want, t.File, candidates, fresh)
	}
	if nearest != nil {
		e := mustRun(ReasonFingerprintChanged)
		e.Fields = nearest
		return e
	}
	// Prior results exist but none declares a complete fingerprint.
	e := mustRun(ReasonInputsMissing)
	e.Detail = "prior"
	e.Fields = prior[0].fp.missing()
	return e
}

// allPassed is the MUST_RUN row for the first observation that is not a qualified pass, if any.
func allPassed(obs []priorObservation) (ReuseEntry, bool) {
	for _, o := range obs {
		switch {
		case o.qualifier != "":
			e := mustRun(ReasonPriorNotQualified)
			e.Detail, e.PriorRuns = o.qualifier, []int{o.run}
			return e, false
		case !o.passed:
			e := mustRun(ReasonPriorNotPassed)
			e.Detail, e.PriorRuns = o.outcome, []int{o.run}
			return e, false
		}
	}
	return ReuseEntry{}, true
}

func priorRunsOf(obs []priorObservation) []int {
	set := map[int]bool{}
	out := []int{}
	for _, o := range obs {
		if !set[o.run] {
			set[o.run] = true
			out = append(out, o.run)
		}
	}
	sort.Ints(out)
	return out
}

// decideFresh applies freshness as an extra reuse condition (PWR-V0-036). Each prior fingerprint
// that differs only in revisions is checked pair by pair; a FRESH fingerprint whose observations
// include anything but qualified passes makes the test MUST_RUN, a FRESH fingerprint of qualified
// passes makes it REUSABLE as FRESH_PASS, and otherwise the test is MUST_RUN as STALE (naming the
// deciding paths) or, when nothing was decided, FRESHNESS_UNRESOLVED.
func decideFresh(want Fingerprint, file string, candidates map[string][]priorObservation, fresh FreshnessCheck) ReuseEntry {
	keys := make([]string, 0, len(candidates))
	for k := range candidates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var reusable []priorObservation
	var stale, unresolved *ReuseEntry
	for _, k := range keys {
		obs := candidates[k]
		v := pairFreshness(want, obs[0].fp, file, fresh)
		switch v.Verdict {
		case "FRESH":
			if e, ok := allPassed(obs); !ok {
				return e
			}
			reusable = append(reusable, obs...)
		case "STALE":
			if stale == nil {
				e := mustRun(ReasonStale)
				e.Fields, e.PriorRuns = want.differs(obs[0].fp), priorRunsOf(obs)
				e.Paths, e.PathsTruncated = capPaths(v.Paths, v.Truncated)
				stale = &e
			}
		default:
			if unresolved == nil {
				e := mustRun(ReasonFreshnessUnresolved)
				e.Detail, e.Fields, e.PriorRuns = v.Reason, want.differs(obs[0].fp), priorRunsOf(obs)
				unresolved = &e
			}
		}
	}
	switch {
	case reusable != nil:
		return ReuseEntry{Decision: DecisionReusable, Reason: ReuseFreshPass, PriorRuns: priorRunsOf(reusable), Fields: want.differs(reusable[0].fp)}
	case stale != nil:
		return *stale
	default:
		return *unresolved
	}
}

// pairFreshness combines the freshness of each differing revision pair: STALE when any pair is
// stale (paths united), UNRESOLVED when any other pair is not FRESH, else FRESH.
func pairFreshness(want, old Fingerprint, file string, fresh FreshnessCheck) FreshVerdict {
	out := FreshVerdict{Verdict: "FRESH"}
	paths := map[string]bool{}
	for _, pair := range [][2]string{{old.ApplicationRevision, want.ApplicationRevision}, {old.TestRevision, want.TestRevision}} {
		if pair[0] == pair[1] {
			continue
		}
		v := fresh(pair[0], pair[1], file)
		switch v.Verdict {
		case "FRESH":
		case "STALE":
			out.Verdict = "STALE"
			for _, p := range v.Paths {
				paths[p] = true
			}
			out.Truncated += v.Truncated
		default:
			if out.Verdict == "FRESH" {
				out.Verdict, out.Reason = "UNRESOLVED", v.Reason
			}
		}
	}
	if out.Verdict == "STALE" {
		out.Reason = ""
		out.Paths = sortedKeys(paths)
	}
	return out
}

func capPaths(paths []string, truncated int) ([]string, int) {
	out := []string{}
	for _, p := range paths {
		if len(out) >= MaxRowPaths {
			truncated++
			continue
		}
		out = append(out, screenText(p))
	}
	return out, truncated
}
