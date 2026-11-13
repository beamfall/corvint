package jstestprovider

import (
	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// OutboundCallsAnnotation is the one per-attempt annotation a test or its fixture publishes to state,
// from the server log, which outbound calls the server made while answering the attempt's requests
// and whether the environment's mock layer answered each (LPCV-V0-123). Its description is the
// closed JSON OutboundCallRecord.
const OutboundCallsAnnotation = "corvint-outbound-calls"

// MaxOutboundResponses is jstestreceipt.MaxOutboundResponses, kept under its old name.
const MaxOutboundResponses = jstestreceipt.MaxOutboundResponses

// OutboundCallRecord is jstestreceipt.OutboundCallRecord, kept under its old name.
type OutboundCallRecord = jstestreceipt.OutboundCallRecord

// OutboundResponse is jstestreceipt.OutboundResponse, kept under its old name.
type OutboundResponse = jstestreceipt.OutboundResponse

// ValidOutboundCallRecord is jstestreceipt.ValidOutboundCallRecord, kept under its old name.
func ValidOutboundCallRecord(record OutboundCallRecord) bool {
	return jstestreceipt.ValidOutboundCallRecord(record)
}

// ScreenOutboundCallRecord is jstestreceipt.ScreenOutboundCallRecord, kept under its old name.
func ScreenOutboundCallRecord(record OutboundCallRecord) OutboundCallRecord {
	return jstestreceipt.ScreenOutboundCallRecord(record)
}

// outboundCallRecord reads the attempt's corvint-outbound-calls annotation. Only exactly one closed,
// in-bounds annotation is a record; none, a duplicate or a malformed one yields nil, so the consumer
// reports NOT_PRODUCED rather than classifying a failure from a status code alone.
func outboundCallRecord(annotations []playwrightAnnotation) *OutboundCallRecord {
	description, ok := singleAnnotation(annotations, OutboundCallsAnnotation, jstestreceipt.MaxOutboundCallDescription)
	if !ok {
		return nil
	}
	var record OutboundCallRecord
	if !decodeOneRecord(description, &record) || !ValidOutboundCallRecord(record) {
		return nil
	}
	screened := ScreenOutboundCallRecord(record)
	return &screened
}
