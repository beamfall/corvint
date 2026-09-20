package doccorpus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func stabilityFixture(t *testing.T, editReceipt func(int, *jstestprovider.Receipt), editRegistry func(*StabilityRegistry)) (string, Manifest) {
	t.Helper()
	root, manifest := behaviorFixtureWithRun(t, nil, true)
	providerPath := manifest.Providers[len(manifest.Providers)-1].Record
	providerBytes, err := os.ReadFile(filepath.Join(root, providerPath))
	if err != nil {
		t.Fatal(err)
	}
	var provider ProviderRecord
	if err := decode(providerBytes, &provider); err != nil {
		t.Fatal(err)
	}
	nativePath := provider.Observations[0].Input
	nativeBytes, err := os.ReadFile(filepath.Join(root, nativePath))
	if err != nil {
		t.Fatal(err)
	}
	var nativeDocument struct {
		Receipt *jstestprovider.Receipt `json:"receipt"`
	}
	if err := json.Unmarshal(nativeBytes, &nativeDocument); err != nil || nativeDocument.Receipt == nil {
		t.Fatalf("decode native receipt: %v", err)
	}

	inputs := []StabilityReceiptInput{}
	receipts := []*jstestprovider.Receipt{}
	for repetition := 1; repetition <= 4; repetition++ {
		copy := *nativeDocument.Receipt
		external := *nativeDocument.Receipt.External
		copy.External = &external
		copy.Tests = append([]jstestprovider.TestOutcome{}, nativeDocument.Receipt.Tests...)
		copy.Tests[0].Attempts = append([]jstestprovider.Attempt{}, nativeDocument.Receipt.Tests[0].Attempts...)
		copy.Tests[0].Artifacts = append([]jstestprovider.FailureArtifact{}, nativeDocument.Receipt.Tests[0].Artifacts...)
		copy.Tests[0].DurationMS = float64(repetition)
		copy.External.DeclaredAppIdentity = provider.BehaviorContracts.SourceRevision
		if editReceipt != nil {
			editReceipt(repetition, &copy)
		}
		data, err := jstestprovider.EncodeQualified(copy)
		if err != nil {
			t.Fatal(err)
		}
		path := "evidence/stability/run-" + string(rune('0'+repetition)) + ".json"
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", path)
		inputs = append(inputs, StabilityReceiptInput{Path: path, SHA256: Digest(data)})
		receipts = append(receipts, &copy)
	}
	git(t, root, "commit", "-qm", "synthetic stability receipts")
	receiptRevision := git(t, root, "rev-parse", "HEAD")
	manifest.Scopes = append(manifest.Scopes, Scope{Path: "evidence/stability", Revision: receiptRevision})
	for i := range inputs {
		inputs[i].Revision = receiptRevision
		manifest.Inputs = append(manifest.Inputs, Input{Path: inputs[i].Path, Revision: receiptRevision, Blob: git(t, root, "rev-parse", receiptRevision+":"+inputs[i].Path), SHA256: inputs[i].SHA256, Provider: "behavior", Purpose: "observation"})
	}

	policy := StabilityPolicy{ID: "repository:playwright-stability", MatrixDimensions: []string{}, Thresholds: []StabilityThreshold{
		{Scope: "one-spec", RequiredRepetitions: 3, MinimumPassed: 3},
		{Scope: "feature-batch", RequiredRepetitions: 3, MinimumPassed: 3},
		{Scope: "suite", RequiredRepetitions: 3, MinimumPassed: 3},
	}}
	policy.SHA256 = hashValue(policy)
	aggregate := StabilityAggregate{ID: "stability:summary:chromium", Scope: "one-spec", PolicyID: policy.ID, Planned: 3, TestID: provider.BehaviorContracts.Tests[0].ID, Project: provider.BehaviorContracts.Tests[0].Project, ContractID: provider.BehaviorContracts.ContractID, ContractSHA256: provider.BehaviorContracts.ContractSHA256}
	for i, receipt := range receipts {
		outcome := receipt.Tests[0]
		identity := StabilityIdentity{ApplicationRevision: receipt.External.DeclaredAppIdentity, TestRevision: provider.BehaviorContracts.SourceRevision, ConfigSHA256: receipt.Identity.ConfigDigest, ContractSHA256: provider.BehaviorContracts.ContractSHA256, Runner: receipt.Identity.RunnerName, RunnerVersion: receipt.Identity.RunnerVersion, Browser: outcome.Project.Browser, Project: outcome.Project.Name, WorkerPolicy: "workers:1", RetryPolicy: "retries:0", EnvironmentClass: "local", EnvironmentSHA256: hashValue(receipt.Identity.Environment), FixtureSchema: "playwright-use", FixtureSHA256: Digest(outcome.Project.Use)}
		contribution := StabilityContribution{RunKind: "planned-repetition", Repetition: i + 1, Receipt: inputs[i], SourcePaths: map[string]string{"/repo/config.cjs": "src/value.go", "/repo/test.ts": "src/view.ts"}, Identity: identity, Cleanup: "passed", Attempts: []StabilityAttempt{}}
		if i == 3 {
			contribution.RunKind = "manual-rerun"
			contribution.Repetition = 0
			contribution.ManualRunID = "manual-1"
		}
		for _, attempt := range outcome.Attempts {
			class := "none"
			if attempt.FailureKind == "assertion-or-test" {
				class = "assertion"
			}
			if attempt.FailureKind == "browser-or-fixture" {
				class = "fixture"
			}
			if attempt.FailureKind == "test-timeout" || attempt.State == jstestprovider.StateTimedOut {
				class = "timeout"
			}
			if attempt.State == jstestprovider.StateInterrupted {
				class = "interruption"
			}
			if attempt.State == jstestprovider.StateInfrastructure {
				class = "infrastructure"
			}
			artifacts := []jstestprovider.FailureArtifact{}
			if class != "none" {
				artifacts = outcome.Artifacts
			}
			contribution.Attempts = append(contribution.Attempts, StabilityAttempt{Retry: attempt.Retry, State: string(attempt.State), FailureClass: class, Artifacts: artifacts, Cleanup: "passed"})
		}
		aggregate.Contributions = append(aggregate.Contributions, contribution)
	}
	registry := &StabilityRegistry{Schema: StabilitySchema, Policy: policy, Aggregates: []StabilityAggregate{aggregate}}
	if editRegistry != nil {
		editRegistry(registry)
	}
	provider.Schema = BehaviorStabilityProviderSchema
	provider.BehaviorContracts.Stability = registry
	updated, err := Encode(provider)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, providerPath), updated, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", providerPath)
	git(t, root, "commit", "-qm", "synthetic stability provider")
	providerRevision := git(t, root, "rev-parse", "HEAD")
	providerBlob := git(t, root, "rev-parse", providerRevision+":"+providerPath)
	for i := range manifest.Providers {
		if manifest.Providers[i].ID == "behavior" {
			manifest.Providers[i].Revision = providerRevision
		}
	}
	for i := range manifest.Inputs {
		if manifest.Inputs[i].Path == providerPath && manifest.Inputs[i].Purpose == "provider" {
			oldRevision := manifest.Inputs[i].Revision
			manifest.Inputs[i].Revision = providerRevision
			manifest.Inputs[i].Blob = providerBlob
			manifest.Inputs[i].SHA256 = Digest(updated)
			for n := range manifest.Scopes {
				if manifest.Scopes[n].Path == providerPath && manifest.Scopes[n].Revision == oldRevision {
					manifest.Scopes[n].Revision = providerRevision
				}
			}
		}
	}
	return root, manifest
}

func TestPlaywrightStabilityAggregateEndToEnd(t *testing.T) {
	t.Run("DCP-V1-021 identities DCP-V1-022 distinctions DCP-V1-025 corpus join", func(t *testing.T) {
		root, manifest := stabilityFixture(t, nil, nil)
		artifact, err := Build(context.Background(), root, manifest)
		if err != nil {
			t.Fatal(err)
		}
		if len(artifact.StabilityEvidence) != 1 || len(artifact.BehaviorContracts) != 1 || len(artifact.BehaviorContracts[0].VerifiedFlows) != 1 {
			t.Fatalf("stability/behavior axes collapsed: %+v %+v", artifact.StabilityEvidence, artifact.BehaviorContracts)
		}
		report := artifact.StabilityEvidence[0]
		if report.Verdict != "clean" || report.Counts.Planned != 3 || report.Counts.Started != 3 || report.Counts.Completed != 3 || report.Counts.Passed != 3 || report.Counts.ManualReruns != 1 || len(report.ContributingReceipts) != 4 {
			t.Fatalf("aggregate counts: %+v", report)
		}
		receipt, err := Query(artifact, Request{Operation: "stability", ID: report.ID}, "fresh", nil)
		if err != nil || len(receipt.Results) != 1 {
			t.Fatalf("corpus join: %+v %v", receipt, err)
		}
	})
}

func TestPlaywrightStabilityNegativeControls(t *testing.T) {
	t.Run("DCP-V1-022 negative controls DCP-V1-023 cleanup", func(t *testing.T) {
		tests := map[string]struct {
			editReceipt  func(int, *jstestprovider.Receipt)
			editRegistry func(*StabilityRegistry)
			wantError    bool
			check        func(*testing.T, *Artifact)
		}{
			"omitted iteration": {editRegistry: func(r *StabilityRegistry) { r.Aggregates[0].Contributions = r.Aggregates[0].Contributions[1:] }, wantError: true},
			"consumed retry": {
				editReceipt: func(i int, receipt *jstestprovider.Receipt) {
					if i != 2 {
						return
					}
					receipt.Tests[0].State = jstestprovider.StateFlaky
					receipt.Tests[0].Retries = 1
					receipt.Tests[0].Attempts = []jstestprovider.Attempt{{State: jstestprovider.StateFailed, Retry: 0, FailureKind: "assertion-or-test"}, {State: jstestprovider.StatePassed, Retry: 1, FailureKind: "none"}}
					receipt.Tests[0].Artifacts = []jstestprovider.FailureArtifact{{Name: "trace", Path: "trace.zip"}}
				},
				editRegistry: func(r *StabilityRegistry) {
					r.Aggregates[0].Contributions[1].Attempts = r.Aggregates[0].Contributions[1].Attempts[1:]
				},
				wantError: true,
			},
			"failed cleanup": {
				editRegistry: func(r *StabilityRegistry) { r.Aggregates[0].Contributions[1].Attempts[0].Cleanup = "failed" },
				check: func(t *testing.T, a *Artifact) {
					report := a.StabilityEvidence[0]
					if report.Verdict != "not-stable" || report.Counts.CleanupFailed != 1 {
						t.Fatalf("cleanup failure hidden: %+v", report)
					}
				},
			},
			"failed runner cleanup": {
				editReceipt: func(i int, receipt *jstestprovider.Receipt) {
					if i == 2 {
						receipt.External.RunnerDescendantsGone = false
					}
				},
				wantError: true,
			},
			"passed attempt with assertion failure": {
				editReceipt: func(i int, receipt *jstestprovider.Receipt) {
					if i != 2 {
						return
					}
					receipt.Tests[0].Attempts[0].FailureKind = "assertion-or-test"
					receipt.Tests[0].Artifacts = []jstestprovider.FailureArtifact{{Name: "trace", Path: "contradictory-trace.zip"}}
				},
				wantError: true,
			},
			"qualified infrastructure counted": {
				editReceipt: func(i int, receipt *jstestprovider.Receipt) {
					if i != 2 {
						return
					}
					receipt.Tests[0].State = jstestprovider.StateInfrastructure
					receipt.Tests[0].Attempts = []jstestprovider.Attempt{{State: jstestprovider.StateInfrastructure, Retry: 0, FailureKind: "browser-or-fixture"}}
					receipt.Tests[0].Artifacts = []jstestprovider.FailureArtifact{{Name: "trace", Path: "infrastructure-trace.zip"}}
				},
				check: func(t *testing.T, a *Artifact) {
					report := a.StabilityEvidence[0]
					if report.Verdict != "not-stable" || report.Counts.InfrastructureFailed != 1 {
						t.Fatalf("qualified infrastructure outcome hidden: %+v", report)
					}
				},
			},
			"infrastructure with failed runner cleanup": {
				editReceipt: func(i int, receipt *jstestprovider.Receipt) {
					if i != 2 {
						return
					}
					receipt.Tests[0].State = jstestprovider.StateInfrastructure
					receipt.Tests[0].Attempts = []jstestprovider.Attempt{{State: jstestprovider.StateInfrastructure, Retry: 0, FailureKind: "browser-or-fixture"}}
					receipt.Tests[0].Artifacts = []jstestprovider.FailureArtifact{{Name: "trace", Path: "infrastructure-trace.zip"}}
					receipt.External.RunnerDescendantsGone = false
				},
				wantError: true,
			},
			"stale mapped test source": {
				editRegistry: func(r *StabilityRegistry) {
					r.Aggregates[0].Contributions[1].SourcePaths["/repo/test.ts"] = "src/value.go"
				},
				wantError: true,
			},
			"manual first cross identity": {
				editRegistry: func(r *StabilityRegistry) {
					a := &r.Aggregates[0]
					a.Contributions[3].Identity.WorkerPolicy = "workers:2"
					a.Contributions = append([]StabilityContribution{a.Contributions[3]}, a.Contributions[:3]...)
				},
				wantError: true,
			},
			"noncontiguous retry ordinal": {
				editReceipt: func(i int, receipt *jstestprovider.Receipt) {
					if i != 2 {
						return
					}
					receipt.Tests[0].State = jstestprovider.StateFlaky
					receipt.Tests[0].Retries = 2
					receipt.Tests[0].Attempts = []jstestprovider.Attempt{{State: jstestprovider.StateFailed, Retry: 0, FailureKind: "assertion-or-test"}, {State: jstestprovider.StatePassed, Retry: 2, FailureKind: "none"}}
					receipt.Tests[0].Artifacts = []jstestprovider.FailureArtifact{{Name: "trace", Path: "trace.zip"}}
				},
				wantError: true,
			},
			"manual cleanup failed": {
				editRegistry: func(r *StabilityRegistry) { r.Aggregates[0].Contributions[3].Cleanup = "failed" },
				check: func(t *testing.T, a *Artifact) {
					report := a.StabilityEvidence[0]
					if report.Verdict != "not-stable" || report.Counts.CleanupFailed != 1 {
						t.Fatalf("manual cleanup failure hidden: %+v", report)
					}
				},
			},
			"application revision change": {editRegistry: func(r *StabilityRegistry) {
				r.Aggregates[0].Contributions[1].Identity.ApplicationRevision = "different"
			}, wantError: true},
			"duplicate receipt": {editRegistry: func(r *StabilityRegistry) {
				r.Aggregates[0].Contributions[3].Receipt = r.Aggregates[0].Contributions[2].Receipt
			}, wantError: true},
		}
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				root, manifest := stabilityFixture(t, test.editReceipt, test.editRegistry)
				artifact, err := Build(context.Background(), root, manifest)
				if test.wantError {
					if err == nil {
						t.Fatal("invalid aggregate accepted")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				test.check(t, artifact)
			})
		}
	})
}

func TestPlaywrightStabilityPreservesEarlierFailureAndPolicyScopes(t *testing.T) {
	t.Run("DCP-V1-023 failure preservation DCP-V1-024 policy scopes", func(t *testing.T) {
		for _, scope := range []string{"one-spec", "feature-batch", "suite"} {
			t.Run(scope, func(t *testing.T) {
				root, manifest := stabilityFixture(t, func(i int, receipt *jstestprovider.Receipt) {
					if i != 2 {
						return
					}
					receipt.Tests[0].State = jstestprovider.StateFailed
					receipt.Tests[0].Attempts = []jstestprovider.Attempt{{State: jstestprovider.StateFailed, Retry: 0, FailureKind: "assertion-or-test"}}
					receipt.Tests[0].FailureMessage = "product count mismatch"
					receipt.Tests[0].Artifacts = []jstestprovider.FailureArtifact{{Name: "trace", Path: "trace.zip"}}
				}, func(r *StabilityRegistry) {
					r.Aggregates[0].Scope = scope
					r.Aggregates[0].Contributions[1].Attempts[0].FailureClass = "product"
				})
				artifact, err := Build(context.Background(), root, manifest)
				if err != nil {
					t.Fatal(err)
				}
				report := artifact.StabilityEvidence[0]
				if report.Scope != scope || report.Verdict != "not-stable" || report.Counts.Failed != 1 || report.Counts.Passed != 2 || report.Counts.ManualReruns != 1 || report.ContributingReceipts[1].Attempts[0].FailureClass != "product" || len(report.ContributingReceipts[1].Attempts[0].Artifacts) != 1 {
					t.Fatalf("failure or policy scope erased: %+v", report)
				}
			})
		}
	})
}

func TestPlaywrightStabilityCountsRetriesWithoutErasingFailure(t *testing.T) {
	t.Run("DCP-V1-023 flaky retry failure evidence", func(t *testing.T) {
		root, manifest := stabilityFixture(t, func(i int, receipt *jstestprovider.Receipt) {
			if i != 2 {
				return
			}
			receipt.Tests[0].State = jstestprovider.StateFlaky
			receipt.Tests[0].Retries = 1
			receipt.Tests[0].Attempts = []jstestprovider.Attempt{{State: jstestprovider.StateFailed, Retry: 0, FailureKind: "assertion-or-test"}, {State: jstestprovider.StatePassed, Retry: 1, FailureKind: "none"}}
			receipt.Tests[0].Artifacts = []jstestprovider.FailureArtifact{{Name: "trace", Path: "trace.zip"}}
		}, nil)
		artifact, err := Build(context.Background(), root, manifest)
		if err != nil {
			t.Fatal(err)
		}
		report := artifact.StabilityEvidence[0]
		if report.Verdict != "not-stable" || report.Counts.Failed != 1 || report.Counts.Flaky != 1 || report.Counts.RetryConsumed != 1 || report.Counts.Passed != 2 || len(report.ContributingReceipts[1].Attempts[0].Artifacts) != 1 {
			t.Fatalf("retry erased prior failure: %+v", report)
		}
	})
}

func TestPlaywrightStabilityCountsEarlierTimeoutWithoutErasingRecovery(t *testing.T) {
	t.Run("DCP-V1-023 timeout retry evidence", func(t *testing.T) {
		root, manifest := stabilityFixture(t, func(i int, receipt *jstestprovider.Receipt) {
			if i != 2 {
				return
			}
			receipt.Tests[0].State = jstestprovider.StateFlaky
			receipt.Tests[0].Retries = 1
			receipt.Tests[0].Attempts = []jstestprovider.Attempt{{State: jstestprovider.StateTimedOut, Retry: 0, FailureKind: "test-timeout"}, {State: jstestprovider.StatePassed, Retry: 1, FailureKind: "none"}}
			receipt.Tests[0].Artifacts = []jstestprovider.FailureArtifact{{Name: "trace", Path: "timeout-trace.zip"}}
		}, nil)
		artifact, err := Build(context.Background(), root, manifest)
		if err != nil {
			t.Fatal(err)
		}
		report := artifact.StabilityEvidence[0]
		if report.Verdict != "not-stable" || report.Counts.TimedOut != 1 || report.Counts.Flaky != 1 || report.Counts.RetryConsumed != 1 || report.Counts.Passed != 2 || report.ContributingReceipts[1].Attempts[0].FailureClass != "timeout" || len(report.ContributingReceipts[1].Attempts[0].Artifacts) != 1 {
			t.Fatalf("retry erased prior timeout: %+v", report)
		}
	})
}

func TestPlaywrightStabilityCountsEarlierInfrastructureWithoutErasingRecovery(t *testing.T) {
	root, manifest := stabilityFixture(t, func(i int, receipt *jstestprovider.Receipt) {
		if i != 2 {
			return
		}
		receipt.Tests[0].State = jstestprovider.StateFlaky
		receipt.Tests[0].Retries = 1
		receipt.Tests[0].Attempts = []jstestprovider.Attempt{{State: jstestprovider.StateInfrastructure, Retry: 0, FailureKind: "browser-or-fixture"}, {State: jstestprovider.StatePassed, Retry: 1, FailureKind: "none"}}
		receipt.Tests[0].Artifacts = []jstestprovider.FailureArtifact{{Name: "trace", Path: "infrastructure-trace.zip"}}
	}, nil)
	artifact, err := Build(context.Background(), root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	report := artifact.StabilityEvidence[0]
	if report.Verdict != "not-stable" || report.Counts.InfrastructureFailed != 1 || report.Counts.Flaky != 1 || report.Counts.RetryConsumed != 1 || report.ContributingReceipts[1].Attempts[0].FailureClass != "infrastructure" {
		t.Fatalf("retry erased prior infrastructure failure: %+v", report)
	}
}

func TestStabilityCountsEveryOutcomeDenominator(t *testing.T) {
	t.Run("DCP-V1-023 outcome denominators", func(t *testing.T) {
		tests := []struct {
			state jstestprovider.ExecutionState
			check func(StabilityCounts) bool
		}{
			{jstestprovider.StatePassed, func(c StabilityCounts) bool { return c.Completed == 1 && c.Passed == 1 }},
			{jstestprovider.StateFailed, func(c StabilityCounts) bool { return c.Completed == 1 && c.Failed == 1 }},
			{jstestprovider.StateTimedOut, func(c StabilityCounts) bool { return c.Completed == 0 && c.TimedOut == 1 }},
			{jstestprovider.StateInterrupted, func(c StabilityCounts) bool { return c.Completed == 0 && c.Interrupted == 1 }},
			{jstestprovider.StateInfrastructure, func(c StabilityCounts) bool { return c.Completed == 1 && c.InfrastructureFailed == 1 }},
			{jstestprovider.StateSkipped, func(c StabilityCounts) bool { return c.Completed == 1 && c.Skipped == 1 }},
			{jstestprovider.StateFlaky, func(c StabilityCounts) bool {
				return c.Completed == 1 && c.Flaky == 1 && c.Failed == 1 && c.RetryConsumed == 1
			}},
		}
		for _, test := range tests {
			attempts := []jstestprovider.Attempt{{State: test.state, Retry: 0}}
			evidence := []StabilityAttempt{{State: string(test.state), Retry: 0, Cleanup: "passed"}}
			if test.state == jstestprovider.StateFailed || test.state == jstestprovider.StateFlaky {
				attempts[0].State = jstestprovider.StateFailed
				evidence[0].State = string(jstestprovider.StateFailed)
			}
			if test.state == jstestprovider.StateFlaky {
				attempts = append(attempts, jstestprovider.Attempt{State: jstestprovider.StatePassed, Retry: 1})
				evidence = append(evidence, StabilityAttempt{State: string(jstestprovider.StatePassed), Retry: 1, Cleanup: "passed"})
			}
			counts := StabilityCounts{}
			countStabilityOutcome(&counts, testvaliditydoc.Test{State: string(test.state), Attempts: attempts}, &jstestprovider.Receipt{}, StabilityContribution{Cleanup: "passed", Attempts: evidence})
			if !test.check(counts) {
				t.Fatalf("%s counts: %+v", test.state, counts)
			}
		}
	})
}

func TestStabilityCountsEarlierIncompleteAttemptCategories(t *testing.T) {
	tests := []struct {
		state jstestprovider.ExecutionState
		check func(StabilityCounts) bool
	}{
		{jstestprovider.StateTimedOut, func(c StabilityCounts) bool { return c.TimedOut == 1 }},
		{jstestprovider.StateInterrupted, func(c StabilityCounts) bool { return c.Interrupted == 1 }},
		{jstestprovider.StateInfrastructure, func(c StabilityCounts) bool { return c.InfrastructureFailed == 1 }},
	}
	for _, test := range tests {
		t.Run(string(test.state), func(t *testing.T) {
			counts := StabilityCounts{}
			outcome := testvaliditydoc.Test{State: string(jstestprovider.StateFlaky), Attempts: []jstestprovider.Attempt{{State: test.state, Retry: 0}, {State: jstestprovider.StatePassed, Retry: 1}}}
			contribution := StabilityContribution{Cleanup: "passed", Attempts: []StabilityAttempt{{State: string(test.state), Retry: 0, Cleanup: "passed"}, {State: string(jstestprovider.StatePassed), Retry: 1, Cleanup: "passed"}}}
			countStabilityOutcome(&counts, outcome, &jstestprovider.Receipt{}, contribution)
			if !test.check(counts) || counts.Flaky != 1 || counts.RetryConsumed != 1 {
				t.Fatalf("earlier %s attempt erased: %+v", test.state, counts)
			}
		})
	}
}
