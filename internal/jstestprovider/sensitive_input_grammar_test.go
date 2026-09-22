package jstestprovider

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSensitiveInputPolicyGrammarAgreement(t *testing.T) {
	t.Run("PWP-V2-004 policy grammar agreement", func(t *testing.T) {
		for _, pattern := range []string{"custom+entry", "custom💠entry", "custom\u200bentry", "custom\u00a0entry"} {
			r := Receipt{Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{AdditionalActionPatterns: []string{pattern}}, Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{{Title: pattern + " unquoted-secret"}}}}}}}
			if len(ValidateSensitiveInputEvidence(r)) == 0 {
				t.Errorf("admitted action is not recognized: %q", pattern)
			}
			safe, err := RedactSensitiveInputEvidence(r)
			if err != nil {
				t.Fatal(err)
			}
			data, err := EncodeQualified(safe)
			if err != nil || strings.Contains(string(data), "unquoted-secret") || safe.Tests[0].Attempts[0].Steps[0].Title != `custom entry "[REDACTED]"` {
				t.Errorf("pattern mismatch %q: %v %s", pattern, err, data)
			}
		}
		for _, pattern := range []string{"***", "+💠\u200b", "", strings.Repeat("x", 129)} {
			r := Receipt{Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{AdditionalActionPatterns: []string{pattern}}}
			findings := ValidateSensitiveInputEvidence(r)
			if len(findings) != 1 || findings[0].Code != "sensitive-input-policy-invalid" {
				t.Errorf("invalid policy admitted: %+v", findings)
			}
			if _, err := EncodeQualified(r); err == nil {
				t.Error("invalid policy encoded")
			}
			if _, err := RedactSensitiveInputEvidence(r); err == nil || err.Error() != "sensitive-input-policy-invalid" {
				t.Errorf("invalid policy error: %v", err)
			}
		}
	})
}

func TestSensitiveInputUnsupportedReceiverSyntaxRejected(t *testing.T) {
	for _, title := range []string{`page.getByLabel("Password").fill("unquoted-secret")`, `page.locator("#password").first().type("unquoted-secret")`, `page["getByLabel"]("Password")["fill"]("unquoted-secret")`, `page.locator("#password").custom+entry("unquoted-secret")`} {
		r := Receipt{Profile: SensitiveExternalProfile, SensitiveInputPolicy: &SensitiveInputPolicy{AdditionalActionPatterns: []string{"custom+entry"}}, Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{{Title: title}}}}}}}
		findings := ValidateSensitiveInputEvidence(r)
		if len(findings) == 0 || findings[0].Code != "sensitive-input-action-syntax-unsupported" {
			t.Errorf("unsupported action admitted: %q %+v", title, findings)
		}
		for _, encode := range []func(Receipt) error{func(r Receipt) error { _, err := EncodeQualified(r); return err }, func(r Receipt) error { _, err := RedactSensitiveInputEvidence(r); return err }} {
			err := encode(r)
			var rejection *SensitiveInputValidationError
			if !errors.As(err, &rejection) || strings.Contains(err.Error(), "unquoted-secret") {
				t.Errorf("not a value-free rejection: %v", err)
			}
		}
		data, _ := json.Marshal(findings)
		if strings.Contains(string(data), "Password") || strings.Contains(string(data), "unquoted-secret") {
			t.Error("finding echoed input")
		}
	}
	policy, _ := sensitivePolicy(&SensitiveInputPolicy{AdditionalActionPatterns: []string{"custom+entry"}})
	for _, title := range []string{`Expect page.getByLabel("Password").fill to fail`, `Navigate to page.getByLabel("Password").fill`, `page.goto("/fill")`, `page.getByText("fill").click()`, `Expect custom+entry to succeed`} {
		r := Receipt{Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{{Title: title}}}}}}}
		safe, err := RedactSensitiveInputEvidence(r)
		if err != nil || safe.Tests[0].Attempts[0].Steps[0].Title != title {
			t.Errorf("negative control changed: %q %v", title, err)
		}
		if display, _ := boundaryActionMatch(title, policy); display != "" {
			t.Errorf("negative control classified: %q", title)
		}
	}
}

func TestSensitiveInputArgumentCandidatesRespectStructure(t *testing.T) {
	policy, _ := sensitivePolicy(&SensitiveInputPolicy{AdditionalActionPatterns: []string{"custom entry"}})
	for _, test := range []struct {
		title               string
		required, forbidden []string
	}{
		{`Fill("#pass,word", "secret,with(paren)")`, []string{"#pass,word", "secret,with(paren)"}, []string{"secret", "with(paren)"}},
		{`Fill(select("#password", "label"), unquoted-secret)`, []string{`select("#password", "label")`, "unquoted-secret"}, []string{`select("#password"`}},
		{`custom entry "p\"ass"`, []string{`p"ass`}, nil},
		{`Type 'p\'ass'`, []string{"p'ass"}, nil},
		{`Fill "#password" with hunter2`, []string{"hunter2"}, nil},
	} {
		t.Run(test.title, func(t *testing.T) {
			r := Receipt{Tests: []TestOutcome{{Attempts: []Attempt{{Steps: []BrowserStep{{Title: test.title}}}}}}}
			values := map[string]bool{}
			for _, value := range boundaryCollectSensitiveValues(r, policy) {
				values[value] = true
			}
			for _, value := range test.required {
				if !values[value] {
					t.Errorf("missing argument candidate %q", value)
				}
			}
			for _, value := range test.forbidden {
				if values[value] {
					t.Errorf("split inside argument %q", value)
				}
			}
		})
	}
}
