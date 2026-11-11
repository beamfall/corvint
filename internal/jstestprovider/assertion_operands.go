package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// AssertionOperandsAnnotation is the one per-attempt annotation a test or its fixture publishes to
// state the operand sizes of its count and equality assertions (LPCV-V0-120). Its description is
// the closed JSON AssertionOperandRecord.
const AssertionOperandsAnnotation = "corvint-assertion-operands"

// MaxAssertionOperands is jstestreceipt.MaxAssertionOperands, kept under its old name.
const MaxAssertionOperands = jstestreceipt.MaxAssertionOperands

// AssertionOperandRecord is jstestreceipt.AssertionOperandRecord, kept under its old name.
type AssertionOperandRecord = jstestreceipt.AssertionOperandRecord

// AssertionOperands is jstestreceipt.AssertionOperands, kept under its old name.
type AssertionOperands = jstestreceipt.AssertionOperands

// ValidAssertionOperandRecord is jstestreceipt.ValidAssertionOperandRecord, kept under its old name.
func ValidAssertionOperandRecord(record AssertionOperandRecord) bool {
	return jstestreceipt.ValidAssertionOperandRecord(record)
}

// ScreenAssertionOperandRecord is jstestreceipt.ScreenAssertionOperandRecord, kept under its old name.
func ScreenAssertionOperandRecord(record AssertionOperandRecord) AssertionOperandRecord {
	return jstestreceipt.ScreenAssertionOperandRecord(record)
}

// assertionOperandRecord reads the attempt's corvint-assertion-operands annotation. Only exactly
// one closed, in-bounds annotation is a record; none, a duplicate or a malformed one yields nil, so
// the consumer reports NOT_PRODUCED rather than guessing that an operand was empty.
func assertionOperandRecord(annotations []playwrightAnnotation) *AssertionOperandRecord {
	description, ok := singleAnnotation(annotations, AssertionOperandsAnnotation, jstestreceipt.MaxAssertionOperandDescription)
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
