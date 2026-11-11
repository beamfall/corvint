package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// RequestOutcomesAnnotation is the one per-attempt annotation a test or its fixture publishes to
// state which requests its decisive assertions checked and how it classifies them (LPCV-V0-080).
// Its description is the closed JSON RequestOutcomeRecord.
const RequestOutcomesAnnotation = "corvint-request-outcomes"

// AssertionRejectionOnly is jstestreceipt.AssertionRejectionOnly, kept under its old name.
const AssertionRejectionOnly = jstestreceipt.AssertionRejectionOnly

// AssertionPositive is jstestreceipt.AssertionPositive, kept under its old name.
const AssertionPositive = jstestreceipt.AssertionPositive

// MaxRequestOutcomes is jstestreceipt.MaxRequestOutcomes, kept under its old name.
const MaxRequestOutcomes = jstestreceipt.MaxRequestOutcomes

// RequestOutcomeRecord is jstestreceipt.RequestOutcomeRecord, kept under its old name.
type RequestOutcomeRecord = jstestreceipt.RequestOutcomeRecord

// RequestOutcome is jstestreceipt.RequestOutcome, kept under its old name.
type RequestOutcome = jstestreceipt.RequestOutcome

// IsSuccessStatus is jstestreceipt.IsSuccessStatus, kept under its old name.
func IsSuccessStatus(status int) bool { return jstestreceipt.IsSuccessStatus(status) }

// ValidRequestOutcomeRecord is jstestreceipt.ValidRequestOutcomeRecord, kept under its old name.
func ValidRequestOutcomeRecord(record RequestOutcomeRecord) bool {
	return jstestreceipt.ValidRequestOutcomeRecord(record)
}

// ScreenedRequestOutcomeText is jstestreceipt.ScreenedRequestOutcomeText, kept under its old name.
func ScreenedRequestOutcomeText(value string) bool {
	return jstestreceipt.ScreenedRequestOutcomeText(value)
}

// ScreenRequestOutcomeRecord applies the product secret screen to the actor, sibling title and
// every route template.
func ScreenRequestOutcomeRecord(record RequestOutcomeRecord) RequestOutcomeRecord {
	screened := record
	screened.Actor, _ = secretscreen.Screen(record.Actor)
	screened.PositiveSibling, _ = secretscreen.Screen(record.PositiveSibling)
	screened.Requests = make([]RequestOutcome, len(record.Requests))
	for i, request := range record.Requests {
		request.Route, _ = secretscreen.Screen(request.Route)
		screened.Requests[i] = request
	}
	return screened
}

// requestOutcomeRecord reads the attempt's corvint-request-outcomes annotation. Only exactly one
// closed, in-bounds annotation is a record; none, a duplicate or a malformed one yields nil, so the
// consumer reports NOT_PRODUCED rather than guessing a classification.
func requestOutcomeRecord(annotations []playwrightAnnotation) *RequestOutcomeRecord {
	var found *playwrightAnnotation
	for i := range annotations {
		if annotations[i].Type != RequestOutcomesAnnotation {
			continue
		}
		if found != nil {
			return nil
		}
		found = &annotations[i]
	}
	if found == nil || found.malformed || len(found.Description) > jstestreceipt.MaxRequestOutcomeDescription {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(found.Description)))
	var record RequestOutcomeRecord
	if decoder.Decode(&record) != nil {
		return nil
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil
	}
	if !ValidRequestOutcomeRecord(record) {
		return nil
	}
	screened := ScreenRequestOutcomeRecord(record)
	return &screened
}
