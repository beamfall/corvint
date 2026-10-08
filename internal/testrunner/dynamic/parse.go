package dynamic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func problem(o *tr.Observation, code, detail string) {
	o.Problems = append(o.Problems, tr.Problem{Code: code, Detail: detail})
}
func decode(b []byte, v any) error {
	if len(b) > tr.MaxReportBytes {
		return fmt.Errorf("report exceeds byte bound")
	}
	if err := uniqueJSON(b); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing report data")
	}
	return nil
}

// Parse retains native runner errors and refuses incomplete process/report boundaries.
func Parse(in tr.Input) (tr.Observation, error) {
	o := tr.Observation{Runner: in.Runner, RetryInformation: tr.NotReported}
	if !known(in.Runner) {
		return o, fmt.Errorf("unknown dynamic runner %q", in.Runner)
	}
	if len(in.Reports) > tr.MaxReports || len(in.Stdout) > tr.MaxReportBytes || len(in.Stderr) > tr.MaxReportBytes {
		return o, fmt.Errorf("report/output bounds exceeded")
	}
	names := make([]string, 0, len(in.Reports))
	for n, b := range in.Reports {
		if len(b) > tr.MaxReportBytes {
			return o, fmt.Errorf("report exceeds byte bound")
		}
		names = append(names, n)
	}
	sort.Strings(names)
	if in.Runner == "ava" {
		if len(names) > 0 {
			return o, fmt.Errorf("AVA profile requires native TAP stdout")
		}
		if err := parseTAP(in.Stdout, &o); err != nil {
			return o, err
		}
	} else {
		if len(names) == 0 {
			problem(&o, "missing-report", "runner produced no report")
		}
		retryStates := []string{}
		for _, n := range names {
			o.RetryInformation = tr.NotReported
			b := in.Reports[n]
			var err error
			switch in.Runner {
			case "jest", "vitest", "detox", "storybook-test-runner", "storybook-vitest":
				err = parseJest(b, &o)
			case "node-test":
				err = parseNode(b, &o)
			case "bun-test", "deno-test", "pytest":
				err = parseXML(b, &o)
			case "playwright":
				err = parsePlaywright(b, &o)
			case "rspec":
				err = parseRSpec(b, &o)
			case "unittest", "minitest", "rails-test", "test-unit":
				err = parseOwned(b, &o)
			case "testcafe":
				err = parseTestCafe(b, &o)
			case "cypress", "mocha":
				var shape struct{ Profile string }
				if e := decode(b, &shape); e != nil {
					return o, e
				}
				if shape.Profile != "" {
					err = parseOwned(b, &o)
					o.RetryInformation = tr.Retained
				} else {
					err = parseMocha(b, &o)
				}
			case "webdriverio":
				err = parseWDIO(b, &o)
			case "nightwatch":
				err = parseNightwatch(b, &o)
			case "jasmine":
				err = parseJasmine(b, &o, in)
			}
			if err != nil {
				return o, fmt.Errorf("%s: %w", n, err)
			}
			retryStates = append(retryStates, o.RetryInformation)
		}
		for _, r := range retryStates {
			if r == tr.NotReported {
				o.RetryInformation = tr.NotReported
			}
		}
	}
	seen := map[string]bool{}
	failed := false
	for _, t := range o.Tests {
		if t.ID == "" || seen[t.ID] {
			return o, fmt.Errorf("missing or duplicate test identity %q", t.ID)
		}
		seen[t.ID] = true
		if len(t.Attempts) > tr.MaxAttempts {
			return o, fmt.Errorf("attempt bound exceeded")
		}
		if t.State == tr.Failed || t.State == tr.TimedOut || t.State == tr.Interrupted {
			failed = true
		}
		switch t.State {
		case tr.Passed, tr.Failed, tr.Skipped, tr.Flaky, tr.TimedOut, tr.Interrupted, tr.Unknown:
		default:
			return o, fmt.Errorf("invalid normalized state %q", t.State)
		}
		for _, a := range t.Attempts {
			switch a.State {
			case tr.Passed, tr.Failed, tr.Skipped, tr.Flaky, tr.TimedOut, tr.Interrupted, tr.Unknown:
			default:
				return o, fmt.Errorf("invalid attempt state %q", a.State)
			}
		}
		if t.State == tr.Unknown {
			problem(&o, "unknown-state", t.ID)
		}
	}
	if len(o.Tests) > tr.MaxTests {
		return o, fmt.Errorf("test bound exceeded")
	}
	if len(o.Tests) == 0 {
		problem(&o, "no-tests", "report contains no test outcomes")
	}
	if in.ExitCode != 0 && !failed && len(o.Problems) == 0 {
		problem(&o, "unexplained-exit", fmt.Sprint(in.ExitCode))
	}
	if in.ExitCode == 0 && failed {
		problem(&o, "exit-report-conflict", "successful exit with failed test")
	}
	if in.TimedOut {
		problem(&o, "timeout", "process timed out")
	}
	if in.Interrupted {
		problem(&o, "interrupted", "process interrupted")
	}
	if in.Overflow {
		problem(&o, "output-overflow", "process output exceeded bound")
	}
	o.Complete = len(o.Problems) == 0
	return o, nil
}

func state(s string) string {
	switch s {
	case "passed", "pass", "success":
		return tr.Passed
	case "failed", "fail":
		return tr.Failed
	case "pending", "skipped", "skip", "todo", "disabled":
		return tr.Skipped
	case "timedOut", "timedout":
		return tr.TimedOut
	case "interrupted":
		return tr.Interrupted
	case "flaky":
		return tr.Flaky
	}
	return tr.Unknown
}
func add(o *tr.Observation, t tr.Test) { o.Tests = append(o.Tests, t) }

type jestAssertion struct {
	Title, FullName, Status                      string
	FailureMessages, RetryReasons, RetryMessages []string
	Invocations                                  *int
}

func parseJest(b []byte, o *tr.Observation) error {
	var r struct {
		NumTotalTests                                                 *int
		NumPassedTests, NumFailedTests, NumPendingTests, NumTodoTests *int
		Success                                                       *bool
		WasInterrupted                                                bool
		NumRuntimeErrorTestSuites                                     int
		TestResults                                                   []struct {
			Name, Status, Message string
			AssertionResults      []jestAssertion
		}
	}
	if err := decode(b, &r); err != nil {
		return err
	}
	if r.NumTotalTests == nil || r.Success == nil || r.TestResults == nil || r.NumPassedTests == nil || r.NumFailedTests == nil || r.NumPendingTests == nil || r.NumTodoTests == nil {
		return fmt.Errorf("missing native Jest/Vitest report fields")
	}
	count := 0
	passed, failed, pending, todo := 0, 0, 0, 0
	allAttempts := true
	for _, f := range r.TestResults {
		fileFailed := false
		for _, a := range f.AssertionResults {
			count++
			switch a.Status {
			case "passed":
				passed++
			case "failed":
				failed++
			case "pending", "skipped", "disabled":
				pending++
			case "todo":
				todo++
			}
			if a.Invocations != nil && (*a.Invocations < 0 || (*a.Invocations == 0 && state(a.Status) != tr.Skipped)) {
				return fmt.Errorf("invalid invocation count")
			}
			if a.Invocations != nil && len(a.RetryReasons) > 0 && len(a.RetryReasons) != *a.Invocations-1 {
				return fmt.Errorf("invalid retry history")
			}
			s := state(a.Status)
			t := tr.Test{ID: f.Name + "::" + a.FullName, Name: a.Title, File: f.Name, State: s}
			if a.FullName == "" {
				return fmt.Errorf("missing assertion fullName")
			}
			if s == tr.Failed {
				fileFailed = true
			}
			if a.Invocations != nil && *a.Invocations > 0 && *a.Invocations <= tr.MaxAttempts && len(a.RetryReasons) == *a.Invocations-1 && s != tr.Skipped {
				for _, m := range a.RetryReasons {
					t.Attempts = append(t.Attempts, tr.Attempt{State: tr.Failed, FailureKind: tr.Unknown, Message: m})
				}
				t.Attempts = append(t.Attempts, tr.Attempt{State: s, FailureKind: failureKind(s), Message: strings.Join(a.FailureMessages, "\n")})
				if *a.Invocations > 1 && s == tr.Passed {
					t.State = tr.Flaky
				}
			} else {
				allAttempts = false
				if a.Invocations != nil && *a.Invocations > tr.MaxAttempts {
					return fmt.Errorf("attempt bound exceeded")
				}
				if a.Invocations != nil && *a.Invocations > 1 {
					problem(o, "retry-history-missing", t.ID)
				}
				if s == tr.Passed && len(a.FailureMessages) > 0 {
					t.State = tr.Flaky
					problem(o, "retry-history-missing", t.ID)
				}
			}
			add(o, t)
		}
		if f.Status == "failed" && !fileFailed {
			problem(o, "collection-or-hook-error", f.Name+": "+f.Message)
		}
		if state(f.Status) == tr.Unknown {
			problem(o, "unknown-suite-state", f.Status)
		}
	}
	for _, pair := range []struct {
		n      *int
		actual int
	}{{r.NumPassedTests, passed}, {r.NumFailedTests, failed}, {r.NumPendingTests, pending}, {r.NumTodoTests, todo}} {
		if pair.n != nil && *pair.n != pair.actual {
			problem(o, "count-mismatch", "Jest/Vitest category")
		}
	}
	if *r.Success && failed > 0 {
		problem(o, "success-conflict", "Jest/Vitest success=true with failure")
	}
	if count != *r.NumTotalTests {
		problem(o, "count-mismatch", "native total differs from assertion rows")
	}
	if r.WasInterrupted {
		problem(o, "interrupted", "runner interrupted")
	}
	if r.NumRuntimeErrorTestSuites > 0 {
		problem(o, "runtime-error", "runner reported suite runtime errors")
	}
	if !*r.Success && count > 0 {
		hasFailure := false
		for _, t := range o.Tests {
			hasFailure = hasFailure || t.State == tr.Failed
		}
		if !hasFailure {
			problem(o, "runner-unsuccessful", "native report success=false")
		}
	}
	if allAttempts && count > 0 {
		o.RetryInformation = tr.Retained
	} else {
		o.RetryInformation = tr.NotReported
	}
	return nil
}
func failureKind(s string) string {
	if s == tr.Failed {
		return tr.Unknown
	}
	return ""
}

func parseRSpec(b []byte, o *tr.Observation) error {
	var r struct {
		Version  string
		Examples []struct {
			ID, Description, Status string
			File                    string `json:"file_path"`
			Exception               *struct{ Class, Message string }
		}
		Summary *struct {
			Count    int `json:"example_count"`
			Failures int `json:"failure_count"`
			Pending  int `json:"pending_count"`
			Errors   int `json:"errors_outside_of_examples_count"`
		}
	}
	if err := decode(b, &r); err != nil {
		return err
	}
	if r.Version == "" || r.Summary == nil || r.Examples == nil {
		return fmt.Errorf("missing RSpec fields")
	}
	if r.Summary.Count != len(r.Examples) {
		problem(o, "count-mismatch", "RSpec examples")
	}
	if r.Summary.Errors != 0 {
		problem(o, "outside-example-errors", fmt.Sprint(r.Summary.Errors))
	}
	failed, pending := 0, 0
	for _, e := range r.Examples {
		t := tr.Test{ID: e.ID, Name: e.Description, File: e.File, State: state(e.Status)}
		if t.State == tr.Failed {
			failed++
		}
		if t.State == tr.Skipped {
			pending++
		}
		if e.Exception != nil && t.State == tr.Passed {
			problem(o, "exception-conflict", e.ID)
		}
		add(o, t)
	}
	if failed != r.Summary.Failures || pending != r.Summary.Pending {
		problem(o, "count-mismatch", "RSpec categories")
	}
	return nil
}

func parseOwned(b []byte, o *tr.Observation) error {
	var r struct {
		Profile  string
		Complete bool
		Count    int
		Tests    []tr.Test
		Problems []tr.Problem
	}
	if err := decode(b, &r); err != nil {
		return err
	}
	want := o.Runner
	if want == "rails-test" {
		want = "minitest"
	}
	if r.Profile != "corvint-"+want+"/0" || !r.Complete || r.Tests == nil {
		return fmt.Errorf("missing or incomplete profile-owned reporter output")
	}
	if r.Count != len(r.Tests) {
		problem(o, "count-mismatch", "profile reporter")
	}
	for _, t := range r.Tests {
		if o.Runner != "cypress" && o.Runner != "mocha" {
			if len(t.Attempts) > 0 {
				return fmt.Errorf("unexpected owned-reporter attempts")
			}
		} else {
			validateAttempts(t, o)
		}
	}
	o.Tests = append(o.Tests, r.Tests...)
	o.Problems = append(o.Problems, r.Problems...)
	o.RetryInformation = tr.NotApplicable
	if o.Runner == "cypress" || o.Runner == "mocha" {
		o.RetryInformation = tr.Retained
	}
	return nil
}
