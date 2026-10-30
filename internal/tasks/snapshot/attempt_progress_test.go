package snapshot

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
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

// ATR-V0-084: the latest recorded hand-off reference is the current
// generation's, else the newest prior generation's recorded fingerprint,
// skipping generations that recorded none or were recorded without it.
func TestATRV0084_LatestHandoffEvidence(t *testing.T) {
	t.Run("ATR-V0-084 LatestHandoffEvidence", func(t *testing.T) {
		gen := func(ref string, signal bool) PriorGeneration {
			e := &LoopEvidence{Disposition: wire.CodeHandoff, GateResults: "0", Reviews: "0"}
			if signal {
				e.Progress = &LoopProgress{Signal: intent.ProgressSignalHandoffEvidence}
				if ref != "" {
					r := ref
					e.Progress.Fingerprint = &r
				}
			}
			return PriorGeneration{History: &GenerationHistory{Loop: e}}
		}
		cases := []struct {
			current string
			prior   []PriorGeneration
			want    string
		}{
			{"", nil, ""},
			{"local:c", []PriorGeneration{gen("local:a", true)}, "local:c"},
			{"", []PriorGeneration{gen("local:a", true), gen("", true)}, "local:a"},
			{"", []PriorGeneration{gen("local:a", true), gen("local:b", true), gen("", false), {}}, "local:b"},
			{"", []PriorGeneration{gen("", true), gen("", false)}, ""},
		}
		for i, c := range cases {
			if got := LatestHandoffEvidence(&Attempt{HandoffEvidence: c.current, PriorGenerations: c.prior}); got != c.want {
				t.Fatalf("case %d: %q want %q", i, got, c.want)
			}
		}
	})
}

// ATR-V0-193 (V1-1692): a claim recorded while the policy declares no
// progressSignal writes a loopEvidence member with exactly the key set a
// build before the ATR-V0-081 progress member (PR #1085) accepts, so a binary
// rollback stays readable; a claim recorded under the signal writes the
// `progress` member that such a build refuses as MALFORMED (closed object,
// recorded experiment in the V1-1692 build-log entry), whose only supported
// recovery is a build that reads it.
func TestATRV0193_RollbackReadableWithoutProgressSignal(t *testing.T) {
	t.Run("ATR-V0-193 RollbackReadableWithoutProgressSignal", func(t *testing.T) {
		preProgress := []string{"candidateTreeOid", "disposition", "gateResults", "reviews"}
		ended := &Attempt{RuntimeID: RuntimeExternalAgent, RetryAccounting: &RetryAccounting{Disposition: wire.CodeHandoff}, HandoffEvidence: "local:wp-step-3"}
		keys := func(signal string) []string {
			a := legacyPriorAttempt()
			a.PriorGenerations[1].History = &GenerationHistory{Stage: ptr("implement"), Loop: LoopEvidenceWith(ended, signal)}
			raw, err := a.Encode()
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				PriorGenerations []struct {
					LoopEvidence map[string]json.RawMessage `json:"loopEvidence"`
				} `json:"priorGenerations"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			var out []string
			for k := range doc.PriorGenerations[1].LoopEvidence {
				out = append(out, k)
			}
			sort.Strings(out)
			return out
		}
		if got := keys(""); !reflect.DeepEqual(got, preProgress) {
			t.Fatalf("undeclared signal wrote loopEvidence keys %v, want the pre-#1085 set %v", got, preProgress)
		}
		if got := keys(intent.ProgressSignalHandoffEvidence); !reflect.DeepEqual(got, append(append([]string(nil), preProgress[:3]...), "progress", "reviews")) {
			t.Fatalf("declared signal wrote loopEvidence keys %v", got)
		}
	})
}
