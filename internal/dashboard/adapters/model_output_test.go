package adapters

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/model"
)

func TestTraceModelInputsCompileWithoutRawTraceData(t *testing.T) {
	summary := TraceSummary{
		Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1", RetainedRows: 2,
		Outcomes:            []TraceOutcomeCount{{Outcome: "failed", Count: 1}, {Outcome: "passed", Count: 1}},
		repositoryWitnesses: aggregateMemberWitnesses(testRevision),
		traceIDs:            []string{strings.Repeat("1", 64), strings.Repeat("2", 64)},
	}
	members := []VerifiedTraceMember{{
		Summary: summary, ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 17,
		Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z",
	}}
	aggregate, code := AggregateTraceMembers(snapshotHeadWitness(), members)
	if code != "" {
		t.Fatal(code)
	}
	dirty := "sha256:" + strings.Repeat("2", 64)
	sourceInput, metrics, issues, err := TraceModelInputs(0, aggregate, members, &dirty, model.CompletenessComplete, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 3 || sourceInput.Members == nil || len(*sourceInput.Members) != 1 {
		t.Fatalf("source/metrics = %+v %+v", sourceInput, metrics)
	}
	_, encoded, err := model.Compile(model.Input{
		GeneratedAt: "2026-08-23T12:00:00.000000000Z",
		Observation: model.ObservationInput{ClockSource: model.ClockCaller, Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z", ScanState: model.ScanComplete},
		Repository:  model.Repository{DirtyPathCount: pointer("0"), DirtyPathsSHA256: &dirty, HeadRevision: pointer(testRevision), ObjectFormat: pointer("sha1"), TreeRevision: pointer(testTree), WorktreeState: model.WorktreeClean},
		Registry:    Registrations(), Sources: []model.SourceInput{sourceInput}, Metrics: metrics, Issues: issues,
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(issues) != 1 || issues[0].Code != "OBSERVATION_TIME_UNKNOWN" || issues[0].Severity != model.SeverityInfo ||
		issues[0].Observed != nil || issues[0].Limit != nil || len(sourceInput.Exclusions) != 0 {
		t.Fatalf("observation issue/source exclusions = %#v %#v", issues, sourceInput.Exclusions)
	}
	for _, raw := range []string{"private incident", "internal/", "go test", "trace_id"} {
		if strings.Contains(string(encoded), raw) {
			t.Fatalf("snapshot disclosed %q: %s", raw, encoded)
		}
	}
}

func TestTraceModelInputsPreserveOrdinalAndMergeExactUsageKeys(t *testing.T) {
	summary := TraceSummary{
		Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1", RetainedRows: 2,
		Outcomes:            []TraceOutcomeCount{{Outcome: "failed", Count: 1}, {Outcome: "passed", Count: 1}},
		repositoryWitnesses: aggregateMemberWitnesses(testRevision),
		traceIDs:            []string{strings.Repeat("1", 64), strings.Repeat("2", 64)},
	}
	members := []VerifiedTraceMember{{
		Summary: summary, ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 17,
		Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z",
	}}
	aggregate, code := AggregateTraceMembers(snapshotHeadWitness(), members)
	if code != "" {
		t.Fatal(code)
	}
	dirty := "sha256:" + strings.Repeat("2", 64)
	first, firstMetrics, firstIssues, err := TraceModelInputs(1, aggregate, members, &dirty, model.CompletenessComplete, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, secondMetrics, secondIssues, err := TraceModelInputs(2, aggregate, members, &dirty, model.CompletenessComplete, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfiguredOrdinal != "1" || second.ConfiguredOrdinal != "2" {
		t.Fatalf("ordinals = %q, %q", first.ConfiguredOrdinal, second.ConfiguredOrdinal)
	}
	merged, err := MergeTraceUsageMetrics(append(firstMetrics, secondMetrics...))
	if err != nil || len(merged) != 3 {
		t.Fatalf("merged=%#v err=%v", merged, err)
	}
	for _, metric := range merged {
		if len(metric.SourceIDs) != 2 || metric.Denominator == nil || *metric.Denominator != "4" {
			t.Fatalf("merged metric = %#v", metric)
		}
		outcome := *metric.Dimensions[0].Value
		want := "2"
		if outcome == "blocked" {
			want = "0"
		}
		if metric.Value == nil || *metric.Value != want || metric.Numerator == nil || *metric.Numerator != want {
			t.Fatalf("merged %s metric = %#v", outcome, metric)
		}
	}
	_, encoded, err := model.Compile(model.Input{
		GeneratedAt: "2026-08-23T12:00:00.000000000Z",
		Observation: model.ObservationInput{ClockSource: model.ClockCaller, Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z", ScanState: model.ScanComplete},
		Repository: model.Repository{DirtyPathCount: pointer("0"), DirtyPathsSHA256: &dirty, HeadRevision: pointer(testRevision),
			ObjectFormat: pointer("sha1"), TreeRevision: pointer(testTree), WorktreeState: model.WorktreeClean},
		Registry: Registrations(), Sources: []model.SourceInput{first, second}, Metrics: merged,
		Issues: append(firstIssues, secondIssues...),
	})
	if err != nil {
		t.Fatalf("compile merged metrics: %v", err)
	}
	if strings.Contains(string(encoded), "internal/") {
		t.Fatalf("snapshot disclosed path: %s", encoded)
	}
}

func TestTraceModelInputsPartialUsesOnlyTerminalExclusionsAndStillReportsUnknownTime(t *testing.T) {
	summary := TraceSummary{
		Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1", RetainedRows: 1,
		Outcomes:            []TraceOutcomeCount{{Outcome: "passed", Count: 1}},
		repositoryWitnesses: aggregateMemberWitnesses(testRevision), traceIDs: []string{strings.Repeat("1", 64)},
	}
	members := []VerifiedTraceMember{{
		Summary: summary, ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 17,
		Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z",
	}}
	aggregate, code := AggregateTraceMembers(snapshotHeadWitness(), members)
	if code != "" {
		t.Fatal(code)
	}
	dirty := "sha256:" + strings.Repeat("2", 64)
	sourceInput, _, issues, err := TraceModelInputs(0, aggregate, members, &dirty, model.CompletenessPartial,
		[]TraceTerminalCount{{Code: IssueVerifierRejected, Count: 2}})
	if err != nil || len(issues) != 2 || len(sourceInput.Exclusions) != 1 {
		t.Fatalf("source=%#v issues=%#v err=%v", sourceInput, issues, err)
	}
	for _, issue := range issues {
		switch issue.Code {
		case "VERIFIER_REJECTED":
			if issue.Observed == nil || *issue.Observed != "2" || issue.Severity != model.SeverityError {
				t.Fatalf("terminal issue = %#v", issue)
			}
		case "OBSERVATION_TIME_UNKNOWN":
			if issue.Observed != nil || issue.Limit != nil || issue.Severity != model.SeverityInfo {
				t.Fatalf("observation issue = %#v", issue)
			}
		default:
			t.Fatalf("unexpected issue = %#v", issue)
		}
	}
	observation, _ := model.NewIssue(issues[1])
	if sourceInput.Exclusions[0] == observation.ID {
		t.Fatal("observation-time issue entered source exclusions")
	}
	if _, _, _, err := TraceModelInputs(0, aggregate, members, &dirty, model.CompletenessPartial, nil); err == nil {
		t.Fatal("partial trace source accepted without a terminal member count")
	}
}
