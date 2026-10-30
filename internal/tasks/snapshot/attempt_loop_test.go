package snapshot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-102: an opted-in prior generation records its work evidence under
// loopEvidence; it round-trips canonically, legacy entries stay byte-identical
// (TestCALV0096_LegacyPriorGenerationsRoundTrip pins the N-1 sha256), and a
// malformed or contradictory member is refused.
func TestCALV0102_PriorLoopEvidenceRoundTrip(t *testing.T) {
	t.Run("CAL-V0-102 PriorLoopEvidenceRoundTrip", func(t *testing.T) {
		tree := strings.Repeat("c", 40)
		a := legacyPriorAttempt()
		a.Generation = "4"
		a.PriorGenerations = append(a.PriorGenerations, PriorGeneration{Generation: "3", Quiescence: "FENCED", ProvedSeq: "6", History: &GenerationHistory{Stage: ptr("review"), HandoffTo: "implement", HandoffReason: "CHANGES_REQUESTED", Loop: &LoopEvidence{Disposition: wire.CodeReviewReturned, CandidateTreeOid: &tree, GateResults: "1", Reviews: "2"}}})
		a.PriorGenerations[1].History = &GenerationHistory{Stage: ptr("implement"), Loop: &LoopEvidence{Disposition: wire.CodeHandoff, GateResults: "0", Reviews: "0"}}
		raw, err := a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`"loopEvidence":{"candidateTreeOid":null,"disposition":"HANDOFF","gateResults":"0","reviews":"0"}`, `"loopEvidence":{"candidateTreeOid":"` + tree + `","disposition":"REVIEW_RETURNED","gateResults":"1","reviews":"2"}`, `{"generation":"1","provedSeq":"2","quiescence":"FENCED"}`} {
			if !strings.Contains(string(raw), want) {
				t.Fatalf("missing %s in %s", want, raw)
			}
		}
		b, err := DecodeAttempt(raw)
		if err != nil {
			t.Fatal(err)
		}
		if b.PriorGenerations[0].History != nil || b.PriorGenerations[1].History.Loop == nil || b.PriorGenerations[2].History.Loop.Reviews != "2" || *b.PriorGenerations[2].History.Loop.CandidateTreeOid != tree {
			t.Fatalf("decoded %+v", b.PriorGenerations)
		}
		again, err := b.Encode()
		if err != nil || !bytes.Equal(raw, again) {
			t.Fatalf("bytes changed: %v", err)
		}
		a.PriorGenerations[1].History.Loop = nil
		plain, err := a.Encode()
		if err != nil || strings.Count(string(plain), `"loopEvidence"`) != 1 {
			t.Fatalf("absent evidence wrote a key: %s %v", plain, err)
		}

		entry := `{"candidateTreeOid":null,"disposition":"HANDOFF","gateResults":"0","reviews":"0"}`
		for name, bad := range map[string][2]string{
			"no-history":     {`{"generation":"1","provedSeq":"2","quiescence":"FENCED"}`, `{"generation":"1","loopEvidence":` + entry + `,"provedSeq":"2","quiescence":"FENCED"}`},
			"unknown-key":    {entry, `{"candidateTreeOid":null,"disposition":"HANDOFF","extra":"1","gateResults":"0","reviews":"0"}`},
			"bad-disp":       {entry, `{"candidateTreeOid":null,"disposition":"GATE_FAILED","gateResults":"0","reviews":"0"}`},
			"bad-tree":       {entry, `{"candidateTreeOid":"xyz","disposition":"HANDOFF","gateResults":"0","reviews":"0"}`},
			"bad-count":      {entry, `{"candidateTreeOid":null,"disposition":"HANDOFF","gateResults":"-1","reviews":"0"}`},
			"missing-key":    {entry, `{"candidateTreeOid":null,"disposition":"HANDOFF","gateResults":"0"}`},
			"return-no-rev":  {entry, `{"candidateTreeOid":null,"disposition":"REVIEW_RETURNED","gateResults":"0","reviews":"0"}`},
			"target-no-disp": {`"disposition":"REVIEW_RETURNED","gateResults":"1"`, `"disposition":"NONE","gateResults":"1"`},
		} {
			mutated := bytes.Replace(raw, []byte(bad[0]), []byte(bad[1]), 1)
			if bytes.Equal(mutated, raw) {
				t.Fatalf("%s: fixture did not apply", name)
			}
			if _, err := DecodeAttempt(mutated); err == nil {
				t.Fatalf("%s: accepted %s", name, bad[1])
			}
		}
	})
}

// CAL-V0-102: LoopEvidenceOf copies only an ended external-agent generation
// with retry accounting, without aliasing its tree.
func TestCALV0102_LoopEvidenceOf(t *testing.T) {
	t.Run("CAL-V0-102 LoopEvidenceOf", func(t *testing.T) {
		if LoopEvidenceOf(nil) != nil {
			t.Fatal("nil attempt")
		}
		a := handedOff("implement", wire.CodeHandoff)
		a.Reviews = nil
		e := LoopEvidenceOf(a)
		if e == nil || e.Disposition != wire.CodeHandoff || e.CandidateTreeOid == nil || *e.CandidateTreeOid != *a.CandidateTreeOid || e.GateResults.Int() != int64(len(a.GateResults)) || e.Reviews != "0" {
			t.Fatalf("evidence %+v", e)
		}
		*a.CandidateTreeOid = strings.Repeat("d", 40)
		if *e.CandidateTreeOid == *a.CandidateTreeOid {
			t.Fatal("evidence aliases the attempt tree")
		}
		a.RetryAccounting = nil
		if LoopEvidenceOf(a) != nil {
			t.Fatal("no accounting is UNKNOWN")
		}
		a = handedOff("implement", wire.CodeHandoff)
		a.RuntimeID = "supervisor"
		if LoopEvidenceOf(a) != nil {
			t.Fatal("supervised generation is UNKNOWN")
		}
	})
}
