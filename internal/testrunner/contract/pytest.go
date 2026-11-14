// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/testduration"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// The typed reasons a runner run that is not a clean set of outcomes
// carries (PYR-V1-006, PYR-V1-009). Each starts the line the receipt names.
const (
	// ReasonRunnerUnavailable: the interpreter or the runner cannot be
	// started or imported; nothing ran for the instance.
	ReasonRunnerUnavailable = "runner-unavailable"
	// ReasonNoTestsCollected: the runner collected none of the selected
	// tests (pytest exit status 5).
	ReasonNoTestsCollected = "no-tests-collected"
	// ReasonTestsDeselected: a plugin or option deselected collected tests
	// of a narrowed selection, so some selected tests did not run.
	ReasonTestsDeselected = "tests-deselected"
	// ReasonReportInvalid: the machine report is missing, oversized or
	// cannot be read.
	ReasonReportInvalid = "runner-report-invalid"
	// ReasonRunnerExit: the runner ended with an exit status that is not a
	// test outcome (pytest 2, 3, 4 or one outside 0 to 5).
	ReasonRunnerExit = "runner-exit"
	// ReasonKillSwitch: CORVINT_SELECTION forced the runner's full suite.
	ReasonKillSwitch = "kill-switch"
)

// pytestRunner selects by node ID and reads pytest's own JUnit XML report
// (`--junitxml`), which needs no plugin (PYR-V1-003, PYR-V1-004). The
// instance root is the pytest rootdir. Parse still reads a
// pytest-json-report document (CTI-V1-006) when one is given.
type pytestRunner struct{}

func (pytestRunner) Name() string                         { return testid.Pytest }
func (pytestRunner) Lister() testinventory.ListerIdentity { return lister(testid.Pytest) }
func (pytestRunner) Default(Instance, func(string) bool) []string {
	return []string{"python", "-m", "pytest"}
}

// PytestReportArgs are the arguments that make pytest write the JUnit XML
// report to report: rooted at the instance root, in the xunit1 family,
// whose testcase carries the file a node ID needs, with each testcase's
// time the call phase alone (PYR-V1-004). An `-o` value overrides the
// project's ini value; a later --junitxml wins over one in addopts. Option
// names follow the pytest reference
// (https://docs.pytest.org/en/stable/reference/reference.html, not run
// against a live pytest here: NOT_VERIFIED).
func PytestReportArgs(report string) []string {
	return []string{"--rootdir=.", "--junitxml=" + report, "-o", "junit_family=xunit1", "-o", "junit_duration_report=call"}
}

// Command names each selection as a pytest node ID relative to the instance
// root: a file, or a file and its `::` parts. A node ID selects exactly that
// node, so the selection is exact; plugins that deselect are caught from
// the run's output (PytestDeselected).
func (pytestRunner) Command(request Request) (Command, error) {
	if err := checkRequest(request, testid.Pytest); err != nil {
		return Command{}, err
	}
	report := PytestReportArgs(request.Report)
	if request.Tests == nil {
		return full(request, report...), nil
	}
	nodes, err := PytestNodes(request.Instance, request.Tests)
	if err != nil {
		return Command{}, err
	}
	argv := append(append(slices.Clone(request.Full), report...), nodes...)
	return narrowed(request, argv, nil, nil), nil
}

// PytestNodes is the sorted, distinct pytest node IDs of tests, each
// relative to the instance root; a path starting with `-` is passed as
// `./path`. An empty selection is refused: it would run the whole suite.
func PytestNodes(instance Instance, tests []string) ([]string, error) {
	if len(tests) == 0 {
		return nil, fmt.Errorf("%s: an empty selection runs nothing", instance.Key())
	}
	nodes := []string{}
	for _, text := range tests {
		id, err := testid.Parse(text)
		if err != nil {
			return nil, err
		}
		node := strings.Join(append([]string{pathArg(instance.Root, id.File)}, id.Segments...), "::")
		if !slices.Contains(nodes, node) {
			nodes = append(nodes, node)
		}
	}
	slices.Sort(nodes)
	return nodes, nil
}

// Parse reads a pytest report: the JUnit XML report Command asks for, or a
// pytest-json-report document.
func (pytestRunner) Parse(request ParseRequest) ([]Result, error) {
	if trimmed := bytes.TrimLeft(request.Report, " \t\r\n\ufeff"); len(trimmed) != 0 && trimmed[0] == '<' {
		return parsePytestJUnit(request)
	}
	return parsePytestJSON(request)
}

// The pytest-native outcomes a Result's RunnerOutcome names (PYR-V1-004).
const (
	PytestPassed  = "passed"
	PytestFailed  = "failed"
	PytestSkipped = "skipped"
	PytestError   = "error"
	PytestXfailed = "xfailed"
	PytestXpassed = "xpassed"
)

type junitDetail struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
}

type junitCase struct {
	ClassName *string       `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	File      string        `xml:"file,attr"`
	Time      string        `xml:"time,attr"`
	Failures  []junitDetail `xml:"failure"`
	Errors    []junitDetail `xml:"error"`
	Skipped   []junitDetail `xml:"skipped"`
}

type junitSuite struct {
	Cases  []junitCase  `xml:"testcase"`
	Suites []junitSuite `xml:"testsuite"`
}

// parsePytestJUnit reads pytest's JUnit XML (xunit1 family). A testcase's
// node ID is its file, then the classname parts after the file's dotted
// module name, then its name with any parameter part; pytest builds both
// from the node ID (junitxml.mangle_test_address), so the mapping is exact.
// A testcase with an empty classname is a collector: a file that failed to
// import is a file-level error (its tests did not run), one skipped at
// module level a file-level skip. A testcase whose classname does not start
// with its file's module name, or with no file, is refused: the report was
// not written with the xunit1 family or a --junitprefix renamed it.
func parsePytestJUnit(request ParseRequest) ([]Result, error) {
	var root struct {
		XMLName xml.Name
		junitSuite
	}
	if err := xml.Unmarshal(request.Report, &root); err != nil {
		return nil, fmt.Errorf("pytest junit report: %w", err)
	}
	var cases []junitCase
	switch root.XMLName.Local {
	case "testsuites", "testsuite":
		var walk func(suite junitSuite, depth int) error
		walk = func(suite junitSuite, depth int) error {
			if depth > 4 {
				return fmt.Errorf("pytest junit report: testsuite nested deeper than 4")
			}
			cases = append(cases, suite.Cases...)
			for _, child := range suite.Suites {
				if err := walk(child, depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(root.junitSuite, 0); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("pytest junit report: root element %q is not testsuites or testsuite", root.XMLName.Local)
	}
	modules := pytestModules(request)
	results := make([]Result, 0, len(cases))
	for _, test := range cases {
		result, err := pytestJUnitResult(request.Instance.Root, modules, test)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// pytestCandidates are the modules a JUnit classname is split against:
// each the dotted module name relative to the instance root (the rootdir)
// and its root-relative file. The first selected are the selected files';
// the rest are the inventory's other files, a fallback.
type pytestCandidates struct {
	modules  [][2]string
	selected int
}

// pytestModules reads the candidate modules of a parse request.
func pytestModules(request ParseRequest) pytestCandidates {
	root := request.Instance.Root
	var candidates pytestCandidates
	seen := map[string]bool{}
	add := func(file string) {
		if root != "." {
			rel, ok := strings.CutPrefix(file, root+"/")
			if !ok {
				return
			}
			file = rel
		}
		if !strings.HasSuffix(file, ".py") || seen[file] {
			return
		}
		seen[file] = true
		candidates.modules = append(candidates.modules, [2]string{strings.ReplaceAll(strings.TrimSuffix(file, ".py"), "/", "."), file})
	}
	for _, text := range request.Tests {
		if id, err := testid.Parse(text); err == nil {
			add(id.File)
		}
	}
	candidates.selected = len(candidates.modules)
	for _, file := range request.Files {
		add(file)
	}
	return candidates
}

// pytestClassModule is the file a testcase's classname names and the
// classname parts after that file's module: among the selected files'
// modules first, then the inventory's, then the module of the testcase's
// own file attribute. In xunit1 that attribute is the file that defines the
// test, which for an inherited method or an imported test function is not
// the module pytest collected it from; the classname is built from the
// collecting node ID (junitxml.mangle_test_address), so its module is the
// file the test runs under. Two candidate modules of one group that both
// prefix the classname are ambiguous and refused, as is a classname no
// module prefixes.
func pytestClassModule(class, file string, modules [][2]string, selected int) (string, []string, error) {
	match := func(module string) ([]string, bool) {
		switch {
		case class == module:
			return nil, true
		case strings.HasPrefix(class, module+"."):
			return strings.Split(class[len(module)+1:], "."), true
		}
		return nil, false
	}
	groups := [][][2]string{modules[:selected], modules[selected:]}
	for _, group := range groups {
		found, segments, count := "", []string(nil), 0
		for _, candidate := range group {
			if parts, ok := match(candidate[0]); ok {
				found, segments, count = candidate[1], parts, count+1
			}
		}
		if count > 1 {
			return "", nil, fmt.Errorf("pytest junit report: testcase classname %q starts with more than one candidate module", class)
		}
		if count == 1 {
			return found, segments, nil
		}
	}
	module := strings.ReplaceAll(strings.TrimSuffix(file, ".py"), "/", ".")
	if parts, ok := match(module); ok {
		return file, parts, nil
	}
	return "", nil, fmt.Errorf("pytest junit report: testcase classname %q does not start with module %q of file %s or of any selected or inventoried file", class, module, file)
}

func pytestJUnitResult(root string, modules pytestCandidates, test junitCase) (Result, error) {
	file := strings.TrimPrefix(strings.ReplaceAll(test.File, `\`, "/"), "./")
	collector := test.ClassName == nil || *test.ClassName == ""
	if file == "" && collector {
		// A collector record from a writer that left out the file: its
		// name is the module's dotted path (pytest's mangling).
		file = strings.ReplaceAll(test.Name, ".", "/") + ".py"
	}
	if file == "" {
		return Result{}, fmt.Errorf("pytest junit report: testcase %q names no file (want junit_family=xunit1)", test.Name)
	}
	outcome, native := OutcomePass, PytestPassed
	switch {
	case len(test.Errors) != 0:
		outcome, native = OutcomeError, PytestError
	case len(test.Failures) != 0 && strings.HasPrefix(test.Failures[0].Message, "[XPASS(strict)]"):
		outcome, native = OutcomeFail, PytestXpassed
	case len(test.Failures) != 0:
		outcome, native = OutcomeFail, PytestFailed
	case len(test.Skipped) != 0 && test.Skipped[0].Type == "pytest.xfail":
		outcome, native = OutcomeSkip, PytestXfailed
	case len(test.Skipped) != 0 && test.Skipped[0].Message == "xfail-marked test passes unexpectedly":
		outcome, native = OutcomePass, PytestXpassed
	case len(test.Skipped) != 0:
		outcome, native = OutcomeSkip, PytestSkipped
	}
	if collector {
		id, err := testid.New(testid.Pytest, path.Join(root, file))
		if err != nil {
			return Result{}, fmt.Errorf("pytest junit report: collector %q: %w", test.Name, err)
		}
		// PTD-V1-003: a collector's record times no test; it is never a
		// measured 0 ms.
		return Result{ID: id.String(), Outcome: outcome, Granularity: testid.GranularityFile, RunnerOutcome: native}.timed(testduration.Missing()), nil
	}
	file, segments, err := pytestClassModule(*test.ClassName, file, modules.modules, modules.selected)
	if err != nil {
		return Result{}, err
	}
	segments = append(segments, test.Name)
	id, err := testid.New(testid.Pytest, path.Join(root, file), segments...)
	if err != nil {
		return Result{}, fmt.Errorf("pytest junit report: testcase %q: %w", test.Name, err)
	}
	// PTD-V1-002, PTD-V1-003, PTD-V1-007: the testcase time attribute in
	// seconds; absent is not measured, and one that is not a valid duration
	// is invalid, never a refusal of the report.
	measure := testduration.FromSecondsText(test.Time, testduration.SourcePytest)
	return Result{ID: id.String(), Outcome: outcome, Granularity: testid.GranularityTest, RunnerOutcome: native}.timed(measure), nil
}

// parsePytestJSON reads the pytest-json-report report: each test's outcome
// and the summed duration of its setup, call and teardown phases under the
// ID testid.PytestNodeID gives it. A collector that failed (a file that did
// not import) is a file-level fail; a report whose exit code is not 0 or 1
// (5, no tests collected, counts as clean) with nothing failing returns a
// *ReportError with a run-level failure.
func parsePytestJSON(request ParseRequest) ([]Result, error) {
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
		seconds, phases := 0.0, 0
		for _, p := range []*phase{test.Setup, test.Call, test.Teardown} {
			if p != nil {
				seconds += p.Duration
				phases++
			}
		}
		measure := testduration.Missing()
		if phases != 0 {
			measure = testduration.FromSeconds(seconds, testduration.SourcePytest)
		}
		results = append(results, Result{ID: id.String(), Outcome: outcome, Granularity: testid.GranularityTest}.timed(measure))
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
		results = append(results, Result{ID: id.String(), Outcome: OutcomeFail, Granularity: testid.GranularityFile}.timed(testduration.Missing()))
	}
	if report.ExitCode != nil && *report.ExitCode != 0 && *report.ExitCode != 1 && *report.ExitCode != 5 && !failed {
		return results, &ReportError{Failures: []string{fmt.Sprintf("pytest report: exit code %d with no failing test", *report.ExitCode)}}
	}
	return results, nil
}

// PytestExitReason is the typed reason of a pytest exit status that is not
// a test outcome, or "" for 0 (all passed) and 1 (some failed). pytest's
// documented statuses are 0 to 5
// (https://docs.pytest.org/en/stable/reference/exit-codes.html, NOT_VERIFIED
// against a live pytest here).
func PytestExitReason(code int) string {
	switch code {
	case 0, 1:
		return ""
	case 2:
		return ReasonRunnerExit + ": pytest exit status 2 (interrupted)"
	case 3:
		return ReasonRunnerExit + ": pytest exit status 3 (internal error)"
	case 4:
		return ReasonRunnerExit + ": pytest exit status 4 (usage error)"
	case 5:
		return ReasonNoTestsCollected + ": pytest exit status 5 (no tests collected)"
	}
	return fmt.Sprintf("%s: pytest exit status %d", ReasonRunnerExit, code)
}

var (
	pytestVersionLine = regexp.MustCompile(`^(?:pytest (\S+)|This is pytest version ([^,\s]+))`)
	pytestHeaderLine  = regexp.MustCompile(`^platform \S+ -- Python (\S+), pytest-([^,\s]+)`)
	pytestDeselected  = regexp.MustCompile(`(?:^|[\s,(=])(\d+) deselected\b`)
)

// PytestVersion reads the pytest version from `pytest --version` output:
// "pytest X" (pytest 7 and later) or "This is pytest version X, ..."
// (earlier releases), or "" when neither line is present.
func PytestVersion(output []byte) string {
	for _, line := range strings.Split(string(output), "\n") {
		if match := pytestVersionLine.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			return match[1] + match[2]
		}
	}
	return ""
}

// PytestHeader reads the Python and pytest versions from the session header
// line pytest prints first ("platform linux -- Python 3.12.1, pytest-8.3.2,
// ..."); ok is false for any other line.
func PytestHeader(line string) (python, pytest string, ok bool) {
	match := pytestHeaderLine.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
}

// PytestDeselected reads the count pytest prints for deselected items on
// its collection line ("collected 4 items / 1 deselected / 3 selected") or
// its summary line ("3 passed, 1 deselected in 0.05s"); ok is false when
// the line names none.
func PytestDeselected(line string) (int, bool) {
	match := pytestDeselected.FindStringSubmatch(line)
	if match == nil {
		return 0, false
	}
	count, err := strconv.Atoi(match[1])
	return count, err == nil && count > 0
}
