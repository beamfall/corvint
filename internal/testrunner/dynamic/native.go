package dynamic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

type xmlCase struct {
	Name     string `xml:"name,attr"`
	Class    string `xml:"classname,attr"`
	File     string `xml:"file,attr"`
	Failures []struct {
		Message string `xml:"message,attr"`
	} `xml:"failure"`
	Errors []struct {
		Message string `xml:"message,attr"`
	} `xml:"error"`
	Skipped *struct{} `xml:"skipped"`
}
type xmlSuite struct {
	Name     string     `xml:"name,attr"`
	Errors   *int       `xml:"errors,attr"`
	Failures *int       `xml:"failures,attr"`
	Skipped  *int       `xml:"skipped,attr"`
	Disabled *int       `xml:"disabled,attr"`
	Tests    *int       `xml:"tests,attr"`
	Cases    []xmlCase  `xml:"testcase"`
	Suites   []xmlSuite `xml:"testsuite"`
}

func parseXML(b []byte, o *tr.Observation) error {
	if err := closedXML(b); err != nil {
		return err
	}
	var root struct {
		XMLName xml.Name
		xmlSuite
	}
	if err := xml.Unmarshal(b, &root); err != nil {
		return err
	}
	if root.XMLName.Local != "testsuites" && root.XMLName.Local != "testsuite" {
		return fmt.Errorf("not a native JUnit report")
	}
	var walk func(xmlSuite) int
	walk = func(s xmlSuite) int {
		n := len(s.Cases)
		start := len(o.Tests)
		if len(s.Cases) > 0 && s.Tests == nil {
			problem(o, "missing-count", s.Name)
		}
		for _, c := range s.Cases {
			st := tr.Passed
			if c.Skipped != nil {
				st = tr.Skipped
			}
			if len(c.Failures) > 0 {
				st = tr.Failed
			}
			if len(c.Errors) > 0 {
				st = tr.Unknown
				problem(o, "setup-or-runtime-error", c.Name)
			}
			if c.Name == "" {
				problem(o, "missing-name", s.Name)
			}
			if c.Skipped != nil && (len(c.Errors) > 0 || len(c.Failures) > 0) {
				problem(o, "conflicting-outcomes", c.Name)
			}
			id := s.Name + "::" + c.Class + "::" + c.Name
			add(o, tr.Test{ID: id, Name: c.Name, File: c.File, Suite: s.Name, State: st})
		}
		for _, child := range s.Suites {
			n += walk(child)
		}
		fail, errs, skip := 0, 0, 0
		for _, t := range o.Tests[start:] {
			switch t.State {
			case tr.Failed:
				fail++
			case tr.Unknown:
				errs++
			case tr.Skipped:
				skip++
			}
		}
		for _, pair := range []struct {
			declared *int
			actual   int
		}{{s.Errors, errs}, {s.Failures, fail}, {s.Skipped, skip}, {s.Disabled, skip}} {
			if pair.declared != nil && *pair.declared != pair.actual {
				problem(o, "count-mismatch", s.Name)
			}
		}
		if s.Tests != nil && *s.Tests != n {
			problem(o, "count-mismatch", s.Name)
		}
		return n
	}
	walk(root.xmlSuite)
	return nil
}

func parseNode(b []byte, o *tr.Observation) error {
	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Buffer(make([]byte, 4096), tr.MaxReportBytes)
	final := false
	reported := 0
	for scanner.Scan() {
		var e struct {
			Type string
			Data struct {
				Name, File                        string
				Line, Column, TestNumber, Nesting int
				Skip, Todo                        json.RawMessage
				Success                           bool
				Counts                            struct{ Tests, Failed, Passed, Cancelled, Skipped, Todo int }
				Details                           struct {
					Type                     string
					Error                    *struct{ Message, FailureType string }
					Attempt, PassedOnAttempt int `json:"-"`
				}
			}
		}
		if final {
			return fmt.Errorf("Node event after final summary")
		}
		if err := decode(scanner.Bytes(), &e); err != nil {
			return err
		}
		switch e.Type {
		case "test:pass", "test:fail":
			if e.Data.Details.Type == "suite" {
				if e.Type == "test:fail" || e.Data.Details.Error != nil {
					explained := false
					if e.Data.Details.Error != nil && e.Data.Details.Error.FailureType == "subtestsFailed" {
						for _, t := range o.Tests {
							explained = explained || t.State == tr.Failed || t.State == tr.Interrupted || t.State == tr.TimedOut
						}
					}
					if !explained {
						problem(o, "node-suite-error", e.Data.Name)
					}
				}
				continue
			}
			if e.Data.Details.Type != "test" {
				problem(o, "unknown-node-event-kind", e.Data.Name)
			}
			st := tr.Passed
			if e.Type == "test:fail" {
				st = tr.Failed
			}
			if meaningful(e.Data.Skip) || meaningful(e.Data.Todo) {
				st = tr.Skipped
			}
			if e.Data.Details.Error != nil {
				if e.Type == "test:pass" && !meaningful(e.Data.Todo) {
					problem(o, "case-error-conflict", e.Data.Name)
				}
				switch e.Data.Details.Error.FailureType {
				case "testCodeFailure":
				case "testTimeoutFailure":
					st = tr.TimedOut
				case "cancelledByParent":
					st = tr.Interrupted
				default:
					st = tr.Unknown
					problem(o, "node-runtime-error", e.Data.Details.Error.Message)
				}
			}
			id := fmt.Sprintf("%s:%d:%d:%d:%s", e.Data.File, e.Data.Line, e.Data.Column, e.Data.TestNumber, e.Data.Name)
			add(o, tr.Test{ID: id, Name: e.Data.Name, File: e.Data.File, State: st})
		case "test:summary":
			if e.Data.File == "" {
				final = true
				reported = e.Data.Counts.Tests
				p, f, c, sk := 0, 0, 0, 0
				for _, t := range o.Tests {
					switch t.State {
					case tr.Passed:
						p++
					case tr.Failed, tr.TimedOut:
						f++
					case tr.Interrupted:
						c++
					case tr.Skipped:
						sk++
					}
				}
				counts := e.Data.Counts
				if counts.Passed != p || counts.Failed != f || counts.Cancelled != c || counts.Skipped+counts.Todo != sk || counts.Skipped < 0 || counts.Todo < 0 {
					problem(o, "count-mismatch", "Node categories")
				}
				if e.Data.Success && (f+c > 0 || len(o.Problems) > 0) {
					problem(o, "success-conflict", "Node successful summary with errors")
				}
				if !e.Data.Success {
					hasFail := false
					for _, t := range o.Tests {
						hasFail = hasFail || t.State == tr.Failed || t.State == tr.TimedOut || t.State == tr.Interrupted
					}
					if !hasFail {
						problem(o, "node-run-unsuccessful", "summary success=false")
					}
				}
			}
		default:
			return fmt.Errorf("unexpected node reporter event %q", e.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !final {
		problem(o, "missing-summary", "Node final summary absent")
	}
	if final && reported != len(o.Tests) {
		problem(o, "count-mismatch", "Node summary")
	}
	return nil
}
func meaningful(b json.RawMessage) bool {
	return len(b) > 0 && string(b) != "false" && string(b) != "null" && string(b) != `""`
}

func parseTAP(b []byte, o *tr.Observation) error {
	if len(b) == 0 {
		return fmt.Errorf("missing TAP stdout")
	}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Buffer(make([]byte, 4096), tr.MaxReportBytes)
	plan := -1
	number := 0
	version := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "TAP version 13" {
			version = true
			continue
		}
		if strings.HasPrefix(line, "Bail out!") {
			problem(o, "tap-bailout", line)
			continue
		}
		if strings.HasPrefix(line, "1..") {
			if plan >= 0 {
				return fmt.Errorf("duplicate TAP plan")
			}
			v := strings.Fields(strings.TrimPrefix(line, "1.."))
			if len(v) == 0 {
				return fmt.Errorf("invalid TAP plan")
			}
			var err error
			plan, err = strconv.Atoi(v[0])
			if err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "not ok ") {
			st := tr.Passed
			body := strings.TrimPrefix(line, "ok ")
			if strings.HasPrefix(line, "not ok ") {
				st = tr.Failed
				body = strings.TrimPrefix(line, "not ok ")
			}
			parts := strings.SplitN(body, " - ", 2)
			if len(parts) != 2 {
				return fmt.Errorf("unrecognized AVA TAP test line")
			}
			n, err := strconv.Atoi(parts[0])
			if err != nil || n != number+1 {
				return fmt.Errorf("TAP sequence mismatch")
			}
			number = n
			name := parts[1]
			if strings.Contains(name, "# SKIP") || strings.Contains(name, "# TODO") {
				st = tr.Skipped
			}
			add(o, tr.Test{ID: strconv.Itoa(n) + ":" + name, Name: name, State: st})
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !version || plan < 0 {
		return fmt.Errorf("missing native AVA TAP header or final plan")
	}
	if plan != number {
		problem(o, "collection-or-plan-error", "AVA native plan does not match outcomes")
	}
	return nil
}

type pwTest struct {
	ProjectID, ProjectName, Status, ExpectedStatus string
	Results                                        []struct {
		Status string
		Retry  int
		Error  *struct{ Message string }
		Errors []struct{ Message string }
	}
}
type pwSuite struct {
	Title, File string
	Suites      []pwSuite
	Specs       []struct {
		ID, Title, File string
		Tests           []pwTest
	}
}

func parsePlaywright(b []byte, o *tr.Observation) error {
	var r struct {
		Suites []pwSuite
		Errors []struct{ Message string }
		Stats  *struct{ Expected, Unexpected, Flaky, Skipped int }
	}
	if err := decode(b, &r); err != nil {
		return err
	}
	if r.Suites == nil || r.Stats == nil {
		return fmt.Errorf("missing native Playwright completion fields")
	}
	for _, e := range r.Errors {
		problem(o, "playwright-global-error", e.Message)
	}
	n := 0
	categories := map[string]int{}
	var walk func(pwSuite, string)
	walk = func(s pwSuite, parent string) {
		suite := parent + "/" + s.Title
		for _, spec := range s.Specs {
			for _, t := range spec.Tests {
				n++
				categories[t.Status]++
				x := tr.Test{ID: spec.ID + "::" + t.ProjectID, Name: spec.Title, File: spec.File, Suite: suite, State: tr.Unknown}
				if spec.ID == "" {
					problem(o, "missing-native-id", spec.Title)
				}
				switch t.Status {
				case "expected":
					x.State = tr.Passed
					if t.ExpectedStatus != "passed" {
						x.State = tr.Skipped
					}
				case "unexpected":
					x.State = tr.Failed
				case "skipped":
					x.State = tr.Skipped
				case "flaky":
					x.State = tr.Flaky
				}
				for i, a := range t.Results {
					if a.Retry != i {
						problem(o, "attempt-sequence", x.ID)
					}
					st := state(a.Status)
					msg := ""
					if a.Error != nil {
						msg = a.Error.Message
					}
					for _, e := range a.Errors {
						msg += "\n" + e.Message
					}
					x.Attempts = append(x.Attempts, tr.Attempt{State: st, FailureKind: failureKind(st), Message: msg})
					if st == tr.Unknown {
						problem(o, "unknown-attempt-state", a.Status)
					}
				}
				if len(t.Results) > 0 {
					last := t.Results[len(t.Results)-1].Status
					expected := t.ExpectedStatus
					if state(expected) == tr.Unknown {
						problem(o, "unknown-expected-status", x.ID)
					}
					matches := 0
					for _, a := range t.Results {
						if a.Status == expected {
							matches++
						}
					}
					switch t.Status {
					case "expected":
						if matches != len(t.Results) || last != expected {
							problem(o, "attempt-outcome-conflict", x.ID)
						}
					case "unexpected":
						if matches > 0 || last == "skipped" {
							problem(o, "attempt-outcome-conflict", x.ID)
						}
					case "flaky":
						if matches == 0 || matches == len(t.Results) || last != expected {
							problem(o, "attempt-outcome-conflict", x.ID)
						}
					case "skipped":
						if last != "skipped" {
							problem(o, "attempt-outcome-conflict", x.ID)
						}
					}
				}
				if len(t.Results) == 0 && x.State != tr.Skipped {
					problem(o, "missing-attempt", x.ID)
				}
				add(o, x)
			}
		}
		for _, child := range s.Suites {
			walk(child, suite)
		}
	}
	for _, s := range r.Suites {
		walk(s, "")
	}
	if categories["expected"] != r.Stats.Expected || categories["unexpected"] != r.Stats.Unexpected || categories["flaky"] != r.Stats.Flaky || categories["skipped"] != r.Stats.Skipped || n != r.Stats.Expected+r.Stats.Unexpected+r.Stats.Flaky+r.Stats.Skipped {
		problem(o, "count-mismatch", "Playwright stats")
	}
	o.RetryInformation = tr.Retained
	return nil
}
