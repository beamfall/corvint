// SPDX-License-Identifier: Apache-2.0

// Package testduration holds the per-test duration rules every runner adapter and consumer shares
// (docs/specs/per-test-durations-v1.md, PTD-V1-001 to PTD-V1-009): the named source of each runner,
// NOT_MEASURED for a time the report does not give, the bound that makes a reported value invalid,
// and the cost figure that uses measured times only. It has no Corvint imports.
package testduration

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

// NotMeasured is the durationSource of an outcome whose report gave no valid time (PTD-V1-003,
// PTD-V1-007).
const NotMeasured = "NOT_MEASURED"

// GranularityFile marks a duration recorded for a file or package rather than one test
// (PTD-V1-004).
const GranularityFile = "file"

// ReasonInvalid names the list of tests whose reported time was negative, non-numeric or above
// MaxMilliseconds (PTD-V1-007).
const ReasonInvalid = "duration-invalid"

// MaxMilliseconds is the largest valid duration: 24 hours (the change's Open item 1, as proposed).
const MaxMilliseconds = 24 * 60 * 60 * 1000

// The durationSource of each runner: the report field its time is read from (PTD-V1-002).
const (
	SourceGo         = "go-test2json"
	SourceVitest     = "vitest-json"
	SourceJest       = "jest-json"
	SourcePlaywright = "playwright-json"
	SourcePytest     = "pytest-junit"
	SourceRSpec      = "rspec-json"
	SourceMinitest   = "minitest-reporter"
)

// Measure is one test's duration as an outcome records it: Milliseconds is nil and Source is
// NotMeasured unless a valid time was reported; Invalid is set when a time was reported but is not
// a valid duration, so the caller lists the test under duration-invalid.
type Measure struct {
	Milliseconds *int64
	Source       string
	Invalid      bool
}

// FromMilliseconds measures a reported time in milliseconds. A negative, NaN, infinite or
// above-bound value is invalid and NOT_MEASURED; the value is rounded to the nearest millisecond and
// never estimated (PTD-V1-003, PTD-V1-007).
func FromMilliseconds(value float64, source string) Measure {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > MaxMilliseconds {
		return Measure{Source: NotMeasured, Invalid: true}
	}
	ms := int64(math.Round(value))
	return Measure{Milliseconds: &ms, Source: source}
}

// FromSeconds measures a reported time in seconds, as go test -json Elapsed and the JUnit time
// attribute give it.
func FromSeconds(value float64, source string) Measure {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Measure{Source: NotMeasured, Invalid: true}
	}
	return FromMilliseconds(value*1000, source)
}

// FromSecondsText measures a reported time written as text in seconds. Empty text is a report
// without the field, NOT_MEASURED and not invalid; text that is not a number is invalid.
func FromSecondsText(text, source string) Measure {
	text = strings.TrimSpace(text)
	if text == "" {
		return Missing()
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return Measure{Source: NotMeasured, Invalid: true}
	}
	return FromSeconds(value, source)
}

// Missing is the measure of a test whose report has no time for it (PTD-V1-003).
func Missing() Measure {
	return Measure{Source: NotMeasured}
}

// Attempts measures a test run more than once in one invocation: each attempt's time and their sum
// (PTD-V1-006). One invalid attempt makes the whole test invalid and NOT_MEASURED; one missing
// attempt makes it NOT_MEASURED, since a sum without it would be an estimate.
func Attempts(attempts []Measure, source string) (Measure, []int64) {
	if len(attempts) == 0 {
		return Missing(), nil
	}
	values := make([]int64, 0, len(attempts))
	var sum int64
	for _, attempt := range attempts {
		if attempt.Invalid {
			return Measure{Source: NotMeasured, Invalid: true}, nil
		}
		if attempt.Milliseconds == nil {
			return Missing(), nil
		}
		values = append(values, *attempt.Milliseconds)
		sum += *attempt.Milliseconds
	}
	if sum > MaxMilliseconds {
		return Measure{Source: NotMeasured, Invalid: true}, nil
	}
	return Measure{Milliseconds: &sum, Source: source}, values
}

// Figure is the G4 cost of one selection (PTD-V1-009): the summed measured time of the selected
// tests against the full suite's, and how many of each were NOT_MEASURED. A figure with any
// unmeasured test is Incomplete, and its Ratio is absent rather than filled in.
type Figure struct {
	SelectedMs          int64    `json:"selectedMs"`
	FullMs              int64    `json:"fullMs"`
	Ratio               *float64 `json:"ratio,omitempty"`
	SelectedNotMeasured int      `json:"selectedNotMeasured"`
	FullNotMeasured     int      `json:"fullNotMeasured"`
	Incomplete          bool     `json:"incomplete"`
	Label               string   `json:"label"`
}

// Cost computes one selection's figure from the durations of the selected tests and of the full
// suite; a nil entry is a NOT_MEASURED test. Build and setup time never enter it: only per-test
// durations do (G4, PTD-V1-005).
func Cost(selected, full []*int64) Figure {
	figure := Figure{}
	figure.SelectedMs, figure.SelectedNotMeasured = sum(selected)
	figure.FullMs, figure.FullNotMeasured = sum(full)
	figure.Incomplete = figure.SelectedNotMeasured+figure.FullNotMeasured != 0
	switch {
	case figure.Incomplete:
		figure.Label = "incomplete: " + strconv.Itoa(figure.SelectedNotMeasured) + " selected and " + strconv.Itoa(figure.FullNotMeasured) + " full-suite tests NOT_MEASURED"
	case figure.FullMs == 0:
		figure.Label = "incomplete: the full suite has no measured time"
		figure.Incomplete = true
	default:
		ratio := float64(figure.SelectedMs) / float64(figure.FullMs)
		figure.Ratio = &ratio
		figure.Label = "complete"
	}
	return figure
}

// Median is the median ratio of complete figures. It reports incomplete, with no value, when any
// figure is incomplete, so a published median never rests on an unmeasured test (PTD-V1-009).
func Median(figures []Figure) (*float64, bool) {
	ratios := make([]float64, 0, len(figures))
	for _, figure := range figures {
		if figure.Incomplete || figure.Ratio == nil {
			return nil, false
		}
		ratios = append(ratios, *figure.Ratio)
	}
	if len(ratios) == 0 {
		return nil, false
	}
	slices.Sort(ratios)
	middle := len(ratios) / 2
	median := ratios[middle]
	if len(ratios)%2 == 0 {
		median = (ratios[middle-1] + ratios[middle]) / 2
	}
	return &median, true
}

func sum(durations []*int64) (int64, int) {
	var total int64
	missing := 0
	for _, duration := range durations {
		if duration == nil {
			missing++
			continue
		}
		total += *duration
	}
	return total, missing
}
