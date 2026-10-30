package appflows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// Wire identity of the separate run registry and its stability report (AFU-V1-041, AFU-V1-042).
const (
	RunRegistrySchema  = "flows-run-registry/0"
	RunStabilitySchema = "flows-run-stability/0"
)

// Named refusals of a run-registry aggregate; each leaves no verdict (AFU-V1-042, DCP-V1-022).
const (
	RegistryInvalid          = "run-registry-invalid"
	RegistryRepetition       = "run-registry-invalid-repetition"
	RegistryIncomplete       = "run-registry-planned-incomplete"
	RegistryDuplicateRun     = "run-registry-duplicate-run"
	RegistryMissingRecord    = "run-registry-missing-record"
	RegistryAmbiguousRecord  = "run-registry-ambiguous-record"
	RegistryUnusableRecord   = "run-registry-unusable-record"
	RegistryInfrastructure   = "run-registry-contradictory-infrastructure"
	RegistryCrossIdentity    = "run-registry-cross-identity"
	runKindPlanned           = "planned-repetition"
	runKindManual            = "manual-rerun"
	stabilityVerdictClean    = "clean"
	stabilityVerdictUnstable = "not-stable"
)

var (
	stabilityScopes = []string{"one-spec", "feature-batch", "suite"}
	// completedResults are the classifications of a repetition that ran to an end (DCP-V1-023).
	completedResults = []string{"passed", "failed", "flaky", "skipped"}
	// infrastructureOutcomes are the attempt outcomes an owner may class as an infrastructure failure.
	infrastructureOutcomes = []string{"failed", "timedOut", "interrupted"}
)

// RunRegistry is the repository-owned source of what a run-evidence record does not carry: planned
// ordinals, run kind, the infrastructure class and the stability policy (AFU-V1-041, decision 0417).
type RunRegistry struct {
	Schema     string         `json:"schema"`
	Policy     RunPolicy      `json:"policy"`
	Aggregates []RunAggregate `json:"aggregates"`
}

type RunPolicy struct {
	ID         string                         `json:"id"`
	Thresholds []doccorpus.StabilityThreshold `json:"thresholds"`
}

type RunAggregate struct {
	ID            string            `json:"id"`
	Scope         string            `json:"scope"`
	TestKey       string            `json:"test_key"`
	Project       string            `json:"project,omitempty"`
	Planned       int               `json:"planned"`
	Contributions []RunContribution `json:"contributions"`
}

type RunContribution struct {
	RunID                  string `json:"run_id"`
	RunKind                string `json:"run_kind"`
	Repetition             int    `json:"repetition"`
	InfrastructureAttempts []int  `json:"infrastructure_attempts"`
}

// RunStabilityReport is `flows stability` (AFU-V1-042).
type RunStabilityReport struct {
	Schema         string               `json:"schema"`
	Revision       string               `json:"revision"`
	Registry       string               `json:"registry"`
	RegistrySHA256 string               `json:"registry_sha256"`
	PolicyID       string               `json:"policy_id"`
	Aggregates     []RunAggregateReport `json:"aggregates"`
	Limitations    []string             `json:"limitations"`
}

type RunAggregateReport struct {
	ID            string                       `json:"id"`
	Scope         string                       `json:"scope"`
	TestKey       string                       `json:"test_key"`
	Project       string                       `json:"project"`
	Runner        RunRunner                    `json:"runner"`
	Source        RunSource                    `json:"source"`
	Threshold     doccorpus.StabilityThreshold `json:"threshold"`
	Counts        doccorpus.StabilityCounts    `json:"counts"`
	Verdict       string                       `json:"verdict"`
	Contributions []RunContributionReport      `json:"contributions"`
}

type RunContributionReport struct {
	RunContribution
	Classification string       `json:"classification"`
	Cleanup        string       `json:"cleanup"`
	Attempts       []RunAttempt `json:"attempts"`
}

// runJoin is the exact key a contribution joins its record by (AFU-V1-041).
type runJoin struct{ runID, testKey, project string }

// runIdentity is what every contribution of one aggregate must share (DCP-V1-022).
type runIdentity struct {
	runner      RunRunner
	source      RunSource
	build       string
	environment RunDigestRef
	fixture     RunDigestRef
	project     string
}

var stabilityLimitations = []string{
	"stability is repeated-run evidence, not test adequacy or behavior parity",
	"planned ordinals, run kind and infrastructure class are repository-owned registry declarations",
	"manual reruns are retained but never satisfy planned repetition thresholds",
}

// FlowStability aggregates repeated runs of each registry aggregate from the registry committed at
// HEAD and the given records (AFU-V1-041, AFU-V1-042). It reads only; any refusal leaves no report.
func FlowStability(ctx context.Context, root, name string, evidence []TestRunEvidence) ([]byte, error) {
	revision, raw, err := committedFileAtHead(ctx, root, name)
	if err != nil {
		return nil, err
	}
	var registry RunRegistry
	if err = Decode(raw, &registry); err != nil {
		return nil, err
	}
	thresholds, err := registryThresholds(registry)
	if err != nil {
		return nil, err
	}
	records := joinRecords(evidence)
	report := RunStabilityReport{Schema: RunStabilitySchema, Revision: revision, Registry: name, RegistrySHA256: Digest(raw),
		PolicyID: registry.Policy.ID, Aggregates: []RunAggregateReport{}, Limitations: stabilityLimitations}
	seen := map[string]bool{}
	for _, aggregate := range registry.Aggregates {
		if seen[aggregate.ID] {
			return nil, refuseAggregate(RegistryInvalid, aggregate.ID)
		}
		seen[aggregate.ID] = true
		result, err := aggregateRuns(aggregate, thresholds, records)
		if err != nil {
			return nil, err
		}
		report.Aggregates = append(report.Aggregates, result)
	}
	return doccorpus.Encode(report)
}

// committedFileAtHead reads one regular committed file at HEAD, never the working tree.
func committedFileAtHead(ctx context.Context, root, name string) (string, []byte, error) {
	if !safePath(name) {
		return "", nil, errors.New("--registry must be a canonical repository-relative path")
	}
	revision, err := ResolveRevision(ctx, root, "HEAD")
	if err != nil {
		return "", nil, err
	}
	out, err := git(ctx, root, "--literal-pathspecs", "ls-tree", "-z", "-l", "--full-tree", revision, "--", name)
	meta, entryPath, _ := strings.Cut(string(bytes.TrimSuffix(out, []byte{0})), "\t")
	fields := strings.Fields(meta)
	if err != nil || len(fields) != 4 || entryPath != name {
		return "", nil, fmt.Errorf("%s: run registry is absent at HEAD; commit it first", name)
	}
	raw, err := committedBlob(ctx, root, treeEntry{name: name, mode: fields[0], kind: fields[1], oid: fields[2], size: fields[3]})
	return revision, raw, err
}

// registryThresholds checks the registry's closed shape and returns one threshold per scope (DCP-V1-024).
func registryThresholds(r RunRegistry) (map[string]doccorpus.StabilityThreshold, error) {
	if r.Schema != RunRegistrySchema || !flowText(r.Policy.ID) || len(r.Aggregates) == 0 || len(r.Aggregates) > maxRunRecords {
		return nil, errors.New(RegistryInvalid)
	}
	thresholds := map[string]doccorpus.StabilityThreshold{}
	for _, t := range r.Policy.Thresholds {
		if _, repeated := thresholds[t.Scope]; repeated || !validThreshold(t) {
			return nil, errors.New(RegistryInvalid)
		}
		thresholds[t.Scope] = t
	}
	if len(thresholds) != len(stabilityScopes) {
		return nil, errors.New(RegistryInvalid)
	}
	return thresholds, nil
}

func validThreshold(t doccorpus.StabilityThreshold) bool {
	scoped := slices.Contains(stabilityScopes, t.Scope)
	bounded := t.RequiredRepetitions >= 1 && t.RequiredRepetitions <= maxRunRecords
	return scoped && bounded && t.MinimumPassed >= 0 && t.MinimumPassed <= t.RequiredRepetitions && !doccorpus.NegativeStabilityThreshold(t)
}

func joinRecords(evidence []TestRunEvidence) map[runJoin][]TestRunEvidence {
	records := map[runJoin][]TestRunEvidence{}
	for _, r := range evidence {
		key := runJoin{r.RunID, r.TestKey, r.Project}
		records[key] = append(records[key], r)
	}
	return records
}

// aggregateRuns applies DCP-V1-022 to one aggregate, then counts it under DCP-V1-023 and judges it
// under DCP-V1-024. A refusal never shrinks the denominator into a verdict.
func aggregateRuns(a RunAggregate, thresholds map[string]doccorpus.StabilityThreshold, records map[runJoin][]TestRunEvidence) (RunAggregateReport, error) {
	threshold, scoped := thresholds[a.Scope]
	if !validAggregate(a) || !scoped || a.Planned != threshold.RequiredRepetitions {
		return RunAggregateReport{}, refuseAggregate(RegistryInvalid, a.ID)
	}
	report := RunAggregateReport{ID: a.ID, Scope: a.Scope, TestKey: a.TestKey, Project: a.Project, Threshold: threshold,
		Counts: doccorpus.StabilityCounts{Planned: a.Planned}, Contributions: []RunContributionReport{}}
	planned := map[int]bool{}
	runs := map[string]bool{}
	var baseline *runIdentity
	for _, c := range a.Contributions {
		if err := checkRepetition(a, c, planned); err != nil {
			return RunAggregateReport{}, err
		}
		if runs[c.RunID] {
			return RunAggregateReport{}, refuseAggregate(RegistryDuplicateRun, a.ID)
		}
		runs[c.RunID] = true
		record, err := joinedRecord(a, c, records)
		if err != nil {
			return RunAggregateReport{}, err
		}
		identity := identityOf(record)
		if baseline == nil {
			baseline = &identity
		}
		if identity != *baseline {
			return RunAggregateReport{}, refuseAggregate(RegistryCrossIdentity, a.ID)
		}
		countContribution(&report.Counts, c, record)
		report.Contributions = append(report.Contributions, RunContributionReport{RunContribution: c,
			Classification: Classify(record.Attempts), Cleanup: record.Cleanup, Attempts: record.Attempts})
	}
	if len(planned) != a.Planned {
		return RunAggregateReport{}, refuseAggregate(RegistryIncomplete, a.ID)
	}
	report.Runner, report.Source = baseline.runner, baseline.source
	report.Verdict = stabilityVerdictUnstable
	if doccorpus.StabilityThresholdPassed(report.Counts, threshold) {
		report.Verdict = stabilityVerdictClean
	}
	return report, nil
}

func validAggregate(a RunAggregate) bool {
	identified := flowText(a.ID) && flowText(a.TestKey) && (a.Project == "" || flowText(a.Project))
	shaped := len(a.Contributions) <= maxRunRecords && !slices.ContainsFunc(a.Contributions, func(c RunContribution) bool {
		return !flowText(c.RunID) || c.InfrastructureAttempts == nil
	})
	return identified && shaped
}

// checkRepetition admits a planned ordinal once, within 1..planned, and a manual rerun only without
// one, so a manual rerun never fills a planned repetition (DCP-V1-022).
func checkRepetition(a RunAggregate, c RunContribution, planned map[int]bool) error {
	switch c.RunKind {
	case runKindPlanned:
		if c.Repetition < 1 || c.Repetition > a.Planned || planned[c.Repetition] {
			return refuseAggregate(RegistryRepetition, a.ID)
		}
		planned[c.Repetition] = true
	case runKindManual:
		if c.Repetition != 0 {
			return refuseAggregate(RegistryRepetition, a.ID)
		}
	default:
		return refuseAggregate(RegistryInvalid, a.ID)
	}
	return nil
}

// joinedRecord is the one usable record a contribution names, with its infrastructure ordinals
// checked against that record's attempts.
func joinedRecord(a RunAggregate, c RunContribution, records map[runJoin][]TestRunEvidence) (TestRunEvidence, error) {
	matches := records[runJoin{c.RunID, a.TestKey, a.Project}]
	if len(matches) == 0 {
		return TestRunEvidence{}, refuseAggregate(RegistryMissingRecord, a.ID)
	}
	if len(matches) > 1 {
		return TestRunEvidence{}, refuseAggregate(RegistryAmbiguousRecord, a.ID)
	}
	record := matches[0]
	if record.Authority == AuthorityStatic || !record.Source.Clean {
		return TestRunEvidence{}, refuseAggregate(RegistryUnusableRecord, a.ID)
	}
	if !infrastructureConsistent(c.InfrastructureAttempts, record.Attempts) {
		return TestRunEvidence{}, refuseAggregate(RegistryInfrastructure, a.ID)
	}
	return record, nil
}

func infrastructureConsistent(ordinals []int, attempts []RunAttempt) bool {
	seen := map[int]bool{}
	for _, ordinal := range ordinals {
		if seen[ordinal] || ordinal < 1 || ordinal > len(attempts) {
			return false
		}
		seen[ordinal] = true
		if !slices.Contains(infrastructureOutcomes, attempts[ordinal-1].Outcome) {
			return false
		}
	}
	return true
}

func identityOf(r TestRunEvidence) runIdentity {
	return runIdentity{runner: r.Runner, source: r.Source, build: r.BuildArtifactDigest, environment: r.Environment, fixture: r.Fixture, project: r.Project}
}

// countContribution adds one contribution to the DCP-V1-023 raw counts. A manual rerun adds only its
// own count and its cleanup, never a planned outcome.
func countContribution(counts *doccorpus.StabilityCounts, c RunContribution, r TestRunEvidence) {
	counts.CleanupFailed += boolCount(r.Cleanup != "done")
	if c.RunKind == runKindManual {
		counts.ManualReruns++
		return
	}
	result := Classify(r.Attempts)
	counts.Started++
	counts.Completed += boolCount(slices.Contains(completedResults, result))
	counts.Passed += boolCount(result == "passed")
	counts.Flaky += boolCount(result == "flaky")
	counts.Failed += boolCount(hasOutcome(r.Attempts, "failed"))
	counts.TimedOut += boolCount(hasOutcome(r.Attempts, "timedOut"))
	counts.Interrupted += boolCount(hasOutcome(r.Attempts, "interrupted"))
	counts.Skipped += boolCount(hasOutcome(r.Attempts, "skipped"))
	counts.InfrastructureFailed += boolCount(len(c.InfrastructureAttempts) != 0)
	counts.RetryConsumed += max(len(r.Attempts)-1, 0)
}

func hasOutcome(attempts []RunAttempt, outcome string) bool {
	return slices.ContainsFunc(attempts, func(a RunAttempt) bool { return a.Outcome == outcome })
}

func boolCount(b bool) int {
	if b {
		return 1
	}
	return 0
}

func refuseAggregate(code, aggregate string) error {
	return fmt.Errorf("%s: aggregate %q", code, aggregate)
}
