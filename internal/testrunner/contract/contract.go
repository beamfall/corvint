// SPDX-License-Identifier: Apache-2.0

// Package contract is the compiled-in runner contract of the seven blocking
// runners (docs/specs/runner-contract-v1.md). It is derived from
// registry.RunnerPlugin (RSP-V0-025): Command keeps the plugin's command
// description, now keyed by canonical test ID (CTI-V1-001) and runner
// instance (RNC-V1-014), and its Superset flag becomes the `exact` flag with
// a reason and the selection tier (RNC-V1-003); Parse keeps the plugin's
// report parsing, now one outcome and duration per canonical test ID
// (RNC-V1-005). Discovery leaves the run path: each runner names the
// lister `corvint inventory refresh` runs for it (RNC-V1-002), and every
// other command reads the committed inventory. Evidence and qualification
// stay outside the contract, as in RunnerPlugin.
//
// The package describes commands and reads reports. It starts no process,
// reads no file and writes nothing; the caller executes the command.
package contract

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// The selection tiers (owner decision R16): how the plan reached the tests.
const (
	TierLine = "line"
	TierFile = "file"
	TierFlow = "flow"
	TierFull = "full"
)

// The per-test outcomes a runner reports (RNC-V1-005) and NOT_RUN for a
// selected test the report does not name (RNC-V1-006).
const (
	OutcomePass   = "pass"
	OutcomeFail   = "fail"
	OutcomeSkip   = "skip"
	OutcomeError  = "error"
	OutcomeNotRun = "NOT_RUN"
)

// ReasonNotImplemented is printed when a runner has no selection or report
// parser yet: its full command runs instead (RNC-V1-004).
const ReasonNotImplemented = "runner-not-implemented"

// Instance is one runner instance: runner, repository-relative root
// directory and config file (RNC-V1-014). It is the instance
// testinventory.Detect declares, under the same key.
type Instance struct {
	Runner string `json:"runner"`
	Root   string `json:"root"`
	Config string `json:"config"`
}

// Key is the instance key `<runner>@<root>`, the inventory's key.
func (i Instance) Key() string { return testinventory.Key(i.Runner, i.Root) }

// Owns reports whether the canonical test ID id belongs to the instance.
func (i Instance) Owns(id string) bool { return testid.MatchesInstance(i.Runner, i.Root, id) }

// Request is one selection to turn into a command.
type Request struct {
	Instance Instance
	// Full is the instance's full command (RNC-V1-013), run from the
	// instance root; the selection argv is derived from it.
	Full []string
	// Tests are the selected canonical test IDs; nil runs the full command.
	Tests []string
	// Tier is the tier the plan selected Tests at; a nil Tests is full.
	Tier string
	// Files are the instance's test files from the committed inventory
	// (repository-relative). A file filter is exact only when no other
	// listed file can match it; nil proves nothing.
	Files []string
	// Report is the absolute path the runner writes its machine report to.
	Report string
}

// Command is the argv one selection runs, from Dir (the instance root,
// repository-relative), with Env added to the caller's environment. Exact
// is false when the argv may run more than the selection, never fewer, and
// Reason says why (RNC-V1-003).
type Command struct {
	Argv   []string `json:"argv"`
	Dir    string   `json:"dir"`
	Env    []string `json:"env,omitempty"`
	Report string   `json:"report,omitempty"`
	Tier   string   `json:"tier"`
	Exact  bool     `json:"exact"`
	Reason string   `json:"reason,omitempty"`
}

// ParseRequest is one machine report to read.
type ParseRequest struct {
	Instance Instance
	Report   []byte
	// Root is the absolute repository root the report's paths are under.
	Root string
	// Module is the module path of a Go instance's go.mod.
	Module string
}

// Result is one test's outcome and duration under its canonical test ID
// (RNC-V1-005, RNC-V1-012). Granularity is `test` unless the report names
// no finer unit than the file (or the Go package).
type Result struct {
	ID             string             `json:"testId"`
	Outcome        string             `json:"outcome"`
	DurationMillis int64              `json:"durationMillis"`
	Granularity    testid.Granularity `json:"granularity"`
}

// Runner is one runner's implementation of the contract (RNC-V1-001).
type Runner interface {
	// Name is the runner token (CTI-V1-001).
	Name() string
	// Lister is the lister `corvint inventory refresh` runs for this runner,
	// marked static or executing (RNC-V1-002).
	Lister() testinventory.ListerIdentity
	// Default is the built-in full command of instance (RNC-V1-013); exists
	// reports whether a path relative to the instance root exists.
	Default(instance Instance, exists func(rel string) bool) []string
	// Command turns a selection into argv (RNC-V1-003).
	Command(Request) (Command, error)
	// Parse reads the machine report into one result per test (RNC-V1-005).
	Parse(ParseRequest) ([]Result, error)
}

// The test kinds a runner may know (RNC-V1-007); KindSourceRunner marks a
// kind the runner itself supplied. Kind is reporting only.
const (
	KindUnit         = "unit"
	KindIntegration  = "integration"
	KindE2E          = "e2e"
	KindSourceRunner = "runner"
)

// Kind is the test kind runner knows for every test it runs, or "" when it
// knows none: only Playwright, a browser end-to-end runner, does. The other
// runners run any kind, so their tests take a kind from another source.
func Kind(runner string) string {
	if runner == testid.Playwright {
		return KindE2E
	}
	return ""
}

// ErrNotImplemented is returned by Parse for a runner whose report parser
// is not built; every selected test of such a runner is NOT_RUN.
var ErrNotImplemented = errors.New(ReasonNotImplemented)

// ReportError is returned by Parse together with the results it read when
// the report was read only in part or records a failure no result carries:
// Malformed counts unreadable lines (the run is incomplete), and Failures
// are run-level failures, such as a Jest report's `success: false` or a
// Playwright report's top-level `errors` (the run failed). ReconcileParsed
// keeps the results and applies both.
type ReportError struct {
	Malformed int
	Failures  []string
}

func (e *ReportError) Error() string {
	var parts []string
	if e.Malformed != 0 {
		parts = append(parts, fmt.Sprintf("%d report line(s) could not be read", e.Malformed))
	}
	parts = append(parts, e.Failures...)
	return "incomplete or failed report: " + strings.Join(parts, "; ")
}

var runners = map[string]Runner{
	testid.Go:         goRunner{},
	testid.Vitest:     vitestRunner{},
	testid.Playwright: playwrightRunner{},
	testid.Jest:       jestRunner{},
	testid.Pytest:     pytestRunner{},
	testid.RSpec:      rspecRunner{},
	testid.Minitest:   fullOnly{name: testid.Minitest, argv: []string{"bundle", "exec", "rake", "test"}},
}

// For returns the contract implementation of runner.
func For(runner string) (Runner, bool) {
	r, ok := runners[runner]
	return r, ok
}

// Runners returns every implementation in the spec's runner order.
func Runners() []Runner {
	out := make([]Runner, 0, len(testid.Runners))
	for _, name := range testid.Runners {
		out = append(out, runners[name])
	}
	return out
}

// lister is the inventory's lister identity for runner; the listers
// themselves live in testinventory and are not duplicated here.
func lister(runner string) testinventory.ListerIdentity {
	identity, _ := testinventory.CurrentLister(runner)
	return identity
}

// checkRequest refuses a selection the instance does not own, so one
// instance never runs, or reports on, another's tests.
func checkRequest(request Request, runner string) error {
	if request.Instance.Runner != runner || !testid.CleanFile(request.Instance.Root) {
		return fmt.Errorf("instance %s is not a %s instance", request.Instance.Key(), runner)
	}
	if len(request.Full) == 0 {
		return fmt.Errorf("%s: no full command", request.Instance.Key())
	}
	for _, id := range request.Tests {
		if !request.Instance.Owns(id) {
			return fmt.Errorf("%s does not own test %q", request.Instance.Key(), id)
		}
	}
	return nil
}

// full is the command that runs request's whole instance.
func full(request Request, extra ...string) Command {
	return Command{Argv: append(slices.Clone(request.Full), extra...), Dir: request.Instance.Root, Report: request.Report, Tier: TierFull, Exact: true}
}

// tierOf is the tier of a narrowed selection: the plan's, file when unset.
func tierOf(request Request) string {
	switch request.Tier {
	case TierLine, TierFile, TierFlow:
		return request.Tier
	}
	return TierFile
}

// fullOnly is a runner whose selection and report parser are not built: it
// supplies its lister and built-in default, and every selection runs its
// full command with ReasonNotImplemented (RNC-V1-004).
type fullOnly struct {
	name string
	argv []string
}

func (r fullOnly) Name() string                         { return r.name }
func (r fullOnly) Lister() testinventory.ListerIdentity { return lister(r.name) }

func (r fullOnly) Default(_ Instance, exists func(string) bool) []string {
	if r.name == testid.Minitest && exists != nil && exists("bin/rails") {
		return []string{"bin/rails", "test"}
	}
	return slices.Clone(r.argv)
}

func (r fullOnly) Command(request Request) (Command, error) {
	if err := checkRequest(request, r.name); err != nil {
		return Command{}, err
	}
	command := full(request)
	if request.Tests != nil {
		command.Exact, command.Reason = false, ReasonNotImplemented+": the "+r.name+" selection is not built, so its full command runs"
	}
	return command, nil
}

func (r fullOnly) Parse(ParseRequest) ([]Result, error) { return nil, ErrNotImplemented }

// Run is the reconciled outcome of one selection (RNC-V1-006): one result
// per selected test and per reported test, sorted by ID. Incomplete is set
// when a selected test has no result, a package or file did not finish
// (an error result coarser than a test), or the report was read only in
// part. Failed is set when any result fails or errors, or the report
// records a run-level failure. Exact is false when the report named a test
// more than once (Duplicates); the merged result keeps the most severe
// outcome (error, then fail, pass, skip).
type Run struct {
	Results    []Result `json:"results"`
	NotRun     []string `json:"notRun,omitempty"`
	Incomplete bool     `json:"incomplete"`
	Failed     bool     `json:"failed"`
	Exact      bool     `json:"exact"`
	Duplicates []string `json:"duplicates,omitempty"`
}

// severity orders outcomes for merging duplicate results.
var severity = map[string]int{OutcomeNotRun: 0, OutcomeSkip: 1, OutcomePass: 2, OutcomeFail: 3, OutcomeError: 4}

// Reconcile joins the selected IDs with a report's results. A test-level
// selection is answered by its own result, or by a file-level result of
// its file when the report names no test of that file; a file-level
// selection by any result in its file. A selection with no answer is
// NOT_RUN and the run is Incomplete. A nil selection is the full command:
// the run is Incomplete only when the report names nothing or a package or
// file did not finish.
func Reconcile(selected []string, results []Result) Run {
	run := Run{Results: []Result{}, Exact: true}
	byID := map[string]Result{}
	testsIn := map[string]bool{}
	duplicates := map[string]bool{}
	for _, result := range results {
		if prior, seen := byID[result.ID]; seen {
			duplicates[result.ID] = true
			if severity[result.Outcome] < severity[prior.Outcome] || (severity[result.Outcome] == severity[prior.Outcome] && result.DurationMillis <= prior.DurationMillis) {
				continue
			}
		}
		byID[result.ID] = result
		if result.Granularity == testid.GranularityTest {
			if id, err := testid.Parse(result.ID); err == nil {
				testsIn[id.FileLevel().String()] = true
			}
		}
	}
	answered := func(selection string) bool {
		id, err := testid.Parse(selection)
		if err != nil {
			return false
		}
		if _, ok := byID[selection]; ok {
			return true
		}
		file := id.FileLevel().String()
		if !id.IsFileLevel() {
			_, ok := byID[file]
			return ok && !testsIn[file]
		}
		for _, result := range results {
			if testid.Matches(selection, result.ID) {
				return true
			}
		}
		return false
	}
	for _, selection := range selected {
		if !answered(selection) {
			run.NotRun = append(run.NotRun, selection)
			byID[selection] = Result{ID: selection, Outcome: OutcomeNotRun, Granularity: granularity(selection)}
		}
	}
	unfinished := false
	for _, result := range byID {
		run.Results = append(run.Results, result)
		switch {
		case result.Outcome == OutcomeError && result.Granularity != testid.GranularityTest:
			unfinished = true
			run.Failed = true
		case result.Outcome == OutcomeFail || result.Outcome == OutcomeError:
			run.Failed = true
		}
	}
	sort.Slice(run.Results, func(i, j int) bool { return run.Results[i].ID < run.Results[j].ID })
	sort.Strings(run.NotRun)
	run.Incomplete = len(run.NotRun) != 0 || unfinished || (selected == nil && len(results) == 0)
	if len(duplicates) != 0 {
		run.Exact = false
		run.Duplicates = slices.Sorted(maps.Keys(duplicates))
	}
	return run
}

// ReconcileParsed is Reconcile over what Parse returned. A *ReportError
// keeps the results it came with: unreadable lines make the run Incomplete
// and run-level failures make it Failed. Any other error leaves the report
// unusable, so every selected test is NOT_RUN and the run is Incomplete.
func ReconcileParsed(selected []string, results []Result, err error) Run {
	var partial *ReportError
	switch {
	case err == nil:
		return Reconcile(selected, results)
	case errors.As(err, &partial):
		run := Reconcile(selected, results)
		run.Incomplete = run.Incomplete || partial.Malformed != 0
		run.Failed = run.Failed || len(partial.Failures) != 0
		return run
	}
	run := Reconcile(selected, nil)
	run.Incomplete = true
	return run
}

func granularity(id string) testid.Granularity {
	parsed, err := testid.Parse(id)
	if err != nil {
		return testid.GranularityTest
	}
	return parsed.Granularity()
}
