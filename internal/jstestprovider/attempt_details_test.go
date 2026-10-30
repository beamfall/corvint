package jstestprovider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func attemptTestOutcome() TestOutcome {
	anchor := &Anchor{File: "/fixture/test.cjs", Line: 8}
	return TestOutcome{Name: "retry", FullName: "retry", State: StateFlaky, Retries: 1, DurationMS: 4, Anchor: anchor,
		Attempts:       []Attempt{{State: StateFailed, Retry: 0}, {State: StatePassed, Retry: 1}},
		AttemptDetails: []AttemptDetail{{State: StateFailed, Retry: 0, DurationMS: 2, FailureMessage: "first failure", Anchor: anchor}, {State: StatePassed, Retry: 1, DurationMS: 4, Anchor: anchor}}}
}

func TestPWPV3AttemptInventory(t *testing.T) {
	t.Run("PWP-V3-002 every attempt matches its inventory and last summary", func(t *testing.T) {
		if err := ValidateAttemptDetails([]TestOutcome{attemptTestOutcome()}); err != nil {
			t.Fatal(err)
		}
	})
	for _, tc := range []struct {
		name   string
		mutate func(*TestOutcome)
	}{
		{"missing", func(o *TestOutcome) { o.AttemptDetails = o.AttemptDetails[1:] }},
		{"duplicate", func(o *TestOutcome) { o.AttemptDetails[1].Retry = 0 }},
		{"reordered", func(o *TestOutcome) {
			o.AttemptDetails[0], o.AttemptDetails[1] = o.AttemptDetails[1], o.AttemptDetails[0]
		}},
		{"duration", func(o *TestOutcome) { o.AttemptDetails[1].DurationMS++ }},
		{"clean pass", func(o *TestOutcome) { o.State = StatePassed }},
		{"failure", func(o *TestOutcome) { o.AttemptDetails[1].FailureMessage = "hidden" }},
		{"infrastructure", func(o *TestOutcome) { o.AttemptDetails[0].State = StateInfrastructure }},
		{"unknown state", func(o *TestOutcome) { o.AttemptDetails[0].State = "unknown"; o.Attempts[0].State = "unknown" }},
	} {
		t.Run("PWP-V3-002 refuses "+tc.name, func(t *testing.T) {
			o := attemptTestOutcome()
			tc.mutate(&o)
			if ValidateAttemptDetails([]TestOutcome{o}) == nil {
				t.Fatal("contradictory detail accepted")
			}
		})
	}
}

// PWP-V3-008: a failed attempt may name its error location; a passed attempt, an empty file or a
// non-positive line refuses the receipt.
func TestPWPV3008FailureLocation(t *testing.T) {
	o := attemptTestOutcome()
	o.AttemptDetails[0].FailureLocation = &Anchor{File: "/fixture/test.cjs", Line: 12}
	if err := ValidateAttemptDetails([]TestOutcome{o}); err != nil {
		t.Fatalf("located failure refused: %v", err)
	}
	data, err := json.Marshal(o.AttemptDetails[0])
	if err != nil || !strings.Contains(string(data), `"failureLocation":{"file":"/fixture/test.cjs","line":12}`) {
		t.Fatalf("wire member = %s, %v", data, err)
	}
	for name, mutate := range map[string]func(*TestOutcome){
		"passed attempt": func(o *TestOutcome) {
			o.AttemptDetails[1].FailureLocation = &Anchor{File: "/fixture/test.cjs", Line: 3}
		},
		"empty file": func(o *TestOutcome) { o.AttemptDetails[0].FailureLocation = &Anchor{Line: 3} },
		"zero line":  func(o *TestOutcome) { o.AttemptDetails[0].FailureLocation = &Anchor{File: "/fixture/test.cjs"} },
	} {
		o := attemptTestOutcome()
		mutate(&o)
		if ValidateAttemptDetails([]TestOutcome{o}) == nil {
			t.Fatalf("%s: invalid failure location accepted", name)
		}
	}
}

func TestPWPV3ProfileBoundaries(t *testing.T) {
	o := attemptTestOutcome()
	t.Run("PWP-V3-001 earlier profiles still refuse the new field", func(t *testing.T) {
		for _, profile := range []string{ExternalProfile, AttestedExternalProfile, SensitiveExternalProfile} {
			if _, err := EncodeQualified(Receipt{Profile: profile, Tests: []TestOutcome{o}}); err == nil || err.Error() != "external-profile-has-attempt-details" {
				t.Fatalf("legacy profile %s: %v", profile, err)
			}
		}
	})
	t.Run("PWP-V3-004 unsupported composition refuses before execution", func(t *testing.T) {
		for _, cfg := range []E2EConfig{{RetainAttemptDetails: true}, {RetainAttemptDetails: true, ExternalServer: true, SensitiveInputPolicy: &SensitiveInputPolicy{}}, {RetainAttemptDetails: true, ExternalServer: true, ApplicationAttestation: &ApplicationAttestationProvider{}}} {
			if _, err := RunE2E(context.Background(), cfg); err == nil || err.Error() != "external-attempt-details-composition-unsupported" {
				t.Fatalf("composition: %v", err)
			}
		}
	})
	t.Run("PWP-V3-005 canonical consumer rejects invented projections", func(t *testing.T) {
		r := Receipt{Profile: AttemptExternalProfile, Kind: "e2e", Tests: []TestOutcome{o}}
		raw, err := EncodeQualified(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = DecodeAttemptReceipt(raw); err != nil {
			t.Fatal(err)
		}
		var changed map[string]json.RawMessage
		if err = json.Unmarshal(raw, &changed); err != nil {
			t.Fatal(err)
		}
		changed["runProjection"] = json.RawMessage(`{"outcome":"passed"}`)
		bad, _ := json.Marshal(changed)
		if _, err = DecodeAttemptReceipt(bad); err == nil {
			t.Fatal("invented projection accepted")
		}
	})
	t.Run("PWP-V3-004 earlier attempts receive the product secret screen", func(t *testing.T) {
		o.AttemptDetails[0].FailureMessage = "password=synthetic-credential-value"
		_, err := EncodeQualified(Receipt{Profile: AttemptExternalProfile, Kind: "e2e", Tests: []TestOutcome{o}})
		if err == nil || !strings.Contains(err.Error(), "secret") {
			t.Fatalf("secret accepted: %v", err)
		}
	})
}

// PWP-V3-009 (V1-2410): a `/3` report pairs one location (or null) with each run-level error; the
// receipt's reporter-global-error carries the first located one, and earlier profiles refuse the
// member.
func TestPWPV3009RunLevelErrorLocation(t *testing.T) {
	raw := `{"version":"1.64.0","files":{},"configFiles":{},"status":"failed","tests":[],"errors":["unlocated","teardown boom"],"errorLocations":[null,{"file":"/repo/e2e/a.spec.ts","line":4}]}`
	report, failure := decodeQualifiedReport([]byte(raw), AttemptExternalProfile)
	if failure != nil {
		t.Fatalf("/3 report refused: %+v", failure)
	}
	if got := firstErrorLocation(report); got == nil || *got != (Anchor{File: "/repo/e2e/a.spec.ts", Line: 4}) {
		t.Fatalf("first located error = %+v", got)
	}
	report.ErrorLocations = report.ErrorLocations[:1]
	if got := firstErrorLocation(report); got != nil {
		t.Fatalf("unpaired locations must not locate an error: %+v", got)
	}
	for _, profile := range []string{ExternalProfile, AttestedExternalProfile} {
		if _, failure := decodeQualifiedReport([]byte(raw), profile); failure == nil || failure.Reason != "report-unparseable" {
			t.Fatalf("%s accepted errorLocations: %+v", profile, failure)
		}
	}
	located := Receipt{Profile: AttemptExternalProfile, Infrastructure: &InfrastructureFailure{Reason: ReasonReporterGlobalError, Location: &Anchor{File: "/repo/a.spec.ts", Line: 4}}}
	if !locatedGlobalError(located) {
		t.Fatal("a located /3 run-level error must explain the exit status")
	}
	located.Profile = ExternalProfile
	if locatedGlobalError(located) {
		t.Fatal("only a /3 run-level error explains the exit status")
	}
}
