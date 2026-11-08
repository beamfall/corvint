// SPDX-License-Identifier: Apache-2.0
package runnerqualification

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	s "github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// RSP-V0-034: the head plan must be retained for this exact first-parent diff.
func TestRSPV0034ReplayPlanBaseBinding(t *testing.T) {
	req := fixture()
	driver := &fixtureDriver{}
	mustPass(t, Run(context.Background(), req, driver))
	population, err := driver.Population(context.Background(), req.Declaration, req.Checkout, s.BodyEdit)
	if err != nil {
		t.Fatal(err)
	}
	replay := Replay{Request: req, Populations: []ClassPopulation{{s.BodyEdit, population}}, Captures: driver.captures}
	head := 3 // baseline, control, replay base, replay head
	if got := replay.Captures[head].Experiment; got.Base != req.Merges[0].Base || got.Commit != req.Merges[0].Head {
		t.Fatalf("head plan has no exact diff binding: %+v", got)
	}
	for i, c := range replay.Captures {
		if i != head && c.Experiment.Base != "" {
			t.Fatalf("non-replay experiment has a diff base: %+v", c.Experiment)
		}
	}
	encoded, err := json.Marshal(replay)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Replay
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	verify := func(context.Context, Declaration, string, []Merge) error { return nil }
	mustPass(t, ReplayRetained(context.Background(), decoded, verify))
	for _, tc := range []struct{ name, base, reason string }{
		{"unpinned", "", "retained-replay-base-unpinned"},
		{"different-parent", strings.Repeat("9", 40), "retained-replay-base-mismatch"},
		{"head-as-base", req.Merges[0].Head, "retained-replay-base-mismatch"},
		{"symbolic", "HEAD~1", "retained-replay-base-mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := decoded
			bad.Captures = slices.Clone(decoded.Captures)
			bad.Captures[head].Experiment.Base = tc.base
			mustBlock(t, ReplayRetained(context.Background(), bad, verify), tc.reason)
		})
	}
}

// RSP-V0-034: an unchanged control must preserve the effective selected set.
func TestRSPV0034NegativeControlPlan(t *testing.T) {
	for _, tc := range []struct {
		name string
		plan testrunner.RunSet
	}{
		{"drops-test", testrunner.RunSet{}},
		{"swaps-test", testrunner.RunSet{Tests: []string{"b"}}},
		{"adds-test", testrunner.RunSet{Tests: []string{"a", "b"}}},
		{"widens-to-full", testrunner.RunSet{Full: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &fixtureDriver{change: func(e Experiment, p *Pair) {
				if e.Control != "" {
					p.Traced.Plan, p.Untraced.Plan = tc.plan, tc.plan
				}
			}}
			r := Run(context.Background(), fixture(), d)
			mustBlock(t, r, "negative-control-plan-differs")
			if len(r.Classes) != 1 || r.Classes[0].ControlPassed {
				t.Fatal("changed control plan passed")
			}
		})
	}
	// FULL and an explicit inventory are the same selection, regardless of order.
	d := &fixtureDriver{change: func(e Experiment, p *Pair) {
		if e.Mutant == nil && e.Commit == fixture().Declaration.Corpus.Commit {
			plan := testrunner.RunSet{Full: true}
			if e.Control != "" {
				plan = testrunner.RunSet{Tests: []string{"b", "a"}}
			}
			p.Traced.Plan, p.Untraced.Plan = plan, plan
		}
	}}
	mustPass(t, Run(context.Background(), fixture(), d))
}

// RSP-V0-036: even an admitted passing record cannot bypass the trusted ceiling.
func TestRSPV0036QualificationBudgetCeiling(t *testing.T) {
	req := fixture()
	r := Run(context.Background(), req, &fixtureDriver{})
	mustPass(t, r)
	r.Wall = time.Second
	for _, tc := range []struct {
		name                  string
		budget, wall, ceiling time.Duration
		reason                string
	}{
		{"unset-ceiling", time.Minute, time.Second, 0, "ci-cost-ceiling-unset"},
		{"negative-ceiling", time.Minute, time.Second, -1, "ci-cost-ceiling-unset"},
		{"budget-over-ceiling", time.Minute, time.Second, 30 * time.Second, "qualification-cost-ceiling-exceeded"},
		{"unset-budget", 0, 0, time.Minute, "qualification-cost-invalid"},
		{"negative-budget", -1, 0, time.Minute, "qualification-cost-invalid"},
		{"negative-wall", time.Minute, -1, time.Minute, "qualification-cost-invalid"},
		{"wall-over-budget", time.Second, 2 * time.Second, time.Minute, "qualification-budget-exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := r
			bad.Budget, bad.Wall = tc.budget, tc.wall
			p := PlanFor(req.Declaration, []Admission{{Record: bad, Admitted: true, Sequence: 1}}, tc.ceiling, true)
			if p.Mode != "full" || p.Status != "requalifying" || p.Qualification.Passed || !slices.Contains(p.Reasons, tc.reason) {
				t.Fatalf("invalid cost admitted or reason lost: %+v", p)
			}
			if tc.ceiling <= 0 {
				if len(p.Jobs) != 0 {
					t.Fatal("unbounded job scheduled")
				}
			} else if len(p.Jobs) != 1 || p.Jobs[0].Budget != tc.ceiling {
				t.Fatalf("bounded requalification job lost: %+v", p)
			}
		})
	}
	r.Wall = r.Budget
	if p := PlanFor(req.Declaration, []Admission{{Record: r, Admitted: true, Sequence: 1}}, r.Budget, true); p.Mode != "selected" {
		t.Fatalf("inclusive budget boundary refused: %+v", p)
	}
}
