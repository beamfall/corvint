package obligation

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TestOutcome is one test's outcome in a report, for the experimental batch
// runner and final qualification (TOL-V0-028..033, GitHub #714). Status and
// Error are the retry-0 result's; MaxRetry is the highest retry index any of
// the test's results carries, so a value above 0 means the test was retried.
type TestOutcome struct {
	File      string // spec.file, relative to config.rootDir
	TitlePath []string
	Project   string
	Expected  string // expectedStatus
	Status    string // the retry-0 status, "" when the test has no retry-0 result
	Error     string // the retry-0 error's first line, screened; "" when none
	MaxRetry  int
}

// OK reports whether the retry-0 result has the expected status.
func (o TestOutcome) OK() bool { return o.Status != "" && o.Status == o.Expected }

// Key identifies a test across runs: its file and title path.
func (o TestOutcome) Key() string { return o.File + "\x00" + strings.Join(o.TitlePath, "\x00") }

// Outcomes returns every test of the report in report order.
func (r *Report) Outcomes() []TestOutcome {
	var out []TestOutcome
	var walk func(s jsonSuite, describes []string, top bool)
	walk = func(s jsonSuite, describes []string, top bool) {
		if !top {
			describes = append(append([]string{}, describes...), s.Title)
		}
		for _, sp := range s.Specs {
			titlePath := append(append([]string{}, describes...), sp.Title)
			for _, t := range sp.Tests {
				o := TestOutcome{File: sp.File, TitlePath: titlePath, Project: t.ProjectName, Expected: t.ExpectedStatus}
				for _, res := range t.Results {
					if res.Retry == nil {
						continue
					}
					o.MaxRetry = max(o.MaxRetry, *res.Retry)
					if *res.Retry == 0 {
						o.Status = res.Status
						if res.Error != nil {
							o.Error = Excerpt(res.Error.Message)
						}
					}
				}
				out = append(out, o)
			}
		}
		for _, c := range s.Suites {
			walk(c, describes, false)
		}
	}
	for _, s := range r.suites {
		walk(s, nil, true)
	}
	return out
}

// Errors is the number of the report's top-level errors, such as a spec
// file that failed to load.
func (r *Report) Errors() int { return r.errors }

// RepoPath maps a report spec file to its repository path, "" when it lies
// outside repoRoot (TOL-V0-012).
func (r *Report) RepoPath(repoRoot, file string) string { return r.repoPath(repoRoot, file) }

// FailureErrors returns, for each id of prefix with a failing ordinary
// retry-0 match, the first such match's error line, screened ("" when the
// report gives none).
func (r *Report) FailureErrors(prefix string) map[string]string {
	out := map[string]string{}
	for id, ms := range r.collect(prefix) {
		for _, m := range ms {
			if m.kind == kindOrdinary && !m.passed {
				if _, ok := out[id]; !ok || out[id] == "" {
					out[id] = Excerpt(m.failMsg)
				}
			}
		}
	}
	return out
}

// NamedTest is one test declaration in a spec file with every obligation id
// of the prefix that its title path or one of its steps names.
type NamedTest struct {
	Path      string
	TitlePath []string
	IDs       []string
}

// NamedTests scans files (path to content) with the TOL-V0-025 heuristic
// scanner and returns each test that names at least one id of prefix,
// sorted by path and line.
func NamedTests(files map[string][]byte, prefix string) []NamedTest {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var out []NamedTest
	for _, p := range paths {
		for _, t := range fileTests(p, parseNodes(lex(string(files[p]))), prefix) {
			titlePath := append(append([]string{}, t.describes...), t.title)
			var ids []string
			for _, title := range titlePath {
				ids = append(ids, FindIDs(title, prefix)...)
			}
			for _, s := range t.steps {
				ids = append(ids, FindIDs(s.title, prefix)...)
			}
			if len(ids) > 0 {
				out = append(out, NamedTest{Path: p, TitlePath: titlePath, IDs: sortedUnique(ids)})
			}
		}
	}
	return out
}

// Batch causes (TOL-V0-029): why each obligation of the ledger stands as it
// does after one batch run.
const (
	CauseWitnessed        = "WITNESSED"
	CauseAlreadyWitnessed = "ALREADY_WITNESSED"
	CauseFailed           = "FAILED"
	CauseNotReached       = "NOT_REACHED"
	CauseNotNamed         = "NOT_NAMED"
	CauseNotObserved      = "NOT_OBSERVED"
	CauseDefectConfirmed  = "DEFECT_CONFIRMED"
	CauseUncredited       = "UNCREDITED"
	CauseDeferred         = "DEFERRED"
	CauseOutOfScope       = "OUT_OF_SCOPE"
)

// Cause is one obligation's batch cause. Error is the failing step's (or
// test's) error line for FAILED, the error of the earlier failure for
// NOT_REACHED, and the classifier's reason for UNCREDITED.
type Cause struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Cause string `json:"cause"`
	Error string `json:"error"`
	Test  string `json:"test"`
}

// BatchCauses assigns each ledger entry its TOL-V0-029 cause from one
// classification of the batch report. credited says whether the witness
// was written (a refused witness credits nothing, so a would-be credit is
// UNCREDITED with reason). named are the source tests of the batch's spec
// files at the commit; repoRoot maps the report's spec files. ids, when not
// nil, is the batch's --ids subset.
func BatchCauses(r *Report, l *ticket.ObligationLedger, res *Result, repoRoot string, named []NamedTest, ids []string, credited bool, refusal string) []Cause {
	errs := r.FailureErrors(l.Prefix)
	inSubset := map[string]bool{}
	for _, id := range ids {
		inSubset[id] = true
	}
	set := func(list []string) map[string]bool {
		m := map[string]bool{}
		for _, id := range list {
			m[id] = true
		}
		return m
	}
	creditedSet, already, failed, conflicting, unmatched := set(res.Credited), set(res.AlreadyWitnessed), set(res.Failed), set(res.Conflicting), set(res.Unmatched)
	defects := map[string]DefectConfirmed{}
	for _, d := range res.DefectConfirmed {
		defects[d.ID] = d
	}
	unbound := map[string]string{}
	for _, u := range res.Unbound {
		unbound[u.ID] = u.Reason
	}
	mixed := map[string]bool{}
	for _, m := range res.MixedExpectedFail {
		mixed[m.ID] = true
	}
	// The failed tests of the report, by repository path and title path.
	failedTests := map[string]TestOutcome{}
	for _, o := range r.Outcomes() {
		if o.Status != "" && !o.OK() {
			key := r.repoPath(repoRoot, o.File) + "\x00" + strings.Join(o.TitlePath, "\x00")
			if _, ok := failedTests[key]; !ok {
				failedTests[key] = o
			}
		}
	}
	out := make([]Cause, 0, len(l.Entries))
	for _, e := range l.Entries {
		c := Cause{ID: e.ID, State: e.State}
		switch {
		case ids != nil && !inSubset[e.ID]:
			c.Cause = CauseOutOfScope
		case creditedSet[e.ID] && credited:
			c.Cause = CauseWitnessed
		case creditedSet[e.ID]:
			c.Cause, c.Error = CauseUncredited, refusal
		case already[e.ID]:
			c.Cause = CauseAlreadyWitnessed
		case failed[e.ID] || conflicting[e.ID]:
			c.Cause, c.Error = CauseFailed, errs[e.ID]
		case defects[e.ID].ID != "":
			c.Cause, c.Error = CauseDefectConfirmed, defects[e.ID].Error
		case mixed[e.ID]:
			c.Cause, c.Error = CauseUncredited, "MIXED_EXPECTED_FAIL: "+MixedRemedy
		case unbound[e.ID] != "":
			c.Cause, c.Error = CauseUncredited, unbound[e.ID]
		case e.State == ticket.ObligationDeferred:
			c.Cause = CauseDeferred
		case e.State == ticket.ObligationWitnessed:
			c.Cause = CauseAlreadyWitnessed
		case unmatched[e.ID]:
			c.Cause, c.Error, c.Test = notReached(e.ID, named, failedTests)
		default:
			c.Cause = CauseNotObserved
		}
		out = append(out, c)
	}
	return out
}

// notReached tells NOT_REACHED (a source test of the batch names id and
// that test failed before reaching it), NOT_OBSERVED (a source test names
// it but the report shows no failure that stopped it) and NOT_NAMED (no
// source test of the batch names it).
func notReached(id string, named []NamedTest, failedTests map[string]TestOutcome) (cause, errLine, test string) {
	cause = CauseNotNamed
	for _, t := range named {
		if !contains(t.IDs, id) {
			continue
		}
		key := t.Path + "\x00" + strings.Join(t.TitlePath, "\x00")
		if o, ok := failedTests[key]; ok {
			return CauseNotReached, o.Error, t.Path + " > " + strings.Join(t.TitlePath, " > ")
		}
		cause = CauseNotObserved
	}
	return cause, "", ""
}
