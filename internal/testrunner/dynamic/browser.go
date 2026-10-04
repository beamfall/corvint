package dynamic

import (
	"encoding/json"
	"fmt"
	"sort"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func parseTestCafe(b []byte, o *tr.Observation) error {
	var r struct {
		EndTime  string
		Total    *int
		Passed   int
		Skipped  int
		Fixtures []struct {
			Name, Path string
			Tests      []struct {
				Name              string
				Errs              []json.RawMessage
				Skipped, Unstable bool
			}
		}
	}
	if err := decode(b, &r); err != nil {
		return err
	}
	if r.EndTime == "" || r.Total == nil || r.Fixtures == nil {
		return fmt.Errorf("missing native TestCafe completion fields")
	}
	n, p, sk := 0, 0, 0
	for _, f := range r.Fixtures {
		for _, t := range f.Tests {
			n++
			st := tr.Passed
			if t.Skipped {
				sk++
				st = tr.Skipped
			}
			if len(t.Errs) > 0 {
				st = tr.Failed
			}
			if st == tr.Passed {
				p++
			}
			if t.Unstable && st == tr.Passed {
				st = tr.Flaky
			}
			if t.Skipped && len(t.Errs) > 0 {
				problem(o, "conflicting-outcomes", t.Name)
			}
			add(o, tr.Test{ID: f.Path + "::" + f.Name + "::" + t.Name, Name: t.Name, File: f.Path, Suite: f.Name, State: st})
		}
	}
	if n-sk != *r.Total || p != r.Passed || sk != r.Skipped {
		problem(o, "count-mismatch", "TestCafe total")
	}
	return nil
}

func parseMocha(b []byte, o *tr.Observation) error {
	type test struct {
		Title, FullTitle, File string
		Err                    map[string]json.RawMessage
	}
	var r struct {
		Stats *struct {
			Tests, Failures, Passes, Pending int
			End                              string
		}
		Tests, Pending, Failures, Passes []test
	}
	if err := decode(b, &r); err != nil {
		return err
	}
	if r.Stats == nil || r.Stats.End == "" || r.Tests == nil {
		return fmt.Errorf("missing native Cypress/Mocha JSON completion fields")
	}
	statuses := map[string]string{}
	key := func(t test) string { return t.File + "::" + t.FullTitle }
	for _, p := range []struct {
		tests []test
		state string
	}{{r.Passes, tr.Passed}, {r.Failures, tr.Failed}, {r.Pending, tr.Skipped}} {
		for _, t := range p.tests {
			k := key(t)
			if _, ok := statuses[k]; ok {
				return fmt.Errorf("duplicate Cypress/Mocha result")
			}
			statuses[k] = p.state
		}
	}
	for _, t := range r.Tests {
		st := statuses[key(t)]
		if st == "" {
			st = tr.Unknown
		}
		add(o, tr.Test{ID: key(t), Name: t.Title, File: t.File, State: st})
	}
	if r.Stats.Tests != len(r.Tests) || r.Stats.Failures != len(r.Failures) || r.Stats.Passes != len(r.Passes) || r.Stats.Pending != len(r.Pending) {
		problem(o, "count-mismatch", "Cypress/Mocha stats")
	}
	if len(statuses) != len(r.Tests) {
		problem(o, "hook-or-unmatched-result", "Mocha result outside collected test list")
	}
	return nil
}

func parseWDIO(b []byte, o *tr.Observation) error {
	var r struct {
		End       string
		Framework string
		Specs     []string
		State     *struct{ Passed, Failed, Skipped int }
		Suites    []struct {
			Name, SessionID string
			Tests           []struct {
				Name, State string
				Error       json.RawMessage
			}
			Hooks []struct {
				Title, State string
				Error        json.RawMessage
			}
		}
	}
	if err := decode(b, &r); err != nil {
		return err
	}
	if r.End == "" || r.Framework == "" || r.Suites == nil || r.State == nil {
		return fmt.Errorf("missing native WebdriverIO completion fields")
	}
	p, f, s := 0, 0, 0
	for _, suite := range r.Suites {
		for _, t := range suite.Tests {
			st := state(t.State)
			if nativeError(t.Error) && st != tr.Failed {
				problem(o, "case-error-conflict", t.Name)
			}
			switch st {
			case tr.Passed:
				p++
			case tr.Failed:
				f++
			case tr.Skipped:
				s++
			}
			add(o, tr.Test{ID: suite.SessionID + "::" + suite.Name + "::" + t.Name, Name: t.Name, Suite: suite.Name, State: st})
		}
		for _, h := range suite.Hooks {
			if nativeError(h.Error) || h.State == "failed" {
				problem(o, "hook-error", h.Title)
				f++
			}
		}
	}
	if p != r.State.Passed || f != r.State.Failed || s != r.State.Skipped {
		problem(o, "count-mismatch", "WebdriverIO state counts")
	}
	return nil
}

type nightTest struct {
	Failed, Errors, Retries int
	Status                  string
	RetryTestData           []nightTest
	Message                 string
}

func parseNightwatch(b []byte, o *tr.Observation) error {
	var r struct {
		Name, Systemerr string
		Report          *struct {
			ModulePath, SessionID, TestEnv, ProjectName string
			Completed                                   map[string]nightTest
			CompletedSections                           map[string]nightTest
			Skipped                                     []string
			ErrorsCount                                 int
			TestsCount                                  *int
			Errmessages                                 []string
			LastError                                   json.RawMessage
		}
	}
	if err := decode(b, &r); err != nil {
		return err
	}
	if r.Name == "" || r.Report == nil || r.Report.Completed == nil || r.Report.TestsCount == nil {
		return fmt.Errorf("missing native Nightwatch module report")
	}
	if r.Systemerr != "" || r.Report.ErrorsCount != 0 || len(r.Report.Errmessages) > 0 {
		problem(o, "nightwatch-runtime-error", r.Systemerr)
	}
	m := r.Report.Completed
	for n, hook := range r.Report.CompletedSections {
		if _, test := m[n]; !test && (hook.Errors > 0 || hook.Failed > 0 || hook.Status == "fail") {
			problem(o, "nightwatch-hook-error", n)
		}
	}
	keys := make([]string, 0, len(m))
	for n := range m {
		keys = append(keys, n)
	}
	sort.Strings(keys)
	suiteID := r.Report.ModulePath + "::" + r.Name + "::" + r.Report.TestEnv + "::" + r.Report.SessionID
	attemptCount := 0
	for _, n := range keys {
		t := m[n]
		if section, ok := r.Report.CompletedSections[n]; ok {
			if nightState(section) != nightState(t) {
				problem(o, "section-test-conflict", n)
			}
			t.RetryTestData = section.RetryTestData
			t.Retries = section.Retries
		}
		if t.Retries < 0 || t.Failed < 0 || t.Errors < 0 {
			problem(o, "negative-count", n)
		}
		attemptCount += 1 + len(t.RetryTestData)
		st := nightState(t)
		x := tr.Test{ID: suiteID + "::" + n, Name: n, File: r.Report.ModulePath, Suite: r.Name, State: st}
		if len(t.RetryTestData) > 0 || t.Retries > 0 {
			if t.Retries > 0 && len(t.RetryTestData) != t.Retries {
				problem(o, "retry-history-missing", x.ID)
			}
			for i := len(t.RetryTestData) - 1; i >= 0; i-- {
				a := t.RetryTestData[i]
				x.Attempts = append(x.Attempts, tr.Attempt{State: nightState(a), FailureKind: failureKind(nightState(a)), Message: a.Message})
			}
			if st == tr.Passed {
				x.State = tr.Flaky
			}
		}
		x.Attempts = append(x.Attempts, tr.Attempt{State: st, FailureKind: failureKind(st), Message: t.Message})
		validateAttempts(x, o)
		if t.Errors > 0 {
			problem(o, "nightwatch-test-error", n)
		}
		add(o, x)
	}
	if nativeError(r.Report.LastError) {
		var last struct{ Name string }
		err := decode(r.Report.LastError, &last)
		assertionWitness := false
		for _, t := range o.Tests {
			for _, a := range t.Attempts {
				assertionWitness = assertionWitness || a.State == tr.Failed
			}
		}
		if err != nil || last.Name != "NightwatchAssertError" || !assertionWitness {
			problem(o, "nightwatch-runtime-error", "unexplained lastError")
		}
	}
	if *r.Report.TestsCount != attemptCount {
		problem(o, "count-mismatch", "Nightwatch testsCount")
	} else {
		o.RetryInformation = tr.Retained
	}
	for _, n := range r.Report.Skipped {
		add(o, tr.Test{ID: suiteID + "::" + n, Name: n, File: r.Report.ModulePath, Suite: r.Name, State: tr.Skipped})
	}
	return nil
}
func nightState(t nightTest) string {
	if t.Errors > 0 {
		return tr.Unknown
	}
	if t.Failed > 0 {
		return tr.Failed
	}
	switch t.Status {
	case "pass", "passed":
		return tr.Passed
	case "fail", "failed":
		return tr.Failed
	case "skip", "skipped":
		return tr.Skipped
	default:
		return tr.Unknown
	}
}
