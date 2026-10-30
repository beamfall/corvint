// Package testvaliditydoc builds the corvint-test-validity/0 document
// (docs/specs/live-proof-carrying-verification-v0.md LPCV-V0-051): the shared
// five-axis projection of every test-level result a JavaScript receipt or Go
// preview-session event carries. Projections are recomputed from observations,
// never trusted from the input. The corvint test-validity verb and the
// experimental MCP test-validity profile both call this package.
package testvaliditydoc

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const (
	// Schema names the emitted document.
	Schema = "corvint-test-validity/0"
	// MaxInputBytes bounds one provider document.
	MaxInputBytes    = 4 << 20
	goSessionProfile = "corvint-go-live-session-event/0"
)

// Document is the corvint-test-validity/0 document. Tier and Promotable are
// present only for preview inputs, keeping JavaScript output byte-identical.
type Document struct {
	Playwright   *jstestprovider.Receipt `json:"playwright,omitempty"`
	Schema       string                  `json:"schema"`
	Source       string                  `json:"source"`
	Kind         string                  `json:"kind"`
	Tests        []Test                  `json:"tests"`
	Run          testvalidity.Projection `json:"run"`
	TestsOmitted int                     `json:"testsOmitted,omitempty"`
	Tier         string                  `json:"tier,omitempty"`
	Promotable   *bool                   `json:"promotable,omitempty"`
	Discovery    *Discovery              `json:"discovery,omitempty"`
	// Static is present only on a static source projection (LPCV-V0-150).
	Static *Static `json:"static,omitempty"`
}

// Test is one test-level observation and its recomputed projection. Package is
// emitted only for Go, whose test identity is the (package, name) pair.
type Test struct {
	ID         string                          `json:"id,omitempty"`
	Project    *jstestprovider.ProjectIdentity `json:"project,omitempty"`
	Attempts   []jstestprovider.Attempt        `json:"attempts,omitempty"`
	Name       string                          `json:"name"`
	Package    string                          `json:"package,omitempty"`
	State      string                          `json:"state"`
	Projection testvalidity.Projection         `json:"projection"`
	// Location is where the test is declared, each member present or named absent (LPCV-V0-145).
	Location *Location `json:"location,omitempty"`
	// RouteStubs is present only when the receipt carries a route-stub record (LPCV-V0-073).
	RouteStubs *RouteStubs `json:"routeStubs,omitempty"`
	// RejectionAnchor is present only when the receipt carries a request-outcome record (LPCV-V0-081).
	RejectionAnchor *RejectionAnchor `json:"rejectionAnchor,omitempty"`
	// VacuousEquality is present only when the receipt carries an assertion-operand record
	// (LPCV-V0-121).
	VacuousEquality *VacuousEquality `json:"vacuousEquality,omitempty"`
	// UnstubbedCall is present only when the receipt carries an outbound-call record (LPCV-V0-124).
	UnstubbedCall *UnstubbedCall `json:"unstubbedCall,omitempty"`
	// SourceRules is present only for a discovered Ruby example (TSA-V0-068) or JS/TS outcome
	// (TSA-V0-082) whose test source was read.
	SourceRules *testvalidity.ExampleSource `json:"sourceRules,omitempty"`
}

var jsKinds = map[string]bool{"unit": true, "e2e": true}

// Input is one closed provider input accepted by Decode. Its fields remain
// private so Project can receive only a successfully classified document.
type Input struct {
	js        *jstestprovider.Receipt
	goSession *goSessionDocument
	ruby      *rubytestprovider.Document
}

// jsProviderDocument is the corvint-js-test-provider stdout shape. The carried
// projections are accepted only so the document decodes closed; they are never
// read.
type jsProviderDocument struct {
	Receipt         *jstestprovider.Receipt `json:"receipt"`
	TestProjections json.RawMessage         `json:"testProjections"`
	RunProjection   json.RawMessage         `json:"runProjection"`
}

// goSessionDocument is one completed corvint-go-test-provider preview event.
// Projection members are deliberately opaque: only state, identity, and the
// retained per-test package/name/action observations are projected.
type goSessionDocument struct {
	Profile                string              `json:"profile"`
	State                  string              `json:"state"`
	Identity               string              `json:"identity"`
	Sequence               uint64              `json:"sequence"`
	Scope                  []string            `json:"scope"`
	Detail                 string              `json:"detail"`
	Projection             json.RawMessage     `json:"projection"`
	TestProjections        []goTestObservation `json:"testProjections"`
	TestProjectionsOmitted int                 `json:"testProjectionsOmitted"`
}

type goTestObservation struct {
	Package    string          `json:"package"`
	Name       string          `json:"name"`
	Action     string          `json:"action"`
	Projection json.RawMessage `json:"projection"`
}

type kindProbe struct {
	Receipt json.RawMessage `json:"receipt"`
	Profile json.RawMessage `json:"profile"`
}

// Unsupported is the document for no input: no test-level evidence, so the
// run projection states every axis UNSUPPORTED (LPCV-V0-049), never a pass.
func Unsupported() Document {
	return Document{Schema: Schema, Source: "none", Tests: []Test{}, Run: testvalidity.Project(testvalidity.Input{})}
}

// Decode classifies and decodes exactly one closed JavaScript provider
// document or completed Go preview-session event. Unknown and ambiguous kinds
// are refused instead of guessed; callers expose the shared coded refusal.
func Decode(data []byte) (Input, error) {
	var probe kindProbe
	if err := decode(data, &probe); err != nil {
		return Input{}, invalidSensitiveDocument()
	}
	if probe.Receipt != nil && probe.Profile != nil {
		return Input{}, errors.New("provider document kind is ambiguous")
	}
	if probe.Receipt != nil {
		var profile struct {
			Profile string `json:"profile"`
		}
		profileErr := json.Unmarshal(probe.Receipt, &profile)
		var document jsProviderDocument
		if err := decodeClosed(data, &document); err != nil {
			if profileErr != nil || (profile.Profile != "" && profile.Profile != jstestprovider.ExternalProfile && profile.Profile != jstestprovider.AttestedExternalProfile) {
				return Input{}, invalidSensitiveDocument()
			}
			return Input{}, errors.New("is not a corvint-js-test-provider document: " + err.Error())
		}
		if document.Receipt == nil {
			return Input{}, errors.New("has no receipt member")
		}
		if !jsKinds[document.Receipt.Kind] {
			return Input{}, errors.New("kind is neither unit nor e2e")
		}
		if document.Receipt.Profile != "" {
			if document.Receipt.Profile != jstestprovider.ExternalProfile && document.Receipt.Profile != jstestprovider.AttestedExternalProfile && document.Receipt.Profile != jstestprovider.SensitiveExternalProfile && document.Receipt.Profile != jstestprovider.AttemptExternalProfile {
				return Input{}, errors.New("unknown JavaScript receipt profile")
			}
			if document.Receipt.Profile == jstestprovider.SensitiveExternalProfile {
				if findings := jstestprovider.ValidateSensitiveInputEvidence(*document.Receipt); len(findings) != 0 {
					return Input{}, &jstestprovider.SensitiveInputValidationError{Findings: findings}
				}
			}
			canonical, err := jstestprovider.EncodeQualified(*document.Receipt)
			if err != nil || !bytes.Equal(data, canonical) {
				return Input{}, errors.New("noncanonical qualified Playwright document")
			}
		} else if hasQualifiedMetadata(*document.Receipt) {
			return Input{}, errors.New("qualified Playwright metadata requires its exact profile")
		}
		return Input{js: document.Receipt}, nil
	}
	if probe.Profile != nil {
		var profile string
		if err := json.Unmarshal(probe.Profile, &profile); err != nil || (profile != goSessionProfile && profile != rubytestprovider.Profile) {
			return Input{}, errors.New("provider document kind is unrecognized")
		}
		if profile == rubytestprovider.Profile {
			var document rubytestprovider.Document
			if err := decodeClosed(data, &document); err != nil {
				return Input{}, errors.New("is not a corvint-ruby-test-provider document: " + err.Error())
			}
			if err := rubytestprovider.Validate(document); err != nil {
				return Input{}, err
			}
			return Input{ruby: &document}, nil
		}
		var document goSessionDocument
		if err := decodeClosed(data, &document); err != nil {
			return Input{}, errors.New("is not a corvint-go-test-provider session document: " + err.Error())
		}
		if document.State != "passed" && document.State != "failed" && document.State != "stale" {
			return Input{}, errors.New("Go session event is not a completed run")
		}
		if document.TestProjectionsOmitted < 0 {
			return Input{}, errors.New("Go session event has a negative omitted count")
		}
		return Input{goSession: &document}, nil
	}
	return Input{}, errors.New("provider document kind is unrecognized")
}

func invalidSensitiveDocument() error {
	return &jstestprovider.SensitiveInputValidationError{Findings: []jstestprovider.SensitiveInputFinding{{Code: jstestprovider.SensitiveInputDocumentInvalid, Path: "receipt"}}}
}

func hasQualifiedMetadata(r jstestprovider.Receipt) bool {
	if r.External != nil || r.ApplicationAttestation != nil || r.SensitiveInputPolicy != nil || r.Preflight != nil || len(r.Identity.ConfigInputDigests) != 0 {
		return true
	}
	for _, test := range r.Tests {
		if test.ID != "" || test.Project != nil || len(test.Attempts) != 0 {
			return true
		}
	}
	return false
}

// Project recomputes every per-test and run projection from the decoded
// observations. No carried projection is consulted.
func Project(input Input) Document {
	if input.js != nil {
		return projectJavaScript(*input.js, false)
	}
	if input.goSession != nil {
		return projectGoSession(*input.goSession)
	}
	if input.ruby != nil {
		return projectRuby(*input.ruby)
	}
	return Unsupported()
}

// ProjectRouteStubs is Project with the routeStubs member requested (LPCV-V0-077): every test of a
// JavaScript receipt carries it, so a receipt without any producer record, or a profiled one, states
// NOT_PRODUCED with an explicit reason instead of omitting the member. Project keeps the omission
// and every legacy byte.
func ProjectRouteStubs(input Input) Document {
	if input.js != nil {
		return projectJavaScript(*input.js, true)
	}
	return Project(input)
}

func projectJavaScript(receipt jstestprovider.Receipt, routeStubsRequested bool) Document {
	tests := make([]Test, 0, len(receipt.Tests))
	stubs := routeStubEvidence(receipt, routeStubsRequested)
	anchors := rejectionAnchorEvidence(receipt)
	vacuous := vacuousEqualityEvidence(receipt)
	unstubbed := unstubbedCallEvidence(receipt)
	for i, outcome := range receipt.Tests {
		test := Test{ID: outcome.ID, Project: outcome.Project, Attempts: outcome.Attempts, Name: outcome.Name, State: string(outcome.State), Projection: jstestprovider.ReceiptTestProjection(receipt, outcome), Location: jsLocation(receipt, outcome)}
		if externalProfile(receipt.Profile) {
			test.Projection = externalTestProjection(test.Projection)
		}
		if stubs != nil {
			test.RouteStubs = stubs[i]
		}
		if anchors != nil {
			test.RejectionAnchor = anchors[i]
		}
		if vacuous != nil {
			test.VacuousEquality = vacuous[i]
		}
		if unstubbed != nil {
			test.UnstubbedCall = unstubbed[i]
		}
		tests = append(tests, test)
	}
	var playwright *jstestprovider.Receipt
	run := jstestprovider.ReceiptRunProjection(receipt)
	if externalProfile(receipt.Profile) {
		playwright = &receipt
		run = externalRunProjection(run, tests)
	}
	return Document{
		Playwright: playwright,
		Schema:     Schema,
		Source:     "corvint-js-test-provider",
		Kind:       receipt.Kind,
		Tests:      tests,
		Run:        run,
	}
}

func projectGoSession(session goSessionDocument) Document {
	tests := make([]Test, 0, len(session.TestProjections))
	stale := session.State == "stale"
	for _, observation := range session.TestProjections {
		tests = append(tests, Test{
			Name:       observation.Name,
			Package:    observation.Package,
			State:      observation.Action,
			Projection: testvalidity.ProjectGoTest(observation.Action, session.Identity, stale, ""),
			Location:   goLocation(),
		})
	}
	promotable := false
	return Document{
		Schema:       Schema,
		Source:       "corvint-go-test-provider",
		Kind:         "go-session",
		Tests:        tests,
		Run:          testvalidity.ProjectGoSession(session.State, session.Identity),
		TestsOmitted: session.TestProjectionsOmitted,
		Tier:         "preview",
		Promotable:   &promotable,
	}
}

// projectRuby recomputes every example and run projection of an RSpec
// provider document (LPCV-V0-093). The provider is experimental, so the
// document is a non-promotable preview.
func projectRuby(document rubytestprovider.Document) Document {
	tests := make([]Test, 0, len(document.Examples))
	for _, example := range document.Examples {
		name := example.Description
		if name == "" {
			name = example.ID
		}
		tests = append(tests, Test{ID: example.ID, Name: name, State: example.Status, Projection: rubytestprovider.ProjectExample(document, example), Location: rubyLocation(document, example)})
	}
	promotable := false
	return Document{
		Schema:       Schema,
		Source:       rubytestprovider.Producer,
		Kind:         "rspec",
		Tests:        tests,
		Run:          rubytestprovider.ProjectRun(document),
		TestsOmitted: document.ExamplesOmitted,
		Tier:         "preview",
		Promotable:   &promotable,
	}
}

func decode(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("carries data after the document")
	}
	return nil
}

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
