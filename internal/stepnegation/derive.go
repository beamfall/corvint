package stepnegation

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
)

// Fault kinds seed the marker and name the install action.
const (
	KindNetwork = "network"
	KindDOM     = "dom"

	ActionText  = "text"
	ActionValue = "value"
	ActionHide  = "hide"
)

var literalMatchers = map[string]string{"toHaveText": "to.have.text", "toContainText": "to.have.text", "toHaveValue": "to.have.value"}

// Plan is the completed fault plan. Its digest binds the witness; it is
// computed after the marker, which never depends on it (LPCV-V0-061).
type Plan struct {
	Kind       string `json:"kind"`
	Test       string `json:"test"`
	Step       string `json:"step"`
	Assertion  string `json:"assertion"`
	Occurrence int    `json:"occurrence"`
	Marker     string `json:"marker"`
	// Network fault.
	Method           string `json:"method,omitempty"`
	OriginPath       string `json:"originPath,omitempty"`
	RequestOrdinal   int    `json:"requestOrdinal,omitempty"`
	ValueDigest      string `json:"valueDigest,omitempty"`
	ValueLength      int    `json:"valueLength,omitempty"`
	BodyDigestBefore string `json:"bodyDigestBefore,omitempty"`
	BodyDigestAfter  string `json:"bodyDigestAfter,omitempty"`
	// DOM fault.
	Selector   string `json:"selector,omitempty"`
	Expression string `json:"expression,omitempty"`
	DOMOrdinal int    `json:"domOrdinal,omitempty"`
	Action     string `json:"action,omitempty"`
}

// Digest is the SHA-256 of the plan's canonical JSON.
func (plan Plan) Digest() string {
	data, _ := json.Marshal(plan)
	return digestHex(data)
}

// Install is the private instruction the injection module reads from the
// scratch directory. It carries the raw request target, search string and
// selector, so it is never retained (LPCV-V0-069).
type Install struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Marker string `json:"marker"`
	// Network fault: the request whose path matches, ignoring the query.
	Method         string `json:"method,omitempty"`
	OriginPath     string `json:"originPath,omitempty"`
	RequestOrdinal int    `json:"requestOrdinal,omitempty"`
	Search         string `json:"search,omitempty"`
	// DOM fault.
	Selector   string `json:"selector,omitempty"`
	Expression string `json:"expression,omitempty"`
	DOMOrdinal int    `json:"domOrdinal,omitempty"`
	Action     string `json:"action,omitempty"`
}

// Fault is one derived fault: its plan, digest and install instruction.
type Fault struct {
	Plan    Plan
	Digest  string
	Install Install
}

// Derivation is the derivation result for one step.
type Derivation struct {
	Ordinal   int
	Title     string
	Witnessed *Assertion
	Network   *Fault
	DOM       *Fault
	// NetworkReason and DOMReason explain an absent fault.
	NetworkReason string
	DOMReason     string
	// NoAssertion is set for a step that owns no assertion.
	NoAssertion bool
}

// Derive derives the network and DOM faults of one baseline step against
// the declared application origin (LPCV-V0-061, LPCV-V0-062).
func Derive(trace *Trace, key TestKey, ordinal int, applicationOrigin string) Derivation {
	step := trace.Step(ordinal)
	derivation := Derivation{Ordinal: ordinal}
	if step == nil {
		return derivation
	}
	derivation.Title = step.Title
	assertions := trace.StepAssertions(ordinal)
	if len(assertions) == 0 {
		derivation.NoAssertion = true
		return derivation
	}
	for index := range assertions {
		assertion := assertions[index]
		network, networkReason := deriveNetwork(trace, key, step.Title, assertion, applicationOrigin)
		dom, domReason := deriveDOM(key, step.Title, assertion)
		if index == 0 {
			derivation.NetworkReason, derivation.DOMReason = networkReason, domReason
		}
		if network != nil || dom != nil {
			derivation.Witnessed = &assertion
			derivation.Network, derivation.DOM = network, dom
			derivation.NetworkReason, derivation.DOMReason = networkReason, domReason
			return derivation
		}
	}
	return derivation
}

// literal returns the assertion's single nonempty literal expected string.
func literal(assertion Assertion) (ExpectedText, string) {
	if _, ok := literalMatchers[assertion.Matcher]; !ok {
		return ExpectedText{}, ReasonNoDOMFault
	}
	frame := assertion.Frame
	if frame == nil || frame.Selector == "" || frame.Expression != literalMatchers[assertion.Matcher] {
		return ExpectedText{}, ReasonExpectedValueNotLiteral
	}
	if len(frame.ExpectedText) != 1 {
		return ExpectedText{}, ReasonExpectedValueNotLiteral
	}
	expected := frame.ExpectedText[0]
	if expected.RegexSource != nil || expected.String == nil || *expected.String == "" {
		return ExpectedText{}, ReasonExpectedValueNotLiteral
	}
	return expected, ""
}

func deriveNetwork(trace *Trace, key TestKey, stepTitle string, assertion Assertion, applicationOrigin string) (*Fault, string) {
	if assertion.Negated || (assertion.Frame != nil && assertion.Frame.IsNot) {
		return nil, ReasonNegatedAssertion
	}
	if _, ok := literalMatchers[assertion.Matcher]; !ok {
		return nil, ReasonExpectedValueNotLiteral
	}
	expected, reason := literal(assertion)
	if reason != "" {
		return nil, ReasonExpectedValueNotLiteral
	}
	value := *expected.String
	escaped := jsonEscape(value)
	type candidate struct {
		index    int
		response Response
		search   string
	}
	var candidates []candidate
	ambiguous, outside := false, false
	for index, response := range trace.Responses {
		if response.Body == nil || !(response.Completed < assertion.StartTime) {
			continue
		}
		count, search := occurrences(response.Body, value, escaped)
		if count == 0 {
			continue
		}
		if origin(response.URL) != applicationOrigin {
			outside = true
			continue
		}
		if count > 1 {
			ambiguous = true
			continue
		}
		candidates = append(candidates, candidate{index, response, search})
	}
	switch {
	case len(candidates) > 1 || ambiguous:
		return nil, ReasonNetworkDependencyAmbiguous
	case len(candidates) == 0 && outside:
		return nil, ReasonOutsideApplicationOrigin
	case len(candidates) == 0:
		return nil, ReasonNoNetworkDependency
	}
	chosen := candidates[0]
	originPath := originAndPath(chosen.response.URL)
	ordinal := 0
	for _, response := range trace.Responses[:chosen.index+1] {
		if response.Method == chosen.response.Method && originAndPath(response.URL) == originPath {
			ordinal++
		}
	}
	marker := Marker(key, stepTitle, assertion.Location, KindNetwork)
	after := bytes.Replace(chosen.response.Body, []byte(chosen.search), []byte(marker), 1)
	plan := Plan{
		Kind: KindNetwork, Test: key.Digest(), Step: stepTitle, Assertion: assertion.Location, Occurrence: assertion.Occurrence, Marker: marker,
		Method: chosen.response.Method, OriginPath: digestHex([]byte(originPath)), RequestOrdinal: ordinal,
		ValueDigest: digestHex([]byte(value)), ValueLength: len(value),
		BodyDigestBefore: digestHex(chosen.response.Body), BodyDigestAfter: digestHex(after),
	}
	digest := plan.Digest()
	return &Fault{Plan: plan, Digest: digest, Install: Install{ID: digest, Kind: KindNetwork, Marker: marker, Method: chosen.response.Method, OriginPath: originPath, RequestOrdinal: ordinal, Search: chosen.search}}, ""
}

func deriveDOM(key TestKey, stepTitle string, assertion Assertion) (*Fault, string) {
	if assertion.Negated || (assertion.Frame != nil && assertion.Frame.IsNot) {
		return nil, ReasonNegatedAssertion
	}
	marker := Marker(key, stepTitle, assertion.Location, KindDOM)
	var action string
	switch assertion.Matcher {
	case "toBeVisible":
		if assertion.Frame == nil || assertion.Frame.Selector == "" || assertion.Frame.Expression != "to.be.visible" {
			return nil, ReasonNoDOMFault
		}
		action = ActionHide
	case "toHaveText", "toContainText", "toHaveValue":
		expected, reason := literal(assertion)
		if reason != "" {
			if assertion.Frame == nil || assertion.Frame.Selector == "" {
				return nil, ReasonNoDOMFault
			}
			return nil, reason
		}
		if satisfies(marker, expected) {
			return nil, ReasonFaultDoesNotFalsify
		}
		action = ActionText
		if assertion.Matcher == "toHaveValue" {
			action = ActionValue
		}
	default:
		return nil, ReasonNoDOMFault
	}
	frame := assertion.Frame
	plan := Plan{
		Kind: KindDOM, Test: key.Digest(), Step: stepTitle, Assertion: assertion.Location, Occurrence: assertion.Occurrence, Marker: marker,
		Selector: digestHex([]byte(frame.Selector)), Expression: frame.Expression, DOMOrdinal: frame.Ordinal, Action: action,
	}
	digest := plan.Digest()
	return &Fault{Plan: plan, Digest: digest, Install: Install{ID: digest, Kind: KindDOM, Marker: marker, Selector: frame.Selector, Expression: frame.Expression, DOMOrdinal: frame.Ordinal, Action: action}}, ""
}

// satisfies reports whether the marker, as the received text, would meet
// the recorded expectation.
func satisfies(marker string, expected ExpectedText) bool {
	received, want := marker, *expected.String
	if expected.NormalizeWhiteSpace {
		received, want = strings.Join(strings.Fields(received), " "), strings.Join(strings.Fields(want), " ")
	}
	if expected.IgnoreCase {
		received, want = strings.ToLower(received), strings.ToLower(want)
	}
	if expected.MatchSubstring {
		return strings.Contains(received, want)
	}
	return received == want
}

// occurrences counts the raw value plus, when it differs, its JSON string
// escape in body, returning the search string of a single match.
func occurrences(body []byte, value, escaped string) (int, string) {
	count := bytes.Count(body, []byte(value))
	search := value
	if escaped != value {
		if extra := bytes.Count(body, []byte(escaped)); extra > 0 {
			count += extra
			search = escaped
		}
	}
	return count, search
}

func jsonEscape(value string) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	encoded := strings.TrimSuffix(buffer.String(), "\n")
	return encoded[1 : len(encoded)-1]
}

// origin returns scheme://host[:port] in lower case, or "" when raw is not
// an absolute http(s) URL.
func origin(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

// Origin is the exported form used for the readiness URL.
func Origin(raw string) string { return origin(raw) }

func originAndPath(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	return origin(raw) + path
}
