// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// pathArg is a repository file as an argument relative to root that no
// argument parser reads as a flag: a path starting with `-` gets `./`.
func pathArg(root, file string) string {
	rel := relativeTo(root, file)
	if strings.HasPrefix(rel, "-") {
		return "./" + rel
	}
	return rel
}

// jestRunner selects by exact test file path (--runTestsByPath) and reads
// the --json report written to Report through --outputFile.
type jestRunner struct{}

func (jestRunner) Name() string                         { return testid.Jest }
func (jestRunner) Lister() testinventory.ListerIdentity { return lister(testid.Jest) }
func (jestRunner) Default(Instance, func(string) bool) []string {
	return []string{"npx", "jest"}
}

// Command runs the selected files by path. --runTestsByPath takes each path
// literally, not as a pattern, so a file selection is exact; a test-level
// ID runs its whole file, since a --testNamePattern applies to every file.
func (jestRunner) Command(request Request) (Command, error) {
	if err := checkRequest(request, testid.Jest); err != nil {
		return Command{}, err
	}
	report := []string{"--json", "--outputFile=" + request.Report}
	if request.Tests == nil {
		return full(request, report...), nil
	}
	files, reasons, err := testFiles(request, "a selected test runs its whole file; --testNamePattern applies to every file")
	if err != nil {
		return Command{}, err
	}
	argv := append(append(slices.Clone(request.Full), report...), "--runTestsByPath")
	for _, file := range files {
		argv = append(argv, pathArg(request.Instance.Root, file))
	}
	return narrowed(request, argv, nil, reasons), nil
}

// Parse reads the Jest --json report (CTI-V1-004).
func (jestRunner) Parse(request ParseRequest) ([]Result, error) {
	return jestShapeResults(testid.Jest, request)
}

// testFiles is the sorted files a selection names, with reason once when
// any ID is test-level.
func testFiles(request Request, wholeFile string) (files, reasons []string, err error) {
	for _, text := range request.Tests {
		id, err := testid.Parse(text)
		if err != nil {
			return nil, nil, err
		}
		if !id.IsFileLevel() && !slices.Contains(reasons, wholeFile) {
			reasons = append(reasons, wholeFile)
		}
		if !slices.Contains(files, id.File) {
			files = append(files, id.File)
		}
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("%s: an empty selection runs nothing", request.Instance.Key())
	}
	slices.Sort(files)
	return files, reasons, nil
}

// pytestRunner selects by node ID and reads the pytest-json-report report
// (`--json-report --json-report-file`), the shape testid.PytestJSON maps
// (CTI-V1-006). The instance root is the pytest rootdir.
type pytestRunner struct{}

func (pytestRunner) Name() string                         { return testid.Pytest }
func (pytestRunner) Lister() testinventory.ListerIdentity { return lister(testid.Pytest) }
func (pytestRunner) Default(Instance, func(string) bool) []string {
	return []string{"python", "-m", "pytest"}
}

// Command names each selection as a pytest node ID relative to the instance
// root: a file, or a file and its `::` parts. A node ID selects exactly that
// node, so the selection is exact.
func (pytestRunner) Command(request Request) (Command, error) {
	if err := checkRequest(request, testid.Pytest); err != nil {
		return Command{}, err
	}
	report := []string{"--json-report", "--json-report-file=" + request.Report}
	if request.Tests == nil {
		return full(request, report...), nil
	}
	if len(request.Tests) == 0 {
		return Command{}, fmt.Errorf("%s: an empty selection runs nothing", request.Instance.Key())
	}
	nodes := []string{}
	for _, text := range request.Tests {
		id, err := testid.Parse(text)
		if err != nil {
			return Command{}, err
		}
		node := strings.Join(append([]string{pathArg(request.Instance.Root, id.File)}, id.Segments...), "::")
		if !slices.Contains(nodes, node) {
			nodes = append(nodes, node)
		}
	}
	slices.Sort(nodes)
	argv := append(append(slices.Clone(request.Full), report...), nodes...)
	return narrowed(request, argv, nil, nil), nil
}

// Parse reads the pytest-json-report report: each test's outcome and the
// summed duration of its setup, call and teardown phases under the ID
// testid.PytestNodeID gives it. A collector that failed (a file that did
// not import) is a file-level fail; a report whose exit code is not 0 or 1
// (5, no tests collected, counts as clean) with nothing failing returns a
// *ReportError with a run-level failure.
func (pytestRunner) Parse(request ParseRequest) ([]Result, error) {
	type phase struct {
		Duration float64 `json:"duration"`
	}
	var report struct {
		ExitCode   *int `json:"exitcode"`
		Collectors []struct {
			NodeID  string `json:"nodeid"`
			Outcome string `json:"outcome"`
		} `json:"collectors"`
		Tests []struct {
			NodeID   string `json:"nodeid"`
			Outcome  string `json:"outcome"`
			Setup    *phase `json:"setup"`
			Call     *phase `json:"call"`
			Teardown *phase `json:"teardown"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(request.Report, &report); err != nil {
		return nil, fmt.Errorf("pytest report: %w", err)
	}
	var results []Result
	failed := false
	for _, test := range report.Tests {
		id, err := testid.PytestNodeID(request.Instance.Root, test.NodeID)
		if err != nil {
			return nil, err
		}
		outcome := map[string]string{"passed": OutcomePass, "failed": OutcomeFail, "skipped": OutcomeSkip, "error": OutcomeError, "xfailed": OutcomeSkip, "xpassed": OutcomePass}[test.Outcome]
		if outcome == "" {
			outcome = OutcomeError
		}
		failed = failed || outcome == OutcomeFail || outcome == OutcomeError
		seconds := 0.0
		for _, p := range []*phase{test.Setup, test.Call, test.Teardown} {
			if p != nil {
				seconds += p.Duration
			}
		}
		results = append(results, Result{ID: id.String(), Outcome: outcome, DurationMillis: millis(seconds), Granularity: testid.GranularityTest})
	}
	for _, collector := range report.Collectors {
		if collector.Outcome != "failed" || collector.NodeID == "" || strings.Contains(collector.NodeID, "::") {
			continue
		}
		file := collector.NodeID
		if request.Instance.Root != "." {
			file = request.Instance.Root + "/" + file
		}
		id, err := testid.New(testid.Pytest, file)
		if err != nil {
			return nil, err
		}
		failed = true
		results = append(results, Result{ID: id.String(), Outcome: OutcomeFail, Granularity: testid.GranularityFile})
	}
	if report.ExitCode != nil && *report.ExitCode != 0 && *report.ExitCode != 1 && *report.ExitCode != 5 && !failed {
		return results, &ReportError{Failures: []string{fmt.Sprintf("pytest report: exit code %d with no failing test", *report.ExitCode)}}
	}
	return results, nil
}

// rspecRunner selects by spec file and reads the stock `--format json`
// report. Its examples carry no group chain, so results are per file with
// the worst outcome and summed run time (CTI-V1-007, CTI-V1-011).
type rspecRunner struct{}

func (rspecRunner) Name() string                         { return testid.RSpec }
func (rspecRunner) Lister() testinventory.ListerIdentity { return lister(testid.RSpec) }
func (rspecRunner) Default(Instance, func(string) bool) []string {
	return []string{"bundle", "exec", "rspec"}
}

// Command runs the selected spec files, each a path RSpec loads as given,
// so a file selection is exact; a test-level ID runs its whole file. The
// progress formatter keeps the console output and the json formatter
// writes Report.
func (rspecRunner) Command(request Request) (Command, error) {
	if err := checkRequest(request, testid.RSpec); err != nil {
		return Command{}, err
	}
	report := []string{"--format", "progress", "--format", "json", "--out", request.Report}
	if request.Tests == nil {
		return full(request, report...), nil
	}
	files, reasons, err := testFiles(request, "a selected example runs its whole file; the report names no example group chain to filter by")
	if err != nil {
		return Command{}, err
	}
	argv := append(slices.Clone(request.Full), report...)
	for _, file := range files {
		argv = append(argv, pathArg(request.Instance.Root, file))
	}
	return narrowed(request, argv, nil, reasons), nil
}

// Parse reads the RSpec json report into one file-level result per spec
// file. Errors outside examples (a spec file that did not load) return a
// *ReportError with a run-level failure.
func (rspecRunner) Parse(request ParseRequest) ([]Result, error) {
	var report struct {
		Examples []struct {
			ID       string  `json:"id"`
			FilePath string  `json:"file_path"`
			Status   string  `json:"status"`
			RunTime  float64 `json:"run_time"`
		} `json:"examples"`
		Summary struct {
			ErrorsOutsideOfExamples int `json:"errors_outside_of_examples_count"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(request.Report, &report); err != nil {
		return nil, fmt.Errorf("rspec report: %w", err)
	}
	byFile := map[string]*Result{}
	var order []string
	for _, example := range report.Examples {
		rerun := example.FilePath
		if file, _, ok := strings.Cut(example.ID, "["); ok && file != "" {
			rerun = file
		}
		rerun = strings.TrimPrefix(rerun, "./")
		if request.Instance.Root != "." {
			rerun = request.Instance.Root + "/" + rerun
		}
		id, err := testid.New(testid.RSpec, rerun)
		if err != nil {
			return nil, err
		}
		outcome := map[string]string{"passed": OutcomePass, "failed": OutcomeFail, "pending": OutcomeSkip}[example.Status]
		if outcome == "" {
			outcome = OutcomeError
		}
		result := byFile[id.String()]
		if result == nil {
			result = &Result{ID: id.String(), Outcome: outcome, Granularity: testid.GranularityFile}
			byFile[id.String()] = result
			order = append(order, id.String())
		} else if severity[outcome] > severity[result.Outcome] {
			result.Outcome = outcome
		}
		result.DurationMillis += millis(example.RunTime)
	}
	results := make([]Result, 0, len(order))
	for _, id := range order {
		results = append(results, *byFile[id])
	}
	if report.Summary.ErrorsOutsideOfExamples != 0 {
		return results, &ReportError{Failures: []string{fmt.Sprintf("rspec report: %d error(s) outside of examples", report.Summary.ErrorsOutsideOfExamples)}}
	}
	return results, nil
}
