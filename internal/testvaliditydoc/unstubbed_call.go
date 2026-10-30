package testvaliditydoc

import "github.com/Beamfall/corvint/internal/jstestprovider"

// Run-time unstubbed-call states, reasons and code (LPCV-V0-124, LPCV-V0-125).
const (
	UnstubbedClassified    = "CLASSIFIED"
	UnstubbedNotClassified = "NOT_CLASSIFIED"
	UnstubbedNotApplicable = "NOT_APPLICABLE"
	UnstubbedNotProduced   = "NOT_PRODUCED"

	// UnstubbedRecordAbsent keeps the TSA-V0-174 reason: no server-log record was produced.
	UnstubbedRecordAbsent    = "server-outbound-log-not-produced"
	UnstubbedRecordInvalid   = "outbound-call-record-invalid"
	UnstubbedTestNotFailed   = "test-not-failed"
	UnstubbedNoServerError   = "no-server-error"
	UnstubbedAllCallsStubbed = "no-unstubbed-call"
	UnstubbedCallOnServerErr = "unstubbed-call-on-server-error"

	// EnvUnstubbedCall classifies a failure as the environment's: a 5xx whose server log shows an
	// outbound call the mock layer did not answer.
	EnvUnstubbedCall = "ENV-UNSTUBBED-CALL"
)

// UnstubbedCall states whether a failed test's failure is classified as an environment failure
// caused by an outbound call the environment did not stub (LPCV-V0-124). It is not an axis: it never
// rewrites the five-axis projection, and a classified test is still not a witness (LPCV-V0-125).
type UnstubbedCall struct {
	State  string               `json:"state"`
	Reason string               `json:"reason"`
	Code   string               `json:"code,omitempty"`
	Calls  []UnstubbedCallEntry `json:"calls,omitempty"`
}

// UnstubbedCallEntry names one unstubbed outbound call and the 5xx response it was made for.
type UnstubbedCallEntry struct {
	Method         string `json:"method"`
	Host           string `json:"host"`
	Path           string `json:"path"`
	ResponseMethod string `json:"responseMethod"`
	ResponseRoute  string `json:"responseRoute"`
	ResponseStatus int    `json:"responseStatus"`
}

// unstubbedCallEvidence returns each test's unstubbed-call classification. It returns nil when the
// receipt is profiled or no attempt carries an outbound-call record, so such a document keeps every
// byte (LPCV-V0-124).
func unstubbedCallEvidence(receipt jstestprovider.Receipt) []*UnstubbedCall {
	if receipt.Profile != "" || !anyOutboundCallRecord(receipt) {
		return nil
	}
	evidence := make([]*UnstubbedCall, len(receipt.Tests))
	for i, outcome := range receipt.Tests {
		evidence[i] = testUnstubbedCall(outcome)
	}
	return evidence
}

func anyOutboundCallRecord(receipt jstestprovider.Receipt) bool {
	for _, outcome := range receipt.Tests {
		for _, detail := range outcome.AttemptDetails {
			if detail.OutboundCalls != nil {
				return true
			}
		}
	}
	return false
}

// testUnstubbedCall classifies one test from its last attempt. A status code alone never
// classifies: the record must tie a 5xx response to an outbound call that no stub answered.
func testUnstubbedCall(outcome jstestprovider.TestOutcome) *UnstubbedCall {
	if len(outcome.AttemptDetails) == 0 {
		return &UnstubbedCall{State: UnstubbedNotProduced, Reason: UnstubbedRecordAbsent}
	}
	record := outcome.AttemptDetails[len(outcome.AttemptDetails)-1].OutboundCalls
	switch {
	case record == nil:
		return &UnstubbedCall{State: UnstubbedNotProduced, Reason: UnstubbedRecordAbsent}
	case !jstestprovider.ValidOutboundCallRecord(*record):
		return &UnstubbedCall{State: UnstubbedNotProduced, Reason: UnstubbedRecordInvalid}
	case !testFailed(outcome.State):
		return &UnstubbedCall{State: UnstubbedNotApplicable, Reason: UnstubbedTestNotFailed}
	}
	screened := jstestprovider.ScreenOutboundCallRecord(*record)
	serverError := false
	var calls []UnstubbedCallEntry
	for _, response := range screened.Responses {
		if !jstestprovider.IsServerErrorStatus(response.Status) {
			continue
		}
		serverError = true
		for _, call := range response.Calls {
			if !call.Stubbed {
				calls = append(calls, UnstubbedCallEntry{Method: call.Method, Host: call.Host, Path: call.Path, ResponseMethod: response.Method, ResponseRoute: response.Route, ResponseStatus: response.Status})
			}
		}
	}
	switch {
	case !serverError:
		return &UnstubbedCall{State: UnstubbedNotClassified, Reason: UnstubbedNoServerError}
	case len(calls) == 0:
		return &UnstubbedCall{State: UnstubbedNotClassified, Reason: UnstubbedAllCallsStubbed}
	}
	return &UnstubbedCall{State: UnstubbedClassified, Reason: UnstubbedCallOnServerErr, Code: EnvUnstubbedCall, Calls: calls}
}
