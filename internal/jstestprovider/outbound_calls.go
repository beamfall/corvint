package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// OutboundCallsAnnotation is the one per-attempt annotation a test or its fixture publishes to state,
// from the server log, which outbound calls the server made while answering the attempt's requests
// and whether the environment's mock layer answered each (LPCV-V0-123). Its description is the
// closed JSON OutboundCallRecord.
const OutboundCallsAnnotation = "corvint-outbound-calls"

// Outbound-call record bounds. A record outside them is not a record.
const (
	MaxOutboundResponses        = 64
	MaxOutboundCallsPerResponse = 64
	maxOutboundCallDescription  = 128 << 10
	minServerErrorStatus        = 500
	maxServerErrorStatus        = 599
	outboundCallForbiddenInPath = "?#"
	outboundCallForbiddenInHost = "/?#@ \t\r\n"
)

// OutboundCallRecord is one attempt's declared server-log evidence (LPCV-V0-123). Responses are the
// application responses the attempt received, each with the outbound calls the server log ties to
// it. The record is provider authority: Corvint does not read the server log it summarizes.
type OutboundCallRecord struct {
	Responses []OutboundResponse `json:"responses"`
}

// OutboundResponse is one application response: its method, route template and status, and the
// outbound calls the server made while producing it.
type OutboundResponse struct {
	Method string         `json:"method"`
	Route  string         `json:"route"`
	Status int            `json:"status"`
	Calls  []OutboundCall `json:"calls"`
}

// OutboundCall is one outbound call: its method, host and path without query or fragment, and
// whether a stub of the environment's mock layer answered it.
type OutboundCall struct {
	Method  string `json:"method"`
	Host    string `json:"host"`
	Path    string `json:"path"`
	Stubbed bool   `json:"stubbed"`
}

// IsServerErrorStatus reports a 5xx status.
func IsServerErrorStatus(status int) bool {
	return status >= minServerErrorStatus && status <= maxServerErrorStatus
}

// ValidOutboundCallRecord reports whether a record is inside the closed vocabulary and bounds.
func ValidOutboundCallRecord(record OutboundCallRecord) bool {
	if record.Responses == nil || len(record.Responses) > MaxOutboundResponses {
		return false
	}
	for _, response := range record.Responses {
		if !requestOutcomeMethod(response.Method) || !requestOutcomeText(response.Route) || response.Status < minRequestOutcomeStatus || response.Status > maxRequestOutcomeStatus {
			return false
		}
		if response.Calls == nil || len(response.Calls) > MaxOutboundCallsPerResponse {
			return false
		}
		for _, call := range response.Calls {
			if !requestOutcomeMethod(call.Method) || !outboundHost(call.Host) || !outboundPath(call.Path) {
				return false
			}
		}
	}
	return true
}

func outboundHost(host string) bool {
	return requestOutcomeText(host) && !strings.ContainsAny(host, outboundCallForbiddenInHost)
}

// outboundPath is a path without query or fragment, so a test input carried in a query string is
// never retained.
func outboundPath(path string) bool {
	return requestOutcomeText(path) && strings.HasPrefix(path, "/") && !strings.ContainsAny(path, outboundCallForbiddenInPath)
}

// ScreenOutboundCallRecord applies the product secret screen to every route, host and path.
func ScreenOutboundCallRecord(record OutboundCallRecord) OutboundCallRecord {
	responses := make([]OutboundResponse, len(record.Responses))
	for i, response := range record.Responses {
		response.Route, _ = secretscreen.Screen(response.Route)
		calls := make([]OutboundCall, len(response.Calls))
		for j, call := range response.Calls {
			call.Host, _ = secretscreen.Screen(call.Host)
			call.Path, _ = secretscreen.Screen(call.Path)
			calls[j] = call
		}
		response.Calls = calls
		responses[i] = response
	}
	return OutboundCallRecord{Responses: responses}
}

// outboundCallMembers are the exact member names of a record.
var outboundCallMembers = map[string]bool{"responses": true, "method": true, "route": true, "status": true, "calls": true, "host": true, "path": true, "stubbed": true}

// UnmarshalJSON decodes a record closed: unknown, case-variant and duplicate members are refused at
// every level, every response must state its status and every call whether it was stubbed, so a
// missing or null member is never read as a server error or an unstubbed call.
func (r *OutboundCallRecord) UnmarshalJSON(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("outbound-call record is not valid UTF-8")
	}
	if irregularJSONMemberOf(data, outboundCallMembers) {
		return errors.New("outbound-call record has a duplicate or inexact member")
	}
	type wireCall struct {
		Method  string `json:"method"`
		Host    string `json:"host"`
		Path    string `json:"path"`
		Stubbed *bool  `json:"stubbed"`
	}
	var wire struct {
		Responses []struct {
			Method string     `json:"method"`
			Route  string     `json:"route"`
			Status *int       `json:"status"`
			Calls  []wireCall `json:"calls"`
		} `json:"responses"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	record := OutboundCallRecord{}
	if wire.Responses != nil {
		record.Responses = make([]OutboundResponse, len(wire.Responses))
	}
	for i, response := range wire.Responses {
		if response.Status == nil {
			return errors.New("outbound response has no status")
		}
		decoded := OutboundResponse{Method: response.Method, Route: response.Route, Status: *response.Status}
		if response.Calls != nil {
			decoded.Calls = make([]OutboundCall, len(response.Calls))
		}
		for j, call := range response.Calls {
			if call.Stubbed == nil {
				return errors.New("outbound call does not state whether it was stubbed")
			}
			decoded.Calls[j] = OutboundCall{Method: call.Method, Host: call.Host, Path: call.Path, Stubbed: *call.Stubbed}
		}
		record.Responses[i] = decoded
	}
	*r = record
	return nil
}

// outboundCallRecord reads the attempt's corvint-outbound-calls annotation. Only exactly one closed,
// in-bounds annotation is a record; none, a duplicate or a malformed one yields nil, so the consumer
// reports NOT_PRODUCED rather than classifying a failure from a status code alone.
func outboundCallRecord(annotations []playwrightAnnotation) *OutboundCallRecord {
	description, ok := singleAnnotation(annotations, OutboundCallsAnnotation, maxOutboundCallDescription)
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

// hasOutboundCalls reports the unprofiled-only outboundCalls member.
func hasOutboundCalls(r Receipt) bool {
	for _, test := range r.Tests {
		for _, detail := range test.AttemptDetails {
			if detail.OutboundCalls != nil {
				return true
			}
		}
	}
	return false
}

// runRecordProfileError refuses a profiled receipt that carries an assertion-operand or outbound-call
// record: both are unprofiled-only members (LPCV-V0-120, LPCV-V0-123).
func runRecordProfileError(r Receipt) error {
	switch {
	case r.Profile != "" && hasAssertionOperands(r):
		return errors.New("external-profile-has-assertion-operands")
	case r.Profile != "" && hasOutboundCalls(r):
		return errors.New("external-profile-has-outbound-calls")
	}
	return nil
}
