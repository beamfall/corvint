package testvaliditydoc

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

func TestSensitiveRetainedPolicyAndUnsupportedActionRejection(t *testing.T) {
	for _, test := range []struct{ pattern, title, code string }{
		{"custom+entry", "custom+entry unquoted-secret", jstestprovider.SensitiveInputUnredacted},
		{"***", "*** unquoted-secret", "sensitive-input-policy-invalid"},
		{"custom+entry", `page.getByLabel("Password").fill("unquoted-secret")`, jstestprovider.SensitiveInputActionSyntaxUnsupported},
	} {
		r := jstestprovider.Receipt{Kind: "e2e", Profile: jstestprovider.SensitiveExternalProfile, SensitiveInputPolicy: &jstestprovider.SensitiveInputPolicy{AdditionalActionPatterns: []string{test.pattern}}, Tests: []jstestprovider.TestOutcome{{Attempts: []jstestprovider.Attempt{{Steps: []jstestprovider.BrowserStep{{Title: test.title}}}}}}}
		data, err := json.Marshal(map[string]any{"receipt": r})
		if err != nil {
			t.Fatal(err)
		}
		_, err = Decode(data)
		var rejection *jstestprovider.SensitiveInputValidationError
		if !errors.As(err, &rejection) || len(rejection.Findings) == 0 || rejection.Findings[0].Code != test.code || strings.Contains(err.Error(), "unquoted-secret") {
			t.Errorf("retained admission bypass: %v", err)
		}
	}
}

func TestSensitiveRetainedDecodeNeverEchoesUnknownProperties(t *testing.T) {
	for _, data := range []string{
		`{"receipt":{"profile":"corvint-playwright-external/2","kind":"e2e","hunter2":true}}`,
		`{"hunter2":true,"receipt":{"profile":"corvint-playwright-external/2","kind":"e2e"}}`,
		`{"receipt":{"profile":"corvint-playwright-external/2","kind":"e2e","tests":[{"attempts":[{"steps":[{"hunter2":true}]}]}]}}`,
		`{"receipt":{"profile":"corvint-playwright-external/2","kind":"e2e","hunter2":`,
		`{"receipt":{"profile":"corvint-playwright-external/2","kind":"e2e"}} hunter2`,
	} {
		_, err := Decode([]byte(data))
		if err == nil || strings.Contains(err.Error(), "hunter2") {
			t.Fatalf("retained decoder echoed property: %v", err)
		}
		var rejection *jstestprovider.SensitiveInputValidationError
		if !errors.As(err, &rejection) || rejection.Findings[0].Code != jstestprovider.SensitiveInputDocumentInvalid {
			t.Fatalf("not typed: %T %v", err, err)
		}
	}
}

func TestSensitiveInputConformanceFixtureRejectsLeakAndAcceptsRedaction(t *testing.T) {
	r := jstestprovider.Receipt{
		Profile:              jstestprovider.SensitiveExternalProfile,
		Kind:                 "e2e",
		SensitiveInputPolicy: &jstestprovider.SensitiveInputPolicy{},
		External:             &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown"},
		Tests:                []jstestprovider.TestOutcome{{Name: "login", State: jstestprovider.StateFailed, Attempts: []jstestprovider.Attempt{{State: jstestprovider.StateFailed, Retry: 0, FailureKind: "assertion-or-test", Steps: []jstestprovider.BrowserStep{{Title: `Fill "[REDACTED]"`, Category: "pw:api", Redacted: true}, {Title: "Expect dashboard visible", Category: "expect"}}}}}},
	}
	redacted, err := jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	input, err := Decode(redacted)
	if err != nil {
		t.Fatalf("redacted conformance payload rejected: %v", err)
	}
	projected := Project(input)
	if got := projected.Tests[0].Attempts[0].Steps[0].Title; got != `Fill "[REDACTED]"` {
		t.Fatalf("action traceability lost: %q", got)
	}
	leaking := bytes.Replace(redacted, []byte(`[REDACTED]`), []byte(`deliberately-leaked-value`), 1)
	_, err = Decode(leaking)
	var validationErr *jstestprovider.SensitiveInputValidationError
	if !errors.As(err, &validationErr) || len(validationErr.Findings) != 1 || validationErr.Findings[0].Code != jstestprovider.SensitiveInputUnredacted {
		t.Fatalf("leaking conformance payload error = %T %v", err, err)
	}
}

func TestQualifiedDecodeRejectsNoncanonicalAndPreservesUnknowns(t *testing.T) {
	r := jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown"}, Tests: []jstestprovider.TestOutcome{{Name: "unknown-project", State: jstestprovider.StatePassed}}}
	data, err := jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	input, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	doc := Project(input)
	if doc.Tests[0].Projection.Execution.State == testvalidity.ExecutionPassed || doc.Playwright.External.ServerDescendants != "unknown" {
		t.Fatal("unknown qualified observation became green")
	}
	for _, malformed := range [][]byte{append([]byte(" "), data...), bytes.Replace(data, []byte(`"ownership":"external"`), []byte(`"ownership":"external","unexpected":true`), 1), append(data, []byte("{}")...)} {
		if _, err := Decode(malformed); err == nil {
			t.Fatal("malformed qualified receipt admitted")
		}
	}
	mixed := bytes.Replace(data, []byte(`"profile":"corvint-playwright-external/0"`), []byte(`"profile":"corvint-playwright-external/0","applicationAttestation":{}`), 1)
	if _, err := Decode(mixed); err == nil {
		t.Fatal("legacy profile admitted attested identity fields")
	}
	v1 := bytes.Replace(data, []byte(`"profile":"corvint-playwright-external/0"`), []byte(`"profile":"corvint-playwright-external/1"`), 1)
	input, err = Decode(v1)
	if err != nil || Project(input).Tests[0].Projection.Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("incomplete /1 receipt did not remain explicit infrastructure")
	}
	r.Profile = ""
	data, err = jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Decode(data); err == nil {
		t.Fatal("stripped profile downgraded external metadata to legacy")
	}
	for _, raw := range []string{`{"receipt":{"kind":"e2e","tests":[{"name":"pass","state":"passed","project":{"name":"p"}}]}}`, `{"receipt":{"kind":"e2e","tests":[{"name":"pass","state":"passed","attempts":[{"state":"passed"}]}]}}`} {
		if _, err = Decode([]byte(raw)); err == nil {
			t.Fatal("legacy decoder accepted qualified metadata")
		}
	}
}
