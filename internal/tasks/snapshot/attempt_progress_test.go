package snapshot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ATR-V0-081: a prior generation's loopEvidence may carry the declared
// progress signal and its fingerprint; it round-trips canonically, is absent
// without a signal, and a malformed member is refused.
func TestATRV0081_LoopEvidenceProgressRoundTrip(t *testing.T) {
	t.Run("ATR-V0-081 LoopEvidenceProgressRoundTrip", func(t *testing.T) {
		ref := "local:wp-step-3"
		a := legacyPriorAttempt()
		a.Generation = "4"
		a.PriorGenerations[1].History = &GenerationHistory{Stage: ptr("implement"), Loop: &LoopEvidence{Disposition: wire.CodeHandoff, GateResults: "0", Reviews: "0", Progress: &LoopProgress{Signal: intent.ProgressSignalHandoffEvidence, Fingerprint: &ref}}}
		a.PriorGenerations = append(a.PriorGenerations, PriorGeneration{Generation: "3", Quiescence: "FENCED", ProvedSeq: "6", History: &GenerationHistory{Stage: ptr("implement"), Loop: &LoopEvidence{Disposition: "NONE", GateResults: "0", Reviews: "0", Progress: &LoopProgress{Signal: intent.ProgressSignalHandoffEvidence}}}})
		raw, err := a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		withRef := `"progress":{"fingerprint":"local:wp-step-3","signal":"HANDOFF_EVIDENCE"}`
		for _, want := range []string{withRef, `"progress":{"fingerprint":null,"signal":"HANDOFF_EVIDENCE"}`} {
			if !strings.Contains(string(raw), want) {
				t.Fatalf("missing %s in %s", want, raw)
			}
		}
		b, err := DecodeAttempt(raw)
		if err != nil {
			t.Fatal(err)
		}
		if p := b.PriorGenerations[1].History.Loop.Progress; p == nil || p.Signal != intent.ProgressSignalHandoffEvidence || p.Fingerprint == nil || *p.Fingerprint != ref {
			t.Fatalf("decoded %+v", p)
		}
		if p := b.PriorGenerations[2].History.Loop.Progress; p == nil || p.Fingerprint != nil {
			t.Fatalf("decoded %+v", p)
		}
		if again, err := b.Encode(); err != nil || !bytes.Equal(raw, again) {
			t.Fatalf("bytes changed: %v", err)
		}
		for name, bad := range map[string]string{
			"unknown-signal":  `"progress":{"fingerprint":"local:wp-step-3","signal":"OBLIGATIONS"}`,
			"unknown-key":     `"progress":{"extra":"1","fingerprint":"local:wp-step-3","signal":"HANDOFF_EVIDENCE"}`,
			"missing-key":     `"progress":{"signal":"HANDOFF_EVIDENCE"}`,
			"bad-fingerprint": `"progress":{"fingerprint":"` + strings.Repeat("x", 129) + `","signal":"HANDOFF_EVIDENCE"}`,
			"null-progress":   `"progress":null`,
		} {
			mutated := bytes.Replace(raw, []byte(withRef), []byte(bad), 1)
			if _, err := DecodeAttempt(mutated); err == nil {
				t.Fatalf("%s: accepted %s", name, bad)
			}
		}
		// A fingerprint needs a hand-off disposition.
		mutated := bytes.Replace(raw, []byte(`"progress":{"fingerprint":null`), []byte(`"progress":{"fingerprint":"local:x"`), 1)
		if _, err := DecodeAttempt(mutated); err == nil {
			t.Fatal("a NONE generation accepted a fingerprint")
		}
	})
}

// ATR-V0-081: LoopEvidenceWith records the hand-off reference only under the
// declared signal; LoopEvidenceOf records no progress member.
func TestATRV0081_LoopEvidenceWith(t *testing.T) {
	t.Run("ATR-V0-081 LoopEvidenceWith", func(t *testing.T) {
		a := &Attempt{RuntimeID: RuntimeExternalAgent, RetryAccounting: &RetryAccounting{Disposition: wire.CodeHandoff}, HandoffEvidence: "local:a"}
		if e := LoopEvidenceOf(a); e == nil || e.Progress != nil {
			t.Fatalf("undeclared %+v", e)
		}
		if e := LoopEvidenceWith(a, intent.ProgressSignalHandoffEvidence); e == nil || e.Progress == nil || e.Progress.Fingerprint == nil || *e.Progress.Fingerprint != "local:a" {
			t.Fatalf("declared %+v", e)
		}
		a.HandoffEvidence = ""
		if e := LoopEvidenceWith(a, intent.ProgressSignalHandoffEvidence); e.Progress == nil || e.Progress.Fingerprint != nil {
			t.Fatalf("no reference %+v", e)
		}
	})
}
