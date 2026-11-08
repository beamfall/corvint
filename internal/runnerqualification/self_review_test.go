// SPDX-License-Identifier: Apache-2.0
package runnerqualification

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	s "github.com/Beamfall/corvint/internal/runnerselection"
)

// RSP-V0-036/056: replay evaluation cannot replace the collected execution cost.
func TestRSPV0056ReplayRetainsExecutionCost(t *testing.T) {
	req := fixture()
	driver := &fixtureDriver{}
	mustPass(t, Run(context.Background(), req, driver))
	population, err := driver.Population(context.Background(), req.Declaration, req.Checkout, s.BodyEdit)
	if err != nil {
		t.Fatal(err)
	}
	replay := Replay{Request: req, Populations: []ClassPopulation{{s.BodyEdit, population}}, Captures: driver.captures}
	verify := func(context.Context, Declaration, string, []Merge) error { return nil }
	want := time.Duration(len(replay.Captures)) * 6 * time.Millisecond
	r := ReplayRetained(context.Background(), replay, verify)
	mustPass(t, r)
	if r.Budget != req.Budget || r.Wall != want {
		t.Fatalf("original execution cost replaced: budget=%s wall=%s want=%s", r.Budget, r.Wall, want)
	}
	for _, tc := range []struct {
		name string
		wall time.Duration
	}{
		{"hour-long-collection", time.Hour},
		{"aggregate-over-budget", time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := replay
			bad.Captures = slices.Clone(replay.Captures)
			for i := range bad.Captures {
				bad.Captures[i].Pair.Traced.CollectionWall = tc.wall
			}
			r := ReplayRetained(context.Background(), bad, verify)
			mustBlock(t, r, "qualification-budget-exceeded")
			if r.Budget != req.Budget || r.Wall <= r.Budget || r.Complete {
				t.Fatalf("over-budget replay lost original cost: budget=%s wall=%s complete=%t", r.Budget, r.Wall, r.Complete)
			}
			if p := PlanFor(req.Declaration, []Admission{{Record: r, Admitted: true}}, req.Budget, true); p.Mode != "full" {
				t.Fatal("over-budget replay narrowed")
			}
		})
	}
}

// RSP-V0-036/056: admission checks retained phases even in old records whose
// Wall measured the replay, and refuses cost forms it cannot account for.
func TestRSPV0056AdmissionCostForms(t *testing.T) {
	req := fixture()
	r := Run(context.Background(), req, &fixtureDriver{})
	mustPass(t, r)
	for _, tc := range []struct {
		name, reason string
		change       func(*Record)
	}{
		{"hour-long-collection", "qualification-budget-exceeded", func(r *Record) { r.Timings[0].CollectionWall = time.Hour }},
		{"aggregate-over-budget", "qualification-budget-exceeded", func(r *Record) {
			for i := range r.Timings {
				r.Timings[i].InventoryWall = time.Second
			}
		}},
		{"unknown-model", "qualification-cost-invalid", func(r *Record) { r.Timings[0].Model = "future-model" }},
		{"negative-phase", "qualification-cost-invalid", func(r *Record) { r.Timings[0].CollectionWall = -1 }},
		{"overflow", "qualification-cost-invalid", func(r *Record) { r.Timings[0].CollectionWall = time.Duration(1<<63 - 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := r
			bad.Timings = slices.Clone(r.Timings)
			tc.change(&bad)
			p := PlanFor(req.Declaration, []Admission{{Record: bad, Admitted: true}}, req.Budget, true)
			if p.Mode != "full" || !slices.Contains(p.Reasons, tc.reason) || len(p.Jobs) != 1 {
				t.Fatalf("unproven cost admitted: %+v", p)
			}
		})
	}
}

// RSP-V0-036/053: targets and configurations coexist, but a failure or changed
// verifier/runner identity supersedes older successes within the same scope.
func TestRSPV0053IndependentQualificationScopes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Declaration)
	}{
		{"windows", func(d *Declaration) { d.Identity.Execution.OS = "windows" }},
		{"arm64", func(d *Declaration) { d.Identity.Execution.Architecture = "arm64" }},
		{"configuration", func(d *Declaration) { d.Identity.Runner.ConfigurationDigest = strings.Repeat("b", 64) }},
		{"environment", func(d *Declaration) { d.Identity.Execution.EnvironmentDigest = strings.Repeat("b", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := fixture()
			req.Declaration.Runner = "go-test"
			a := Run(context.Background(), req, &fixtureDriver{})
			tc.change(&req.Declaration)
			b := Run(context.Background(), req, &fixtureDriver{})
			mustPass(t, a)
			mustPass(t, b)
			records := []Admission{{Record: a, Admitted: true, Sequence: 1}, {Record: b, Admitted: true, Sequence: 2}}
			for _, d := range []Declaration{a.Declaration, b.Declaration} {
				if p := PlanFor(d, records, time.Minute, true); p.Mode != "selected" || len(p.Jobs) != 0 {
					t.Fatalf("independent qualification displaced: %+v", p)
				}
			}
			// A tie in a different scope is not ambiguous.
			records[1].Sequence = 1
			if p := PlanFor(a.Declaration, records, time.Minute, true); p.Mode != "selected" {
				t.Fatalf("cross-scope tie: %+v", p)
			}
			failed := a
			failed.Complete = false
			failed.Declaration.Identity.SelectorVersion = "2"
			records = append(records, Admission{Record: failed, Admitted: true, Sequence: 3})
			for _, d := range []Declaration{a.Declaration, failed.Declaration} {
				if p := PlanFor(d, records, time.Minute, true); p.Mode != "full" || len(p.Jobs) != 1 {
					t.Fatalf("same-scope failure revived old success: %+v", p)
				}
			}
			if p := PlanFor(b.Declaration, records, time.Minute, true); p.Mode != "selected" {
				t.Fatalf("failure invalidated another scope: %+v", p)
			}
			unknown := a.Declaration
			unknown.Identity.Execution.OS = "future-target"
			if p := PlanFor(unknown, records, time.Minute, true); p.Mode != "full" || !slices.Contains(p.Reasons, "qualification-not-admitted") {
				t.Fatalf("unknown scope reused a qualification: %+v", p)
			}
		})
	}
}
