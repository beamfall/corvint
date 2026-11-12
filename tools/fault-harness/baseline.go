// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"slices"
)

// baselineRecord is the evaluation of the three full-suite runs at one head
// (FIH-V1-004). Both caps apply to this one baseline, on different sets.
type baselineRecord struct {
	SHA   string `json:"sha"`
	Suite int    `json:"suite"`
	// Quarantined tests failed, or were missing, in at least one run, or
	// passed in some runs and were skipped in others. They are excluded from
	// kill counting.
	Quarantined []string `json:"quarantined"`
	// Unrunnable tests were skipped in every run, or are declared unrunnable
	// in the pinned image.
	Unrunnable []string `json:"unrunnable"`
	// Disqualified: quarantined tests exceed 2 percent of the suite.
	Disqualified bool `json:"disqualified"`
	// Invalid: excluded tests (quarantined plus unrunnable) exceed 5 percent.
	Invalid bool `json:"invalid"`
}

// evaluateBaseline quarantines every test that did not pass in all three
// runs and applies the 2 percent and 5 percent caps exactly, in integers.
func evaluateBaseline(sha string, runs []RunResult, declaredUnrunnable []string) (baselineRecord, error) {
	rec := baselineRecord{SHA: sha, Quarantined: []string{}, Unrunnable: []string{}}
	if len(runs) != confirmRuns {
		return rec, fmt.Errorf("baseline needs %d runs, got %d", confirmRuns, len(runs))
	}
	suite := map[string]bool{}
	for _, r := range runs {
		for id := range r {
			suite[id] = true
		}
	}
	declared := map[string]bool{}
	for _, id := range declaredUnrunnable {
		declared[id] = true
		suite[id] = true
	}
	for id := range suite {
		passes, skips := 0, 0
		for _, r := range runs {
			switch r[id] {
			case Pass:
				passes++
			case Skip:
				skips++
			}
		}
		switch {
		case declared[id] || skips == confirmRuns:
			rec.Unrunnable = append(rec.Unrunnable, id)
		case passes != confirmRuns:
			rec.Quarantined = append(rec.Quarantined, id)
		}
	}
	slices.Sort(rec.Quarantined)
	slices.Sort(rec.Unrunnable)
	rec.Suite = len(suite)
	q, excluded := len(rec.Quarantined), len(rec.Quarantined)+len(rec.Unrunnable)
	rec.Disqualified = 100*q > 2*rec.Suite
	rec.Invalid = 100*excluded > 5*rec.Suite
	return rec, nil
}

// omittedTest is a quarantined test a plan left out for a PR that changed one
// of its dependencies.
type omittedTest struct {
	Test  string   `json:"test"`
	Cases []string `json:"cases"`
}

// omittedQuarantined counts, separately from kills, the quarantined tests of
// a case's baseline whose unit depends on what the case changed and whose
// plan omitted them (FIH-V1-004, review release-tasks 12).
func omittedQuarantined(baselines []baselineRecord, cases []caseResult) []omittedTest {
	quarantined := map[string][]string{}
	for _, b := range baselines {
		quarantined[b.SHA] = b.Quarantined
	}
	byTest := map[string][]string{}
	for _, c := range cases {
		for _, f := range c.Faults {
			if !f.Planned {
				continue
			}
			for _, q := range quarantined[c.HeadSHA] {
				if slices.Contains(f.DependentUnits, testUnit(q)) && !selects(f.Plan, q) && !slices.Contains(byTest[q], c.ID) {
					byTest[q] = append(byTest[q], c.ID)
				}
			}
		}
	}
	out := []omittedTest{}
	for test, ids := range byTest {
		slices.Sort(ids)
		out = append(out, omittedTest{Test: test, Cases: ids})
	}
	slices.SortFunc(out, func(a, b omittedTest) int {
		if a.Test < b.Test {
			return -1
		}
		if a.Test > b.Test {
			return 1
		}
		return 0
	})
	return out
}
