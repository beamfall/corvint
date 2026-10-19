package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// AssertionOperandsAnnotation is the one per-attempt annotation a test or its fixture publishes to
// state the operand sizes of its count and equality assertions (LPCV-V0-120). Its description is
// the closed JSON AssertionOperandRecord.
const AssertionOperandsAnnotation = "corvint-assertion-operands"

// Assertion-operand record bounds. A record outside them is not a record.
const (
	MaxAssertionOperands           = 64
	maxAssertionOperandSize        = 1<<31 - 1
	maxAssertionOperandDescription = 128 << 10
)

// assertionOperandMatchers are the count and equality matchers a record may name (LPCV-V0-120),
// the same set the static VACUOUS-EMPTY-EQUALITY rule reads (TSA-V0-172).
var assertionOperandMatchers = map[string]bool{"toHaveCount": true, "toHaveLength": true, "toBe": true, "toEqual": true, "toStrictEqual": true}

// AssertionOperandRecord is one attempt's declared assertion-operand evidence (LPCV-V0-120). It
// carries operand sizes only, never operand values. The record is provider authority: Corvint does
// not observe the assertions it states.
type AssertionOperandRecord struct {
	Assertions []AssertionOperands `json:"assertions"`
}

// AssertionOperands is one evaluated count or equality assertion: its matcher, where it is, and the
// size (count, length or size) of its actual and expected operands. File and Line are both omitted
// when the producer could not resolve the call.
type AssertionOperands struct {
	Matcher  string `json:"matcher"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Actual   int    `json:"actual"`
	Expected int    `json:"expected"`
}

// ValidAssertionOperandRecord reports whether a record is inside the closed vocabulary and bounds.
func ValidAssertionOperandRecord(record AssertionOperandRecord) bool {
	if record.Assertions == nil || len(record.Assertions) > MaxAssertionOperands {
		return false
	}
	for _, assertion := range record.Assertions {
		if !assertionOperandMatchers[assertion.Matcher] || !operandSize(assertion.Actual) || !operandSize(assertion.Expected) {
			return false
		}
		if assertion.File == "" && assertion.Line == 0 {
			continue
		}
		if !requestOutcomeText(assertion.File) || assertion.Line < 1 || assertion.Line > maxRouteStubLine {
			return false
		}
	}
	return true
}

func operandSize(size int) bool {
	return size >= 0 && size <= maxAssertionOperandSize
}

// ScreenAssertionOperandRecord applies the product secret screen to every assertion path.
func ScreenAssertionOperandRecord(record AssertionOperandRecord) AssertionOperandRecord {
	assertions := make([]AssertionOperands, len(record.Assertions))
	for i, assertion := range record.Assertions {
		assertion.File, _ = secretscreen.Screen(assertion.File)
		assertions[i] = assertion
	}
	return AssertionOperandRecord{Assertions: assertions}
}

// assertionOperandMembers are the exact member names of a record.
var assertionOperandMembers = map[string]bool{"assertions": true, "matcher": true, "file": true, "line": true, "actual": true, "expected": true}

// UnmarshalJSON decodes a record closed: unknown, case-variant and duplicate members are refused at
// every level, and every assertion must state both operand sizes, so a missing or null size is
// never read as empty. It serves both the annotation and a retained receipt's member.
func (r *AssertionOperandRecord) UnmarshalJSON(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("assertion-operand record is not valid UTF-8")
	}
	if irregularJSONMemberOf(data, assertionOperandMembers) {
		return errors.New("assertion-operand record has a duplicate or inexact member")
	}
	var wire struct {
		Assertions []struct {
			Matcher  string `json:"matcher"`
			File     string `json:"file"`
			Line     int    `json:"line"`
			Actual   *int   `json:"actual"`
			Expected *int   `json:"expected"`
		} `json:"assertions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	record := AssertionOperandRecord{}
	if wire.Assertions != nil {
		record.Assertions = make([]AssertionOperands, len(wire.Assertions))
	}
	for i, assertion := range wire.Assertions {
		if assertion.Actual == nil || assertion.Expected == nil {
			return errors.New("assertion operands have no size")
		}
		record.Assertions[i] = AssertionOperands{Matcher: assertion.Matcher, File: assertion.File, Line: assertion.Line, Actual: *assertion.Actual, Expected: *assertion.Expected}
	}
	*r = record
	return nil
}

// assertionOperandRecord reads the attempt's corvint-assertion-operands annotation. Only exactly
// one closed, in-bounds annotation is a record; none, a duplicate or a malformed one yields nil, so
// the consumer reports NOT_PRODUCED rather than guessing that an operand was empty.
func assertionOperandRecord(annotations []playwrightAnnotation) *AssertionOperandRecord {
	description, ok := singleAnnotation(annotations, AssertionOperandsAnnotation, maxAssertionOperandDescription)
	if !ok {
		return nil
	}
	var record AssertionOperandRecord
	if !decodeOneRecord(description, &record) || !ValidAssertionOperandRecord(record) {
		return nil
	}
	screened := ScreenAssertionOperandRecord(record)
	return &screened
}

// utf8CheckedAnnotations are the declared-record annotations whose raw description must be valid
// UTF-8 (LPCV-V0-080, LPCV-V0-120, LPCV-V0-123).
var utf8CheckedAnnotations = map[string]bool{RequestOutcomesAnnotation: true, AssertionOperandsAnnotation: true, OutboundCallsAnnotation: true}

// setDeclaredRecords sets the attempt's declared request-outcome (LPCV-V0-080), assertion-operand
// (LPCV-V0-120) and outbound-call (LPCV-V0-123) records from its annotations.
func setDeclaredRecords(detail *AttemptDetail, annotations []playwrightAnnotation) {
	detail.RequestOutcomes = requestOutcomeRecord(annotations)
	detail.AssertionOperands = assertionOperandRecord(annotations)
	detail.OutboundCalls = outboundCallRecord(annotations)
}

// singleAnnotation returns the description of the attempt's one annotation of the given type. None,
// a duplicate, a malformed or an oversized one is not a record.
func singleAnnotation(annotations []playwrightAnnotation, annotationType string, maxDescription int) (string, bool) {
	var found *playwrightAnnotation
	for i := range annotations {
		if annotations[i].Type != annotationType {
			continue
		}
		if found != nil {
			return "", false
		}
		found = &annotations[i]
	}
	if found == nil || found.malformed || len(found.Description) > maxDescription {
		return "", false
	}
	return found.Description, true
}

// decodeOneRecord decodes exactly one JSON value into record; trailing data is refused.
func decodeOneRecord(description string, record any) bool {
	decoder := json.NewDecoder(bytes.NewReader([]byte(description)))
	if decoder.Decode(record) != nil {
		return false
	}
	_, err := decoder.Token()
	return errors.Is(err, io.EOF)
}

// hasAssertionOperands reports the unprofiled-only assertionOperands member.
func hasAssertionOperands(r Receipt) bool {
	for _, test := range r.Tests {
		for _, detail := range test.AttemptDetails {
			if detail.AssertionOperands != nil {
				return true
			}
		}
	}
	return false
}
