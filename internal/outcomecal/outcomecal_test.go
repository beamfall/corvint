package outcomecal

import (
	"math"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/trace"
)

func observations(count int, stance Stance, success bool) []Observation {
	rows := make([]Observation, count)
	for index := range rows {
		rows[index] = Observation{Stance: stance, Success: success}
	}
	return rows
}

func scored(count int, score float64, success bool) []Observation {
	rows := make([]Observation, count)
	for index := range rows {
		rows[index] = Observation{Stance: Unknown, Success: success, Score: score, HasScore: true}
	}
	return rows
}

// OCL-V0-003: a record with no packet stance field classifies as unknown and is
// counted explicitly with the reason, never folded into answered or abstained.
func TestUnknownClassificationIsCountedExplicitly_OCL003(t *testing.T) {
	rows := []Observation{Observe(trace.Record{Outcome: "passed"}), Observe(trace.Record{Outcome: "failed"}), Observe(trace.Record{Outcome: "blocked"})}
	for _, row := range rows {
		if row.Stance != Unknown || row.HasScore {
			t.Fatalf("record projected a stance or score it does not carry: %+v", row)
		}
	}
	report := Build(rows, "clean")
	if report.Matrix.Classified() != 0 || report.Matrix.UnknownSuccess != 1 || report.Matrix.UnknownFailure != 2 {
		t.Fatalf("unknown rows misclassified: %+v", report.Matrix)
	}
	if report.HasAccuracy || report.UnknownReason == "" {
		t.Fatalf("accuracy invented or reason missing: %+v", report)
	}
	payload := report.Payload()
	if payload["abstention_accuracy"] != nil || payload["unknown"].(map[string]any)["total"] != 3 {
		t.Fatalf("payload hides the unknown sample: %v", payload)
	}
}

// OCL-V0-004: the 2x2 counts each stance against each recorded outcome and
// abstention accuracy scores answered-success plus abstained-failure.
func TestMatrixAndAbstentionAccuracy_OCL004(t *testing.T) {
	rows := append(observations(6, Answered, true), observations(2, Answered, false)...)
	rows = append(rows, observations(3, Abstained, false)...)
	rows = append(rows, observations(1, Abstained, true)...)
	report := Build(rows, "clean")
	matrix := report.Matrix
	if matrix.AnsweredSuccess != 6 || matrix.AnsweredFailure != 2 || matrix.AbstainedFailure != 3 || matrix.AbstainedSuccess != 1 {
		t.Fatalf("matrix: %+v", matrix)
	}
	if !report.HasAccuracy || report.Accuracy != 9.0/12.0 {
		t.Fatalf("accuracy: %v %v", report.Accuracy, report.HasAccuracy)
	}
}

// OCL-V0-005: scores bucket into the ten fixed deciles, each bucket carrying
// its record and success counts, and empty buckets stay in the fixed order.
func TestScoreDecileBuckets_OCL005(t *testing.T) {
	rows := append(scored(3, 0.05, false), scored(1, 0.05, true)...)
	rows = append(rows, scored(2, 0.95, true)...)
	report := Build(rows, "clean")
	if report.ScoreCoverage != 6 || len(report.Buckets) != 10 {
		t.Fatalf("coverage %d buckets %d", report.ScoreCoverage, len(report.Buckets))
	}
	if report.Buckets[0].Records != 4 || report.Buckets[0].Success != 1 || report.Buckets[9].Records != 2 {
		t.Fatalf("buckets: %+v", report.Buckets)
	}
	payload := report.Payload()["buckets"].([]any)
	first, middle := payload[0].(map[string]any), payload[5].(map[string]any)
	if first["records"] != 4 || first["success"] != 1 || first["lower_tenths"] != 0 || middle["records"] != 0 {
		t.Fatalf("bucket payload: %v", payload)
	}
}

// OCL-V0-005: an out-of-range score clamps to its end bucket on every
// architecture, and a NaN score is not a score.
func TestScoreClampIsArchitectureIndependent_OCL005(t *testing.T) {
	rows := append(scored(1, math.Inf(1), true), scored(1, 1e300, true)...)
	rows = append(rows, scored(1, math.Inf(-1), false)...)
	rows = append(rows, scored(1, math.NaN(), true)...)
	report := Build(rows, "clean")
	if report.Buckets[9].Records != 2 || report.Buckets[0].Records != 1 || report.ScoreCoverage != 3 {
		t.Fatalf("coverage %d buckets %+v", report.ScoreCoverage, report.Buckets)
	}
}

// OCL-V0-006: the proposal reports the threshold that maximises abstention
// accuracy on the sample, is labelled applied=false, and carries the gate
// sentence stating it is never applied by this command.
func TestProposalIsReportedButNeverApplied_OCL006(t *testing.T) {
	rows := append(scored(12, 0.15, false), scored(12, 0.85, true)...)
	report := Build(rows, "clean")
	if report.ProposalState != ProposalProposed {
		t.Fatalf("state %s", report.ProposalState)
	}
	if report.Proposal.ThresholdTenths != 2 || report.Proposal.Accuracy() != 1 || report.Proposal.SampleSize != 24 {
		t.Fatalf("proposal: %+v", report.Proposal)
	}
	proposal := report.Payload()["proposal"].(map[string]any)
	if proposal["applied"] != false || !strings.Contains(proposal["gate"].(string), "never applied") {
		t.Fatalf("proposal payload: %v", proposal)
	}
	if !strings.Contains(proposal["gate"].(string), "LTA-V0-001..LTA-V0-003") {
		t.Fatalf("gate sentence does not name the learned-trace admission gate: %v", proposal["gate"])
	}
	if !strings.Contains(report.Table(), "applied false") {
		t.Fatalf("table hides the applied label: %s", report.Table())
	}
}

// OCL-V0-007: below the minimum sample the report abstains from a proposal.
func TestInsufficientSampleSuppressesProposal_OCL007(t *testing.T) {
	report := Build(scored(MinimumSample-1, 0.5, true), "clean")
	if !report.Insufficient || report.ProposalState != ProposalInsufficient {
		t.Fatalf("small sample carried a proposal: %+v", report)
	}
	if report.Payload()["proposal"] != nil || report.Payload()["insufficient_sample"] != true {
		t.Fatalf("payload: %v", report.Payload())
	}
	unscored := Build(observations(MinimumSample, Answered, true), "clean")
	if unscored.Insufficient || unscored.ProposalState != ProposalNoScoredRecords || unscored.Payload()["proposal"] != nil {
		t.Fatalf("proposal invented without scores: %+v", unscored)
	}
}
