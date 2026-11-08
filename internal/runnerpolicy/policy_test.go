// SPDX-License-Identifier: Apache-2.0
package runnerpolicy

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	q "github.com/Beamfall/corvint/internal/runnerqualification"
	s "github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func selectionFixture() s.Input {
	r := testevidence.New("repo", "suite/test", strings.Repeat("a", 40))
	r.Runner = testevidence.RunnerIdentity{Plugin: "fixture-plugin", PluginVersion: "1", ExecutableDigest: strings.Repeat("b", 64), RunnerVersion: "1", ConfigurationDigest: strings.Repeat("c", 64)}
	r.Execution = testevidence.ExecutionIdentity{OS: "target-os", Architecture: "target-arch", TracingBackend: "trace", TracingVersion: "1", EnvironmentDigest: strings.Repeat("d", 64)}
	r.Sources = []testevidence.Producer{{CoreDigest: strings.Repeat("e", 64), VerifierRevision: strings.Repeat("f", 40), ConfigurationDigest: strings.Repeat("c", 64), Ref: "refs/heads/main", Run: "run-1", ContentDigest: strings.Repeat("1", 64)}}
	r.CollectionCount = 1
	r.Executed = testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present}
	r.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present}
	r.Loaded = testevidence.Component[testevidence.Blob]{State: testevidence.Present}
	r.Reads = testevidence.Component[testevidence.Blob]{State: testevidence.Present}
	r.Listings = testevidence.Component[testevidence.Path]{State: testevidence.Present}
	r.Lookups = testevidence.Component[testevidence.Path]{State: testevidence.Present}
	r.Writes = testevidence.Component[testevidence.Write]{State: testevidence.Present}
	r.Execs = testevidence.Component[testevidence.Executable]{State: testevidence.Present}
	r.External = testevidence.Component[testevidence.Contact]{State: testevidence.Present}
	head := strings.Repeat("9", 40)
	key := testrunner.InventoryKey{Checkout: head, Runner: "fixture-runner", ConfigDigest: r.Runner.ConfigurationDigest, OS: r.Execution.OS, Arch: r.Execution.Architecture}
	pieces := testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Present: true}}
	runner := s.Runner{Name: key.Runner, Identity: r.Runner, Execution: r.Execution, Pieces: pieces,
		Inventory:     testrunner.RunnerInventory{Key: key, Complete: true, Tests: []testrunner.InventoryTest{{ID: r.Test, Path: "suite/tests"}}},
		Qualification: s.Qualification{Admitted: true, Passed: true, SamplingEnabled: true, SingleCollectionQualified: true, Model: "traced", Classes: []s.Class{s.BodyEdit, s.DeclarationEdit, s.DataEdit, s.Add, s.DeleteRename, s.ExecTarget, s.RunnerConfig, s.E2EServer}, Runner: r.Runner, Execution: r.Execution},
		Records:       []s.Evidence{{Record: r, Admitted: true}}, IndexBase: r.Commit, IndexVerified: true}
	return s.Input{Base: r.Commit, Head: head, Repository: r.Repository, Repositories: map[string]bool{"repo": true},
		Changes: []s.Change{{Repository: "repo", Path: "payload", Kind: "modify", Decoded: true}}, DiffComplete: true,
		BaseDirectories: map[testevidence.PathKey]bool{{Repository: "repo", Path: "."}: true}, DirectoriesComplete: true,
		Snapshot: testevidence.Snapshot{Blobs: map[testevidence.PathKey]string{}, Executables: map[testevidence.PathKey]string{}}, Runners: []s.Runner{runner}}
}

func fixture() Request {
	return Request{Input: selectionFixture(), Sampling: ParseSampling([]byte("1"), true), RolloutAdmitted: true, FindingsComplete: true}
}
func runOf(d Decision) testrunner.RunSet { return d.Runs["fixture-runner"] }
func retained(t *testing.T, req Request) Retained {
	t.Helper()
	d := ResolveCI(req)
	p, err := Retain(d, "fixture-runner", runOf(d))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Excluded()) != 1 {
		t.Fatalf("fixture did not exclude: %+v", d)
	}
	return p
}

// RSP-V0-038: identical inputs replay the identical governing run set.
func TestRSPV0038ParityReplay(t *testing.T) {
	req := fixture()
	ci, local := ResolveCI(req), ResolveLocal(req, req)
	if !reflect.DeepEqual(ci.Runs, local.Runs) || runOf(local).Full || len(runOf(local).Tests) != 0 {
		t.Fatalf("parity: CI=%+v local=%+v", ci.Runs, local.Runs)
	}
}

// RSP-V0-038: local evidence cannot remove a test CI selected.
func TestRSPV0038LocalNarrowingWidens(t *testing.T) {
	ci, local := fixture(), fixture()
	ci.Input.Runners[0].Records = nil
	got := runOf(ResolveLocal(ci, local))
	if !slices.Contains(got.Tests, "suite/test") || got.Reason != "local-ci-parity-widened" {
		t.Fatalf("local narrowed: %+v", got)
	}
	ci.Input.Runners[0].Qualification.Admitted = false
	if !runOf(ResolveLocal(ci, local)).Full {
		t.Fatal("CI FULL narrowed locally")
	}
	ci.Input.Head = strings.Repeat("8", 40)
	if !runOf(ResolveLocal(ci, local)).Full {
		t.Fatal("unbound comparison narrowed")
	}
}

// RSP-V0-046: pre-admission static plans survive; post-admission qualified plans select.
func TestRSPV0046ShadowThenAdmission(t *testing.T) {
	req := fixture()
	req.RolloutAdmitted = false
	req.Governing = map[string]testrunner.RunSet{"fixture-runner": {Tests: []string{"suite/test"}, Reason: "existing-static"}}
	shadow := ResolveCI(req)
	if !shadow.Shadow.Shadow || !reflect.DeepEqual(shadow.Runs, req.Governing) {
		t.Fatalf("shadow changed governing plan: %+v", shadow)
	}
	req.RolloutAdmitted = true
	if got := runOf(ResolveCI(req)); got.Full || len(got.Tests) != 0 {
		t.Fatalf("admitted runner did not narrow: %+v", got)
	}
}

// RSP-V0-046: no static narrowing survives unqualified/unknown inputs after rollout.
func TestRSPV0046UnqualifiedAndUnknownFull(t *testing.T) {
	cases := map[string]func(*Request){
		"unqualified":          func(r *Request) { r.Input.Runners[0].Qualification.Passed = false },
		"not-admitted":         func(r *Request) { r.Input.Runners[0].Qualification.Admitted = false },
		"missing-piece":        func(r *Request) { r.Input.Runners[0].Pieces.Inventory = testrunner.RunnerPiece{} },
		"diff-incomplete":      func(r *Request) { r.Input.DiffComplete = false },
		"history-incomplete":   func(r *Request) { r.FindingsComplete = false },
		"history-malformed":    func(r *Request) { r.Findings = []Finding{{Runner: "fixture-runner", Kind: "miss"}} },
		"history-capped":       func(r *Request) { r.Findings = make([]Finding, s.MaxInputs+1) },
		"inventory-malformed":  func(r *Request) { r.Input.Runners[0].Inventory.Complete = false },
		"local-os-unqualified": func(r *Request) { r.Input.Runners[0].Execution.OS = "another-os" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := fixture()
			mutate(&r)
			r.Governing = map[string]testrunner.RunSet{"fixture-runner": {Tests: []string{}}}
			if got := runOf(ResolveCI(r)); !got.Full || got.Reason == "" {
				t.Fatalf("unknown narrowed: %+v", got)
			}
		})
	}
}

// RSP-V0-046: owner-set probability is retained, with no invented measurement.
func TestRSPV0046OwnerSamplingRate(t *testing.T) {
	req := fixture()
	req.Sampling = ParseSampling([]byte("0.25\n"), true)
	if rate, set := req.Sampling.Rate(); !set || rate != 0.25 || req.Sampling.Status() != "owner-set" {
		t.Fatal("owner rate lost")
	}
	if runOf(ResolveCI(req)).Full {
		t.Fatal("nonzero admitted rate blocked")
	}
	for i := 0; i < 5; i++ {
		if req.Sampling.Sample(req.Input.Base, req.Input.Head, true) != req.Sampling.Sample(req.Input.Base, req.Input.Head, true) {
			t.Fatal("sample not deterministic")
		}
	}
	if req.Sampling.Sample(req.Input.Base, req.Input.Head, false) {
		t.Fatal("sample outside default branch")
	}
}

// RSP-V0-046, RSP-V0-057: unset/invalid rates schedule nothing and cannot narrow.
func TestRSPV0046UnsetSamplingFull(t *testing.T) {
	for _, text := range []string{"", "0", " ", "NaN", "Inf", "-1", "1.1", "1e-3", ". .", strings.Repeat("1", 129)} {
		req := fixture()
		req.Sampling = ParseSampling([]byte(text), true)
		if !runOf(ResolveCI(req)).Full || req.Sampling.Sample(req.Input.Base, req.Input.Head, true) || req.Sampling.Status() == "owner-set" {
			t.Fatalf("unsafe rate %q", text)
		}
	}
	if p := ParseSampling([]byte("1"), false); p.Status() != "sampling-not-admitted" {
		t.Fatal("candidate config admitted")
	}
	if _, set := (Sampling{}).Rate(); set {
		t.Fatal("zero value claims measurement")
	}
}

func captureFixture(p Retained, flake bool) Capture {
	heads := 0
	return func(_ context.Context, commit, runner string, run testrunner.RunSet) (q.Observation, error) {
		inv := p.inventory
		inv.Key.Checkout = commit
		obs := q.Observation{Inventory: inv, Plan: run}
		if commit == p.head {
			heads++
		}
		for _, test := range inv.Tests {
			if run.Full || slices.Contains(run.Tests, test.ID) {
				outcome := "passed"
				if commit == p.head && (!flake || heads == 1) {
					outcome = "failed"
				}
				obs.Results = append(obs.Results, testrunner.RunnerResult{ID: test.ID, Outcome: outcome})
			}
		}
		return obs, nil
	}
}

// RSP-V0-057: confirmed excluded head failure blocks its class on later selection.
func TestRSPV0057ConfirmedMissDisablesClass(t *testing.T) {
	req := fixture()
	p := retained(t, req)
	result := Detect(context.Background(), req.Sampling, true, p, 10, captureFixture(p, false))
	if !result.Sampled || len(result.Findings) != 1 || result.Findings[0].Kind != "miss" {
		t.Fatalf("no confirmed miss: %+v", result)
	}
	req.Findings = result.Findings
	got := runOf(ResolveCI(req))
	if !got.Full || !strings.Contains(got.Reason, "class-unqualified:data-edit") {
		t.Fatalf("miss did not disable: %+v", got)
	}
	// A miss disables its runner's class, not unrelated classes or runners.
	req.Input.Changes[0] = s.Change{Repository: "repo", Path: "new-file", Kind: "add", Decoded: true}
	if got = runOf(ResolveCI(req)); got.Full {
		t.Fatalf("unrelated class disabled: %+v", got)
	}
}

// RSP-V0-057: failure that does not reproduce is retained as flake, never a miss.
func TestRSPV0057FlakeDoesNotDisableClass(t *testing.T) {
	req := fixture()
	p := retained(t, req)
	result := Detect(context.Background(), req.Sampling, true, p, 10, captureFixture(p, true))
	if len(result.Findings) != 1 || result.Findings[0].Kind != "flake" {
		t.Fatalf("flake lost: %+v", result)
	}
	req.Findings = result.Findings
	if runOf(ResolveCI(req)).Full {
		t.Fatal("flake disabled class")
	}
}

// RSP-V0-057: retain the original set and rerun the exact base/head pair once.
func TestRSPV0057PinnedReplayAndNoAliasing(t *testing.T) {
	req := fixture()
	p := retained(t, req)
	p.Excluded()[0] = "changed"
	req.Input.Head = strings.Repeat("8", 40)
	req.Input.Runners[0].Inventory.Tests[0].ID = "changed"
	var commits []string
	capture := captureFixture(p, false)
	result := Detect(context.Background(), req.Sampling, true, p, 10, func(c context.Context, commit, runner string, run testrunner.RunSet) (q.Observation, error) {
		commits = append(commits, commit)
		return capture(c, commit, runner, run)
	})
	if !reflect.DeepEqual(commits, []string{p.base, p.head, p.base, p.head}) || len(result.Findings) != 1 || result.Findings[0].Test != "suite/test" {
		t.Fatalf("not original replay: %+v %+v", commits, result)
	}
}

// RSP-V0-057: incomplete/unknown samples abstain, keep FULL, and never invent a miss.
func TestRSPV0057UnknownCaptureFull(t *testing.T) {
	for _, mode := range []string{"error", "missing", "duplicate", "wrong-commit", "not-full", "unknown-outcome", "skip"} {
		t.Run(mode, func(t *testing.T) {
			req := fixture()
			p := retained(t, req)
			capture := captureFixture(p, false)
			result := Detect(context.Background(), req.Sampling, true, p, 10, func(c context.Context, commit, runner string, run testrunner.RunSet) (q.Observation, error) {
				o, e := capture(c, commit, runner, run)
				switch mode {
				case "error":
					e = errors.New("capture unavailable")
				case "missing":
					o.Results = nil
				case "duplicate":
					o.Results = append(o.Results, o.Results[0])
				case "wrong-commit":
					o.Inventory.Key.Checkout = "HEAD"
				case "not-full":
					o.Plan.Full = false
				case "unknown-outcome":
					o.Results[0].Outcome = "unknown"
				case "skip":
					o.Results[0].Outcome = "skipped"
				}
				return o, e
			})
			if len(result.Findings) != 1 || result.Findings[0].Kind != "unknown" {
				t.Fatalf("unknown invented result: %+v", result)
			}
			req.Findings = result.Findings
			if !runOf(ResolveCI(req)).Full {
				t.Fatal("unknown narrowed")
			}
		})
	}
}

// RSP-V0-057: absent owner rate executes no sampler calls, including reruns.
func TestRSPV0057UnsetSamplesNothing(t *testing.T) {
	req := fixture()
	p := retained(t, req)
	got := Detect(context.Background(), Sampling{}, true, p, 10, func(context.Context, string, string, testrunner.RunSet) (q.Observation, error) {
		t.Fatal("unset sampler executed")
		return q.Observation{}, nil
	})
	if got.Sampled || got.Reason != "sampling-unset" || len(got.Findings) != 0 {
		t.Fatalf("unset not explicit: %+v", got)
	}
}

// RSP-V0-057: native supersets change the retained exclusion set; subsets refuse.
func TestRSPV0057ActualRunBinding(t *testing.T) {
	req := fixture()
	d := ResolveCI(req)
	p, err := Retain(d, "fixture-runner", testrunner.RunSet{Full: true})
	if err != nil || len(p.Excluded()) != 0 {
		t.Fatal("FULL retained exclusions", err)
	}
	req.Input.Runners[0].Records = nil
	d = ResolveCI(req)
	if _, err = Retain(d, "fixture-runner", testrunner.RunSet{}); err == nil {
		t.Fatal("native subset admitted")
	}
	if _, err = Retain(d, "unknown", testrunner.RunSet{Full: true}); err == nil {
		t.Fatal("unknown runner admitted")
	}
}

func qualificationFixture(req Request, sequence uint64) (q.Declaration, []q.Admission) {
	r := req.Input.Runners[0]
	d := q.Declaration{Runner: r.Name, Corpus: q.Corpus{Repository: "repo", Commit: req.Input.Base}, Model: "traced", Classes: []s.Class{s.DataEdit}, Identity: q.Identity{Runner: r.Identity, Execution: r.Execution, SelectorVersion: "1", RecordFormat: testevidence.Format, NonSemanticDigest: strings.Repeat("a", 64), VerifierRevision: req.Input.Base}}
	record := q.Record{Declaration: d, Complete: true, Budget: time.Second, Merges: []q.Merge{{Base: req.Input.Base, Head: req.Input.Head, Commit: req.Input.Head}}, Timings: []q.Timing{{Model: "traced"}}, Classes: []q.ClassRecord{{Class: s.DataEdit, PopulationDigest: strings.Repeat("a", 64), PopulationSize: 1, Trials: q.TrialFloor, Sampled: q.TrialFloor, Draws: make([]string, q.TrialFloor), SelectedSizes: make([]int, q.TrialFloor), Upper95: q.Upper95(0, q.TrialFloor), Passed: true, ControlPassed: true, ReplayPassed: true}}}
	return d, []q.Admission{{Record: record, Admitted: true, Sequence: sequence}}
}

// RSP-V0-057: only newer admitted requalification clears a confirmed class block.
func TestRSPV0057RequalificationClearsBlock(t *testing.T) {
	req := fixture()
	p := retained(t, req)
	findings := Detect(context.Background(), req.Sampling, true, p, 10, captureFixture(p, false)).Findings
	d, records := qualificationFixture(req, 11)
	got, err := Requalify(findings, d, records, time.Minute)
	if err != nil || len(got) != 0 {
		t.Fatalf("new qualification failed: %+v %v", got, err)
	}
	req.Findings = got
	if runOf(ResolveCI(req)).Full {
		t.Fatal("class remains blocked")
	}
}

// RSP-V0-057: older, forged, failed and incomplete qualifications cannot clear a miss.
func TestRSPV0057InvalidRequalificationKeepsBlock(t *testing.T) {
	req := fixture()
	p := retained(t, req)
	findings := Detect(context.Background(), req.Sampling, true, p, 10, captureFixture(p, false)).Findings
	for _, mode := range []string{"older", "forged", "failed", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			d, records := qualificationFixture(req, 11)
			switch mode {
			case "older":
				records[0].Sequence = 10
			case "forged":
				records[0].Admitted = false
			case "failed":
				records[0].Record.Classes[0].Passed = false
			case "incomplete":
				records[0].Record.Complete = false
			}
			if _, err := Requalify(findings, d, records, time.Minute); err == nil {
				t.Fatal("invalid requalification cleared miss")
			}
			req.Findings = findings
			if !runOf(ResolveCI(req)).Full {
				t.Fatal("block lost")
			}
		})
	}
}

// RSP-V0-039: soundness widening is represented in both baseline and build log.
func TestRSPV0039BaselineUpdateNamesReselectedTests(t *testing.T) {
	before := testrunner.RunSet{Tests: []string{"a"}}
	fixed := testrunner.RunSet{Tests: []string{"a", "b"}}
	if err := CheckBaselineUpdate(before, fixed, fixed, BaselineUpdate{Fix: "retain data read", BuildLog: "docs/build-log/2026-11-08-fix.md", Reselected: []string{"b"}}); err != nil {
		t.Fatal(err)
	}
}

// RSP-V0-039: a conflicting baseline or missing build-log evidence fails closed.
func TestRSPV0039ConflictingBaselineRefused(t *testing.T) {
	before := testrunner.RunSet{Tests: []string{"a"}}
	fixed := testrunner.RunSet{Tests: []string{"a", "b"}}
	entry := BaselineUpdate{Fix: "retain data read", BuildLog: "docs/build-log/2026-11-08-fix.md", Reselected: []string{"b"}}
	if CheckBaselineUpdate(before, fixed, before, entry) == nil {
		t.Fatal("conflicting baseline accepted")
	}
	entry.Reselected = nil
	if CheckBaselineUpdate(before, fixed, fixed, entry) == nil {
		t.Fatal("unnamed reselected test accepted")
	}
	entry.BuildLog = ""
	if CheckBaselineUpdate(before, fixed, fixed, entry) == nil {
		t.Fatal("missing log accepted")
	}
}

// RSP-V0-046: the existing public shadow projection is unchanged before admission.
func TestRSPV0046FrozenShadowProjection(t *testing.T) {
	req := fixture()
	req.RolloutAdmitted = false
	req.FindingsComplete = false
	req.Sampling = Sampling{}
	direct, got := s.Select(req.Input), ResolveLocal(req, req).Shadow
	direct.SelectionNS = nil
	got.SelectionNS = nil
	if !reflect.DeepEqual(direct, got) {
		t.Fatal("pre-admission shadow projection changed")
	}
}

// RSP-V0-038: a missing CI runner is uncertainty, never permission to exclude.
func TestRSPV0038MissingReferenceFull(t *testing.T) {
	ci, local := fixture(), fixture()
	ci.Input.Runners = nil
	got := runOf(ResolveLocal(ci, local))
	if !got.Full || got.Reason != "ci-runner-unavailable" {
		t.Fatalf("missing reference narrowed: %+v", got)
	}
}

// RSP-V0-057: deterministic failures already present at base are not change misses.
func TestRSPV0057ExistingFailureIsNotMiss(t *testing.T) {
	req := fixture()
	p := retained(t, req)
	capture := captureFixture(p, false)
	calls := 0
	result := Detect(context.Background(), req.Sampling, true, p, 10, func(c context.Context, commit, runner string, run testrunner.RunSet) (q.Observation, error) {
		calls++
		o, e := capture(c, commit, runner, run)
		o.Results[0].Outcome = "failed"
		return o, e
	})
	if !result.Complete || len(result.Findings) != 0 || calls != 2 {
		t.Fatalf("base failure classified as miss: %+v calls=%d", result, calls)
	}
}

// RSP-V0-057: changed head inventories and failed reruns never certify a clean sample.
func TestRSPV0057ConfirmationAndInventoryUnknown(t *testing.T) {
	for _, mode := range []string{"rerun-error", "rerun-head-skip", "base-fails-rerun", "head-inventory"} {
		t.Run(mode, func(t *testing.T) {
			req := fixture()
			p := retained(t, req)
			capture := captureFixture(p, false)
			result := Detect(context.Background(), req.Sampling, true, p, 10, func(c context.Context, commit, runner string, run testrunner.RunSet) (q.Observation, error) {
				o, e := capture(c, commit, runner, run)
				switch {
				case mode == "rerun-error" && !run.Full:
					e = errors.New("rerun unavailable")
				case mode == "rerun-head-skip" && !run.Full && commit == p.head:
					o.Results[0].Outcome = "skipped"
					o.Results[0].Reason = "conditional skip"
				case mode == "base-fails-rerun" && !run.Full && commit == p.base:
					o.Results[0].Outcome = "failed"
				case mode == "head-inventory" && commit == p.head:
					o.Inventory.Tests = append(slices.Clone(o.Inventory.Tests), testrunner.InventoryTest{ID: "extra", Path: "suite/extra"})
					o.Results = append(o.Results, testrunner.RunnerResult{ID: "extra", Outcome: "passed"})
				}
				return o, e
			})
			if result.Complete || len(result.Findings) != 1 || result.Findings[0].Kind != "unknown" {
				t.Fatalf("unknown confirmed: %+v", result)
			}
			req.Findings = result.Findings
			if !runOf(ResolveCI(req)).Full {
				t.Fatal("unknown allowed narrowing")
			}
		})
	}
}

// RSP-V0-057: qualification in another corpus or OS cannot clear this history.
func TestRSPV0057RequalificationScopeRetained(t *testing.T) {
	req := fixture()
	p := retained(t, req)
	findings := Detect(context.Background(), req.Sampling, true, p, 10, captureFixture(p, false)).Findings
	for _, scope := range []string{"repository", "os"} {
		t.Run(scope, func(t *testing.T) {
			d, records := qualificationFixture(req, 11)
			if scope == "repository" {
				d.Corpus.Repository = "other"
			} else {
				d.Identity.Execution.OS = "other"
			}
			records[0].Record.Declaration = d
			kept, err := Requalify(findings, d, records, time.Minute)
			if err != nil || len(kept) != 1 {
				t.Fatalf("other scope cleared block: %+v %v", kept, err)
			}
		})
	}
}

// RSP-V0-038: identical FULL inputs preserve parity including their reasons.
func TestRSPV0038FullParityReplay(t *testing.T) {
	req := fixture()
	req.Sampling = Sampling{}
	ci, local := ResolveCI(req), ResolveLocal(req, req)
	if !runOf(ci).Full || !reflect.DeepEqual(ci.Runs, local.Runs) {
		t.Fatalf("FULL parity diverged: CI=%+v local=%+v", ci.Runs, local.Runs)
	}
}

// RSP-V0-039: malformed/capped baseline evidence is not an accepted update.
func TestRSPV0039UnknownBaselineRefused(t *testing.T) {
	entry := BaselineUpdate{Fix: "soundness", BuildLog: "docs/build-log/fix.md"}
	for _, tests := range [][]string{{""}, {"a", "a"}, {"bad\nidentity"}, make([]string, testrunner.MaxTests+1)} {
		run := testrunner.RunSet{Tests: tests}
		if CheckBaselineUpdate(run, run, run, entry) == nil {
			t.Fatal("invalid baseline accepted")
		}
	}
}
