// Package externalorigins reports external-origin dependence (PWP-V0-022..027; V1-1762; GitHub
// #1106). The report reads producer-stated network evidence for one or more runs and a project
// policy that declares the test environment and the external origins it accepts as deterministic.
// It is diagnostic: it never rewrites a receipt, the five-axis projection or the passing predicate.
//
// It is a separate package from testvaliditydoc because origin parsing imports net, net/netip and
// net/url, and the MCP tool path imports testvaliditydoc, which must stay free of network packages
// (MTV-V0-007). Only the JS test provider imports this package.
package externalorigins

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Profiles of the closed input and output documents (PWP-V0-022, PWP-V0-025, PWP-V0-027).
const (
	NetworkEvidenceProfile       = "corvint-network-evidence/0"
	ExternalOriginsPolicyProfile = "corvint-external-origins-policy/0"
	ExternalOriginsReportProfile = "corvint-external-origins/0"
)

// Bounds (PWP-V0-022, PWP-V0-025). A document outside them is refused, never truncated.
const (
	MaxNetworkEvidenceBytes       = 4 << 20
	MaxExternalOriginsPolicyBytes = 64 << 10
	MaxNetworkEvidenceRuns        = 16
	MaxNetworkEvidenceTests       = 4096
	MaxNetworkEvidenceRequests    = 1024
	MaxExternalOriginsPolicy      = 256
	maxNetworkEvidenceText        = 1024
	maxNetworkEvidenceDurationMs  = 24 * 60 * 60 * 1000
	// ExternalSlowerThresholdMs is the duration increase, failed over passed, that a correlation
	// names as slower-in-failed (PWP-V0-026).
	ExternalSlowerThresholdMs = 1000
)

// Report verdicts (PWP-V0-027).
const (
	ExternalOriginsDependent = "ENVIRONMENT_DEPENDENT"
	ExternalOriginsClean     = "CLEAN"
	ExternalOriginsUnknown   = "UNKNOWN"
)

// Origin classes and request dependence (PWP-V0-023, PWP-V0-025).
const (
	OriginUndeclared   = "undeclared"
	OriginAllowed      = "allowed"
	RequestBlocking    = "blocking"
	RequestIncidental  = "incidental"
	NetworkRecorded    = "RECORDED"
	NetworkNotProduced = "NOT_PRODUCED"
)

// Codes (PWP-V0-022..027).
const (
	ExternalOriginDependence     = "external-origin-dependence"
	NetworkEvidenceAbsent        = "network-evidence-absent"
	NetworkEvidenceInvalid       = "network-evidence-invalid"
	ExternalOriginsPolicyInvalid = "external-origins-policy-invalid"
)

// Correlation states and difference kinds (PWP-V0-026).
const (
	CorrelationDiffers       = "EXTERNAL_DIFFERENCE"
	CorrelationNoDifference  = "NO_EXTERNAL_DIFFERENCE"
	CorrelationUnknown       = "UNKNOWN"
	DifferenceMissingFailed  = "missing-in-failed"
	DifferenceMissingPassed  = "missing-in-passed"
	DifferenceStatus         = "status-differs"
	DifferenceFollowUps      = "follow-ups-differ"
	DifferenceSlowerInFailed = "slower-in-failed"
)

// NetworkEvidence is one run's producer-stated requests per test, in run order (PWP-V0-022).
type NetworkEvidence struct {
	Profile  string        `json:"profile"`
	Run      string        `json:"run"`
	Revision string        `json:"revision"`
	Tests    []NetworkTest `json:"tests"`
}

// NetworkTest is one test's outcome and requests. Requests absent or null means the run recorded
// no network evidence for the test; an empty array means it recorded none were made.
type NetworkTest struct {
	File     string           `json:"file"`
	Title    string           `json:"title"`
	Outcome  string           `json:"outcome"`
	Requests []NetworkRequest `json:"requests"`
}

// NetworkRequest is one observed request. Status 0 means no response was received. FollowUps
// counts the requests the response itself initiated. Awaited is stated only for fetch and xhr.
type NetworkRequest struct {
	URL          string `json:"url"`
	ResourceType string `json:"resourceType"`
	Awaited      *bool  `json:"awaited,omitempty"`
	Status       int    `json:"status"`
	DurationMs   int    `json:"durationMs"`
	FollowUps    int    `json:"followUps"`
}

// UnmarshalJSON decodes one closed request and refuses an absent or null status, durationMs or
// followUps, so a missing observation can never decode as 0 (PWP-V0-022).
func (r *NetworkRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		URL          string `json:"url"`
		ResourceType string `json:"resourceType"`
		Awaited      *bool  `json:"awaited"`
		Status       *int   `json:"status"`
		DurationMs   *int   `json:"durationMs"`
		FollowUps    *int   `json:"followUps"`
	}
	if err := decodeClosed(data, &raw); err != nil {
		return err
	}
	if raw.Status == nil || raw.DurationMs == nil || raw.FollowUps == nil {
		return errors.New("request status, durationMs and followUps are required and non-null")
	}
	*r = NetworkRequest{URL: raw.URL, ResourceType: raw.ResourceType, Awaited: raw.Awaited,
		Status: *raw.Status, DurationMs: *raw.DurationMs, FollowUps: *raw.FollowUps}
	return nil
}

// ExternalOriginsPolicy is the project's declared test environment (the application under test and
// its declared mocks) and its allow-list of deterministic external origins (PWP-V0-025).
type ExternalOriginsPolicy struct {
	Profile       string          `json:"profile"`
	Environment   []string        `json:"environment"`
	Deterministic []AllowedOrigin `json:"deterministic"`
}

// AllowedOrigin is one deterministic external origin and the project's stated reason.
type AllowedOrigin struct {
	Origin string `json:"origin"`
	Reason string `json:"reason"`
}

// ExternalOriginsReport is the closed corvint-external-origins/0 output (PWP-V0-027).
type ExternalOriginsReport struct {
	Profile      string                      `json:"profile"`
	Verdict      string                      `json:"verdict"`
	Runs         []ExternalOriginsRun        `json:"runs"`
	Origins      []ExternalOrigin            `json:"origins"`
	Findings     []ExternalOriginFinding     `json:"findings"`
	Unknown      []ExternalUnknownTest       `json:"unknown"`
	Correlations []ExternalOriginCorrelation `json:"correlations"`
}

// ExternalOriginsRun summarizes one input run.
type ExternalOriginsRun struct {
	Run                  string `json:"run"`
	Revision             string `json:"revision"`
	Network              string `json:"network"`
	Tests                int    `json:"tests"`
	TestsWithoutEvidence int    `json:"testsWithoutEvidence"`
}

// ExternalOrigin is one origin outside the declared environment (PWP-V0-024).
type ExternalOrigin struct {
	Origin        string   `json:"origin"`
	Class         string   `json:"class"`
	Reason        string   `json:"reason,omitempty"`
	Blocking      int      `json:"blocking"`
	Incidental    int      `json:"incidental"`
	ResourceTypes []string `json:"resourceTypes"`
}

// ExternalTestRef names a test by file and full title.
type ExternalTestRef struct {
	File  string `json:"file"`
	Title string `json:"title"`
}

// ExternalOriginFinding is one validity finding: blocking requests of one resource type to one
// undeclared origin, the first test they affected and how many tests they affected (PWP-V0-024).
type ExternalOriginFinding struct {
	Code         string          `json:"code"`
	Origin       string          `json:"origin"`
	ResourceType string          `json:"resourceType"`
	FirstRun     string          `json:"firstRun"`
	FirstTest    ExternalTestRef `json:"firstTest"`
	Tests        int             `json:"tests"`
}

// ExternalUnknownTest is one test whose run recorded no network evidence (PWP-V0-027).
type ExternalUnknownTest struct {
	Run    string          `json:"run"`
	Test   ExternalTestRef `json:"test"`
	Reason string          `json:"reason"`
}

// ExternalOriginCorrelation pairs a passing and a failing run of one test at one revision and names
// the external requests that differed (PWP-V0-026).
type ExternalOriginCorrelation struct {
	Test        ExternalTestRef             `json:"test"`
	Revision    string                      `json:"revision"`
	PassedRun   string                      `json:"passedRun"`
	FailedRun   string                      `json:"failedRun"`
	State       string                      `json:"state"`
	Reason      string                      `json:"reason,omitempty"`
	Differences []ExternalRequestDifference `json:"differences"`
}

// ExternalRequestDifference is one external request, keyed by resource type and query-free
// locator, whose presence, status, follow-ups or duration differed between the paired runs.
type ExternalRequestDifference struct {
	Origin       string              `json:"origin"`
	Locator      string              `json:"locator"`
	ResourceType string              `json:"resourceType"`
	Kinds        []string            `json:"kinds"`
	Passed       *RequestObservation `json:"passed,omitempty"`
	Failed       *RequestObservation `json:"failed,omitempty"`
}

// RequestObservation is the compared part of one request.
type RequestObservation struct {
	Status     int `json:"status"`
	DurationMs int `json:"durationMs"`
	FollowUps  int `json:"followUps"`
}

var (
	networkOutcomes = map[string]bool{"passed": true, "failed": true, "timedOut": true, "interrupted": true, "skipped": true}
	// blockingTypes are always blocking; fetch and xhr are blocking unless stated not awaited.
	// "other" is blocking because its dependence is unknown (invariant 2).
	blockingTypes   = map[string]bool{"document": true, "script": true, "module": true, "stylesheet": true, "websocket": true, "other": true}
	awaitableTypes  = map[string]bool{"fetch": true, "xhr": true}
	incidentalTypes = map[string]bool{"beacon": true, "image": true, "font": true, "media": true}
	localSchemes    = map[string]bool{"data": true, "blob": true, "about": true}
	defaultPorts    = map[string]string{"http": "80", "https": "443", "ws": "80", "wss": "443"}
	revisionPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// RequestDependence classifies one request as blocking or incidental (PWP-V0-023).
func RequestDependence(request NetworkRequest) string {
	if awaitableTypes[request.ResourceType] && request.Awaited != nil && !*request.Awaited {
		return RequestIncidental
	}
	if incidentalTypes[request.ResourceType] {
		return RequestIncidental
	}
	return RequestBlocking
}

// RequestOrigin returns the canonical origin (lower-case scheme and host, an IP host in its
// canonical address form, the port as a decimal number without leading zeros, default port elided) and
// the query- and fragment-free locator of a request URL. A data:, blob: or about: URL has no
// network origin and returns ok false (PWP-V0-022).
func RequestOrigin(raw string) (origin, locator string, ok bool, err error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", false, fmt.Errorf("request url does not parse")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if localSchemes[scheme] {
		return "", "", false, nil
	}
	if _, network := defaultPorts[scheme]; !network {
		return "", "", false, fmt.Errorf("request url scheme %q is not http, https, ws, wss, data, blob or about", scheme)
	}
	if parsed.User != nil {
		return "", "", false, errors.New("request url carries userinfo")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", "", false, errors.New("request url has no host")
	}
	if address, err := netip.ParseAddr(host); err == nil {
		host = address.String()
	}
	port := parsed.Port()
	if port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return "", "", false, fmt.Errorf("request url port %q is not 0..65535", port)
		}
		port = strconv.FormatUint(number, 10)
	}
	if port == defaultPorts[scheme] {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	origin = scheme + "://" + host
	if port != "" {
		origin = scheme + "://" + net.JoinHostPort(strings.Trim(host, "[]"), port)
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	return origin, origin + path, true, nil
}

// DecodeNetworkEvidence decodes one closed, bounded network-evidence document (PWP-V0-022).
func DecodeNetworkEvidence(data []byte) (NetworkEvidence, error) {
	var evidence NetworkEvidence
	if len(data) > MaxNetworkEvidenceBytes {
		return evidence, refusal(NetworkEvidenceInvalid, "document exceeds %d bytes", MaxNetworkEvidenceBytes)
	}
	if err := decodeClosed(data, &evidence); err != nil {
		return evidence, refusal(NetworkEvidenceInvalid, "%v", err)
	}
	if err := validateNetworkEvidence(evidence); err != nil {
		return evidence, refusal(NetworkEvidenceInvalid, "%v", err)
	}
	return evidence, nil
}

// DecodeExternalOriginsPolicy decodes the closed, bounded project policy (PWP-V0-025).
func DecodeExternalOriginsPolicy(data []byte) (ExternalOriginsPolicy, error) {
	var policy ExternalOriginsPolicy
	if len(data) > MaxExternalOriginsPolicyBytes {
		return policy, refusal(ExternalOriginsPolicyInvalid, "document exceeds %d bytes", MaxExternalOriginsPolicyBytes)
	}
	if err := decodeClosed(data, &policy); err != nil {
		return policy, refusal(ExternalOriginsPolicyInvalid, "%v", err)
	}
	if err := validateExternalOriginsPolicy(policy); err != nil {
		return policy, refusal(ExternalOriginsPolicyInvalid, "%v", err)
	}
	return policy, nil
}

func refusal(code, format string, args ...any) error {
	return fmt.Errorf("%s: %s", code, fmt.Sprintf(format, args...))
}

func evidenceText(value string) bool {
	return value != "" && len(value) <= maxNetworkEvidenceText && utf8.ValidString(value) &&
		!strings.ContainsRune(value, 0) && !strings.ContainsRune(value, utf8.RuneError)
}

func validateNetworkEvidence(evidence NetworkEvidence) error {
	if evidence.Profile != NetworkEvidenceProfile {
		return fmt.Errorf("profile is not %s", NetworkEvidenceProfile)
	}
	if !evidenceText(evidence.Run) {
		return errors.New("run label is empty, invalid or longer than 1024 bytes")
	}
	if !revisionPattern.MatchString(evidence.Revision) {
		return errors.New("revision is not a full lower-case Git object id")
	}
	if evidence.Tests == nil || len(evidence.Tests) > MaxNetworkEvidenceTests {
		return fmt.Errorf("tests is absent or exceeds %d", MaxNetworkEvidenceTests)
	}
	seen := map[ExternalTestRef]bool{}
	for i, test := range evidence.Tests {
		ref := ExternalTestRef{File: test.File, Title: test.Title}
		if !evidenceText(test.File) || !evidenceText(test.Title) {
			return fmt.Errorf("test %d has an empty or invalid file or title", i)
		}
		if seen[ref] {
			return fmt.Errorf("test %d repeats a file and title in one run", i)
		}
		seen[ref] = true
		if !networkOutcomes[test.Outcome] {
			return fmt.Errorf("test %d outcome %q is outside the closed set", i, test.Outcome)
		}
		if len(test.Requests) > MaxNetworkEvidenceRequests {
			return fmt.Errorf("test %d exceeds %d requests", i, MaxNetworkEvidenceRequests)
		}
		for j, request := range test.Requests {
			if err := validateNetworkRequest(request); err != nil {
				return fmt.Errorf("test %d request %d: %v", i, j, err)
			}
		}
	}
	return nil
}

func validateNetworkRequest(request NetworkRequest) error {
	if !evidenceText(request.URL) {
		return errors.New("url is empty, invalid or longer than 1024 bytes")
	}
	if _, _, _, err := RequestOrigin(request.URL); err != nil {
		return err
	}
	known := blockingTypes[request.ResourceType] || awaitableTypes[request.ResourceType] || incidentalTypes[request.ResourceType]
	if !known {
		return fmt.Errorf("resourceType %q is outside the closed set", request.ResourceType)
	}
	if request.Awaited != nil && !awaitableTypes[request.ResourceType] {
		return errors.New("awaited is stated only for fetch and xhr")
	}
	if request.Status != 0 && (request.Status < 100 || request.Status > 599) {
		return fmt.Errorf("status %d is neither 0 nor 100..599", request.Status)
	}
	if request.DurationMs < 0 || request.DurationMs > maxNetworkEvidenceDurationMs {
		return errors.New("durationMs is outside 0..86400000")
	}
	if request.FollowUps < 0 || request.FollowUps > MaxNetworkEvidenceRequests {
		return fmt.Errorf("followUps is outside 0..%d", MaxNetworkEvidenceRequests)
	}
	return nil
}

func validateExternalOriginsPolicy(policy ExternalOriginsPolicy) error {
	if policy.Profile != ExternalOriginsPolicyProfile {
		return fmt.Errorf("profile is not %s", ExternalOriginsPolicyProfile)
	}
	if policy.Environment == nil || policy.Deterministic == nil {
		return errors.New("environment and deterministic are both required arrays")
	}
	if len(policy.Environment)+len(policy.Deterministic) > MaxExternalOriginsPolicy {
		return fmt.Errorf("more than %d origins", MaxExternalOriginsPolicy)
	}
	seen := map[string]bool{}
	check := func(origin string) error {
		canonical, _, ok, err := RequestOrigin(origin)
		if err != nil || !ok || canonical != origin {
			return fmt.Errorf("origin %q is not a canonical http, https, ws or wss origin", origin)
		}
		if seen[origin] {
			return fmt.Errorf("origin %q is declared twice", origin)
		}
		seen[origin] = true
		return nil
	}
	for _, origin := range policy.Environment {
		if err := check(origin); err != nil {
			return err
		}
	}
	for _, allowed := range policy.Deterministic {
		if err := check(allowed.Origin); err != nil {
			return err
		}
		if !evidenceText(allowed.Reason) {
			return fmt.Errorf("deterministic origin %q has no reason", allowed.Origin)
		}
	}
	return nil
}

type findingKey struct{ origin, resourceType string }

type findingState struct {
	finding ExternalOriginFinding
	tests   map[ExternalTestRef]bool
}

// BuildExternalOriginsReport classifies every request of every run against the policy and returns
// the report. Runs are read in the given order, and each run's tests in run order (PWP-V0-024..027).
func BuildExternalOriginsReport(policy ExternalOriginsPolicy, runs []NetworkEvidence) (ExternalOriginsReport, error) {
	report := ExternalOriginsReport{
		Profile: ExternalOriginsReportProfile, Runs: []ExternalOriginsRun{}, Origins: []ExternalOrigin{},
		Findings: []ExternalOriginFinding{}, Unknown: []ExternalUnknownTest{}, Correlations: []ExternalOriginCorrelation{},
	}
	if len(runs) == 0 || len(runs) > MaxNetworkEvidenceRuns {
		return report, refusal(NetworkEvidenceInvalid, "between 1 and %d runs are required", MaxNetworkEvidenceRuns)
	}
	if err := validateExternalOriginsPolicy(policy); err != nil {
		return report, refusal(ExternalOriginsPolicyInvalid, "%v", err)
	}
	environment := map[string]bool{}
	for _, origin := range policy.Environment {
		environment[origin] = true
	}
	allowed := map[string]string{}
	for _, entry := range policy.Deterministic {
		allowed[entry.Origin] = entry.Reason
	}
	origins := map[string]*ExternalOrigin{}
	originTypes := map[string]map[string]bool{}
	findings := map[findingKey]*findingState{}
	labels := map[string]bool{}
	runsWithoutEvidence := 0
	for _, run := range runs {
		if err := validateNetworkEvidence(run); err != nil {
			return report, refusal(NetworkEvidenceInvalid, "%v", err)
		}
		if labels[run.Run] {
			return report, refusal(NetworkEvidenceInvalid, "run label %q repeats", run.Run)
		}
		labels[run.Run] = true
		summary := ExternalOriginsRun{Run: run.Run, Revision: run.Revision, Network: NetworkRecorded, Tests: len(run.Tests)}
		for _, test := range run.Tests {
			ref := ExternalTestRef{File: test.File, Title: test.Title}
			if test.Requests == nil {
				summary.TestsWithoutEvidence++
				report.Unknown = append(report.Unknown, ExternalUnknownTest{Run: run.Run, Test: ref, Reason: NetworkEvidenceAbsent})
				continue
			}
			for _, request := range test.Requests {
				origin, _, network, _ := RequestOrigin(request.URL)
				if !network || environment[origin] {
					continue
				}
				entry := origins[origin]
				if entry == nil {
					entry = &ExternalOrigin{Origin: origin, Class: OriginUndeclared}
					if reason, ok := allowed[origin]; ok {
						entry.Class, entry.Reason = OriginAllowed, reason
					}
					origins[origin] = entry
					originTypes[origin] = map[string]bool{}
				}
				originTypes[origin][request.ResourceType] = true
				dependence := RequestDependence(request)
				if dependence == RequestIncidental {
					entry.Incidental++
					continue
				}
				entry.Blocking++
				if entry.Class != OriginUndeclared {
					continue
				}
				key := findingKey{origin, request.ResourceType}
				state := findings[key]
				if state == nil {
					state = &findingState{
						finding: ExternalOriginFinding{Code: ExternalOriginDependence, Origin: origin, ResourceType: request.ResourceType, FirstRun: run.Run, FirstTest: ref},
						tests:   map[ExternalTestRef]bool{},
					}
					findings[key] = state
				}
				state.tests[ref] = true
			}
		}
		// A run with no test, or with a test lacking evidence, stays NOT_PRODUCED: another run's
		// evidence never clears it (PWP-V0-027).
		if summary.TestsWithoutEvidence > 0 || len(run.Tests) == 0 {
			summary.Network = NetworkNotProduced
			runsWithoutEvidence++
		}
		report.Runs = append(report.Runs, summary)
	}
	for _, origin := range sortedKeys(origins) {
		entry := origins[origin]
		entry.ResourceTypes = sortedKeys(originTypes[origin])
		report.Origins = append(report.Origins, *entry)
	}
	keys := make([]findingKey, 0, len(findings))
	for key := range findings {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].origin != keys[j].origin {
			return keys[i].origin < keys[j].origin
		}
		return keys[i].resourceType < keys[j].resourceType
	})
	for _, key := range keys {
		state := findings[key]
		state.finding.Tests = len(state.tests)
		report.Findings = append(report.Findings, state.finding)
	}
	report.Correlations = correlateExternalOrigins(runs, environment)
	switch {
	case len(report.Findings) > 0:
		report.Verdict = ExternalOriginsDependent
	case runsWithoutEvidence > 0:
		report.Verdict = ExternalOriginsUnknown
	default:
		report.Verdict = ExternalOriginsClean
	}
	return report, nil
}

type testAtRevision struct {
	revision string
	test     ExternalTestRef
}

type testRun struct {
	run  string
	test NetworkTest
}

// correlateExternalOrigins pairs, for each test at each revision, its first passing run with its
// first failing (failed or timedOut) run, in input order (PWP-V0-026).
func correlateExternalOrigins(runs []NetworkEvidence, environment map[string]bool) []ExternalOriginCorrelation {
	passed := map[testAtRevision]*testRun{}
	failed := map[testAtRevision]*testRun{}
	var order []testAtRevision
	for _, run := range runs {
		for _, test := range run.Tests {
			key := testAtRevision{run.Revision, ExternalTestRef{File: test.File, Title: test.Title}}
			if _, ok := passed[key]; !ok && test.Outcome == "passed" {
				passed[key] = &testRun{run.Run, test}
				if failed[key] != nil {
					order = append(order, key)
				}
			}
			if _, ok := failed[key]; !ok && (test.Outcome == "failed" || test.Outcome == "timedOut") {
				failed[key] = &testRun{run.Run, test}
				if passed[key] != nil {
					order = append(order, key)
				}
			}
		}
	}
	correlations := []ExternalOriginCorrelation{}
	for _, key := range order {
		pass, fail := passed[key], failed[key]
		correlation := ExternalOriginCorrelation{
			Test: key.test, Revision: key.revision, PassedRun: pass.run, FailedRun: fail.run,
			Differences: []ExternalRequestDifference{},
		}
		if pass.test.Requests == nil || fail.test.Requests == nil {
			correlation.State, correlation.Reason = CorrelationUnknown, NetworkEvidenceAbsent
		} else {
			correlation.Differences = externalDifferences(pass.test.Requests, fail.test.Requests, environment)
			correlation.State = CorrelationNoDifference
			if len(correlation.Differences) > 0 {
				correlation.State = CorrelationDiffers
			}
		}
		correlations = append(correlations, correlation)
	}
	return correlations
}

type requestKey struct{ resourceType, locator string }

type keyedRequest struct {
	origin  string
	request NetworkRequest
}

func externalRequests(requests []NetworkRequest, environment map[string]bool) map[requestKey]keyedRequest {
	keyed := map[requestKey]keyedRequest{}
	for _, request := range requests {
		origin, locator, network, _ := RequestOrigin(request.URL)
		if !network || environment[origin] {
			continue
		}
		key := requestKey{request.ResourceType, locator}
		if _, ok := keyed[key]; !ok {
			keyed[key] = keyedRequest{origin, request}
		}
	}
	return keyed
}

func externalDifferences(passedRequests, failedRequests []NetworkRequest, environment map[string]bool) []ExternalRequestDifference {
	passed := externalRequests(passedRequests, environment)
	failed := externalRequests(failedRequests, environment)
	keys := map[requestKey]bool{}
	for key := range passed {
		keys[key] = true
	}
	for key := range failed {
		keys[key] = true
	}
	ordered := make([]requestKey, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].locator != ordered[j].locator {
			return ordered[i].locator < ordered[j].locator
		}
		return ordered[i].resourceType < ordered[j].resourceType
	})
	differences := []ExternalRequestDifference{}
	for _, key := range ordered {
		pass, inPassed := passed[key]
		fail, inFailed := failed[key]
		difference := ExternalRequestDifference{Locator: key.locator, ResourceType: key.resourceType, Kinds: []string{}}
		if inPassed {
			difference.Origin = pass.origin
			difference.Passed = observe(pass.request)
		}
		if inFailed {
			difference.Origin = fail.origin
			difference.Failed = observe(fail.request)
		}
		switch {
		case !inFailed:
			difference.Kinds = append(difference.Kinds, DifferenceMissingFailed)
		case !inPassed:
			difference.Kinds = append(difference.Kinds, DifferenceMissingPassed)
		default:
			if pass.request.Status != fail.request.Status {
				difference.Kinds = append(difference.Kinds, DifferenceStatus)
			}
			if pass.request.FollowUps != fail.request.FollowUps {
				difference.Kinds = append(difference.Kinds, DifferenceFollowUps)
			}
			if fail.request.DurationMs-pass.request.DurationMs >= ExternalSlowerThresholdMs {
				difference.Kinds = append(difference.Kinds, DifferenceSlowerInFailed)
			}
		}
		if len(difference.Kinds) > 0 {
			differences = append(differences, difference)
		}
	}
	return differences
}

func observe(request NetworkRequest) *RequestObservation {
	return &RequestObservation{Status: request.Status, DurationMs: request.DurationMs, FollowUps: request.FollowUps}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// decodeClosed decodes exactly one JSON document into target, refusing unknown fields and
// trailing data, as testvaliditydoc does for its own documents.
func decodeClosed(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("carries data after the document")
	}
	return nil
}
