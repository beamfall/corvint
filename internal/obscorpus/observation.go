// Package obscorpus is the experimental OCA-V0 contract
// (docs/specs/observation-corpus-authority-v0.md): bounded harness observations
// and a sealed owner-labelled corpus replay. It is pure: it opens no file,
// network connection, or process, and no ranking, learning, evidence, or
// authority path imports it.
package obscorpus

import (
	"regexp"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Field and verdict states.
const (
	StatusObserved    = "OBSERVED"
	StatusNotObserved = "NOT_OBSERVED"
)

// Closed NOT_OBSERVED reasons for captured fields (OCA-V0-001..002).
const (
	ReasonNotReported      = "not-reported"
	ReasonCaptureFailed    = "capture-failed"
	ReasonOutOfBounds      = "out-of-bounds"
	ReasonNotImmutable     = "revision-not-immutable"
	ReasonUnboundedText    = "unbounded-text"
	ReasonSecretScreened   = "secret-screened"
	ReasonIdentityNotBound = "identity-not-observed"
	ReasonCountBound       = "count-bound"
)

// Capture bounds.
const (
	MaxIdentityBytes = 128
	MaxTokens        = 10_000_000
	MaxLatencyMillis = 3_600_000
	MaxObservations  = 256
)

var (
	identityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
	revisionPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// RawTelemetry is what a harness reports. It has no field for prompts, source
// bodies, commands, or command output, so none can be captured (OCA-V0-006).
type RawTelemetry struct {
	Identity      string `json:"identity"`
	Revision      string `json:"revision"`
	InputTokens   *int64 `json:"inputTokens"`
	OutputTokens  *int64 `json:"outputTokens"`
	LatencyMillis *int64 `json:"latencyMillis"`
}

// Text is a bounded string field; Value is set only when Status is OBSERVED.
type Text struct {
	Status string `json:"status"`
	Value  string `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Count is a bounded integer field. Value is nil unless directly observed, so a
// missing count can never read as zero.
type Count struct {
	Status string `json:"status"`
	Value  *int64 `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Observation binds one event/turn identity to its revision, tokens, and latency.
type Observation struct {
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
	Identity      Text   `json:"identity"`
	Revision      Text   `json:"revision"`
	InputTokens   Count  `json:"inputTokens"`
	OutputTokens  Count  `json:"outputTokens"`
	LatencyMillis Count  `json:"latencyMillis"`
}

// Batch is a count-bounded capture; observations past MaxObservations are dropped and counted.
type Batch struct {
	Observations  []Observation `json:"observations"`
	Dropped       int           `json:"dropped"`
	DroppedReason string        `json:"droppedReason,omitempty"`
}

// Capture bounds one raw report. It never fails: an unbound identity makes the
// whole observation NOT_OBSERVED so no measure is recorded unbound.
func Capture(raw RawTelemetry) Observation {
	identity := boundText(raw.Identity, identityPattern, ReasonUnboundedText)
	if identity.Status != StatusObserved {
		return unbound(ReasonIdentityNotBound)
	}
	return Observation{
		Status:        StatusObserved,
		Identity:      identity,
		Revision:      boundText(raw.Revision, revisionPattern, ReasonNotImmutable),
		InputTokens:   boundCount(raw.InputTokens, MaxTokens),
		OutputTokens:  boundCount(raw.OutputTokens, MaxTokens),
		LatencyMillis: boundCount(raw.LatencyMillis, MaxLatencyMillis),
	}
}

// CaptureFrom is fail-open: a telemetry source that errors or panics yields a
// NOT_OBSERVED capture-failed observation and never blocks the harness event.
func CaptureFrom(source func() (RawTelemetry, error)) (observation Observation) {
	defer func() {
		if recover() != nil {
			observation = unbound(ReasonCaptureFailed)
		}
	}()
	raw, err := source()
	if err != nil {
		return unbound(ReasonCaptureFailed)
	}
	return Capture(raw)
}

// CaptureAll captures at most MaxObservations reports in input order.
func CaptureAll(raws []RawTelemetry) Batch {
	kept := raws[:min(len(raws), MaxObservations)]
	batch := Batch{Observations: make([]Observation, 0, len(kept)), Dropped: len(raws) - len(kept)}
	for _, raw := range kept {
		batch.Observations = append(batch.Observations, Capture(raw))
	}
	if batch.Dropped > 0 {
		batch.DroppedReason = ReasonCountBound
	}
	return batch
}

// Complete reports whether every field was directly observed.
func (o Observation) Complete() bool {
	statuses := []string{o.Status, o.Identity.Status, o.Revision.Status, o.InputTokens.Status, o.OutputTokens.Status, o.LatencyMillis.Status}
	for _, status := range statuses {
		if status != StatusObserved {
			return false
		}
	}
	return true
}

func unbound(reason string) Observation {
	text := Text{Status: StatusNotObserved, Reason: reason}
	count := Count{Status: StatusNotObserved, Reason: reason}
	return Observation{Status: StatusNotObserved, Reason: reason, Identity: text, Revision: text, InputTokens: count, OutputTokens: count, LatencyMillis: count}
}

func boundText(value string, pattern *regexp.Regexp, invalid string) Text {
	if value == "" {
		return Text{Status: StatusNotObserved, Reason: ReasonNotReported}
	}
	if len(value) > MaxIdentityBytes || !pattern.MatchString(value) {
		return Text{Status: StatusNotObserved, Reason: invalid}
	}
	if secretscreen.MatchString(value) {
		return Text{Status: StatusNotObserved, Reason: ReasonSecretScreened}
	}
	return Text{Status: StatusObserved, Value: value}
}

func boundCount(value *int64, limit int64) Count {
	if value == nil {
		return Count{Status: StatusNotObserved, Reason: ReasonNotReported}
	}
	if *value < 0 || *value > limit {
		return Count{Status: StatusNotObserved, Reason: ReasonOutOfBounds}
	}
	observed := *value
	return Count{Status: StatusObserved, Value: &observed}
}
