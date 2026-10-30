package appflows

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func stabilityRecord(runID, testKey, cleanup string, outcomes ...string) TestRunEvidence {
	h := runHeader()
	r := TestRunEvidence{Schema: RunEvidenceSchema, Authority: AuthorityIngested, RunID: runID, Runner: RunRunner{Name: "playwright", Version: h.RunnerVersion},
		Source: h.Source, BuildArtifactDigest: h.BuildArtifactDigest, Environment: h.Environment, Fixture: h.Fixture, TestKey: testKey, Project: "chromium",
		Attempts: []RunAttempt{}, Cleanup: cleanup, NegativeControls: []NegativeControl{}}
	for i, outcome := range outcomes {
		r.Attempts = append(r.Attempts, RunAttempt{Ordinal: i + 1, Outcome: outcome, DurationMS: 10, AssertionAnchors: []RunAnchor{}, Attachments: []RunAttachment{}})
	}
	return r
}

// stabilityRecords are three planned repetitions and one manual rerun of "checkout > pays", and two
// planned repetitions of "checkout > declines".
func stabilityRecords() []TestRunEvidence {
	return []TestRunEvidence{
		stabilityRecord("run-1", "checkout > pays", "done", "passed"),
		stabilityRecord("run-2", "checkout > pays", "done", "failed", "passed"),
		stabilityRecord("run-3", "checkout > pays", "done", "passed"),
		stabilityRecord("run-m", "checkout > pays", "done", "failed"),
		stabilityRecord("run-1", "checkout > declines", "done", "timedOut"),
		stabilityRecord("run-2", "checkout > declines", "not-declared", "passed"),
	}
}

func planned(runID string, repetition int, infrastructure ...int) RunContribution {
	return RunContribution{RunID: runID, RunKind: runKindPlanned, Repetition: repetition, InfrastructureAttempts: append([]int{}, infrastructure...)}
}

func stabilityRegistry() RunRegistry {
	threshold := func(scope string, required int) doccorpus.StabilityThreshold {
		return doccorpus.StabilityThreshold{Scope: scope, RequiredRepetitions: required, MinimumPassed: required - 1, MaximumFailed: 1, MaximumTimedOut: 1,
			MaximumInfrastructureFailed: 1, MaximumFlaky: 1, MaximumRetryConsumed: 2}
	}
	return RunRegistry{Schema: RunRegistrySchema,
		Policy: RunPolicy{ID: "e2e-stability", Thresholds: []doccorpus.StabilityThreshold{threshold("one-spec", 3), threshold("feature-batch", 2), threshold("suite", 5)}},
		Aggregates: []RunAggregate{
			{ID: "pays", Scope: "one-spec", TestKey: "checkout > pays", Project: "chromium", Planned: 3, Contributions: []RunContribution{
				planned("run-1", 1), planned("run-2", 2, 1), planned("run-3", 3), {RunID: "run-m", RunKind: runKindManual, InfrastructureAttempts: []int{}}}},
			{ID: "declines", Scope: "feature-batch", TestKey: "checkout > declines", Project: "chromium", Planned: 2, Contributions: []RunContribution{
				planned("run-1", 1), planned("run-2", 2)}},
		}}
}

// stabilityRepo commits the registry as runs/registry.json.
func stabilityRepo(t *testing.T, registry RunRegistry) string {
	t.Helper()
	root := t.TempDir()
	raw, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(t, root, "runs/registry.json", raw)
	gitTest(t, root, "init", "-q")
	commitAll(t, root, "registry")
	return root
}

func stability(t *testing.T, registry RunRegistry, records []TestRunEvidence) (RunStabilityReport, error) {
	t.Helper()
	data, err := FlowStability(context.Background(), stabilityRepo(t, registry), "runs/registry.json", records)
	var report RunStabilityReport
	if err == nil {
		err = json.Unmarshal(data, &report)
	}
	return report, err
}

// AFU-V1-013 AFU-V1-041 AFU-V1-042
func TestAFUV1RunStabilityCounts(t *testing.T) {
	report, err := stability(t, stabilityRegistry(), stabilityRecords())
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != RunStabilitySchema || report.PolicyID != "e2e-stability" || len(report.Aggregates) != 2 || len(report.Limitations) == 0 {
		t.Fatalf("report %+v", report)
	}
	pays, declines := report.Aggregates[0], report.Aggregates[1]
	wantPays := doccorpus.StabilityCounts{Planned: 3, Started: 3, Completed: 3, Passed: 2, Failed: 1, InfrastructureFailed: 1, Flaky: 1, RetryConsumed: 1, ManualReruns: 1}
	if pays.Counts != wantPays || pays.Verdict != stabilityVerdictClean || pays.Threshold.Scope != "one-spec" || pays.Runner.Name != "playwright" {
		t.Fatalf("pays %+v", pays)
	}
	flaky := pays.Contributions[1]
	if flaky.Classification != "flaky" || outcomes(TestRunEvidence{Attempts: flaky.Attempts}) != "failed,passed" || flaky.InfrastructureAttempts[0] != 1 {
		t.Fatalf("earlier attempts must stay in the report: %+v", flaky)
	}
	if manual := pays.Contributions[3]; manual.RunKind != runKindManual || manual.Classification != "failed" {
		t.Fatalf("manual rerun kept but outside the threshold: %+v", manual)
	}
	wantDeclines := doccorpus.StabilityCounts{Planned: 2, Started: 2, Completed: 1, Passed: 1, TimedOut: 1, CleanupFailed: 1}
	if declines.Counts != wantDeclines || declines.Verdict != stabilityVerdictUnstable {
		t.Fatalf("declines %+v", declines)
	}
}

// AFU-V1-041 AFU-V1-042
func TestAFUV1RunStabilityRefusals(t *testing.T) {
	pays := func(edit func(*RunAggregate)) func(*RunRegistry, *[]TestRunEvidence) {
		return func(r *RunRegistry, _ *[]TestRunEvidence) { edit(&r.Aggregates[0]) }
	}
	record := func(i int, edit func(*TestRunEvidence)) func(*RunRegistry, *[]TestRunEvidence) {
		return func(_ *RunRegistry, records *[]TestRunEvidence) { edit(&(*records)[i]) }
	}
	for name, c := range map[string]struct {
		edit func(*RunRegistry, *[]TestRunEvidence)
		code string
	}{
		"schema":                 {func(r *RunRegistry, _ *[]TestRunEvidence) { r.Schema = "flows-run-registry/1" }, RegistryInvalid},
		"scope missing":          {func(r *RunRegistry, _ *[]TestRunEvidence) { r.Policy.Thresholds = r.Policy.Thresholds[:2] }, RegistryInvalid},
		"minimum above required": {func(r *RunRegistry, _ *[]TestRunEvidence) { r.Policy.Thresholds[0].MinimumPassed = 4 }, RegistryInvalid},
		"planned mismatch":       {pays(func(a *RunAggregate) { a.Planned = 2 }), RegistryInvalid},
		"run kind":               {pays(func(a *RunAggregate) { a.Contributions[3].RunKind = "retry" }), RegistryInvalid},
		"out of range":           {pays(func(a *RunAggregate) { a.Contributions[2].Repetition = 4 }), RegistryRepetition},
		"repeated ordinal":       {pays(func(a *RunAggregate) { a.Contributions[2].Repetition = 2 }), RegistryRepetition},
		"manual ordinal":         {pays(func(a *RunAggregate) { a.Contributions[3].Repetition = 3 }), RegistryRepetition},
		"manual fills gap": {pays(func(a *RunAggregate) {
			a.Contributions = append(a.Contributions[:2], RunContribution{RunID: "run-3", RunKind: runKindManual, InfrastructureAttempts: []int{}})
		}), RegistryIncomplete},
		"duplicate run":    {pays(func(a *RunAggregate) { a.Contributions[3].RunID = "run-2" }), RegistryDuplicateRun},
		"missing record":   {pays(func(a *RunAggregate) { a.Contributions[2].RunID = "run-9" }), RegistryMissingRecord},
		"project join":     {pays(func(a *RunAggregate) { a.Project = "firefox" }), RegistryMissingRecord},
		"ambiguous record": {func(_ *RunRegistry, records *[]TestRunEvidence) { *records = append(*records, (*records)[2]) }, RegistryAmbiguousRecord},
		"static record":    {record(0, func(r *TestRunEvidence) { r.Authority = AuthorityStatic }), RegistryUnusableRecord},
		"dirty source":     {record(0, func(r *TestRunEvidence) { r.Source.Clean = false }), RegistryUnusableRecord},
		"infra on a pass":  {pays(func(a *RunAggregate) { a.Contributions[1].InfrastructureAttempts = []int{2} }), RegistryInfrastructure},
		"infra absent":     {pays(func(a *RunAggregate) { a.Contributions[1].InfrastructureAttempts = []int{3} }), RegistryInfrastructure},
		"infra repeated":   {pays(func(a *RunAggregate) { a.Contributions[1].InfrastructureAttempts = []int{1, 1} }), RegistryInfrastructure},
		"runner version":   {record(2, func(r *TestRunEvidence) { r.Runner.Version = "1.51.0" }), RegistryCrossIdentity},
		"other source":     {record(3, func(r *TestRunEvidence) { r.Source.Commit = strings.Repeat("3", 40) }), RegistryCrossIdentity},
		"other fixture":    {record(1, func(r *TestRunEvidence) { r.Fixture.ID = "seed-2" }), RegistryCrossIdentity},
	} {
		t.Run(name, func(t *testing.T) {
			registry, records := stabilityRegistry(), stabilityRecords()
			c.edit(&registry, &records)
			report, err := stability(t, registry, records)
			if err == nil || !strings.HasPrefix(err.Error(), c.code) {
				t.Fatalf("want %s and no verdict, got %v %+v", c.code, err, report)
			}
		})
	}
}

// AFU-V1-041
func TestAFUV1RunRegistryReadAtHead(t *testing.T) {
	root := stabilityRepo(t, stabilityRegistry())
	committed := gitOut(t, root, "show", "HEAD:runs/registry.json")
	edited := stabilityRegistry()
	edited.Policy.ID = "working-tree-edit"
	raw, err := json.Marshal(edited)
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(t, root, "runs/registry.json", raw)
	data, err := FlowStability(context.Background(), root, "runs/registry.json", stabilityRecords())
	var report RunStabilityReport
	if err != nil || json.Unmarshal(data, &report) != nil {
		t.Fatal(err)
	}
	if report.PolicyID != "e2e-stability" || report.RegistrySHA256 != Digest([]byte(committed)) || report.Revision != gitOut(t, root, "rev-parse", "HEAD") {
		t.Fatalf("registry must be the committed bytes at HEAD: %+v", report)
	}
	writeRaw(t, root, "runs/uncommitted.json", raw)
	for _, name := range []string{"runs/uncommitted.json", "../registry.json", "/runs/registry.json"} {
		if _, err := FlowStability(context.Background(), root, name, stabilityRecords()); err == nil {
			t.Fatalf("%s must be refused", name)
		}
	}
}
