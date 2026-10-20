package jstestprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

func TestQualifiedReporterSensitiveRedaction(t *testing.T) {
	command := exec.Command("node", "--test", "qualified-reporter_test.cjs")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("reporter regression: %v\n%s", err, output)
	}
}

func TestSensitiveInputEvidenceRedactionAndValidation(t *testing.T) {
	t.Run("PWP-V2-002 canonical action redaction", func(t *testing.T) {
		r := Receipt{Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{}, Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{{Title: `Fill "hunter2"`}}}}}}}
		safe, err := RedactSensitiveInputEvidence(r)
		if err != nil {
			t.Fatal(err)
		}
		if got := safe.Tests[0].Attempts[0].Steps[0].Title; got != `Fill "[REDACTED]"` {
			t.Fatalf("canonical title = %q", got)
		}
	})

	policy := SensitiveInputPolicy{
		AdditionalActionPatterns:  []string{"set secret"},
		AdditionalSensitiveFields: []string{"credential"},
	}
	leaking := Receipt{
		Profile:              SensitiveExternalProfile,
		Kind:                 "e2e",
		SensitiveInputPolicy: &policy,
		Infrastructure:       &InfrastructureFailure{Reason: "fixture", Detail: "infrastructure hunter2"},
		Tests: []TestOutcome{{
			Name: "keeps assertion title", State: StateFailed,
			FailureMessage: "assertion hunter2",
			Artifacts:      []FailureArtifact{{Name: "hunter2 artifact", Path: "/tmp/hunter2"}},
			Attempts: []Attempt{{State: StateFailed, Retry: 0, FailureKind: "assertion-or-test", Steps: []BrowserStep{
				{Title: `Navigate to /login`, Category: "pw:api"},
				{Title: `Fill "hunter2"`, Category: "pw:api", Error: `Fill "hunter2" failed`, Attachments: []FailureArtifact{{Name: "hunter2 screenshot", Path: "/tmp/hunter2.png"}}, Steps: []BrowserStep{{Title: `Type "nested secret"`, Category: "pw:api"}}},
				{Title: `keyboard.insertText "inserted secret"`, Category: "pw:api"},
				{Title: `Type unquoted-secret`, Category: "pw:api"},
				{Title: `Set secret "provider value"`, Category: "provider", Metadata: map[string]string{"credential": "provider value", "selector": "#token"}},
				{Title: `Expect input type to be text`, Category: "expect"},
			}}},
		}},
	}

	findings := ValidateSensitiveInputEvidence(leaking)
	if len(findings) == 0 || findings[0].Code != SensitiveInputUnredacted {
		t.Fatalf("leaking provider payload was not rejected with typed finding: %+v", findings)
	}
	var validationErr *SensitiveInputValidationError
	if _, err := EncodeQualified(leaking); !errors.As(err, &validationErr) || len(validationErr.Findings) == 0 {
		t.Fatalf("leaking /2 receipt error = %T %v", err, err)
	}

	redacted, err := RedactSensitiveInputEvidence(leaking)
	if err != nil {
		t.Fatal(err)
	}
	if findings := ValidateSensitiveInputEvidence(redacted); len(findings) != 0 {
		t.Fatalf("redacted receipt rejected: %+v", findings)
	}
	steps := redacted.Tests[0].Attempts[0].Steps
	for _, leaked := range []string{"hunter2", "nested secret", "inserted secret", "provider value"} {
		encoded, _ := json.Marshal(redacted)
		if strings.Contains(string(encoded), leaked) {
			t.Fatalf("redacted receipt retained %q: %s", leaked, encoded)
		}
	}
	if steps[0].Title != `Navigate to /login` || steps[5].Title != `Expect input type to be text` || steps[4].Metadata["selector"] != "#token" {
		t.Fatalf("non-sensitive traceability changed: %+v", steps)
	}
	if steps[1].Title != `Fill "[REDACTED]"` || !steps[1].Redacted || steps[1].Steps[0].Title != `Type "[REDACTED]"` || steps[2].Title != `InsertText "[REDACTED]"` {
		t.Fatalf("default actions were not redacted: %+v", steps)
	}
	if steps[3].Title != `Type "[REDACTED]"` || steps[1].Error != SensitiveInputRedactionMarker || steps[1].Attachments[0].Name != SensitiveInputRedactionMarker || steps[1].Attachments[0].Path != SensitiveInputRedactionMarker || steps[4].Metadata["credential"] != SensitiveInputRedactionMarker {
		t.Fatalf("nested sensitive fields were not redacted: %+v", steps)
	}
	if redacted.Tests[0].FailureMessage != SensitiveInputRedactionMarker || redacted.Tests[0].Artifacts[0].Name != SensitiveInputRedactionMarker || redacted.Infrastructure.Detail != SensitiveInputRedactionMarker {
		t.Fatalf("receipt-level sensitive strings were not redacted: %+v", redacted)
	}
}

func TestSensitiveInputProfileIsAdditiveAndNonPromotable(t *testing.T) {
	legacy := qualifiedFixture(t)
	legacy.Tests[0].Attempts[0].Steps = []BrowserStep{{Title: `Fill "[REDACTED]"`, Redacted: true}}
	if _, err := EncodeQualified(legacy); err == nil || err.Error() != "legacy-external-profile-has-sensitive-input-fields" {
		t.Fatalf("legacy profile admitted /2 fields: %v", err)
	}

	sensitive := qualifiedFixture(t)
	sensitive.Profile = SensitiveExternalProfile
	sensitive.SensitiveInputPolicy = &SensitiveInputPolicy{}
	sensitive.Tests[0].Attempts[0].Steps = []BrowserStep{{Title: `Fill "[REDACTED]"`, Redacted: true}}
	if _, err := EncodeQualified(sensitive); err != nil {
		t.Fatalf("redacted /2 receipt rejected: %v", err)
	}
	if ReceiptTestProjection(sensitive, sensitive.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("unqualified /2 reporter projected passing execution")
	}

	invalid := SensitiveInputPolicy{AdditionalActionPatterns: []string{""}}
	sensitive.SensitiveInputPolicy = &invalid
	if _, err := RedactSensitiveInputEvidence(sensitive); err == nil || err.Error() != "sensitive-input-policy-invalid" {
		t.Fatalf("invalid additive policy admitted: %v", err)
	}
}

func TestSensitiveInputNormalizationBoundsAndNoPanic(t *testing.T) {
	for _, title := range []string{` Fill "hunter2"`, `FILL: hunter2`, `Ⱥ.Fill "hunter2"`, `provider/custom-entry(metadata-secret)`} {
		r := Receipt{Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{AdditionalActionPatterns: []string{"custom entry"}}, Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{{Title: title}}}}}}}
		redacted, err := RedactSensitiveInputEvidence(r)
		if err != nil {
			t.Fatalf("title %q: %v", title, err)
		}
		encoded, _ := json.Marshal(redacted)
		if strings.Contains(string(encoded), "hunter2") || strings.Contains(string(encoded), "metadata-secret") {
			t.Fatalf("title %q leaked: %s", title, encoded)
		}
	}

	deep := BrowserStep{Title: "parent"}
	for range sensitiveInputMaxDepth + 1 {
		deep = BrowserStep{Title: "parent", Steps: []BrowserStep{deep}}
	}
	over := Receipt{Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{}, Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{deep}}}}}}
	if findings := ValidateSensitiveInputEvidence(over); len(findings) != 1 || findings[0].Code != SensitiveInputDepthExceeded {
		t.Fatalf("depth findings=%+v", findings)
	}
	over.Tests[0].Attempts = []Attempt{{Steps: make([]BrowserStep, 3000)}, {Steps: make([]BrowserStep, 3000)}}
	if findings := ValidateSensitiveInputEvidence(over); len(findings) == 0 || findings[0].Code != SensitiveInputStepBoundExceeded || len(findings) > sensitiveInputMaxFindings {
		t.Fatalf("step findings=%+v", findings)
	}
	over.Tests[0].Attempts = []Attempt{{Steps: []BrowserStep{{Title: strings.Repeat("x", sensitiveInputMaxStringBytes+1)}}}}
	if findings := ValidateSensitiveInputEvidence(over); len(findings) != 1 || findings[0].Code != SensitiveInputStringBoundExceeded {
		t.Fatalf("string findings=%+v", findings)
	}
	leaks := make([]BrowserStep, sensitiveInputMaxFindings+10)
	for i := range leaks {
		leaks[i] = BrowserStep{Title: `Fill "leak"`}
	}
	over.Tests[0].Attempts = []Attempt{{Steps: leaks}}
	findings := ValidateSensitiveInputEvidence(over)
	if len(findings) != sensitiveInputMaxFindings || findings[len(findings)-1].Code != SensitiveInputFindingBoundExceeded {
		t.Fatalf("finding bound=%+v", findings)
	}
}

func TestSensitiveInputScrubsSiblingRiskFieldsWithoutChangingStructure(t *testing.T) {
	r := Receipt{Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{}, Identity: Identity{RunnerName: "playwright", RunnerVersion: "passed-a"}, Tests: []TestOutcome{{
		Name: "assertion title", FullName: "suite > assertion title", State: StatePassed, FailureMessage: "actual unquoted-secret",
		Attempts: []Attempt{{State: StatePassed, Steps: []BrowserStep{{Title: "Type unquoted-secret"}, {Title: "Expect visible", Error: "actual unquoted-secret", Attachments: []FailureArtifact{{Name: "unquoted-secret screenshot", Path: "/tmp/unquoted-secret.png"}}}}}, {State: StatePassed, Retry: 1, Steps: []BrowserStep{{Title: "Navigate /account"}}}},
	}}}
	redacted, err := RedactSensitiveInputEvidence(r)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(redacted)
	if strings.Contains(string(encoded), "unquoted-secret") {
		t.Fatalf("sibling leak: %s", encoded)
	}
	if redacted.Tests[0].Name != r.Tests[0].Name || redacted.Tests[0].State != r.Tests[0].State || !reflect.DeepEqual(redacted.Identity, r.Identity) || redacted.Tests[0].Attempts[0].Steps[1].Title != "Expect visible" {
		t.Fatalf("structural fields changed: %+v", redacted)
	}
	if findings := ValidateSensitiveInputEvidence(r); len(findings) == 0 || findings[0].Code != SensitiveInputUnredacted {
		t.Fatalf("raw sibling leak admitted: %+v", findings)
	}
}

func TestSensitiveProfileReportFailuresNeverEchoDecoderText(t *testing.T) {
	leaked := "attacker-property-hunter2"
	_, failure := decodeQualifiedReport([]byte(`{"`+leaked+`":true}`), SensitiveExternalProfile)
	if strings.Contains(failure.Detail, leaked) || failure.Detail != "untrusted reporter output rejected" {
		t.Fatalf("sensitive decoder detail echoed input: %+v", failure)
	}
	_, legacy := decodeQualifiedReport([]byte(`{"`+leaked+`":true}`), ExternalProfile)
	if !strings.Contains(legacy.Detail, leaked) {
		t.Fatalf("legacy detail changed: %+v", legacy)
	}
}

func TestSensitiveInputAlreadyRedactedRiskFieldsFailClosed(t *testing.T) {
	for _, field := range []string{"step-error", "step-attachment", "sibling-error", "retry-error", "failure", "artifact", "infrastructure"} {
		t.Run(field, func(t *testing.T) {
			r := Receipt{Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{}, Tests: []TestOutcome{{Name: "assertion hunter2", State: StateFailed, Attempts: []Attempt{{Steps: []BrowserStep{{Title: `Fill "[REDACTED]"`, Redacted: true}, {Title: "Expect navigation"}}}, {Retry: 1, Steps: []BrowserStep{{Title: "Navigate /account"}}}}}}}
			switch field {
			case "step-error":
				r.Tests[0].Attempts[0].Steps[0].Error = "hunter2"
			case "step-attachment":
				r.Tests[0].Attempts[0].Steps[0].Attachments = []FailureArtifact{{Name: "hunter2", Path: "[REDACTED]"}}
			case "sibling-error":
				r.Tests[0].Attempts[0].Steps[1].Error = "actual hunter2"
			case "retry-error":
				r.Tests[0].Attempts[1].Steps[0].Error = "actual hunter2"
			case "failure":
				r.Tests[0].FailureMessage = "hunter2"
			case "artifact":
				r.Tests[0].Artifacts = []FailureArtifact{{Name: "[REDACTED]", Path: "hunter2"}}
			case "infrastructure":
				r.Infrastructure = &InfrastructureFailure{Detail: "hunter2"}
			}
			findings := ValidateSensitiveInputEvidence(r)
			if len(findings) == 0 || findings[0].Code != SensitiveInputUnredacted {
				t.Fatalf("already-redacted bypass: %+v", findings)
			}
			encoded, _ := json.Marshal(findings)
			if strings.Contains(string(encoded), "hunter2") {
				t.Fatal("finding echoes value")
			}
			safe, err := RedactSensitiveInputEvidence(r)
			if err != nil || len(ValidateSensitiveInputEvidence(safe)) != 0 {
				t.Fatalf("canonical repair rejected: %v %+v", err, safe)
			}
			if safe.Tests[0].Name != r.Tests[0].Name || safe.Tests[0].Attempts[0].Steps[1].Title != "Expect navigation" || safe.Tests[0].Attempts[1].Steps[0].Title != "Navigate /account" {
				t.Fatal("structural fields changed")
			}
		})
	}
}

func TestSensitiveInputReceiverPrefixExtraction(t *testing.T) {
	for _, title := range []string{"keyboard.insertText unquoted-secret", "Ⱥ. InSeRtText: unquoted-secret", "keyboard/insert \t text unquoted-secret", "locator. press   sequentially(unquoted-secret)"} {
		t.Run(title, func(t *testing.T) {
			r := Receipt{Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{{Title: title}, {Title: "Expect visible", Error: "actual unquoted-secret"}}}}}, {FailureMessage: "actual unquoted-secret"}}}
			safe, err := RedactSensitiveInputEvidence(r)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(safe)
			if strings.Contains(string(data), "unquoted-secret") {
				t.Fatalf("receiver-prefix leak: %s", data)
			}
		})
	}
}

func TestSensitiveInputUnicodeGrammarAndReportScope(t *testing.T) {
	for _, title := range []string{"(Fill) unquoted-secret", "\u00a0Fill unquoted-secret", "custom entry unquoted-secret", "Fill(#password, unquoted-secret)", "\u0085(Ⱥ.\u2003InSeRt\u00a0TeXt)(unquoted-secret)", "provider/custom-entry(unquoted-secret)", "“Fill” unquoted-secret", "FİLL unquoted-secret"} {
		t.Run(title, func(t *testing.T) {
			r := Receipt{Kind: "e2e", Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{AdditionalActionPatterns: []string{"custom entry"}}, Tests: []TestOutcome{
				{Name: "input", State: StateFailed, Attempts: []Attempt{{State: StateFailed, Steps: []BrowserStep{{Title: title}}}}},
				{Name: "other", State: StateFailed, FailureMessage: "actual unquoted-secret", Artifacts: []FailureArtifact{{Name: "unquoted-secret", Path: "/tmp/unquoted-secret"}}, Attempts: []Attempt{{Steps: []BrowserStep{{Title: "Expect visible", Error: "actual unquoted-secret"}}}}},
			}}
			if len(ValidateSensitiveInputEvidence(r)) == 0 {
				t.Fatal("raw action admitted")
			}
			policy, _ := sensitivePolicy(r.SensitiveInputPolicy)
			candidates := boundaryCollectSensitiveValues(r, policy)
			found := false
			for _, candidate := range candidates {
				found = found || candidate == "unquoted-secret"
			}
			if !found {
				t.Fatal("input value not extracted")
			}
			safe, err := RedactSensitiveInputEvidence(r)
			if err != nil {
				t.Fatal(err)
			}
			data, err := EncodeQualified(safe)
			if err != nil || strings.Contains(string(data), "unquoted-secret") {
				t.Fatalf("report-wide leak: %v %s", err, data)
			}
			if safe.Tests[1].Attempts[0].Steps[0].Title != "Expect visible" || safe.Tests[1].Name != "other" || safe.Tests[1].State != StateFailed {
				t.Fatal("structural fields changed")
			}
		})
	}
	for _, title := range []string{"Expect input type to be text", "Expect locator.fill to pass", "Navigate /fill", "Navigate https://example.test/type", "Refill account", "Fillable field", "Expect custom entry to succeed"} {
		r := Receipt{SensitiveInputPolicy: &SensitiveInputPolicy{AdditionalActionPatterns: []string{"custom entry"}}, Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{{Title: title, Error: "ordinary assertion"}}}}}}}
		safe, err := RedactSensitiveInputEvidence(r)
		if err != nil || !reflect.DeepEqual(safe.Tests, r.Tests) {
			t.Fatalf("non-action changed: %q %v", title, err)
		}
	}
}

func TestSensitiveInputAlreadyRedactedCrossTestRiskRejected(t *testing.T) {
	t.Run("PWP-V2-003 report-wide cross-test risk", func(t *testing.T) {
		for _, field := range []string{"failure", "artifact", "step-error", "step-attachment"} {
			t.Run(field, func(t *testing.T) {
				r := Receipt{Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{{Title: `Fill "[REDACTED]"`, Redacted: true}}}}}, {Attempts: []Attempt{{Steps: []BrowserStep{{Title: "Expect visible"}}}}}}}
				switch field {
				case "failure":
					r.Tests[1].FailureMessage = "hunter2"
				case "artifact":
					r.Tests[1].Artifacts = []FailureArtifact{{Path: "hunter2"}}
				case "step-error":
					r.Tests[1].Attempts[0].Steps[0].Error = "hunter2"
				case "step-attachment":
					r.Tests[1].Attempts[0].Steps[0].Attachments = []FailureArtifact{{Name: "hunter2"}}
				}
				findings := ValidateSensitiveInputEvidence(r)
				if len(findings) == 0 || findings[0].Code != SensitiveInputUnredacted {
					t.Fatal("cross-test risk admitted")
				}
				data, _ := json.Marshal(findings)
				if strings.Contains(string(data), "hunter2") {
					t.Fatal("finding echoed original")
				}
			})
		}
	})
}

func TestExternalReadiness(t *testing.T) {
	for _, status := range []int{200, 302, 401, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			err := externalReady(context.Background(), server.URL, 80*time.Millisecond)
			if (err == nil) != (status == 200) {
				t.Fatalf("status %d: %v", status, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if externalReady(ctx, "http://127.0.0.1:1", time.Second) == nil {
		t.Fatal("cancelled readiness passed")
	}
}

func qualifiedFixture(t *testing.T) Receipt {
	t.Helper()
	r := Receipt{Profile: ExternalProfile, Kind: "e2e", Identity: Identity{ConfigFile: "/repo/config.cjs", ConfigDigest: "config", TestFileDigests: map[string]string{"/repo/test.cjs": "test"}, Argv: []string{"npx", "playwright", "--config=/tmp/a/config.cjs", "--project=one"}, NodeVersion: "v22", RunnerVersion: "1.60.0"}, External: &ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true}}
	r.Identity.RunnerName = "playwright"
	r.External.ReadyURL = "http://127.0.0.1:3002"
	r.External.DeclaredAppIdentity = "fixture"
	r.External.ConfigOverride = "controlled-fixture-config"
	test := TestOutcome{Name: "pass", FullName: "one > pass", State: StatePassed, Anchor: &Anchor{File: "/repo/test.cjs", Line: 1}, Project: &ProjectIdentity{Name: "one", Browser: "chromium", Device: "unknown", Use: json.RawMessage(`{}`), ConfigDigest: "config"}, Attempts: []Attempt{{State: StatePassed, Retry: 0, FailureKind: "none"}}}
	r.Identity.ConfigInputDigests = map[string]string{r.Identity.ConfigFile: r.Identity.ConfigDigest}
	test.ID = qualifiedTestID(r.Identity, test)
	r.Tests = []TestOutcome{test}
	return r
}

func TestQualifiedReceiptProjection(t *testing.T) {
	r := qualifiedFixture(t)
	if ReceiptTestProjection(r, r.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("complete fixture did not pass")
	}
	for _, mutate := range []func(*Receipt){
		func(r *Receipt) { r.External = nil }, func(r *Receipt) { r.External.ReadyAtStart = false }, func(r *Receipt) { r.External.ReadyAtPublish = false }, func(r *Receipt) { r.External.RunnerDescendantsGone = false }, func(r *Receipt) { r.External.InputsUnchanged = false }, func(r *Receipt) { r.Tests[0].Project = nil }, func(r *Receipt) { r.Tests[0].ID = "forged" }, func(r *Receipt) { r.Tests[0].Project.Name = "" }, func(r *Receipt) { r.Tests[0].Attempts = nil }, func(r *Receipt) { r.Infrastructure = &InfrastructureFailure{Reason: "reporter"} },
		func(r *Receipt) { r.External.ReadyURL = "" }, func(r *Receipt) { r.External.DeclaredAppIdentity = "" }, func(r *Receipt) { r.External.ConfigOverride = "" }, func(r *Receipt) { r.Tests[0].Project.Use = json.RawMessage(`null`) }, func(r *Receipt) { r.Tests[0].Project.Use = json.RawMessage(`[]`) }, func(r *Receipt) { r.Tests[0].Attempts[0].State = "invented" }, func(r *Receipt) { r.Tests[0].Attempts[0].State = StateInfrastructure },
	} {
		r := qualifiedFixture(t)
		mutate(&r)
		if r.Tests[0].ID != "forged" {
			r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
		}
		if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
			t.Fatal("unknown lifecycle/identity emitted pass")
		}
	}
	r.Cancelled = true
	r.Infrastructure = &InfrastructureFailure{Reason: "cancelled"}
	if ReceiptRunProjection(r).Execution.State != testvalidity.ExecutionCancelled {
		t.Fatal("cancellation was lost")
	}
	if ReceiptRunProjection(r).Freshness.State == testvalidity.FreshnessCurrent {
		t.Fatal("unknown external freshness became current")
	}
}

func TestPlaywright163UnqualifiedBrowserTupleAbstains(t *testing.T) {
	r := qualifiedFixture(t)
	r.Identity.RunnerVersion = "1.63.0"
	r.Identity.NodeVersion = "v22.23.2"
	r.Tests[0].Project.Use = json.RawMessage(`{"browserName":"chromium","channel":"","launchOptions":{"executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome 153.0.8010.48","channel":"","executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome","headlessShellAvailable":true}}`)
	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	if ReceiptTestProjection(r, r.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("qualified Playwright 1.63 browser tuple abstained")
	}
	for name, replacement := range map[string][2]string{
		"browser-version":   {"Google Chrome 153.0.8010.48", "Google Chrome 153.0.8010.47"},
		"browser-type":      {`"browserType":"chromium"`, `"browserType":"firefox"`},
		"channel":           {`"channel":""`, `"channel":"chrome"`},
		"channel-missing":   {`"channel":"",`, ``},
		"channel-null":      {`"channel":""`, `"channel":null`},
		"executable-path":   {"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/tmp/chrome"},
		"headless-missing":  {`,"headlessShellAvailable":true`, ``},
		"headless-null":     {`"headlessShellAvailable":true`, `"headlessShellAvailable":null`},
		"headless-mismatch": {`"headlessShellAvailable":true`, `"headlessShellAvailable":false`},
		"node-version":      {"v22.23.2", "v22.23.1"},
	} {
		t.Run(name, func(t *testing.T) {
			mutated := r
			mutated.Tests = append([]TestOutcome(nil), r.Tests...)
			mutated.Tests[0].Project = &ProjectIdentity{}
			*mutated.Tests[0].Project = *r.Tests[0].Project
			mutated.Tests[0].Project.Use = json.RawMessage(strings.Replace(string(r.Tests[0].Project.Use), replacement[0], replacement[1], 1))
			mutated.Tests[0].ID = qualifiedTestID(mutated.Identity, mutated.Tests[0])
			if ReceiptTestProjection(mutated, mutated.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("mismatched Playwright 1.63 tuple projected green")
			}
		})
	}
	contradictory := r
	contradictory.Tests = append([]TestOutcome(nil), r.Tests...)
	contradictory.Tests[0].Project = &ProjectIdentity{}
	*contradictory.Tests[0].Project = *r.Tests[0].Project
	contradictory.Tests[0].Project.Browser = "firefox"
	contradictory.Tests[0].ID = qualifiedTestID(contradictory.Identity, contradictory.Tests[0])
	if ReceiptTestProjection(contradictory, contradictory.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("contradictory retained browser identity projected green")
	}
}

func TestPlaywright163BundledBrowserTupleAbstainsOnDrift(t *testing.T) {
	r := qualifiedFixture(t)
	r.Identity.RunnerVersion = "1.63.0"
	r.Identity.NodeVersion = "v22.23.2"
	r.Tests[0].Project.Use = json.RawMessage(`{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`)
	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	if ReceiptTestProjection(r, r.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("qualified bundled Playwright 1.63 browser tuple abstained")
	}
	for name, replacement := range map[string][2]string{
		"browser-version":   {"Google Chrome for Testing 153.0.8010.12", "Google Chrome for Testing 153.0.8010.11"},
		"browser-revision":  {`"browserRevision":"1243"`, `"browserRevision":"1242"`},
		"manifest-version":  {`"manifestBrowserVersion":"153.0.8010.12"`, `"manifestBrowserVersion":"153.0.8010.11"`},
		"executable-digest": {"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282", "b0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282"},
		"executable-name":   {`"executableName":"chromium-headless-shell"`, `"executableName":"chromium"`},
		"executable-source": {`"executableSource":"playwright-bundled"`, `"executableSource":"configured"`},
		"executable-path":   {"chromium_headless_shell-1243", "chromium_headless_shell-1242"},
		"node-version":      {"v22.23.2", "v22.23.1"},
		"headed":            {`"headless":true`, `"headless":false`},
		"explicit-path":     {`"launchOptions":{}`, `"launchOptions":{"executablePath":"/tmp/browser"}`},
		"remote-connection": {`"launchOptions":{}`, `"connectOptions":{"wsEndpoint":"ws://127.0.0.1:1"},"launchOptions":{}`},
	} {
		t.Run(name, func(t *testing.T) {
			mutated := r
			mutated.Tests = append([]TestOutcome(nil), r.Tests...)
			mutated.Tests[0].Project = &ProjectIdentity{}
			*mutated.Tests[0].Project = *r.Tests[0].Project
			mutated.Tests[0].Project.Use = json.RawMessage(strings.Replace(string(r.Tests[0].Project.Use), replacement[0], replacement[1], 1))
			mutated.Tests[0].ID = qualifiedTestID(mutated.Identity, mutated.Tests[0])
			if ReceiptTestProjection(mutated, mutated.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("mismatched bundled Playwright 1.63 tuple projected green")
			}
		})
	}
}

func TestQualifiedReceiptBindingSeparatesInfrastructureOutcomeFromInvalidLifecycle(t *testing.T) {
	r := qualifiedFixture(t)
	r.Tests[0].State = StateInfrastructure
	r.Tests[0].Attempts[0].State = StateInfrastructure
	r.Tests[0].Attempts[0].FailureKind = "browser-or-fixture"
	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	if !QualifiedReceiptBindingReady(r, r.Tests[0]) {
		t.Fatal("qualified infrastructure outcome lost its valid binding")
	}
	r.External.RunnerDescendantsGone = false
	if QualifiedReceiptBindingReady(r, r.Tests[0]) {
		t.Fatal("failed runner cleanup retained a valid binding")
	}
}

func TestQualifiedIdentityStableAcrossScratchAndDistinctAcrossProjects(t *testing.T) {
	r := qualifiedFixture(t)
	original := r.Tests[0].ID
	r.Identity.Argv[2] = "--config=/tmp/b/config.cjs"
	if qualifiedTestID(r.Identity, r.Tests[0]) != original {
		t.Fatal("scratch changed stable test identity")
	}
	r.Tests[0].Project.Name = "two"
	if qualifiedTestID(r.Identity, r.Tests[0]) == original {
		t.Fatal("project identity collision")
	}
}

func TestExternalAdmission(t *testing.T) {
	base := E2EConfig{Config: Config{ConfigFile: "config.cjs", RunnerVersion: "1.60.0", TestFiles: []string{"test.cjs"}}, ExternalServer: true, AppIdentity: "fixture", ServerReadyURL: "http://127.0.0.1:3002"}
	if err := admitExternal(base); err != nil {
		t.Fatal(err)
	}
	base.RunnerVersion = "1.63.0"
	if err := admitExternal(base); err != nil {
		t.Fatal(err)
	}
	base.RunnerVersion = "1.64.0"
	if err := admitExternal(base); err != nil {
		t.Fatal(err)
	}
	base.RunnerVersion = "1.62.0"
	if admitExternal(base) == nil {
		t.Fatal("unqualified Playwright version admitted")
	}
	base.RunnerVersion = "1.60.0"
	for _, arg := range []string{"--config=evil", "--reporter=json", "--global-setup=evil", "--ui", "--debug"} {
		c := base
		c.TestArgv = []string{arg}
		if admitExternal(c) == nil {
			t.Fatalf("admitted %s", arg)
		}
	}
	base.ServerArgv = []string{"server"}
	if admitExternal(base) == nil {
		t.Fatal("external mode admitted owned server")
	}
}

func TestQualifiedEncodingRejectsSecretAndBounds(t *testing.T) {
	r := qualifiedFixture(t)
	r.Tests[0].FailureMessage = "password=actual-private-value"
	if _, err := EncodeQualified(r); err == nil {
		t.Fatal("secret retained")
	}
	r = qualifiedFixture(t)
	r.Tests[0].FailureMessage = strings.Repeat("x", externalOutputLimit)
	if _, err := EncodeQualified(r); err == nil {
		t.Fatal("oversize retained")
	}
}

func TestSensitiveInputPrefixOverlappingValuesRedactLongestFirst(t *testing.T) {
	r := Receipt{Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{}, Tests: []TestOutcome{{
		Name: "prefix", FullName: "suite > prefix", State: StateFailed, FailureMessage: "saw hunter2extra then hunter2",
		Attempts: []Attempt{{State: StateFailed, Steps: []BrowserStep{{Title: `Fill "hunter2extra"`}, {Title: `Fill "hunter2"`}}}},
	}}}
	policy, err := sensitivePolicy(r.SensitiveInputPolicy)
	if err != nil {
		t.Fatal(err)
	}
	first := boundaryCollectSensitiveValues(r, policy)
	for range 32 {
		values := boundaryCollectSensitiveValues(r, policy)
		if strings.Join(values, "\x00") != strings.Join(first, "\x00") {
			t.Fatalf("sensitive value order drifted: %q vs %q", values, first)
		}
		if got := boundaryScrubText(boundaryScrubReplacer(values), "saw hunter2extra then hunter2"); got != "saw [REDACTED] then [REDACTED]" {
			t.Fatalf("partial redaction: %q", got)
		}
	}
	redacted, err := RedactSensitiveInputEvidence(r)
	if err != nil {
		t.Fatal(err)
	}
	if encoded, _ := json.Marshal(redacted); strings.Contains(string(encoded), "hunter2") || strings.Contains(string(encoded), "extra") {
		t.Fatalf("prefix leak: %s", encoded)
	}
}

// PWP-V0-021: a suite whose non-node_modules module graph exceeds the earlier
// 256-input cap binds when every input is within both bounds; above the count or
// byte bound the report is refused as config-inputs-overflow naming the observed
// value and the limit, never as report-output-overflow.
func TestQualifiedReportConfigInputBounds(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "playwright.config.cjs")
	if err := os.WriteFile(config, []byte("module.exports = {};\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDigest, err := digestFile(config)
	if err != nil {
		t.Fatal(err)
	}
	fixture := func() (Receipt, qualifiedReport) {
		r := qualifiedFixture(t)
		r.Identity.ConfigFile, r.Identity.ConfigDigest = config, configDigest
		r.Tests[0].Project.ConfigDigest = configDigest
		report := qualifiedReport{Version: r.Identity.RunnerVersion, Status: "passed", Tests: r.Tests, ConfigFiles: map[string]string{config: configDigest}, Files: map[string]string{"/repo/test.cjs": "test"}}
		return r, report
	}

	r, report := fixture()
	for i := range 300 {
		helper := filepath.Join(dir, fmt.Sprintf("support-%03d.cjs", i))
		if err := os.WriteFile(helper, []byte(fmt.Sprintf("module.exports = %d;\n", i)), 0o600); err != nil {
			t.Fatal(err)
		}
		digest, err := digestFile(helper)
		if err != nil {
			t.Fatal(err)
		}
		report.ConfigFiles[helper] = digest
	}
	if err := bindQualifiedReport(&r, report); err != nil {
		t.Fatalf("301 config inputs within both bounds refused: %v", err)
	}
	if len(r.Identity.ConfigInputDigests) != 301 || !QualifiedReceiptBindingReady(r, r.Tests[0]) {
		t.Fatalf("bound run lost its config inputs or identity: %d inputs", len(r.Identity.ConfigInputDigests))
	}

	r, report = fixture()
	for i := range externalMaxConfigInputs {
		report.ConfigFiles[fmt.Sprintf("/missing/helper-%d.cjs", i)] = "digest"
	}
	want := fmt.Sprintf("config-inputs-overflow: observed %d config inputs, limit %d", externalMaxConfigInputs+1, externalMaxConfigInputs)
	if err := bindQualifiedReport(&r, report); err == nil || err.Error() != want {
		t.Fatalf("config input count overflow err=%v, want %q", err, want)
	}

	r, report = fixture()
	long := "/missing/" + strings.Repeat("d", 4000)
	size := len(config) + len(configDigest)
	for i := 0; size <= externalMaxConfigInputBytes; i++ {
		path := fmt.Sprintf("%s-%04d.cjs", long, i)
		report.ConfigFiles[path] = configDigest
		size += len(path) + len(configDigest)
	}
	if len(report.ConfigFiles) > externalMaxConfigInputs {
		t.Fatalf("byte-bound fixture also exceeds the count bound: %d", len(report.ConfigFiles))
	}
	want = fmt.Sprintf("config-inputs-overflow: observed %d config input bytes, limit %d", size, externalMaxConfigInputBytes)
	if err := bindQualifiedReport(&r, report); err == nil || err.Error() != want {
		t.Fatalf("config input byte overflow err=%v, want %q", err, want)
	}
	if failure := externalReportFailure(SensitiveExternalProfile, "report-identity-unknown", want); failure.Detail != "untrusted reporter output rejected" {
		t.Fatalf("/2 detail echoed reporter output: %q", failure.Detail)
	}
}

// PWP-V0-019: a Playwright config with a setup project dependency reports the
// setup file's results beside the target spec. Binding only the target spec
// refuses test-file-unbound naming each unbound file and its cause; binding the
// setup file too admits the run. Nothing reported is bound implicitly.
func TestQualifiedReportNamesUnboundSetupProjectFiles(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "playwright.config.cjs")
	if err := os.WriteFile(config, []byte("module.exports = {projects: [{name: 'setup'}, {name: 'one', dependencies: ['setup']}]};\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDigest, err := digestFile(config)
	if err != nil {
		t.Fatal(err)
	}
	setupFile := "/repo/auth.setup.cjs"
	fixture := func() (Receipt, qualifiedReport) {
		r := qualifiedFixture(t)
		r.Identity.ConfigFile, r.Identity.ConfigDigest = config, configDigest
		r.Tests[0].Project.ConfigDigest = configDigest
		setup := r.Tests[0]
		setup.Name, setup.FullName = "authenticate", "setup > authenticate"
		setup.Anchor = &Anchor{File: setupFile, Line: 3}
		setup.Project = &ProjectIdentity{Name: "setup", Browser: "chromium", Device: "unknown", Use: json.RawMessage(`{}`), ConfigDigest: configDigest}
		setup.Attempts = []Attempt{{State: StatePassed, Retry: 0, FailureKind: "none"}}
		r.Tests = []TestOutcome{setup, r.Tests[0]}
		report := qualifiedReport{Version: r.Identity.RunnerVersion, Status: "passed", Tests: r.Tests, ConfigFiles: map[string]string{config: configDigest}, Files: map[string]string{setupFile: "setup", "/repo/test.cjs": "test"}}
		return r, report
	}

	r, report := fixture()
	err = bindQualifiedReport(&r, report)
	want := "test-file-unbound: " + setupFile + " (not passed with --test-file)"
	if err == nil || err.Error() != want {
		t.Fatalf("setup-project run err=%v, want %q", err, want)
	}
	if r.Identity.TestFileDigests[setupFile] != "" {
		t.Fatal("reported setup file was bound without --test-file")
	}

	r, report = fixture()
	r.Identity.TestFileDigests[setupFile] = "setup"
	if err := bindQualifiedReport(&r, report); err != nil {
		t.Fatalf("setup file bound with --test-file still refused: %v", err)
	}
	for _, test := range r.Tests {
		if test.ID == "" || !QualifiedReceiptBindingReady(r, test) {
			t.Fatalf("bound setup-project run lost identity for %s", test.FullName)
		}
	}

	r, report = fixture()
	r.Identity.TestFileDigests[setupFile] = "changed"
	delete(report.Files, "/repo/test.cjs")
	err = bindQualifiedReport(&r, report)
	want = "test-file-unbound: " + setupFile + " (reporter digest differs from the --test-file digest); /repo/test.cjs (not digested by the reporter)"
	if err == nil || err.Error() != want {
		t.Fatalf("cause detail err=%v, want %q", err, want)
	}

	// Anchors arrive unsorted and one file anchors two tests; the detail names
	// distinct files in bytewise order, eight at most, then the remainder count.
	r, report = fixture()
	base := r.Tests[1]
	for _, i := range []int{9, 3, 7, 0, 5, 3, 8, 1, 6, 2, 4} {
		extra := base
		extra.FullName = fmt.Sprintf("one > extra %d > %d", i, len(r.Tests))
		extra.Anchor = &Anchor{File: fmt.Sprintf("/repo/extra-%02d.cjs", i), Line: len(r.Tests)}
		report.Files[extra.Anchor.File] = "extra"
		r.Tests = append(r.Tests, extra)
	}
	report.Tests = r.Tests
	err = bindQualifiedReport(&r, report)
	want = "test-file-unbound: /repo/auth.setup.cjs (not passed with --test-file)"
	for i := range 7 {
		want += fmt.Sprintf("; /repo/extra-%02d.cjs (not passed with --test-file)", i)
	}
	want += "; +3 more"
	if err == nil || err.Error() != want {
		t.Fatalf("bounded detail err=%v, want %q", err, want)
	}

	if failure := externalReportFailure(SensitiveExternalProfile, "report-identity-unknown", want); failure.Detail != "untrusted reporter output rejected" || strings.Contains(failure.Detail, "/repo/") {
		t.Fatalf("/2 detail echoed reporter paths: %q", failure.Detail)
	}
}

func TestProfileAttemptEmptyProject(t *testing.T) {
	r := qualifiedFixture(t)
	r.Profile = AttemptExternalProfile
	r.Identity.RunnerVersion = "1.63.0"
	r.Identity.NodeVersion = "v22.23.2"
	r.Tests[0].Project.Use = json.RawMessage(`{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`)
	r.Tests[0].Project.Name = ""
	r.Tests[0].AttemptDetails = []AttemptDetail{{State: StatePassed, Anchor: r.Tests[0].Anchor}}
	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	if !QualifiedReceiptBindingReady(r, r.Tests[0]) {
		t.Fatal("valid unnamed /3 project refused")
	}
	raw, err := EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeAttemptReceipt(raw); err != nil {
		t.Fatal(err)
	}
	r.Profile = ExternalProfile
	r.Tests[0].AttemptDetails = nil
	if QualifiedReceiptBindingReady(r, r.Tests[0]) {
		t.Fatal("legacy empty-project boundary changed")
	}
}

// PWP-V0-009: one exact Node release is admitted for the base profile's bundled
// headless-shell tuple only; neighbouring releases, the system-browser tuple
// and the other external profiles keep abstaining.
func TestPlaywright163Node24TupleAdmissionIsExact(t *testing.T) {
	bundled := `{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"NODE","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`
	system := `{"browserName":"chromium","channel":"","launchOptions":{"executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"NODE","browserType":"chromium","browserVersion":"Google Chrome 153.0.8010.48","channel":"","executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome","headlessShellAvailable":true}}`
	receipt := func(use, identityNode, browserNode string) Receipt {
		r := qualifiedFixture(t)
		r.Identity.RunnerVersion = "1.63.0"
		r.Identity.NodeVersion = identityNode
		r.Tests[0].Project.Use = json.RawMessage(strings.Replace(use, "NODE", browserNode, 1))
		r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
		return r
	}
	admitted := receipt(bundled, "v24.11.1", "v24.11.1")
	if admitted.Profile != ExternalProfile || ReceiptRuntimeTuple(admitted) != RuntimeTupleCandidate || ReceiptTestProjection(admitted, admitted.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("admitted bundled Node v24.11.1 tuple abstained")
	}
	for name, r := range map[string]Receipt{
		"neighbouring-patch": receipt(bundled, "v24.11.0", "v24.11.0"),
		"later-minor":        receipt(bundled, "v24.21.0", "v24.21.0"),
		"prefix-only":        receipt(bundled, "v24.11", "v24.11"),
		"identity-drift":     receipt(bundled, "v24.11.1", "v22.23.2"),
		"system-browser":     receipt(system, "v24.11.1", "v24.11.1"),
	} {
		t.Run(name, func(t *testing.T) {
			if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("unadmitted Node tuple projected green")
			}
		})
	}
	for _, profile := range []string{AttestedExternalProfile, SensitiveExternalProfile, AttemptExternalProfile} {
		r := admitted
		r.Profile = profile
		if ReceiptRuntimeTuple(r) != RuntimeTupleUnqualified {
			t.Fatalf("%s admitted Node v24.11.1 without its own live evidence", profile)
		}
	}
}

// PWP-V0-009 / LPCV-V0-056: the receipt-level classification is closed and
// fails closed when the Node version was not observed.
func TestReceiptRuntimeTupleClassification(t *testing.T) {
	for _, test := range []struct {
		profile, runner, node, want string
	}{
		{"", "1.63.0", "v24.11.1", RuntimeTupleNotApplicable},
		{ExternalProfile, "1.63.0", "v22.23.2", RuntimeTupleCandidate},
		{AttemptExternalProfile, "1.63.0", "v22.23.2", RuntimeTupleCandidate},
		{ExternalProfile, "1.63.0", "v24.11.1", RuntimeTupleCandidate},
		{ExternalProfile, "1.63.0", "v22.23.3", RuntimeTupleUnqualified},
		{ExternalProfile, "1.63.0", "", RuntimeTupleUnobserved},
		{ExternalProfile, "1.60.0", "", RuntimeTupleUnobserved},
		{ExternalProfile, "1.60.0", "v24.11.1", RuntimeTupleCandidate},
		{AttemptExternalProfile, "1.60.0", "v22.23.2", RuntimeTupleUnqualified},
		{ExternalProfile, "1.64.0", "v22.23.2", RuntimeTupleCandidate},
		{AttemptExternalProfile, "1.64.0", "v22.23.2", RuntimeTupleCandidate},
		{AttestedExternalProfile, "1.64.0", "v22.23.2", RuntimeTupleCandidate},
		{SensitiveExternalProfile, "1.64.0", "v22.23.2", RuntimeTupleCandidate},
		{ExternalProfile, "1.64.0", "v24.11.1", RuntimeTupleCandidate},
		{AttemptExternalProfile, "1.64.0", "v24.11.1", RuntimeTupleCandidate},
		{AttestedExternalProfile, "1.64.0", "v24.11.1", RuntimeTupleUnqualified},
		{SensitiveExternalProfile, "1.64.0", "v24.11.1", RuntimeTupleUnqualified},
		{ExternalProfile, "1.64.0", "v22.23.3", RuntimeTupleUnqualified},
		{ExternalProfile, "1.65.0", "v22.23.2", RuntimeTupleUnqualified},
	} {
		r := Receipt{Profile: test.profile, Identity: Identity{RunnerVersion: test.runner, NodeVersion: test.node}}
		if got := ReceiptRuntimeTuple(r); got != test.want {
			t.Fatalf("%+v: got %q want %q", test, got, test.want)
		}
	}
}

// playwright164Bundled is the bundled headless-shell identity the Playwright
// 1.64.0 live matrix observed (PWP-V0-032).
const playwright164Bundled = `{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"NODE","browserType":"chromium","browserVersion":"Google Chrome for Testing 156.0.8078.4","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1248/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634","browserRevision":"1248","manifestBrowserVersion":"156.0.8078.4","headlessShellAvailable":true}}`

func playwrightTupleReceipt(t *testing.T, profile, runner, node, use string) Receipt {
	t.Helper()
	r := qualifiedFixture(t)
	r.Profile = profile
	r.Identity.RunnerVersion = runner
	r.Identity.NodeVersion = node
	r.Tests[0].Project.Use = json.RawMessage(strings.ReplaceAll(use, "NODE", node))
	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	return r
}

// PWP-V0-032: 1.64.0 is admitted and its bundled headless-shell tuple is exact
// and bound to its own runner version; the 1.63.0 tuple is not inherited in
// either direction, and the 1.63.0 system-browser version is not inherited by
// 1.64.0 (PWP-V0-037 admits only the version 1.64.0 ran live with).
func TestPWPV0032Playwright164BundledTupleIsExact(t *testing.T) {
	base := E2EConfig{Config: Config{ConfigFile: "config.cjs", RunnerVersion: "1.64.0", TestFiles: []string{"test.cjs"}}, ExternalServer: true, AppIdentity: "fixture", ServerReadyURL: "http://127.0.0.1:3002"}
	if err := admitExternal(base); err != nil {
		t.Fatalf("1.64.0 refused: %v", err)
	}
	for _, version := range []string{"1.64", "1.64.1", "1.65.0", "v1.64.0", "^1.64.0"} {
		base.RunnerVersion = version
		if admitExternal(base) == nil {
			t.Fatalf("%s admitted", version)
		}
	}
	admitted := playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", playwright164Bundled)
	if ReceiptTestProjection(admitted, admitted.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("qualified bundled Playwright 1.64 browser tuple abstained")
	}
	attempt := playwrightTupleReceipt(t, AttemptExternalProfile, "1.64.0", "v22.23.2", playwright164Bundled)
	if !qualifiedPlaywrightTuple(attempt, attempt.Tests[0]) {
		t.Fatal("qualified /3 Playwright 1.64 browser tuple abstained")
	}
	bundled163 := strings.NewReplacer("156.0.8078.4", "153.0.8010.12", "1248", "1243", "77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634", "a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282").Replace(playwright164Bundled)
	system := `{"browserName":"chromium","channel":"","launchOptions":{"executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"NODE","browserType":"chromium","browserVersion":"Google Chrome 153.0.8010.48","channel":"","executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome","headlessShellAvailable":true}}`
	if r := playwrightTupleReceipt(t, ExternalProfile, "1.63.0", "v22.23.2", bundled163); ReceiptTestProjection(r, r.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("1.63.0 bundled tuple regressed")
	}
	for name, r := range map[string]Receipt{
		"1.63-browser-on-1.64-runner": playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", bundled163),
		"1.64-browser-on-1.63-runner": playwrightTupleReceipt(t, ExternalProfile, "1.63.0", "v22.23.2", playwright164Bundled),
		"1.63-system-browser-on-1.64": playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", system),
		"revision-drift":              playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", strings.Replace(playwright164Bundled, `"browserRevision":"1248"`, `"browserRevision":"1249"`, 1)),
		"digest-drift":                playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", strings.Replace(playwright164Bundled, "77d8278b", "87d8278b", 1)),
		"manifest-drift":              playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", strings.Replace(playwright164Bundled, `"manifestBrowserVersion":"156.0.8078.4"`, `"manifestBrowserVersion":"156.0.8078.5"`, 1)),
		"path-drift":                  playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", strings.Replace(playwright164Bundled, "chromium_headless_shell-1248", "chromium_headless_shell-1247", 1)),
		"headed":                      playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", strings.Replace(playwright164Bundled, `"headless":true`, `"headless":false`, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("unqualified Playwright 1.64 tuple projected green")
			}
		})
	}
}

// PWP-V0-033: 1.64.0 is a candidate for the base and attempt-retaining
// profiles under Node v22.23.2; PWP-V0-035 adds /1 and /2 there and PWP-V0-036
// adds /0 and /3 under Node v24.11.1. Every other profile and Node abstains.
func TestPWPV0033Playwright164ProfilesAndNodeAreExact(t *testing.T) {
	for _, test := range []struct {
		profile, node string
		want          bool
	}{
		{ExternalProfile, "v22.23.2", true},
		{AttemptExternalProfile, "v22.23.2", true},
		{ExternalProfile, "v24.11.1", true},
		{AttemptExternalProfile, "v24.11.1", true},
		{ExternalProfile, "v22.23.3", false},
		{ExternalProfile, "v24.11.0", false},
		{AttestedExternalProfile, "v24.11.1", false},
		{SensitiveExternalProfile, "v24.11.1", false},
	} {
		r := playwrightTupleReceipt(t, test.profile, "1.64.0", test.node, playwright164Bundled)
		if got := qualifiedPlaywrightTuple(r, r.Tests[0]); got != test.want {
			t.Fatalf("%s on %s: qualified=%v want %v", test.profile, test.node, got, test.want)
		}
	}
}

// PWP-V0-035: /1 and /2 are candidates on 1.64.0 only under Node v22.23.2,
// the release their live runs used; /2 still never projects passing.
func TestPWPV0035Playwright164AttestedAndSensitiveTuples(t *testing.T) {
	for _, profile := range []string{AttestedExternalProfile, SensitiveExternalProfile} {
		for node, want := range map[string]bool{"v22.23.2": true, "v24.11.1": false, "v22.23.3": false, "v22.23": false} {
			r := playwrightTupleReceipt(t, profile, "1.64.0", node, playwright164Bundled)
			if got := qualifiedPlaywrightTuple(r, r.Tests[0]); got != want {
				t.Fatalf("%s on Node %s: qualified=%v want %v", profile, node, got, want)
			}
		}
	}
	r := playwrightTupleReceipt(t, SensitiveExternalProfile, "1.64.0", "v22.23.2", playwright164Bundled)
	if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("/2 projected passing on 1.64.0 before PWP-V2-006 admits it")
	}
}

// PWP-V0-036: Node v24.11.1 is admitted on 1.64.0 for the base and
// attempt-retaining profiles' bundled tuple only; neighbouring releases and
// identity drift keep abstaining.
func TestPWPV0036Playwright164Node24TupleIsExact(t *testing.T) {
	admitted := playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v24.11.1", playwright164Bundled)
	if ReceiptTestProjection(admitted, admitted.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("admitted 1.64.0 Node v24.11.1 bundled tuple abstained")
	}
	if attempt := playwrightTupleReceipt(t, AttemptExternalProfile, "1.64.0", "v24.11.1", playwright164Bundled); !qualifiedPlaywrightTuple(attempt, attempt.Tests[0]) {
		t.Fatal("admitted 1.64.0 Node v24.11.1 /3 tuple abstained")
	}
	drift := playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v24.11.1", playwright164Bundled)
	drift.Tests[0].Project.Use = json.RawMessage(strings.ReplaceAll(playwright164Bundled, "NODE", "v22.23.2"))
	drift.Tests[0].ID = qualifiedTestID(drift.Identity, drift.Tests[0])
	for name, r := range map[string]Receipt{
		"neighbouring-patch": playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v24.11.0", playwright164Bundled),
		"later-minor":        playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v24.12.0", playwright164Bundled),
		"identity-drift":     drift,
	} {
		t.Run(name, func(t *testing.T) {
			if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("unadmitted 1.64.0 Node tuple projected green")
			}
		})
	}
}

// PWP-V0-037: the 1.64.0 system-Chrome tuple is the exact version it ran live
// with, headless on /0 under Node v22.23.2 only; versions never cross runner releases.
func TestPWPV0037Playwright164SystemChromeIsExact(t *testing.T) {
	system := func(version string) string {
		return strings.Replace(`{"browserName":"chromium","channel":"","launchOptions":{"executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"NODE","browserType":"chromium","browserVersion":"VERSION","channel":"","executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome","headlessShellAvailable":true}}`, "VERSION", version, 1)
	}
	chrome155 := strings.Replace(system("Google Chrome 155.0.8059.85"), `"channel":"",`, `"channel":"","headless":true,`, 1)
	admitted := playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", chrome155)
	if ReceiptTestProjection(admitted, admitted.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("qualified 1.64.0 system-Chrome tuple abstained")
	}
	if r := playwrightTupleReceipt(t, ExternalProfile, "1.63.0", "v22.23.2", system("Google Chrome 153.0.8010.48")); ReceiptTestProjection(r, r.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("1.63.0 system-Chrome tuple regressed")
	}
	for name, r := range map[string]Receipt{
		"155-on-1.63":    playwrightTupleReceipt(t, ExternalProfile, "1.63.0", "v22.23.2", chrome155),
		"153-on-1.64":    playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", system("Google Chrome 153.0.8010.48")),
		"other-patch":    playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", system("Google Chrome 155.0.8059.86")),
		"attempts":       playwrightTupleReceipt(t, AttemptExternalProfile, "1.64.0", "v22.23.2", chrome155),
		"attested":       playwrightTupleReceipt(t, AttestedExternalProfile, "1.64.0", "v22.23.2", chrome155),
		"headed":         playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", strings.Replace(chrome155, `"headless":true`, `"headless":false`, 1)),
		"headless-unset": playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", system("Google Chrome 155.0.8059.85")),
		"node24":         playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v24.11.1", chrome155),
		"other-binary":   playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", strings.ReplaceAll(chrome155, "Google Chrome.app/Contents/MacOS/Google Chrome", "Chromium.app/Contents/MacOS/Chromium")),
		"unconfigured":   playwrightTupleReceipt(t, ExternalProfile, "1.64.0", "v22.23.2", strings.Replace(chrome155, `"launchOptions":{"executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}`, `"launchOptions":{}`, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if qualifiedPlaywrightTuple(r, r.Tests[0]) {
				t.Fatal("unqualified system-Chrome tuple admitted")
			}
		})
	}
}

// PWP-V0-034: every pre-run e2e refusal is typed, keeps its earlier error
// text, and encodes one canonical document naming the qualified versions.
func TestPWPV0034PreRunRefusalsAreTyped(t *testing.T) {
	base := E2EConfig{Config: Config{ConfigFile: "config.cjs", RunnerVersion: "1.62.0", TestFiles: []string{"test.cjs"}}, ExternalServer: true, AppIdentity: "fixture", ServerReadyURL: "http://127.0.0.1:3002"}
	_, err := RunE2E(context.Background(), base)
	var refusal *E2ERefusal
	if !errors.As(err, &refusal) || refusal.Code != "external-playwright-version-unqualified" {
		t.Fatalf("version refusal untyped: %v", err)
	}
	if err.Error() != "external-playwright-version-unqualified: qualified runner versions: 1.60.0, 1.63.0, 1.64.0" {
		t.Fatalf("message %q", err.Error())
	}
	want := `{"schema":"corvint-playwright-e2e-refusal/0","code":"external-playwright-version-unqualified","message":"external-playwright-version-unqualified: qualified runner versions: 1.60.0, 1.63.0, 1.64.0","qualifiedRunnerVersions":["1.60.0","1.63.0","1.64.0"],"runtimeTuple":{"profile":"corvint-playwright-external/0","field":"runner","admitted":["1.60.0","1.63.0","1.64.0"]}}` + "\n"
	if got := string(EncodeE2ERefusal(refusal)); got != want {
		t.Fatalf("document\n got %s\nwant %s", got, want)
	}
	for code, mutate := range map[string]func(*E2EConfig){
		"external-server-command-forbidden":                  func(c *E2EConfig) { c.RunnerVersion = "1.64.0"; c.ServerArgv = []string{"server"} },
		"external-readiness-url-required":                    func(c *E2EConfig) { c.RunnerVersion = "1.64.0"; c.ServerReadyURL = "" },
		"external-app-identity-required":                     func(c *E2EConfig) { c.RunnerVersion = "1.64.0"; c.AppIdentity = "" },
		"external-unsupported-option":                        func(c *E2EConfig) { c.RunnerVersion = "1.64.0"; c.TestArgv = []string{"--ui"} },
		"keep-reporters-unsupported-mode":                    func(c *E2EConfig) { c.ExternalServer = false; c.KeepReporters = true },
		"sensitive-input-redaction-requires-external-server": func(c *E2EConfig) { c.ExternalServer = false; c.SensitiveInputPolicy = &SensitiveInputPolicy{} },
	} {
		c := base
		mutate(&c)
		_, err := RunE2E(context.Background(), c)
		if !errors.As(err, &refusal) || refusal.Code != code || !strings.HasPrefix(err.Error(), code) {
			t.Fatalf("%s: %v", code, err)
		}
	}
}
