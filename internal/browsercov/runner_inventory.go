// SPDX-License-Identifier: Apache-2.0
package browsercov

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// RSP reports share Playwright's native JSON, not a second E2E inventory wire.
type selectionReport struct {
	Config struct{ RootDir string }
	Suites []selectionSuite
	Errors json.RawMessage
}
type selectionSuite struct {
	Specs  []selectionSpec
	Suites []selectionSuite
}
type selectionSpec struct {
	ID, File, Title string
	Tests           []struct {
		ProjectName            *string
		Status, ExpectedStatus string
		Results                []struct{ Status string }
	}
}

func playwrightRows(d jslinecov.Discovery, raw string, run bool) (testrunner.RunnerInventory, []testrunner.RunnerResult, error) {
	in := testrunner.RunnerInventory{Key: d.Key}
	var results []testrunner.RunnerResult
	fail := func() (testrunner.RunnerInventory, []testrunner.RunnerResult, error) {
		in.Complete = false
		in.Reason = "list-output-malformed"
		return in, results, fmt.Errorf("list-output-malformed")
	}
	if err := d.Check(); err != nil {
		in.Reason = err.Error()
		return in, nil, err
	}
	if d.Key.Runner != "playwright" || !run && !d.ListOK {
		in.Reason = "inventory-unavailable"
		return in, nil, fmt.Errorf("inventory-unavailable")
	}
	var r selectionReport
	if jslinecov.DecodeRunnerJSON(raw, &r) != nil || r.Suites == nil || !filepath.IsAbs(r.Config.RootDir) {
		return fail()
	}
	nonterminal := false
	var walk func([]selectionSuite, int) error
	walk = func(ss []selectionSuite, depth int) error {
		if depth > 64 {
			return fmt.Errorf("list-output-malformed")
		}
		for _, s := range ss {
			for _, sp := range s.Specs {
				file := sp.File
				if !filepath.IsAbs(file) {
					file = filepath.Join(r.Config.RootDir, file)
				}
				p, err := d.Relative(file)
				if err != nil || sp.ID == "" || sp.Title == "" || len(sp.Tests) == 0 {
					return fmt.Errorf("list-output-malformed")
				}
				for _, t := range sp.Tests {
					if t.ProjectName == nil {
						return fmt.Errorf("list-output-malformed")
					}
					id := jslinecov.TestID("playwright", p, *t.ProjectName, sp.ID)
					in.Tests = append(in.Tests, testrunner.InventoryTest{ID: id, Path: p})
					if len(in.Tests) > testrunner.MaxTests {
						return fmt.Errorf("list-output-malformed")
					}
					if run {
						outcome, reason := "", ""
						// Expected failure and flaky retry remain failures. A skip carries the
						// runner's own expectedStatus/status rather than a fabricated reason.
						switch t.Status {
						case "expected":
							if len(t.Results) == 0 {
								break
							}
							outcome = "passed"
							for _, a := range t.Results {
								if a.Status != "passed" {
									outcome = "failed"
								}
							}
						case "unexpected", "flaky":
							outcome = "failed"
						case "skipped":
							// An interrupted or unknown attempt under an aggregate skip
							// did not finish; only skipped attempts certify the skip.
							outcome, reason = "skipped", "skipped"
							for _, a := range t.Results {
								if a.Status != "skipped" {
									outcome, reason = "", ""
								}
							}
						}
						if outcome == "" {
							// An unfinished row never hides a later independently valid
							// failure: keep reading and fail the report after the walk.
							nonterminal = true
							continue
						}
						results = append(results, testrunner.RunnerResult{ID: id, Outcome: outcome, Reason: reason})
					}
				}
			}
			if err := walk(s.Suites, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(r.Suites, 0); err != nil || nonterminal {
		return fail()
	}
	// Global diagnostics cannot erase independently validated terminal results.
	// Only absent, null or empty-array diagnostics certify a clean report;
	// unknown shapes and nonempty arrays retain the report-level error.
	var errors []json.RawMessage
	if r.Errors != nil && (json.Unmarshal(r.Errors, &errors) != nil || len(errors) > 0) {
		return fail()
	}
	in.Complete = true
	canonical, err := testrunner.CanonicalInventory(in, d.Key)
	if err != nil {
		return fail()
	}
	return canonical, results, nil
}

func PlaywrightInventory(d jslinecov.Discovery, raw string) (testrunner.RunnerInventory, error) {
	in, _, err := playwrightRows(d, raw, false)
	return in, err
}
func PlaywrightResults(d jslinecov.Discovery, raw string) ([]testrunner.RunnerResult, error) {
	_, out, err := playwrightRows(d, raw, true)
	return out, err
}
