package jstestprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
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
