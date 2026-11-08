// SPDX-License-Identifier: Apache-2.0
package runnerpolicy

import (
	"context"
	"reflect"
	"testing"

	q "github.com/Beamfall/corvint/internal/runnerqualification"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// RSP-V0-046: governing membership does not depend on selector input coverage.
func TestRSPV0046GoverningRunnerCoverage(t *testing.T) {
	for _, admitted := range []bool{false, true} {
		for _, empty := range []bool{false, true} {
			for _, full := range []bool{false, true} {
				req := fixture()
				req.RolloutAdmitted = admitted
				if empty {
					req.Input.Runners = nil
				}
				req.Governing = map[string]testrunner.RunSet{
					"unknown-runner-kind": {Full: full, Tests: []string{"governing/test"}, Reason: "existing-static"},
				}
				got := ResolveCI(req)
				run, ok := got.Runs["unknown-runner-kind"]
				want := req.Governing["unknown-runner-kind"]
				want.Full = full || admitted
				if !ok || !reflect.DeepEqual(run, want) {
					t.Fatalf("admitted=%v empty=%v full=%v: got %+v want %+v", admitted, empty, full, got.Runs, want)
				}
				run.Tests[0] = "mutated"
				if req.Governing["unknown-runner-kind"].Tests[0] != "governing/test" {
					t.Fatal("execution aliases governing tests")
				}
			}
		}
	}
}

// RSP-V0-057: only known, successfully evaluated no-work states complete history.
func TestRSPV0057NoWorkHistoryComplete(t *testing.T) {
	for _, reason := range []string{"not-sampled", "not-default-branch", "no-exclusions"} {
		t.Run(reason, func(t *testing.T) {
			req := fixture()
			p := retained(t, req)
			defaultBranch := true
			switch reason {
			case "not-sampled":
				req.Sampling = ParseSampling([]byte("0.000000001"), true)
				if req.Sampling.Sample(p.base, p.head, true) {
					t.Fatal("fixture draw must not be selected")
				}
			case "not-default-branch":
				defaultBranch = false
			case "no-exclusions":
				var err error
				p, err = Retain(ResolveCI(req), "fixture-runner", testrunner.RunSet{Full: true})
				if err != nil {
					t.Fatal(err)
				}
			}
			got := Detect(context.Background(), req.Sampling, defaultBranch, p, 10, func(context.Context, string, string, testrunner.RunSet) (q.Observation, error) {
				t.Fatal("no-work outcome captured tests")
				return q.Observation{}, nil
			})
			if !got.Complete || got.Sampled || got.Reason != reason || len(got.Findings) != 0 {
				t.Fatalf("no-work history incomplete: %+v", got)
			}
			req.Findings, req.FindingsComplete = got.Findings, got.Complete
			if runOf(ResolveCI(req)).Full {
				t.Fatal("normal no-work history disabled subsequent narrowing")
			}
		})
	}
	for _, mode := range []string{"unknown-policy", "invalid-plan", "missing-capture"} {
		t.Run(mode, func(t *testing.T) {
			req := fixture()
			p := retained(t, req)
			sampling := req.Sampling
			switch mode {
			case "unknown-policy":
				sampling = Sampling{status: "future-policy"}
			case "invalid-plan":
				p = Retained{}
			}
			got := Detect(context.Background(), sampling, true, p, 10, nil)
			if got.Complete {
				t.Fatalf("unevaluated request certified: %+v", got)
			}
			req.Findings, req.FindingsComplete = got.Findings, got.Complete
			if !runOf(ResolveCI(req)).Full {
				t.Fatal("incomplete history narrowed")
			}
		})
	}
}

func multiTestRetained(t *testing.T, req Request) Retained {
	t.Helper()
	inv := &req.Input.Runners[0].Inventory
	inv.Tests = append(inv.Tests, testrunner.InventoryTest{ID: "suite/other", Path: "suite/other"})
	return retained(t, req)
}

// RSP-V0-057: native FULL and explicit supersets can confirm misses and flakes.
func TestRSPV0057ConfirmationSupersets(t *testing.T) {
	for _, full := range []bool{false, true} {
		for _, flake := range []bool{false, true} {
			req := fixture()
			p := multiTestRetained(t, req)
			capture := captureFixture(p, flake)
			calls := 0
			got := Detect(context.Background(), req.Sampling, true, p, 10, func(c context.Context, commit, runner string, run testrunner.RunSet) (q.Observation, error) {
				calls++
				if !run.Full {
					run = testrunner.RunSet{Full: full, Tests: []string{"suite/test", "suite/other"}}
				}
				return capture(c, commit, runner, run)
			})
			kind := "miss"
			if flake {
				kind = "flake"
			}
			if !got.Complete || !got.Sampled || calls != 4 || len(got.Findings) != 1 || got.Findings[0].Kind != kind || got.Findings[0].Test != "suite/test" {
				t.Fatalf("full=%v flake=%v calls=%d: %+v", full, flake, calls, got)
			}
		}
	}
}

// RSP-V0-057: only a covering inventoried plan with reconciled results certifies.
func TestRSPV0057ConfirmationUnknownPlans(t *testing.T) {
	for _, mode := range []string{"unknown-test", "unplanned-result", "missing-request", "missing-wider-result", "full-missing-request", "unknown-outcome"} {
		t.Run(mode, func(t *testing.T) {
			req := fixture()
			p := multiTestRetained(t, req)
			capture := captureFixture(p, false)
			got := Detect(context.Background(), req.Sampling, true, p, 10, func(c context.Context, commit, runner string, run testrunner.RunSet) (q.Observation, error) {
				confirmation := !run.Full
				if confirmation {
					run.Tests = []string{"suite/test", "suite/other"}
				}
				o, err := capture(c, commit, runner, run)
				if confirmation {
					switch mode {
					case "unknown-test":
						o.Plan.Tests = append(o.Plan.Tests, "unknown/test")
						o.Results = append(o.Results, testrunner.RunnerResult{ID: "unknown/test", Outcome: "passed"})
					case "unplanned-result":
						o.Plan.Tests = []string{"suite/test"}
					case "missing-request", "full-missing-request":
						o.Plan = testrunner.RunSet{Full: mode == "full-missing-request", Tests: []string{"suite/other"}}
						o.Results = []testrunner.RunnerResult{{ID: "suite/other", Outcome: "passed"}}
					case "missing-wider-result":
						o.Results = []testrunner.RunnerResult{{ID: "suite/test", Outcome: "passed"}}
					case "unknown-outcome":
						o.Results[0].Outcome = "future-outcome"
					}
				}
				return o, err
			})
			if got.Complete || len(got.Findings) != 1 || got.Findings[0].Kind != "unknown" {
				t.Fatalf("unproven capture certified: %+v", got)
			}
			req.Findings, req.FindingsComplete = got.Findings, got.Complete
			if !runOf(ResolveCI(req)).Full {
				t.Fatal("unproven confirmation narrowed")
			}
		})
	}
}
