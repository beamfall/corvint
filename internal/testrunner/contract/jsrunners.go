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

// fileSelection reduces a JS selection to the file filters it names,
// relative to the instance root, with the reasons it is not exact: a
// test-level ID runs its whole file (the title filter is not used), and a
// file filter is a substring or pattern match over the path, so it is exact
// only when the inventory lists no other file of the instance whose path
// contains it. An empty listing proves nothing, as no listing does. A
// filter never starts with `-`, which the runner's argument parser would
// read as a flag: a leading `-` is dropped from it, which still matches the
// file as a substring, and the listing check covers the wider match. A
// file whose name is only dashes cannot be filtered, so ok is false and
// the full command runs.
func fileSelection(request Request) (files, reasons []string, ok bool, err error) {
	selected := []string{}
	for _, text := range request.Tests {
		id, err := testid.Parse(text)
		if err != nil {
			return nil, nil, false, err
		}
		if !id.IsFileLevel() {
			reasons = append(reasons, "a selected test runs its whole file; the title filter is not used")
		}
		if !slices.Contains(selected, id.File) {
			selected = append(selected, id.File)
		}
	}
	if len(selected) == 0 {
		return nil, nil, false, fmt.Errorf("%s: an empty selection runs nothing", request.Instance.Key())
	}
	slices.Sort(selected)
	for _, file := range selected {
		filter := jsFilter(relativeTo(request.Instance.Root, file))
		if filter == "" {
			return nil, nil, false, nil
		}
		files = append(files, filter)
	}
	switch {
	case len(request.Files) == 0:
		reasons = append(reasons, "no inventory listing proves the file filter matches only the selected files")
	default:
		for index, file := range selected {
			filter := files[index]
			for _, other := range request.Files {
				owned := request.Instance.Owns(testid.ID{Runner: request.Instance.Runner, File: other}.String())
				if other != file && owned && strings.Contains(relativeTo(request.Instance.Root, other), filter) {
					reasons = append(reasons, "the file filter "+filter+" also matches "+other)
				}
			}
		}
	}
	return files, slices.Compact(reasons), true, nil
}

// jsFilter is the file filter for rel: rel without leading dashes.
func jsFilter(rel string) string { return strings.TrimLeft(rel, "-") }

// unfilterable is the full command of a selection fileSelection cannot
// express, with exact false.
func unfilterable(command Command) Command {
	command.Exact, command.Reason = false, "a selected file name is only dashes, which no file filter can express, so the full command runs"
	return command
}

func narrowed(request Request, argv []string, env []string, reasons []string) Command {
	command := Command{Argv: argv, Dir: request.Instance.Root, Env: env, Report: request.Report, Tier: tierOf(request), Exact: len(reasons) == 0}
	if !command.Exact {
		command.Reason = strings.Join(reasons, "; ")
	}
	return command
}

// vitestRunner selects by file path and reads the JSON reporter's report,
// written to Report through --outputFile.
type vitestRunner struct{}

func (vitestRunner) Name() string                         { return testid.Vitest }
func (vitestRunner) Lister() testinventory.ListerIdentity { return lister(testid.Vitest) }
func (vitestRunner) Default(Instance, func(string) bool) []string {
	return []string{"npx", "vitest", "run"}
}

func (vitestRunner) Command(request Request) (Command, error) {
	if err := checkRequest(request, testid.Vitest); err != nil {
		return Command{}, err
	}
	report := []string{"--reporter=json", "--outputFile=" + request.Report}
	if request.Tests == nil {
		return full(request, report...), nil
	}
	files, reasons, ok, err := fileSelection(request)
	if err != nil {
		return Command{}, err
	}
	if !ok {
		return unfilterable(full(request, report...)), nil
	}
	argv := append(append(slices.Clone(request.Full), report...), files...)
	return narrowed(request, argv, nil, reasons), nil
}

// Parse reads the Vitest JSON reporter report, Jest's `--json` shape.
func (vitestRunner) Parse(request ParseRequest) ([]Result, error) {
	return jestShapeResults(testid.Vitest, request)
}

// jestShapeResults reads a Jest-shaped report: each assertion result's
// status and duration under the ID testid.JestTest gives it (CTI-V1-004).
// A test file whose entry has status "failed" while none of its assertions
// fails (a file that did not load, or a failing hook) is one file-level fail
// result. A report with `success: false` and no failing result returns a
// *ReportError with a run-level failure.
func jestShapeResults(runner string, request ParseRequest) ([]Result, error) {
	var report struct {
		Success     *bool `json:"success"`
		TestResults []struct {
			Name             string `json:"name"`
			Status           string `json:"status"`
			Message          string `json:"message"`
			AssertionResults []struct {
				AncestorTitles []string `json:"ancestorTitles"`
				Title          string   `json:"title"`
				Status         string   `json:"status"`
				Duration       *float64 `json:"duration"`
			} `json:"assertionResults"`
		} `json:"testResults"`
	}
	if err := json.Unmarshal(request.Report, &report); err != nil {
		return nil, fmt.Errorf("%s report: %w", runner, err)
	}
	var results []Result
	failed := false
	for _, file := range report.TestResults {
		rel, err := repositoryFile(request.Root, file.Name)
		if err != nil {
			return nil, err
		}
		fileFailed := false
		for _, assertion := range file.AssertionResults {
			id, err := testid.JestTest(runner, rel, assertion.AncestorTitles, assertion.Title)
			if err != nil {
				return nil, err
			}
			outcome := map[string]string{"passed": OutcomePass, "failed": OutcomeFail, "skipped": OutcomeSkip, "pending": OutcomeSkip, "todo": OutcomeSkip, "disabled": OutcomeSkip}[assertion.Status]
			if outcome == "" {
				outcome = OutcomeError
			}
			duration := int64(0)
			if assertion.Duration != nil {
				duration = int64(*assertion.Duration + 0.5)
			}
			if outcome == OutcomeFail || outcome == OutcomeError {
				fileFailed = true
			}
			results = append(results, Result{ID: id.String(), Outcome: outcome, DurationMillis: duration, Granularity: testid.GranularityTest})
		}
		if file.Status == "failed" && !fileFailed {
			id, err := testid.New(runner, rel)
			if err != nil {
				return nil, err
			}
			fileFailed = true
			results = append(results, Result{ID: id.String(), Outcome: OutcomeFail, Granularity: testid.GranularityFile})
		}
		failed = failed || fileFailed
	}
	if report.Success != nil && !*report.Success && !failed {
		return results, &ReportError{Failures: []string{runner + " report: success is false but no test or file failed"}}
	}
	return results, nil
}

// repositoryFile is a reported path, absolute or relative to root, as a
// clean repository-relative slash path.
func repositoryFile(root, reported string) (string, error) {
	root = strings.TrimSuffix(strings.ReplaceAll(root, `\`, "/"), "/")
	reported = strings.ReplaceAll(reported, `\`, "/")
	if rel, ok := strings.CutPrefix(reported, root+"/"); ok && root != "" {
		reported = rel
	}
	if !testid.CleanFile(reported) || reported == "." {
		return "", fmt.Errorf("test file %q is not under the repository root %q", reported, root)
	}
	return reported, nil
}

// playwrightRunner selects by file filter and reads the JSON reporter's
// report, written to Report through PLAYWRIGHT_JSON_OUTPUT_NAME.
type playwrightRunner struct{}

func (playwrightRunner) Name() string                         { return testid.Playwright }
func (playwrightRunner) Lister() testinventory.ListerIdentity { return lister(testid.Playwright) }
func (playwrightRunner) Default(Instance, func(string) bool) []string {
	return []string{"npx", "playwright", "test"}
}

// Command appends each selected file as a Playwright filter, a regular
// expression over the file path, with its metacharacters escaped.
func (playwrightRunner) Command(request Request) (Command, error) {
	if err := checkRequest(request, testid.Playwright); err != nil {
		return Command{}, err
	}
	env := []string{"PLAYWRIGHT_JSON_OUTPUT_NAME=" + request.Report}
	if request.Tests == nil {
		command := full(request, "--reporter=json")
		command.Env = env
		return command, nil
	}
	files, reasons, ok, err := fileSelection(request)
	if err != nil {
		return Command{}, err
	}
	if !ok {
		command := unfilterable(full(request, "--reporter=json"))
		command.Env = env
		return command, nil
	}
	argv := append(slices.Clone(request.Full), "--reporter=json")
	for _, file := range files {
		argv = append(argv, regexpLiteral(file))
	}
	return narrowed(request, argv, env, reasons), nil
}

// regexpLiteral escapes every regular expression metacharacter of text, so
// a JavaScript regular expression (a Playwright file filter, a Jest -t
// pattern) matches it literally.
func regexpLiteral(text string) string {
	var escaped strings.Builder
	for _, r := range text {
		if strings.ContainsRune(`\^$.|?*+()[]{}`, r) {
			escaped.WriteByte('\\')
		}
		escaped.WriteRune(r)
	}
	return escaped.String()
}

type playwrightSuite struct {
	Title string `json:"title"`
	File  string `json:"file"`
	Specs []struct {
		Title string `json:"title"`
		Tests []struct {
			ProjectName string `json:"projectName"`
			Status      string `json:"status"`
			Results     []struct {
				Duration float64 `json:"duration"`
			} `json:"results"`
		} `json:"tests"`
	} `json:"specs"`
	Suites []playwrightSuite `json:"suites"`
}

// Parse reads the Playwright JSON report: one result per spec and project
// under testid.PlaywrightTest (CTI-V1-005), with the test's final status
// and the summed duration of its attempts. A flaky test passed on retry.
// Top-level `errors` return a *ReportError with each as a run-level failure.
func (playwrightRunner) Parse(request ParseRequest) ([]Result, error) {
	var report struct {
		Config struct {
			RootDir string `json:"rootDir"`
		} `json:"config"`
		Suites []playwrightSuite `json:"suites"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(request.Report, &report); err != nil {
		return nil, fmt.Errorf("playwright report: %w", err)
	}
	var results []Result
	var walk func(file string, describes []string, suite playwrightSuite) error
	walk = func(file string, describes []string, suite playwrightSuite) error {
		for _, spec := range suite.Specs {
			for _, test := range spec.Tests {
				id, err := testid.PlaywrightTest(file, test.ProjectName, describes, spec.Title)
				if err != nil {
					return err
				}
				outcome := map[string]string{"expected": OutcomePass, "flaky": OutcomePass, "unexpected": OutcomeFail, "skipped": OutcomeSkip}[test.Status]
				if outcome == "" {
					outcome = OutcomeError
				}
				total := 0.0
				for _, attempt := range test.Results {
					total += attempt.Duration
				}
				results = append(results, Result{ID: id.String(), Outcome: outcome, DurationMillis: int64(total + 0.5), Granularity: testid.GranularityTest})
			}
		}
		for _, child := range suite.Suites {
			if err := walk(file, append(slices.Clone(describes), child.Title), child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, suite := range report.Suites {
		reported := suite.File
		if !strings.HasPrefix(strings.ReplaceAll(reported, `\`, "/"), "/") && report.Config.RootDir != "" {
			reported = strings.TrimSuffix(report.Config.RootDir, "/") + "/" + reported
		}
		file, err := repositoryFile(request.Root, reported)
		if err != nil {
			return nil, err
		}
		if err := walk(file, nil, suite); err != nil {
			return nil, err
		}
	}
	if len(report.Errors) != 0 {
		// A top-level error (a file that did not load, a failing global
		// setup) belongs to no test: the run failed.
		failures := make([]string, 0, len(report.Errors))
		for _, e := range report.Errors {
			failures = append(failures, "playwright report error: "+firstLine(e.Message))
		}
		return results, &ReportError{Failures: failures}
	}
	return results, nil
}

// firstLine is the first line of message.
func firstLine(message string) string {
	line, _, _ := strings.Cut(message, "\n")
	return line
}
