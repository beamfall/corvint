package snapshot

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// handedOff is a clean external-agent terminal generation of stage released
// with disposition.
func handedOff(stage, disposition string) *Attempt {
	a := accountingAttempt()
	tree := strings.Repeat("b", 40)
	a.CandidateTreeOid, a.ScopeCheck, a.Cause = &tree, "WITHIN", &disposition
	a.Stage, a.Phase, a.Quiescence = stage, "CANCELLED", "FENCED"
	a.RetryAccounting = &RetryAccounting{Disposition: disposition}
	return a
}

func handoffStr(s string) *string { return &s }

// CAL-V0-082/084: a clean release that records no target writes no new key
// and re-encodes byte-identical; REVIEW_RETURNED still derives implement.
func TestCALV0082_UntargetedHandoffRoundTrip(t *testing.T) {
	t.Run("CAL-V0-082 UntargetedHandoffRoundTrip", func(t *testing.T) {
		for stage, disposition := range map[string]string{"implement": wire.CodeHandoff, "review": wire.CodeReviewReturned} {
			raw, err := handedOff(stage, disposition).Encode()
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte(`"handoffTo"`)) || bytes.Contains(raw, []byte(`"handoffReason"`)) {
				t.Fatalf("%s: untargeted release gained a key: %s", disposition, raw)
			}
			b, err := DecodeAttempt(raw)
			if err != nil {
				t.Fatal(err)
			}
			again, err := b.Encode()
			if err != nil || !bytes.Equal(raw, again) {
				t.Fatalf("%s: bytes changed: %v", disposition, err)
			}
			want := ""
			if disposition == wire.CodeReviewReturned {
				want = "implement"
			}
			if got := b.NextStage(); got != want {
				t.Fatalf("%s: next stage %q, want %q", disposition, got, want)
			}
		}
		if got := accountingAttempt().NextStage(); got != "" {
			t.Fatalf("live generation derived %q", got)
		}
	})
}

// CAL-V0-082/083: a recorded target and reason round-trip canonically on the
// terminal generation and become its next stage.
func TestCALV0082_RecordedHandoffRoundTrip(t *testing.T) {
	t.Run("CAL-V0-082 RecordedHandoffRoundTrip", func(t *testing.T) {
		for _, c := range []struct{ stage, disposition, to, reason string }{
			{"implement", wire.CodeHandoff, "review", "STAGE_COMPLETE"},
			{"implement", wire.CodeHandoff, "implement", "STAGE_INCOMPLETE"},
			{"integrate", wire.CodeHandoff, "implement", "CHANGES_REQUESTED"},
			{"review", wire.CodeHandoff, "integrate", ""},
			{"review", wire.CodeReviewReturned, "implement", "CHANGES_REQUESTED"},
		} {
			a := handedOff(c.stage, c.disposition)
			a.HandoffTo, a.HandoffReason = c.to, c.reason
			raw, err := a.Encode()
			if err != nil {
				t.Fatalf("%+v: %v", c, err)
			}
			if !bytes.Contains(raw, []byte(`"handoffTo":"`+c.to+`"`)) || bytes.Contains(raw, []byte(`"handoffReason"`)) != (c.reason != "") {
				t.Fatalf("%+v: encoding %s", c, raw)
			}
			b, err := DecodeAttempt(raw)
			if err != nil {
				t.Fatal(err)
			}
			if b.HandoffTo != c.to || b.HandoffReason != c.reason || b.NextStage() != c.to {
				t.Fatalf("%+v: decoded %q %q next %q", c, b.HandoffTo, b.HandoffReason, b.NextStage())
			}
			again, err := b.Encode()
			if err != nil || !bytes.Equal(raw, again) {
				t.Fatalf("%+v: bytes changed: %v", c, err)
			}
		}
	})
}

// CAL-V0-082/083: every other combination is MALFORMED, whether built in
// memory or read from bytes.
func TestCALV0082_MalformedHandoffTargets(t *testing.T) {
	t.Run("CAL-V0-082 MalformedHandoffTargets", func(t *testing.T) {
		for name, a := range map[string]*Attempt{
			"live": accountingAttempt(),
			"not-clean": func() *Attempt {
				a := handedOff("implement", wire.CodeHandoff)
				a.RetryAccounting.FailedOrUnknown = true
				return a
			}(),
			"review-returned-to-review": handedOff("review", wire.CodeReviewReturned),
			"unknown-stage":             handedOff("implement", wire.CodeHandoff),
			"reason-without-target":     handedOff("implement", wire.CodeHandoff),
			"changes-from-implement":    handedOff("implement", wire.CodeHandoff),
			"incomplete-to-other-stage": handedOff("implement", wire.CodeHandoff),
			"complete-to-same-stage":    handedOff("review", wire.CodeHandoff),
			"changes-not-to-implement":  handedOff("review", wire.CodeHandoff),
			"unknown-reason":            handedOff("implement", wire.CodeHandoff),
			"review-returned-integrate": handedOff("review", wire.CodeReviewReturned),
		} {
			a.HandoffTo = map[string]string{"live": "review", "not-clean": "review", "review-returned-to-review": "review", "unknown-stage": "deploy", "changes-from-implement": "implement", "incomplete-to-other-stage": "review", "complete-to-same-stage": "review", "changes-not-to-implement": "integrate", "unknown-reason": "review", "review-returned-integrate": "integrate"}[name]
			a.HandoffReason = map[string]string{"reason-without-target": "STAGE_COMPLETE", "changes-from-implement": "CHANGES_REQUESTED", "incomplete-to-other-stage": "STAGE_INCOMPLETE", "complete-to-same-stage": "STAGE_COMPLETE", "changes-not-to-implement": "CHANGES_REQUESTED", "unknown-reason": "DONE"}[name]
			if _, err := a.Encode(); wire.CodeOf(err) != wire.CodeMalformed {
				t.Fatalf("%s: encoded or wrong refusal: %v", name, err)
			}
		}
		good := handedOff("implement", wire.CodeHandoff)
		good.HandoffTo = "review"
		raw, err := good.Encode()
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range [][]byte{
			bytes.Replace(raw, []byte(`"handoffTo":"review"`), []byte(`"handoffTo":"deploy"`), 1),
			bytes.Replace(raw, []byte(`"handoffTo":"review"`), []byte(`"handoffTo":""`), 1),
			bytes.Replace(raw, []byte(`"handoffTo":"review"`), []byte(`"handoffReason":"STAGE_COMPLETE"`), 1),
			bytes.Replace(raw, []byte(`"handoffTo":"review"`), []byte(`"handoffReason":"OTHER","handoffTo":"review"`), 1),
		} {
			if bytes.Equal(bad, raw) {
				t.Fatal("fixture did not apply")
			}
			if _, err := DecodeAttempt(bad); err == nil {
				t.Fatalf("accepted %s", bad)
			}
		}
	})
}

// CAL-V0-083: the reason set is closed and small.
func TestCALV0083_ClosedHandoffReasons(t *testing.T) {
	t.Run("CAL-V0-083 ClosedHandoffReasons", func(t *testing.T) {
		if !slices.Equal(HandoffReasons, []string{"CHANGES_REQUESTED", "STAGE_COMPLETE", "STAGE_INCOMPLETE"}) {
			t.Fatalf("reason set drifted: %v", HandoffReasons)
		}
	})
}

// CAL-V0-082: an ended generation's recorded hand-off survives in its
// prior-generation entry and needs the V1-0788 history keys beside it.
func TestCALV0082_PriorGenerationHandoff(t *testing.T) {
	t.Run("CAL-V0-082 PriorGenerationHandoff", func(t *testing.T) {
		a := accountingAttempt()
		a.Generation = "3"
		a.PriorGenerations = []PriorGeneration{
			{Generation: "1", Quiescence: "FENCED", ProvedSeq: "2", History: &GenerationHistory{Stage: handoffStr("implement"), HandoffTo: "review", HandoffReason: "STAGE_COMPLETE"}},
			{Generation: "2", Quiescence: "FENCED", ProvedSeq: "4", History: &GenerationHistory{Stage: handoffStr("review")}},
		}
		raw, err := a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		entry := `{"generation":"1","handoffReason":"STAGE_COMPLETE","handoffTo":"review","memberId":null,"poolId":null,"provedSeq":"2","quiescence":"FENCED","stage":"implement"}`
		plain := `{"generation":"2","memberId":null,"poolId":null,"provedSeq":"4","quiescence":"FENCED","stage":"review"}`
		if !bytes.Contains(raw, []byte(entry)) || !bytes.Contains(raw, []byte(plain)) {
			t.Fatalf("prior entries: %s", raw)
		}
		b, err := DecodeAttempt(raw)
		if err != nil {
			t.Fatal(err)
		}
		if h := b.PriorGenerations[0].History; h.HandoffTo != "review" || h.HandoffReason != "STAGE_COMPLETE" {
			t.Fatalf("decoded %+v", h)
		}
		again, err := b.Encode()
		if err != nil || !bytes.Equal(raw, again) {
			t.Fatalf("bytes changed: %v", err)
		}
		for name, bad := range map[string]string{
			"no-history":   `{"generation":"1","handoffTo":"review","provedSeq":"2","quiescence":"FENCED"}`,
			"null-stage":   `{"generation":"1","handoffTo":"review","memberId":null,"poolId":null,"provedSeq":"2","quiescence":"FENCED","stage":null}`,
			"reason-only":  `{"generation":"1","handoffReason":"STAGE_COMPLETE","memberId":null,"poolId":null,"provedSeq":"2","quiescence":"FENCED","stage":"implement"}`,
			"bad-target":   `{"generation":"1","handoffTo":"deploy","memberId":null,"poolId":null,"provedSeq":"2","quiescence":"FENCED","stage":"implement"}`,
			"bad-reason":   `{"generation":"1","handoffReason":"DONE","handoffTo":"review","memberId":null,"poolId":null,"provedSeq":"2","quiescence":"FENCED","stage":"implement"}`,
			"inconsistent": `{"generation":"1","handoffReason":"STAGE_COMPLETE","handoffTo":"implement","memberId":null,"poolId":null,"provedSeq":"2","quiescence":"FENCED","stage":"implement"}`,
		} {
			x := bytes.Replace(raw, []byte(entry), []byte(bad), 1)
			if bytes.Equal(x, raw) {
				t.Fatalf("%s: fixture did not apply", name)
			}
			if _, err := DecodeAttempt(x); wire.CodeOf(err) != wire.CodeMalformed {
				t.Fatalf("%s: accepted or wrong refusal: %v", name, err)
			}
		}
	})
}
