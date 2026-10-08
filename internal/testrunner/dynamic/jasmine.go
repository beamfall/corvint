package dynamic

import (
	"fmt"
	"path/filepath"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// jasmineProfile names the event record written by reporters/jasmine.cjs.
const jasmineProfile = "corvint-jasmine/0"

type jasmineExpectation struct {
	MatcherName     string `json:"matcherName"`
	Message         string `json:"message"`
	GlobalErrorType string `json:"globalErrorType"`
}

type jasmineNode struct {
	ID                  string               `json:"id"`
	Description         string               `json:"description"`
	FullName            string               `json:"fullName"`
	ParentSuiteID       *string              `json:"parentSuiteId"`
	Filename            string               `json:"filename"`
	Status              string               `json:"status"`
	PendingReason       string               `json:"pendingReason"`
	NotApplicableReason string               `json:"notApplicableReason"`
	FailedExpectations  []jasmineExpectation `json:"failedExpectations"`
}

type jasmineReport struct {
	Profile string `json:"profile"`
	Started *struct {
		TotalSpecsDefined *int  `json:"totalSpecsDefined"`
		Parallel          *bool `json:"parallel"`
	} `json:"started"`
	Suites []jasmineNode `json:"suites"`
	Specs  []jasmineNode `json:"specs"`
	Done   *struct {
		OverallStatus      string               `json:"overallStatus"`
		IncompleteCode     string               `json:"incompleteCode"`
		IncompleteReason   string               `json:"incompleteReason"`
		FailedExpectations []jasmineExpectation `json:"failedExpectations"`
	} `json:"done"`
}

// jasmineState maps Jasmine 7 spec statuses. Only "passed" can become PASSED;
// pending, notApplicable and excluded specs did not run their assertions.
func jasmineState(s string) string {
	switch s {
	case "passed":
		return tr.Passed
	case "failed":
		return tr.Failed
	case "pending", "notApplicable", "excluded":
		return tr.Skipped
	}
	return tr.Unknown
}

// parseJasmine validates the recorded native Jasmine events. A test identity is
// the spec's source-root-relative file plus its native fullName, which must equal
// the reported suite descriptions and spec description joined by single spaces.
func parseJasmine(b []byte, o *tr.Observation, in tr.Input) error {
	var r jasmineReport
	if err := decode(b, &r); err != nil {
		return err
	}
	if r.Profile != jasmineProfile || r.Started == nil || r.Started.TotalSpecsDefined == nil || r.Started.Parallel == nil || r.Done == nil || r.Suites == nil || r.Specs == nil {
		return fmt.Errorf("missing or incomplete Jasmine reporter events")
	}
	if len(r.Specs) > tr.MaxTests || len(r.Suites) > tr.MaxTests {
		return fmt.Errorf("Jasmine spec or suite bound exceeded")
	}
	o.RetryInformation = tr.NotApplicable
	if *r.Started.Parallel {
		problem(o, "jasmine-parallel-unqualified", "Jasmine parallel mode is not a qualified profile")
	}
	suites := map[string]jasmineNode{}
	for _, s := range r.Suites {
		if s.ID == "" || suites[s.ID].ID != "" {
			return fmt.Errorf("missing or duplicate Jasmine suite id %q", s.ID)
		}
		suites[s.ID] = s
	}
	paths := map[string][]string{}
	var chain func(id *string, depth int) ([]string, bool)
	chain = func(id *string, depth int) ([]string, bool) {
		if id == nil {
			return nil, true
		}
		if p, ok := paths[*id]; ok {
			return p, true
		}
		s, ok := suites[*id]
		if !ok || depth > len(suites) {
			return nil, false
		}
		parent, ok := chain(s.ParentSuiteID, depth+1)
		if !ok {
			return nil, false
		}
		p := append(append([]string{}, parent...), s.Description)
		paths[*id] = p
		return p, true
	}
	suiteErrors := 0
	for _, s := range r.Suites {
		if _, ok := chain(&s.ID, 0); !ok {
			problem(o, "jasmine-identity-conflict", "suite "+s.ID+" has no reported parent chain")
		}
		if s.Status == "failed" || len(s.FailedExpectations) > 0 {
			suiteErrors++
			problem(o, "jasmine-suite-error", s.FullName+": "+jasmineMessages(s.FailedExpectations))
		}
	}
	root := filepath.Clean(in.SourceRoot)
	selected := map[string]int{}
	for _, s := range in.Selectors {
		if !filepath.IsAbs(s) {
			s = filepath.Join(root, s)
		}
		selected[filepath.Clean(s)] = 0
	}
	failed := 0
	for _, s := range r.Specs {
		parents, ok := chain(s.ParentSuiteID, 0)
		if !ok || s.ID == "" || s.Description == "" || s.FullName != strings.Join(append(append([]string{}, parents...), s.Description), " ") {
			problem(o, "jasmine-identity-conflict", "spec "+s.ID+": "+s.FullName)
		}
		file := s.Filename
		rel, err := filepath.Rel(root, s.Filename)
		if in.SourceRoot == "" || !filepath.IsAbs(s.Filename) || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			problem(o, "jasmine-file-outside-root", s.Filename)
		} else {
			file = filepath.ToSlash(rel)
		}
		if len(in.Selectors) > 0 {
			if n, ok := selected[filepath.Clean(s.Filename)]; ok {
				selected[filepath.Clean(s.Filename)] = n + 1
			} else {
				problem(o, "jasmine-unselected-file", s.Filename)
			}
		}
		t := tr.Test{ID: file + "::" + s.FullName, Name: s.Description, File: file, Suite: strings.Join(parents, " "), State: jasmineState(s.Status)}
		switch {
		case t.State == tr.Failed:
			failed++
		case len(s.FailedExpectations) > 0:
			problem(o, "jasmine-outcome-conflict", t.ID)
		}
		add(o, t)
	}
	for _, s := range in.Selectors {
		key := s
		if !filepath.IsAbs(key) {
			key = filepath.Join(root, key)
		}
		if selected[filepath.Clean(key)] == 0 {
			problem(o, "jasmine-selector-without-specs", s)
		}
	}
	if *r.Started.TotalSpecsDefined != len(r.Specs) {
		problem(o, "jasmine-count-mismatch", fmt.Sprintf("totalSpecsDefined=%d reported=%d", *r.Started.TotalSpecsDefined, len(r.Specs)))
	}
	global := len(r.Done.FailedExpectations)
	if global > 0 {
		problem(o, "jasmine-global-error", jasmineMessages(r.Done.FailedExpectations))
	}
	switch r.Done.OverallStatus {
	case "passed":
		if failed+suiteErrors+global > 0 {
			problem(o, "jasmine-status-conflict", "overallStatus passed with reported failures")
		}
	case "failed":
		if failed+suiteErrors+global == 0 {
			problem(o, "jasmine-status-conflict", "overallStatus failed without a reported failure")
		}
	case "incomplete":
		problem(o, "jasmine-incomplete", r.Done.IncompleteCode+": "+r.Done.IncompleteReason)
	default:
		problem(o, "jasmine-unknown-overall-status", r.Done.OverallStatus)
	}
	return nil
}

func jasmineMessages(list []jasmineExpectation) string {
	out := make([]string, 0, len(list))
	for _, e := range list {
		out = append(out, e.Message)
	}
	return strings.Join(out, "; ")
}
