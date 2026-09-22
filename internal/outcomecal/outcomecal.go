// Package outcomecal joins recorded local task outcomes with the packet stance
// (answered or abstained) that preceded them and reports calibration only. It
// never writes, and it never changes a threshold: a proposed threshold is
// reported with applied=false and must pass the learned-trace admission
// evaluation gate before any mechanism change (AGENTS.md invariant 5).
package outcomecal

import (
	"fmt"
	"math"

	"github.com/Beamfall/corvint/internal/trace"
)

// MinimumSample is the smallest joined sample that may carry a proposal. Below
// it the report abstains instead of inventing a calibrated threshold
// (AGENTS.md invariant 2).
const MinimumSample = 20

// Stance is the packet decision recorded for a task.
type Stance string

const (
	// Answered means the packet returned evidence for the task.
	Answered Stance = "answered"
	// Abstained means the packet declined to answer the task.
	Abstained Stance = "abstained"
	// Unknown means no readable field carries the packet decision.
	Unknown Stance = "unknown"
)

// Proposal states.
const (
	ProposalProposed          = "proposed"
	ProposalInsufficient      = "insufficient-sample"
	ProposalNoScoredRecords   = "no-scored-records"
	proposalGateSentence      = "This threshold is a proposal computed from local records only. It requires the learned-trace admission evaluation gate (LTA-V0-001..LTA-V0-003) and is never applied by `corvint calibrate`."
	unknownStanceReasonAbsent = "schema-version-1 trace records carry no packet stance field"
	scoreReasonAbsent         = "schema-version-1 trace records carry no relevance-floor score field"
)

// Observation is one joined row: the packet stance for a task, whether the task
// outcome succeeded, and the relevance-floor score when the record carries one.
type Observation struct {
	Stance   Stance
	Success  bool
	Score    float64
	HasScore bool
}

// Observe projects one recorded trace onto the calibration join.
//
// Stance and Score are absent from the schema-version-1 record: `corvint
// record` stores the task outcome, not the packet decision that preceded it or
// the score that decision used. Every record therefore classifies as Unknown
// with no score until the producer records those fields; the report counts
// those rows explicitly rather than guessing a stance.
func Observe(record trace.Record) Observation {
	return Observation{Stance: Unknown, Success: record.Outcome == "passed"}
}

// Matrix is the answered/abstained by success/failure contingency table.
// Failure covers every non-passed recorded outcome (failed and blocked today).
type Matrix struct {
	AnsweredSuccess  int
	AnsweredFailure  int
	AbstainedSuccess int
	AbstainedFailure int
	UnknownSuccess   int
	UnknownFailure   int
}

// Classified is the number of rows whose stance is known.
func (matrix Matrix) Classified() int {
	return matrix.AnsweredSuccess + matrix.AnsweredFailure + matrix.AbstainedSuccess + matrix.AbstainedFailure
}

// Bucket is one fixed decile of the relevance-floor score.
type Bucket struct {
	Lower   float64
	Upper   float64
	Records int
	Success int
}

// Proposal is the threshold that would have maximised abstention accuracy on
// this sample. It is never applied. The threshold is reported in tenths because
// the candidate grid is the eleven decile edges of [0, 1] and the canonical wire
// encoding carries integers only.
type Proposal struct {
	ThresholdTenths int
	Calibrated      int
	SampleSize      int
}

// Accuracy is the proposal's abstention accuracy on the scored sample.
func (proposal Proposal) Accuracy() float64 {
	if proposal.SampleSize == 0 {
		return 0
	}
	return float64(proposal.Calibrated) / float64(proposal.SampleSize)
}

// Report is the whole read-only calibration result.
type Report struct {
	TraceState    string
	Since         string
	Window        int
	SampleSize    int
	Matrix        Matrix
	UnknownReason string
	Accuracy      float64
	HasAccuracy   bool
	ScoreCoverage int
	ScoreReason   string
	Buckets       []Bucket
	Insufficient  bool
	ProposalState string
	Proposal      Proposal
}

// Build joins the observations into the calibration report. It reads only the
// supplied rows and writes nothing.
func Build(observations []Observation, traceState string) Report {
	report := Report{
		TraceState: traceState, SampleSize: len(observations),
		Insufficient: len(observations) < MinimumSample,
		Buckets:      emptyBuckets(),
	}
	report.Matrix = buildMatrix(observations)
	report.Accuracy, report.HasAccuracy = abstentionAccuracy(report.Matrix)
	report.ScoreCoverage = fillBuckets(report.Buckets, observations)
	if report.Matrix.UnknownSuccess+report.Matrix.UnknownFailure != 0 {
		report.UnknownReason = unknownStanceReasonAbsent
	}
	if report.ScoreCoverage == 0 {
		report.ScoreReason = scoreReasonAbsent
	}
	report.ProposalState, report.Proposal = buildProposal(observations, report.Insufficient, report.ScoreCoverage)
	return report
}

func buildMatrix(observations []Observation) Matrix {
	matrix := Matrix{}
	for _, observation := range observations {
		switch {
		case observation.Stance == Answered && observation.Success:
			matrix.AnsweredSuccess++
		case observation.Stance == Answered:
			matrix.AnsweredFailure++
		case observation.Stance == Abstained && observation.Success:
			matrix.AbstainedSuccess++
		case observation.Stance == Abstained:
			matrix.AbstainedFailure++
		case observation.Success:
			matrix.UnknownSuccess++
		default:
			matrix.UnknownFailure++
		}
	}
	return matrix
}

// abstentionAccuracy scores the packet decision against the recorded outcome:
// answering a task that succeeded and abstaining on a task that did not are the
// calibrated cells.
func abstentionAccuracy(matrix Matrix) (float64, bool) {
	classified := matrix.Classified()
	if classified == 0 {
		return 0, false
	}
	calibrated := matrix.AnsweredSuccess + matrix.AbstainedFailure
	return float64(calibrated) / float64(classified), true
}

func emptyBuckets() []Bucket {
	buckets := make([]Bucket, 10)
	for index := range buckets {
		buckets[index] = Bucket{Lower: float64(index) / 10, Upper: float64(index+1) / 10}
	}
	return buckets
}

func fillBuckets(buckets []Bucket, observations []Observation) int {
	scored := 0
	for _, observation := range observations {
		if !isScored(observation) {
			continue
		}
		scored++
		index := bucketIndex(observation.Score)
		buckets[index].Records++
		if observation.Success {
			buckets[index].Success++
		}
	}
	return scored
}

// isScored reports whether a row carries a usable score. NaN is not a score:
// it orders against no threshold and has no bucket.
func isScored(observation Observation) bool {
	return observation.HasScore && !math.IsNaN(observation.Score)
}

// bucketIndex clamps a score onto the ten fixed deciles of [0, 1]. The clamp
// happens before the int conversion, which is platform-defined out of range.
func bucketIndex(score float64) int {
	clamped := math.Min(math.Max(score*10, 0), 9)
	return int(clamped)
}

// buildProposal reports the decile edge that would have maximised abstention
// accuracy under the rule "abstain when score < threshold". Ties keep the
// lowest threshold so the output is deterministic.
func buildProposal(observations []Observation, insufficient bool, scored int) (string, Proposal) {
	if insufficient {
		return ProposalInsufficient, Proposal{}
	}
	if scored < MinimumSample {
		return ProposalNoScoredRecords, Proposal{}
	}
	best := Proposal{SampleSize: scored}
	for step := 0; step <= 10; step++ {
		calibrated := thresholdCalibrated(observations, float64(step)/10)
		if calibrated > best.Calibrated {
			best.ThresholdTenths, best.Calibrated = step, calibrated
		}
	}
	return ProposalProposed, best
}

// thresholdCalibrated counts the scored rows the rule "abstain when score <
// threshold" would have decided the way the recorded outcome justifies.
func thresholdCalibrated(observations []Observation, threshold float64) int {
	calibrated := 0
	for _, observation := range observations {
		if !isScored(observation) {
			continue
		}
		if (observation.Score < threshold) != observation.Success {
			calibrated++
		}
	}
	return calibrated
}

// Payload renders the report as the canonical wire object. It is pure data: no
// member of it is ever written back to the repository.
func (report Report) Payload() map[string]any {
	payload := map[string]any{
		"ok": true, "mutates": false, "tool": "calibrate",
		"trace_state": report.TraceState, "sample_size": report.SampleSize,
		"minimum_sample": MinimumSample, "insufficient_sample": report.Insufficient,
		"matrix": map[string]any{
			"answered_success":  report.Matrix.AnsweredSuccess,
			"answered_failure":  report.Matrix.AnsweredFailure,
			"abstained_success": report.Matrix.AbstainedSuccess,
			"abstained_failure": report.Matrix.AbstainedFailure,
		},
		"unknown": map[string]any{
			"total":   report.Matrix.UnknownSuccess + report.Matrix.UnknownFailure,
			"success": report.Matrix.UnknownSuccess,
			"failure": report.Matrix.UnknownFailure,
		},
		"abstention_accuracy": nil,
		"score_coverage":      report.ScoreCoverage,
		"buckets":             bucketPayload(report.Buckets),
		"proposal_state":      report.ProposalState,
		"proposal":            nil,
	}
	if report.HasAccuracy {
		payload["abstention_accuracy"] = map[string]any{
			"calibrated": report.Matrix.AnsweredSuccess + report.Matrix.AbstainedFailure,
			"classified": report.Matrix.Classified(),
		}
	}
	if report.UnknownReason != "" {
		payload["unknown_reason"] = report.UnknownReason
	}
	if report.ScoreReason != "" {
		payload["score_reason"] = report.ScoreReason
	}
	if report.Since != "" {
		payload["since"] = report.Since
	}
	if report.Window != 0 {
		payload["window"] = report.Window
	}
	if report.ProposalState == ProposalProposed {
		payload["proposal"] = map[string]any{
			"threshold_tenths": report.Proposal.ThresholdTenths,
			"abstention_accuracy": map[string]any{
				"calibrated": report.Proposal.Calibrated, "scored": report.Proposal.SampleSize,
			},
			"sample_size": report.Proposal.SampleSize, "applied": false, "gate": proposalGateSentence,
		}
	}
	return payload
}

func bucketPayload(buckets []Bucket) []any {
	rows := make([]any, 0, len(buckets))
	for _, bucket := range buckets {
		rows = append(rows, map[string]any{
			"lower_tenths": int(bucket.Lower*10 + 0.5), "upper_tenths": int(bucket.Upper*10 + 0.5),
			"records": bucket.Records, "success": bucket.Success,
		})
	}
	return rows
}

// Table renders the same report as a deterministic fixed-order text table.
func (report Report) Table() string {
	text := fmt.Sprintf("trace_state %s\nsample_size %d (minimum %d)\n", report.TraceState, report.SampleSize, MinimumSample)
	text += fmt.Sprintf("%-12s %10s %10s\n", "stance", "success", "failure")
	text += fmt.Sprintf("%-12s %10d %10d\n", "answered", report.Matrix.AnsweredSuccess, report.Matrix.AnsweredFailure)
	text += fmt.Sprintf("%-12s %10d %10d\n", "abstained", report.Matrix.AbstainedSuccess, report.Matrix.AbstainedFailure)
	text += fmt.Sprintf("%-12s %10d %10d\n", "unknown", report.Matrix.UnknownSuccess, report.Matrix.UnknownFailure)
	text += "abstention_accuracy " + ratioText(report.Accuracy, report.HasAccuracy) + "\n"
	text += fmt.Sprintf("score_coverage %d\n", report.ScoreCoverage)
	for _, bucket := range report.Buckets {
		text += fmt.Sprintf("bucket [%.1f,%.1f) records %d success_rate %s\n",
			bucket.Lower, bucket.Upper, bucket.Records, ratioText(
				bucketRate(bucket), bucket.Records != 0))
	}
	text += "proposal_state " + report.ProposalState + "\n"
	if report.ProposalState == ProposalProposed {
		text += fmt.Sprintf("proposal threshold %.1f abstention_accuracy %.4f sample_size %d applied false\n",
			float64(report.Proposal.ThresholdTenths)/10, report.Proposal.Accuracy(), report.Proposal.SampleSize)
		text += proposalGateSentence + "\n"
	}
	if report.UnknownReason != "" {
		text += "unknown_reason " + report.UnknownReason + "\n"
	}
	if report.ScoreReason != "" {
		text += "score_reason " + report.ScoreReason + "\n"
	}
	return text
}

func bucketRate(bucket Bucket) float64 {
	if bucket.Records == 0 {
		return 0
	}
	return float64(bucket.Success) / float64(bucket.Records)
}

func ratioText(value float64, present bool) string {
	if !present {
		return "null"
	}
	return fmt.Sprintf("%.4f", value)
}
