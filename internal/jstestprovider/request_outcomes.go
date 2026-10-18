package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// RequestOutcomesAnnotation is the one per-attempt annotation a test or its fixture publishes to
// state which requests its decisive assertions checked and how it classifies them (LPCV-V0-080).
// Its description is the closed JSON RequestOutcomeRecord.
const RequestOutcomesAnnotation = "corvint-request-outcomes"

// Request-outcome assertion classes (LPCV-V0-080).
const (
	// AssertionRejectionOnly is a test whose every decisive assertion checks a rejection outcome.
	AssertionRejectionOnly = "rejection-only"
	// AssertionPositive is a test whose decisive assertions expect the requests to succeed.
	AssertionPositive = "positive"
	// AssertionMixed is any other test.
	AssertionMixed = "mixed"
)

// Request-outcome record bounds. A record outside them is not a record.
const (
	MaxRequestOutcomes           = 64
	maxRequestOutcomeTextBytes   = 1024
	maxRequestOutcomeMethodBytes = 16
	maxRequestOutcomeDescription = 128 << 10
	minRequestOutcomeStatus      = 100
	maxRequestOutcomeStatus      = 599
	minRejectionStatus           = 400
	maxRejectionStatus           = 499
	minSuccessStatus             = 200
	maxSuccessStatus             = 299
)

// RequestOutcomeRecord is one attempt's declared request evidence (LPCV-V0-080). Assertion
// classifies the attempt's decisive assertions; Actor names the actor or fixture the requests ran
// as; PositiveSibling optionally names, by full title, the test that shows the same requests can
// succeed. Requests are the requests the decisive assertions checked. The record is provider
// authority: Corvint does not observe the assertions or responses it states.
type RequestOutcomeRecord struct {
	Assertion       string           `json:"assertion"`
	Actor           string           `json:"actor,omitempty"`
	PositiveSibling string           `json:"positiveSibling,omitempty"`
	Requests        []RequestOutcome `json:"requests"`
}

// RequestOutcome is one checked request: its method, route template and response status.
type RequestOutcome struct {
	Method string `json:"method"`
	Route  string `json:"route"`
	Status int    `json:"status"`
}

var requestOutcomeAssertions = map[string]bool{AssertionRejectionOnly: true, AssertionPositive: true, AssertionMixed: true}

// IsRejectionStatus reports a 4xx status.
func IsRejectionStatus(status int) bool {
	return status >= minRejectionStatus && status <= maxRejectionStatus
}

// IsSuccessStatus reports a 2xx status.
func IsSuccessStatus(status int) bool {
	return status >= minSuccessStatus && status <= maxSuccessStatus
}

// ValidRequestOutcomeRecord reports whether a record is inside the closed vocabulary and bounds.
// A rejection-only record whose request did not end in a 4xx status contradicts itself and is not
// a record.
func ValidRequestOutcomeRecord(record RequestOutcomeRecord) bool {
	if !requestOutcomeAssertions[record.Assertion] || record.Requests == nil || len(record.Requests) > MaxRequestOutcomes {
		return false
	}
	if record.Actor != "" && !requestOutcomeText(record.Actor) {
		return false
	}
	if record.PositiveSibling != "" && !requestOutcomeText(record.PositiveSibling) {
		return false
	}
	for _, request := range record.Requests {
		if !requestOutcomeMethod(request.Method) || !requestOutcomeText(request.Route) || request.Status < minRequestOutcomeStatus || request.Status > maxRequestOutcomeStatus {
			return false
		}
		if record.Assertion == AssertionRejectionOnly && !IsRejectionStatus(request.Status) {
			return false
		}
	}
	return true
}

// requestOutcomeText bounds one text member. U+FFFD is refused as well as invalid UTF-8, because
// JSON decoding replaces invalid bytes and lone surrogate escapes with it (LPCV-V0-080).
func requestOutcomeText(value string) bool {
	return value != "" && len(value) <= maxRequestOutcomeTextBytes && utf8.ValidString(value) &&
		!strings.ContainsRune(value, 0) && !strings.ContainsRune(value, utf8.RuneError)
}

// ScreenedRequestOutcomeText reports an actor or route that the secret screen changed, or would
// change: distinct secrets share one placeholder, so such text never takes part in an equality
// join (LPCV-V0-082).
func ScreenedRequestOutcomeText(value string) bool {
	return strings.Contains(value, secretscreen.Placeholder) || secretscreen.MatchString(value)
}

func requestOutcomeMethod(value string) bool {
	if value == "" || len(value) > maxRequestOutcomeMethodBytes {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
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

// requestOutcomeMembers are the exact member names of a record.
var requestOutcomeMembers = map[string]bool{"assertion": true, "actor": true, "positiveSibling": true, "requests": true, "method": true, "route": true, "status": true}

// UnmarshalJSON decodes a record closed: unknown, case-variant and duplicate members are refused at
// every level, and every request must state its status, so a missing or null status is never read
// as a success or a rejection. It serves both the annotation and a retained receipt's member.
func (r *RequestOutcomeRecord) UnmarshalJSON(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("request-outcome record is not valid UTF-8")
	}
	if irregularJSONMemberOf(data, requestOutcomeMembers) {
		return errors.New("request-outcome record has a duplicate or inexact member")
	}
	var wire struct {
		Assertion       string `json:"assertion"`
		Actor           string `json:"actor"`
		PositiveSibling string `json:"positiveSibling"`
		Requests        []struct {
			Method string `json:"method"`
			Route  string `json:"route"`
			Status *int   `json:"status"`
		} `json:"requests"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	record := RequestOutcomeRecord{Assertion: wire.Assertion, Actor: wire.Actor, PositiveSibling: wire.PositiveSibling}
	if wire.Requests != nil {
		record.Requests = make([]RequestOutcome, len(wire.Requests))
	}
	for i, request := range wire.Requests {
		if request.Status == nil {
			return errors.New("request outcome has no status")
		}
		record.Requests[i] = RequestOutcome{Method: request.Method, Route: request.Route, Status: *request.Status}
	}
	*r = record
	return nil
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
	if found == nil || found.malformed || len(found.Description) > maxRequestOutcomeDescription {
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

// hasRequestOutcomes reports the unprofiled-only requestOutcomes member.
func hasRequestOutcomes(r Receipt) bool {
	for _, test := range r.Tests {
		for _, detail := range test.AttemptDetails {
			if detail.RequestOutcomes != nil {
				return true
			}
		}
	}
	return false
}
