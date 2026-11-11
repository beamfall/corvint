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

// fileSelection reduces a JS selection to the files it names, relative to
// the instance root, with the reasons it is not exact: a test-level ID runs
// its whole file (the title filter is not used), and a file filter is a
// substring or pattern match over the path, so it is exact only when the
// inventory lists no other file of the instance whose path contains it.
func fileSelection(request Request) (files, reasons []string, err error) {
	selected := []string{}
	for _, text := range request.Tests {
		id, err := testid.Parse(text)
		if err != nil {
			return nil, nil, err
		}
		if !id.IsFileLevel() {
			reasons = append(reasons, "a selected test runs its whole file; the title filter is not used")
		}
		if !slices.Contains(selected, id.File) {
			selected = append(selected, id.File)
		}
	}
	if len(selected) == 0 {
		return nil, nil, fmt.Errorf("%s: an empty selection runs nothing", request.Instance.Key())
	}
	slices.Sort(selected)
	switch {
	case request.Files == nil:
		reasons = append(reasons, "no inventory listing proves the file filter matches only the selected files")
	default:
		for _, file := range selected {
			filter := relativeTo(request.Instance.Root, file)
			for _, other := range request.Files {
				owned := request.Instance.Owns(testid.ID{Runner: request.Instance.Runner, File: other}.String())
				if other != file && owned && strings.Contains(relativeTo(request.Instance.Root, other), filter) {
					reasons = append(reasons, "the file filter "+filter+" also matches "+other)
				}
			}
		}
	}
	for _, file := range selected {
		files = append(files, relativeTo(request.Instance.Root, file))
	}
	return files, slices.Compact(reasons), nil
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
	files, reasons, err := fileSelection(request)
	if err != nil {
		return Command{}, err
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
func jestShapeResults(runner string, request ParseRequest) ([]Result, error) {
	var report struct {
		TestResults []struct {
			Name             string `json:"name"`
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
	for _, file := range report.TestResults {
		rel, err := repositoryFile(request.Root, file.Name)
		if err != nil {
			return nil, err
		}
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
			results = append(results, Result{ID: id.String(), Outcome: outcome, DurationMillis: duration, Granularity: testid.GranularityTest})
		}
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
	files, reasons, err := fileSelection(request)
	if err != nil {
		return Command{}, err
	}
	argv := append(slices.Clone(request.Full), "--reporter=json")
	for _, file := range files {
		argv = append(argv, playwrightFilter(file))
	}
	return narrowed(request, argv, env, reasons), nil
}

// playwrightFilter escapes the regular expression metacharacters of file.
func playwrightFilter(file string) string {
	var escaped strings.Builder
	for _, r := range file {
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
func (playwrightRunner) Parse(request ParseRequest) ([]Result, error) {
	var report struct {
		Config struct {
			RootDir string `json:"rootDir"`
		} `json:"config"`
		Suites []playwrightSuite `json:"suites"`
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
	return results, nil
}
