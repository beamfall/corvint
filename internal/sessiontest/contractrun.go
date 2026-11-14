// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testduration"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// The contract adapter (RNC-V1-004, RNC-V1-005, RNC-V1-019) runs the test
// files a plan selected for a Vitest, Jest, Playwright, pytest or RSpec
// runner instance through the compiled-in runner contract
// (internal/testrunner/contract): the contract turns the selection into the
// runner's own argv, the runner writes its machine report to a private
// file, and the contract reads one outcome and duration per canonical test
// ID from it. One unit is one selected test file in one instance: its Unit
// is the file-level canonical test ID and its Slice the instance key
// `<runner>@<root>`. Everything the adapter cannot narrow stays with the
// instance's or the language's full command.

// ContractReportPlaceholder stands for the machine report path in a planned
// contract invocation; the launch replaces it with a fresh private file.
const ContractReportPlaceholder = "{report}"

// contractRunners are the runners whose selection and report parser are
// built. Minitest runs its full command (contract.ReasonNotImplemented) and
// Go runs through the go adapter.
var contractRunners = map[string]bool{
	testid.Vitest: true, testid.Jest: true, testid.Playwright: true, testid.Pytest: true, testid.RSpec: true,
}

// contractTest is one selected test file in one owning instance.
type contractTest struct {
	instance contract.Instance
	id       string
}

// contractTests maps one selected plan unit to the test files the contract
// adapter runs, or returns false when the unit stays with its language's
// full command: on the ci profile (STR-V0-012); when the declaration has a
// command for the whole language, which covers all of its runners
// (RNC-V1-013); when the unit names no file-level test ID of a runner the
// contract narrows; or when no declared instance of that runner owns a
// file. A `typescript:<runner>:<path>` or `ruby:rspec:<stem>` unit names its
// file (CTI-V1-012); a `python:<module>` unit names a module, so its test
// files are the plan's own list for it, and a module with no
// listed test file is not narrowed. A file two instances own runs in both.
func contractTests(input Input, id string) ([]contractTest, bool) {
	language, _, _ := strings.Cut(id, ":")
	if input.RunnerProfile == ProfileCI {
		return nil, false
	}
	if _, declared := input.Declared.Command(language); declared {
		return nil, false
	}
	var ids []testid.ID
	switch language {
	case "typescript", "ruby":
		parsed, ok := testid.FromUnit(id, nil)
		if !ok {
			return nil, false
		}
		ids = append(ids, parsed)
	case "python":
		files := input.SelectedTests[id]
		if len(files) == 0 {
			return nil, false
		}
		for _, file := range files {
			parsed, err := testid.New(testid.Pytest, file)
			if err != nil {
				return nil, false
			}
			ids = append(ids, parsed)
		}
	default:
		return nil, false
	}
	var tests []contractTest
	for _, parsed := range ids {
		if !contractRunners[parsed.Runner] {
			return nil, false
		}
		owned := false
		for _, instance := range input.Instances {
			if instance.Runner == parsed.Runner && instance.Owns(parsed.String()) {
				tests, owned = append(tests, contractTest{instance: instance, id: parsed.String()}), true
			}
		}
		if !owned {
			return nil, false
		}
	}
	return tests, true
}

// placeContract adds the contract units of a narrowed plan. An instance the
// caller marked full (an inventory verdict that is not current, or the kill
// switch) runs its full command instead, with the reason; so does an
// instance whose language's full command the plan already runs. Units are
// sorted by instance, then test ID.
func (m *Manifest) placeContract(input Input, tests []contractTest) {
	full := map[string]bool{}
	for _, unit := range m.Units {
		if name, ok := strings.CutPrefix(unit.Unit, "declared:"); ok && unit.Adapter == AdapterDeclared {
			full[name] = true
		}
	}
	seen := map[string]bool{}
	var units []Unit
	for _, test := range tests {
		key := test.instance.Key()
		if full[key] {
			continue
		}
		if reason := input.FullInstances[key]; reason != "" {
			full[key] = true
			m.Units = append(m.Units, Unit{Adapter: AdapterDeclared, Reason: reason, Shard: 0, Slice: SliceWhole, Unit: "declared:" + key})
			continue
		}
		if seen[key+" "+test.id] {
			continue
		}
		seen[key+" "+test.id] = true
		unit := Unit{Adapter: AdapterContract, Shard: 0, Slice: key, Unit: test.id}
		if kind := contract.Kind(test.instance.Runner); kind != "" {
			unit.Kind, unit.KindSource = kind, contract.KindSourceRunner
		}
		units = append(units, unit)
	}
	units = slices.DeleteFunc(units, func(unit Unit) bool { return full[unit.Slice] })
	sort.Slice(units, func(i, j int) bool {
		if units[i].Slice != units[j].Slice {
			return units[i].Slice < units[j].Slice
		}
		return units[i].Unit < units[j].Unit
	})
	m.Units = append(m.Units, units...)
	for _, unit := range units {
		if tests := listedTests(input.ListedTests[unit.Slice], unit.Unit); len(tests) != 0 {
			m.Listed = append(m.Listed, ListedTests{Slice: unit.Slice, Unit: unit.Unit, Tests: tests})
		}
	}
}

// listedTests returns the canonical test IDs files lists for the test file
// whose file-level ID is file, sorted; nil when it lists none (RNC-V1-006).
func listedTests(files map[string][]string, file string) []string {
	parsed, err := testid.Parse(file)
	if err != nil {
		return nil
	}
	var tests []string
	for _, id := range files[parsed.File] {
		if id != file && testid.Matches(file, id) && !slices.Contains(tests, id) {
			tests = append(tests, id)
		}
	}
	sort.Strings(tests)
	return tests
}

// contractPlan is what a contract invocation's launch needs to derive the
// argv again with its real report path and to read the report.
type contractPlan struct {
	implementation contract.Runner
	request        contract.Request
	command        contract.Command
	// expected is what the report must answer: each selected file and
	// every test the inventory lists in it (RNC-V1-006).
	expected []string
	// unlisted are the selected files the inventory lists no test in, so
	// the report's completeness for them cannot be established.
	unlisted []string
	// refusal is why no command could be derived; the units are NOT_RUN.
	refusal string
}

// contractInvocations derives one invocation per runner instance from the
// shard's contract units, in instance order: the instance's full command
// (its entry, else its runner's, else the built-in default, RNC-V1-013)
// narrowed by the contract to the selected files, run from the instance
// root, with ContractReportPlaceholder for the report path. A selection the
// contract refuses has no argv; Execute records its units NOT_RUN.
func contractInvocations(m Manifest, declared Declared, shard int, root string) []Planned {
	listed := map[string][]string{}
	for _, entry := range m.Listed {
		listed[entry.Unit+" "+entry.Slice] = entry.Tests
	}
	var planned []Planned
	index := map[string]int{}
	for _, unit := range m.Units {
		if unit.Shard != shard || unit.Adapter != AdapterContract {
			continue
		}
		if at, seen := index[unit.Slice]; seen {
			planned[at].Units = append(planned[at].Units, unit)
			continue
		}
		index[unit.Slice] = len(planned)
		planned = append(planned, Planned{Adapter: AdapterContract, Argv: []string{}, Env: []string{}, Units: []Unit{unit}})
	}
	sort.SliceStable(planned, func(i, j int) bool { return planned[i].Units[0].Slice < planned[j].Units[0].Slice })
	for at := range planned {
		key := planned[at].Units[0].Slice
		plan := &contractPlan{}
		planned[at].contract = plan
		runner, instanceRoot, _ := strings.Cut(key, "@")
		implementation, ok := contract.For(runner)
		full, dir, _ := declared.InstanceCommand(key, root)
		if !ok || !contractRunners[runner] || len(full) == 0 {
			plan.refusal = key + ": no contract selection or full command for this runner instance"
			continue
		}
		tests := make([]string, 0, len(planned[at].Units))
		for _, unit := range planned[at].Units {
			tests = append(tests, unit.Unit)
			plan.expected = append(plan.expected, unit.Unit)
			plan.expected = append(plan.expected, listed[unit.Key()]...)
			if len(listed[unit.Key()]) == 0 {
				plan.unlisted = append(plan.unlisted, unit.Unit)
			}
		}
		plan.implementation = implementation
		plan.request = contract.Request{
			Instance: contract.Instance{Runner: runner, Root: instanceRoot}, Full: full, Tests: tests,
			Tier: contract.TierFile, Report: ContractReportPlaceholder,
		}
		if runner == testid.Jest {
			// JST-V1-008: a Jest selection narrows only in a setup the
			// contract read and supports; any other runs the full command.
			plan.request.Jest = jestSetup(root, plan.request.Instance, full)
		}
		command, err := implementation.Command(plan.request)
		if err != nil {
			plan.refusal = key + ": the contract refused the selection: " + err.Error()
			continue
		}
		plan.command = command
		planned[at].Argv, planned[at].Env = command.Argv, nonNil(command.Env)
		if dir != "." {
			planned[at].Dir = dir
		}
	}
	return planned
}

// contractUnavailable records, for each contract invocation of planned that
// needs the runner named key, an invocation that never launched: every unit
// NOT_RUN. The caller states the reason in the receipt's incomplete list.
func contractUnavailable(planned []Planned, key string, manifests map[string]AdapterManifest) []Invocation {
	var recorded []Invocation
	for _, invocation := range planned {
		if invocation.Adapter != AdapterContract || runnerKey(invocation) != key {
			continue
		}
		recorded = append(recorded, contractRecord(invocation, manifests))
	}
	return recorded
}

func contractRecord(invocation Planned, manifests map[string]AdapterManifest) Invocation {
	recorded := Invocation{
		Adapter: AdapterContract, AdapterManifest: manifests[AdapterContract].Profile, Argv: nonNil(invocation.Argv),
		Env: []string{}, Exit: -1, Outcomes: notRun(invocation.Units), Instance: invocation.Units[0].Slice,
	}
	if plan := invocation.contract; plan != nil && plan.refusal == "" {
		exact := plan.command.Exact
		recorded.Tier, recorded.Exact, recorded.ExactReason = plan.command.Tier, &exact, plan.command.Reason
	}
	return recorded
}

// contractLaunch is the argv, environment and report path of one launch:
// the planned command derived again with a fresh report file in a private
// directory, which the returned cleanup removes.
func contractLaunch(plan *contractPlan) (contract.Command, string, func(), error) {
	dir, err := os.MkdirTemp("", "corvint-test-report-")
	if err != nil {
		return contract.Command{}, "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	report := filepath.Join(dir, "report.json")
	request := plan.request
	request.Report = report
	command, err := plan.implementation.Command(request)
	if err != nil {
		cleanup()
		return contract.Command{}, "", func() {}, err
	}
	return command, report, cleanup, nil
}

// readContractReport reads the machine report the runner wrote, refusing a
// missing, non-regular or oversized one (testid.MaxReportBytes). The open
// neither follows a final symlink nor blocks on a FIFO the runner left at
// the path (openInput); the opened descriptor is checked.
func readContractReport(name string) ([]byte, error) {
	file, err := openInput(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("the runner wrote no machine report")
		}
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("the machine report is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, testid.MaxReportBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > testid.MaxReportBytes {
		return nil, testid.ErrReportTooLarge
	}
	return data, nil
}

// contractOutcomes reads one finished contract run (RNC-V1-005,
// RNC-V1-006): the report is parsed by the runner's contract and reconciled
// with the selection, so every selected test the report does not answer is
// NOT_RUN. A unit is FAIL when a result in its file failed or errored, or
// when the report records a run-level failure no result carries; NOT_RUN
// when a selected test of its file has no result; PASS when a test of its
// file passed; NO_TESTS when every one was skipped. The per-test results go
// in the returned run, and each reason makes the shard INCOMPLETE. A
// summary line is written to out.
func contractOutcomes(planned Planned, root, report string, readErr error, out io.Writer) ([]Outcome, contract.Run, []string) {
	plan := planned.contract
	key := planned.Units[0].Slice
	tests := plan.request.Tests
	var results []contract.Result
	var parseErr error
	if readErr != nil {
		parseErr = readErr
	} else {
		data, err := readContractReport(report)
		if err != nil {
			parseErr = err
		} else {
			results, parseErr = parseContractReport(plan, root, data)
		}
	}
	// Every test the inventory lists in a selected file must be answered,
	// so one the runner omits (deselected, filtered or not collected) is
	// NOT_RUN even when a sibling in its file passed (RNC-V1-006).
	run := contract.ReconcileParsed(plan.expected, results, parseErr)
	var reasons []string
	var partial *contract.ReportError
	switch {
	case parseErr != nil && !errors.As(parseErr, &partial):
		reasons = append(reasons, fmt.Sprintf("contract %s: the machine report cannot be read (%s); its %d selected test files are NOT_RUN", key, providerText(parseErr.Error()), len(tests)))
	case len(run.NotRun) != 0:
		reasons = append(reasons, fmt.Sprintf("contract %s: %d selected tests have no result in the report, so they are NOT_RUN: %s", key, len(run.NotRun), firstOf(run.NotRun)))
	case run.Incomplete:
		reasons = append(reasons, fmt.Sprintf("contract %s: the report records a file or package that did not finish, or lines it could not read", key))
	}
	if plan.request.Instance.Runner == testid.Pytest && len(plan.unlisted) != 0 {
		// PYR-V1-006: one reported test cannot stand for a whole file. A
		// plugin can deselect a sibling with no trace in the report or, under
		// `-p no:terminal`, in the output; only the inventory's collected
		// test IDs establish that every selected test ran, so a selected
		// file the inventory lists no test in fails closed.
		reasons = append(reasons, fmt.Sprintf("contract %s: %s: completeness cannot be established for %d selected files, as the inventory lists no test in them: %s", key, contract.ReasonTestsDeselected, len(plan.unlisted), firstOf(plan.unlisted)))
	}
	outcomes := make([]Outcome, 0, len(planned.Units))
	anyFail := false
	for _, unit := range planned.Units {
		outcome := unitOutcome(unit.Unit, run.Results)
		anyFail = anyFail || outcome == OutcomeFail
		recorded := Outcome{Outcome: outcome, Slice: unit.Slice, TestID: unit.Unit, Unit: unit.Unit}
		if source := contractSources[plan.request.Instance.Runner]; source != "" && outcome != OutcomeNotRun {
			measure := contractMeasure(unit.Unit, run.Results, source)
			recorded = timedOutcome(recorded, &measure)
		}
		outcomes = append(outcomes, recorded)
	}
	if run.Failed && !anyFail {
		// A run-level failure no result carries fails every unit that would
		// otherwise pass: it is never a pass.
		for index := range outcomes {
			if outcomes[index].Outcome == OutcomePass || outcomes[index].Outcome == OutcomeNoTests {
				outcomes[index].Outcome = OutcomeFail
			}
		}
	}
	counts := map[string]int{}
	for _, result := range run.Results {
		counts[result.Outcome]++
	}
	_, _ = fmt.Fprintf(out, "contract %s: %d results: %d passed, %d failed, %d errored, %d skipped, %d NOT_RUN\n", key, len(run.Results),
		counts[contract.OutcomePass], counts[contract.OutcomeFail], counts[contract.OutcomeError], counts[contract.OutcomeSkip], counts[contract.OutcomeNotRun])
	return outcomes, run, reasons
}

// parseContractReport parses data under the repository root with symbolic
// links resolved, as runners such as Jest report real paths, and again
// under the root as given when that refuses the report.
func parseContractReport(plan *contractPlan, root string, data []byte) ([]contract.Result, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	roots := []string{absolute}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil && resolved != absolute {
		roots = []string{resolved, absolute}
	}
	var results []contract.Result
	for _, candidate := range roots {
		results, err = plan.implementation.Parse(contract.ParseRequest{Instance: plan.request.Instance, Report: data, Root: candidate, Tests: plan.request.Tests, Files: plan.request.Files})
		var partial *contract.ReportError
		if err == nil || errors.As(err, &partial) {
			return results, err
		}
	}
	return results, err
}

// contractSources are the duration sources of the runners whose contract
// parser marks a test the report gave no time, or an invalid one
// (PTD-V1-002). Playwright and Go are absent: their contract parsers keep
// no such mark, so their contract outcomes stay NOT_MEASURED.
var contractSources = map[string]string{
	testid.Jest: testduration.SourceJest, testid.Pytest: testduration.SourcePytest,
	testid.RSpec: testduration.SourceRSpec, testid.Vitest: testduration.SourceVitest,
}

// contractMeasure is the duration of one selected file's unit: the sum of
// the times the report gave its tests, at file granularity (PTD-V1-002,
// PTD-V1-004). It is NOT_MEASURED when the report answers none of its
// tests, leaves one NOT_RUN, gives one no time or answers the file only
// with a file-level result that has none (PTD-V1-003), and invalid
// when one time is invalid (PTD-V1-007). The invocation's elapsed time is
// never used (PTD-V1-005).
func contractMeasure(file string, results []contract.Result, source string) testduration.Measure {
	var times []testduration.Measure
	for _, result := range results {
		if result.ID != file && !testid.Matches(file, result.ID) {
			continue
		}
		switch {
		case result.Outcome == contract.OutcomeNotRun:
			return testduration.Missing()
		case result.Granularity == testid.GranularityFile && result.DurationMissing:
			// A file-level result with no time (a Jest file that failed to
			// load, a pytest collection error) leaves the file NOT_MEASURED,
			// never a measured 0 ms (PTD-V1-003).
			return testduration.Missing()
		case result.DurationInvalid:
			times = append(times, testduration.Measure{Source: testduration.NotMeasured, Invalid: true})
		case result.DurationMissing:
			times = append(times, testduration.Missing())
		default:
			times = append(times, testduration.FromMilliseconds(float64(result.DurationMillis), source))
		}
	}
	measure, _ := testduration.Attempts(times, source)
	return measure
}

// unitOutcome folds the results of one selected file into its unit outcome.
func unitOutcome(file string, results []contract.Result) string {
	seen := map[string]bool{}
	for _, result := range results {
		if result.ID == file || testid.Matches(file, result.ID) {
			seen[result.Outcome] = true
		}
	}
	switch {
	case seen[contract.OutcomeFail] || seen[contract.OutcomeError]:
		return OutcomeFail
	case seen[contract.OutcomeNotRun] || len(seen) == 0:
		return OutcomeNotRun
	case seen[contract.OutcomePass]:
		return OutcomePass
	}
	return OutcomeNoTests
}

// ContractSelection is the instance key and the planned selection of a
// contract invocation (tier, exact flag and reason, RNC-V1-003,
// RNC-V1-011), or false for any other invocation; refusal is set instead
// when the contract refused the selection.
func (p Planned) ContractSelection() (instance string, command contract.Command, refusal string, ok bool) {
	if p.contract == nil {
		return "", contract.Command{}, "", false
	}
	return p.Units[0].Slice, p.contract.command, p.contract.refusal, true
}

// RunnerEnvironment is the environment record of one pytest contract
// invocation (PYR-V1-008): the Python and pytest versions, NOT_OBSERVED
// when neither the session header nor the discovery probe named one, and
// the nearest pytest lockfile with its SHA-256 when there is one.
type RunnerEnvironment struct {
	Python         string `json:"python"`
	Pytest         string `json:"pytest"`
	Lockfile       string `json:"lockfile,omitempty"`
	LockfileSHA256 string `json:"lockfileSha256,omitempty"`
}

// lockfileLimit bounds the lockfile bytes hashed for the environment record.
const lockfileLimit = 256 << 20

// pytestOutput passes a pytest run's stdout through to out and reads two
// kinds of line as they pass: the session header (the Python and pytest
// versions) and a deselected count. A line longer than its buffer is passed
// through unread.
type pytestOutput struct {
	out        io.Writer
	line       []byte
	python     string
	pytest     string
	deselected int
}

func (w *pytestOutput) Write(p []byte) (int, error) {
	_, _ = w.out.Write(p)
	for _, b := range p {
		if b != '\n' {
			if len(w.line) < 64<<10 {
				w.line = append(w.line, b)
			}
			continue
		}
		w.read(string(w.line))
		w.line = w.line[:0]
	}
	return len(p), nil
}

func (w *pytestOutput) read(line string) {
	if python, pytest, ok := contract.PytestHeader(line); ok && w.python == "" {
		w.python, w.pytest = python, pytest
	}
	if count, ok := contract.PytestDeselected(line); ok && count > w.deselected {
		w.deselected = count
	}
}

func (w *pytestOutput) flush() {
	if len(w.line) != 0 {
		w.read(string(w.line))
		w.line = w.line[:0]
	}
}

// pytestRecord adds what a finished pytest contract invocation reports
// beyond its results (PYR-V1-006, PYR-V1-008): the typed reason of an exit
// status that is not a test outcome, a deselected count pytest printed, and
// the environment record. version is the version the discovery probe read
// from the command's first word.
func pytestRecord(repository string, recorded *Invocation, version string, output *pytestOutput) []string {
	var reasons []string
	key := recorded.Instance
	if reason := contract.PytestExitReason(recorded.Exit); reason != "" {
		reasons = append(reasons, "contract "+key+": "+reason)
	}
	if output.deselected != 0 {
		reasons = append(reasons, fmt.Sprintf("contract %s: %s: %d collected tests were deselected by a plugin or option, so the selected files did not all run", key, contract.ReasonTestsDeselected, output.deselected))
	}
	python, pytest := output.python, output.pytest
	if python == "" {
		python = notObservedVersion
		if value, ok := strings.CutPrefix(version, "Python "); ok {
			python = value
		}
	}
	if pytest == "" {
		pytest = notObservedVersion
	}
	_, root, _ := strings.Cut(key, "@")
	lockfile, sum := lockfileRecord(repository, root)
	recorded.Environment = &RunnerEnvironment{Python: python, Pytest: pytest, Lockfile: lockfile, LockfileSHA256: sum}
	return reasons
}

// notObservedVersion is a version no output named.
const notObservedVersion = "NOT_OBSERVED"

// lockfileRecord is the nearest pytest lockfile of root (the instance root
// or the nearest directory above it, testinventory's precedence) and its
// SHA-256, or empty strings when there is none or it cannot be read.
func lockfileRecord(repository, root string) (string, string) {
	name := testinventory.NearestLockfile(func(rel string) bool {
		info, err := os.Lstat(filepath.Join(repository, filepath.FromSlash(rel)))
		return err == nil && info.Mode().IsRegular()
	}, testid.Pytest, root)
	if name == "" {
		return "", ""
	}
	file, err := openInput(filepath.Join(repository, filepath.FromSlash(name)))
	if err != nil {
		return name, ""
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		return name, ""
	}
	hash := sha256.New()
	if n, err := io.Copy(hash, io.LimitReader(file, lockfileLimit+1)); err != nil || n > lockfileLimit {
		return name, ""
	}
	return name, hex.EncodeToString(hash.Sum(nil))
}

// jestConfigNames are the config files Jest itself looks for in its root
// directory when no --config is given, in its order
// (https://jestjs.io/docs/configuration), then package.json's jest member.
var jestConfigNames = []string{"jest.config.js", "jest.config.ts", "jest.config.mjs", "jest.config.cjs", "jest.config.cts", "jest.config.mts", "jest.config.json", "package.json"}

// jestSetupLimit bounds each file jestSetup reads.
const jestSetupLimit = 1 << 20

// jestSetup reads the setup of one Jest instance for the contract
// (JST-V1-008): the installed jest version and the config the full command
// runs with, the file its --config or -c names (relative to the instance
// root) or else the first of Jest's own config files in the instance root.
// Every read is bounded and refuses a symbolic link or a non-regular file;
// what cannot be read leaves the setup unsupported, so the full command
// runs.
func jestSetup(root string, instance contract.Instance, full []string) *contract.JestSetup {
	read := func(rel string) ([]byte, error) {
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return nil, errors.New("outside the repository")
		}
		file, err := openInput(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		defer file.Close()
		if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("not a regular file")
		}
		data, err := io.ReadAll(io.LimitReader(file, jestSetupLimit+1))
		if err != nil || len(data) > jestSetupLimit {
			return nil, errors.New("unreadable or too large")
		}
		return data, nil
	}
	config := ""
	for index, word := range full {
		if value, ok := strings.CutPrefix(word, "--config="); ok {
			config = value
		} else if (word == "--config" || word == "-c") && index+1 < len(full) {
			config = full[index+1]
		}
	}
	if config != "" {
		instance.Config = path.Join(instance.Root, filepath.ToSlash(config))
	} else {
		for _, name := range jestConfigNames {
			candidate := path.Join(instance.Root, name)
			raw, err := read(candidate)
			if err != nil {
				continue
			}
			if name == "package.json" && !bytes.Contains(raw, []byte(`"jest"`)) {
				continue
			}
			instance.Config = candidate
			break
		}
	}
	setup := contract.JestSetupFrom(instance, read)
	return &setup
}
