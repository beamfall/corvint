// SPDX-License-Identifier: Apache-2.0
package jslinecov

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/testrunner"
)

type jsAssertion struct {
	FullName, Status string
	WouldRun         *bool
}
type jsFile struct {
	Name, Status     string
	AssertionResults []jsAssertion
}
type jsReport struct {
	TestResults               []jsFile
	NumTotalTests             *int
	WasInterrupted            bool
	NumRuntimeErrorTestSuites int
}

func readJSReport(raw string) (jsReport, error) {
	var r jsReport
	if err := DecodeRunnerJSON(raw, &r); err != nil {
		return r, err
	}
	if r.TestResults == nil || r.NumTotalTests == nil || *r.NumTotalTests < 0 || r.WasInterrupted || r.NumRuntimeErrorTestSuites != 0 {
		return r, fmt.Errorf("list-output-malformed")
	}
	n := 0
	for _, f := range r.TestResults {
		if f.Name == "" || f.AssertionResults == nil {
			return r, fmt.Errorf("list-output-malformed")
		}
		n += len(f.AssertionResults)
	}
	if n != *r.NumTotalTests || n > testrunner.MaxTests {
		return r, fmt.Errorf("list-output-malformed")
	}
	return r, nil
}

// Results interprets the runner's own terminal report. It does not reconcile or
// silently discard new identities: the shared T3 reconciler sees every result.
func Results(d Discovery, raw string) ([]testrunner.RunnerResult, error) {
	if err := d.Check(); err != nil {
		return nil, err
	}
	if d.Key.Runner == "node-test" {
		return nodeResults(d, raw)
	}
	// Validate the envelope separately from terminal records. A malformed
	// sibling or aggregate cannot erase a validated failure (RSP-V0-003).
	// Raw records keep unknown assertion/file forms isolated; only the closed
	// terminal-status allowlist below certifies a result.
	var report struct{ TestResults []json.RawMessage }
	if err := DecodeRunnerJSON(raw, &report); err != nil {
		return nil, err
	}
	_, reportErr := readJSReport(raw)
	remember := func(err error) {
		if reportErr == nil {
			reportErr = err
		}
	}
	_, _, collect, _ := RunnerVersion(d.Key.Runner, d.Version)
	var out []testrunner.RunnerResult
	for _, rawFile := range report.TestResults {
		var f struct {
			Name             string
			Status           json.RawMessage
			AssertionResults json.RawMessage
		}
		if err := DecodeRunnerJSON(string(rawFile), &f); err != nil {
			remember(err)
			continue
		}
		file, e := d.Relative(f.Name)
		if e != nil {
			remember(e)
			continue
		}
		var status string
		if f.Status != nil {
			if err := DecodeRunnerJSON(string(f.Status), &status); err != nil {
				remember(err)
			}
		}
		fileUnit := d.Key.Runner == "jest" && !collect
		var assertions []json.RawMessage
		if f.AssertionResults != nil {
			if err := DecodeRunnerJSON(string(f.AssertionResults), &assertions); err != nil {
				remember(err)
			}
		}
		valid := assertions != nil
		if !valid {
			remember(fmt.Errorf("list-output-malformed"))
		}
		failed := fileUnit && status == "failed"
		for _, rawAssertion := range assertions {
			var a jsAssertion
			if err := DecodeRunnerJSON(string(rawAssertion), &a); err != nil {
				remember(err)
				valid = false
				continue
			}
			if a.WouldRun != nil || !fileUnit && a.FullName == "" {
				remember(fmt.Errorf("terminal-result-unavailable"))
				valid = false
				continue
			}
			outcome, reason := a.Status, ""
			switch a.Status {
			case "passed", "failed":
			case "pending", "todo", "skipped", "disabled":
				outcome, reason = "skipped", a.Status
			default:
				remember(fmt.Errorf("terminal-result-unavailable"))
				valid = false
				continue
			}
			failed = failed || outcome == "failed"
			if !fileUnit {
				out = append(out, testrunner.RunnerResult{ID: TestID(d.Key.Runner, file, "", a.FullName), Outcome: outcome, Reason: reason})
			}
		}
		if fileUnit {
			if status != "passed" && status != "failed" {
				remember(fmt.Errorf("terminal-result-unavailable"))
				valid = false
			}
			// A known failed file or assertion suffices even when the other
			// details are not understood. A pass still needs valid details.
			if failed || valid {
				outcome := status
				if failed {
					outcome = "failed"
				}
				out = append(out, testrunner.RunnerResult{ID: TestID(d.Key.Runner, file, "", ""), Outcome: outcome})
			}
		} else if status == "failed" {
			remember(fmt.Errorf("runner-file-failed"))
		}
	}
	return out, reportErr
}

// nodeResults consumes the existing dynamic/node.mjs reporter's native events.
// Only per-file completion summaries certify a file unit; dynamic child tests
// and the aggregate summary never manufacture an identity or hide a failure.
func nodeResults(d Discovery, raw string) ([]testrunner.RunnerResult, error) {
	if len(raw) > MaxRunnerArtifact {
		return nil, fmt.Errorf("report-cap-exceeded")
	}
	var out []testrunner.RunnerResult
	final := false
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		var e struct {
			Type string
			Data struct {
				File    string
				Success *bool
			}
		}
		if err := DecodeRunnerJSON(line, &e); err != nil {
			return out, err
		}
		switch e.Type {
		case "test:pass", "test:fail":
			continue
		case "test:summary":
		default:
			return out, fmt.Errorf("terminal-result-unavailable")
		}
		if e.Data.Success == nil {
			return out, fmt.Errorf("terminal-result-unavailable")
		}
		if e.Data.File == "" {
			if final {
				return out, fmt.Errorf("duplicate-summary")
			}
			final = true
			if !*e.Data.Success {
				return out, fmt.Errorf("runner-failed")
			}
			continue
		}
		if final {
			return out, fmt.Errorf("report-after-completion")
		}
		p, err := d.Relative(e.Data.File)
		if err != nil {
			return out, err
		}
		outcome := "passed"
		if !*e.Data.Success {
			outcome = "failed"
		}
		out = append(out, testrunner.RunnerResult{ID: TestID(d.Key.Runner, p, "", ""), Outcome: outcome})
		if len(out) > testrunner.MaxTests {
			return nil, fmt.Errorf("report-cap-exceeded")
		}
	}
	if !final {
		return out, fmt.Errorf("terminal-result-unavailable")
	}
	return out, nil
}
