package companionrelease

import (
	"bytes"
	"strings"
	"testing"
)

// TestCEMV1613GateOutcomeIsExact pins the cem-release-gate-outcome/1 reader:
// the canonical READY and REFUSED encodings round-trip, and every other shape
// is refused with the gate-outcome class.
func TestCEMV1613GateOutcomeIsExact(t *testing.T) {
	ready := StableGateOutcome{Profile: StableGateOutcomeProfile, Revision: strings.Repeat("a", 40), DeclarationSha256: strings.Repeat("b", 64), Verdict: StableGateReady}
	raw, err := EncodeStableGateOutcome(ready)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"profile":"cem-release-gate-outcome/1","revision":"` + strings.Repeat("a", 40) + `","declarationSha256":"` + strings.Repeat("b", 64) + `","records":[],"verdict":"READY","reason":""}` + "\n"
	if string(raw) != want {
		t.Fatalf("encoding = %s, want %s", raw, want)
	}
	if got, err := ReadStableGateOutcome(raw); err != nil || got.Verdict != StableGateReady || len(got.Records) != 0 {
		t.Fatalf("read = %+v, %v", got, err)
	}
	refused := ready
	refused.Records, refused.Verdict, refused.Reason = []string{strings.Repeat("c", 64), strings.Repeat("d", 64)}, StableGateRefused, "release-external-incomplete: x"
	if raw, err = EncodeStableGateOutcome(refused); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadStableGateOutcome(raw); err != nil || got.Verdict != StableGateRefused || len(got.Records) != 2 {
		t.Fatalf("read refused = %+v, %v", got, err)
	}
	for name, mutate := range map[string]func(*StableGateOutcome){
		"profile":         func(o *StableGateOutcome) { o.Profile = "cem-release-gate-outcome/0" },
		"short revision":  func(o *StableGateOutcome) { o.Revision = "abc" },
		"upper digest":    func(o *StableGateOutcome) { o.DeclarationSha256 = strings.Repeat("B", 64) },
		"repeated record": func(o *StableGateOutcome) { o.Records = []string{strings.Repeat("c", 64), strings.Repeat("c", 64)} },
		"ready reason":    func(o *StableGateOutcome) { o.Verdict, o.Reason = StableGateReady, "x" },
		"silent refusal":  func(o *StableGateOutcome) { o.Reason = "" },
		"other verdict":   func(o *StableGateOutcome) { o.Verdict = "PASSED" },
	} {
		bad := refused
		mutate(&bad)
		if _, err := EncodeStableGateOutcome(bad); err == nil {
			t.Fatalf("%s outcome was encoded", name)
		}
	}
	trimmed := bytes.TrimSuffix(raw, []byte("\n"))
	for name, bad := range map[string][]byte{
		"reordered":   []byte(strings.Replace(string(raw), `{"profile":"cem-release-gate-outcome/1","revision":`, `{"revision":`, 1)),
		"extra":       append(append([]byte{}, trimmed[:len(trimmed)-1]...), []byte(`,"x":1}`+"\n")...),
		"null record": []byte(strings.Replace(string(raw), `"records":[`+`"`+strings.Repeat("c", 64)+`","`+strings.Repeat("d", 64)+`"]`, `"records":null`, 1)),
		"no newline":  trimmed,
		"spaced":      append(bytes.ReplaceAll(trimmed, []byte(`,"`), []byte(`, "`)), '\n'),
		"empty":       nil,
	} {
		_, err := ReadStableGateOutcome(bad)
		if refusal, ok := err.(*StableReleaseRefusal); !ok || refusal.Class != StableRefusalGateOutcome {
			t.Fatalf("%s outcome: %v, want a %s refusal", name, err, StableRefusalGateOutcome)
		}
	}
}
