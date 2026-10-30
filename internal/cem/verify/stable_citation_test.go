package verify

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
)

// requireCitationOnlyOnAccept is the CEM-V1-092 invariant for one result:
// the citation member is present exactly when the outcome is ACCEPT.
func requireCitationOnlyOnAccept(t *testing.T, name string, r StableResult) {
	t.Helper()
	if (r.Outcome == "ACCEPT") != (r.Citation != nil) {
		t.Errorf("%s: outcome %s with citation present=%v", name, r.Outcome, r.Citation != nil)
	}
}

// TestCEMV1092CitationOnlyOnAccept runs both public no-Tasks cases and every
// public mutation of them (ACCEPT axis cases, wire, references and artifacts
// rejections) and checks that only an ACCEPT carries the citation block.
func TestCEMV1092CitationOnlyOnAccept(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	n := loadNeutralCase(t)
	repo, base, target := n.materialize(t)
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: n.root}
	accepted, _ := Stable(context.Background(), n.raw, options)
	if accepted.Outcome != "ACCEPT" {
		t.Fatalf("neutral case outcome %s", accepted.Outcome)
	}
	requireCitationOnlyOnAccept(t, "neutral-criterion", accepted)
	for _, r := range n.expect.Refusals {
		var m map[string]any
		var criterion map[string]any
		if err := json.Unmarshal(n.raw, &m); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(r.Criterion, &criterion); err != nil {
			t.Fatal(err)
		}
		m["criterionBindings"] = []any{criterion}
		if r.RebindLink {
			m["criterionLinks"].([]any)[0].(map[string]any)["criterionId"] = criterion["id"]
		}
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		result, _ := Stable(context.Background(), raw, options)
		if result.Outcome != r.Outcome {
			t.Fatalf("neutral-criterion/%s outcome %s, want %s", r.Name, result.Outcome, r.Outcome)
		}
		requireCitationOnlyOnAccept(t, "neutral-criterion/"+r.Name, result)
	}

	c := loadReceiptAuthCase(t)
	repo, base, target = c.materialize(t)
	options = StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root}
	for _, m := range append(append([]receiptAuthMutation(nil), c.expect.AxisCases...), c.expect.Refusals...) {
		o := options
		o.TrustedKeys = c.trusted(t, m.TrustedKeys)
		result, _ := Stable(context.Background(), c.mutate(t, m.Set), o)
		if result.Outcome != m.Outcome {
			t.Fatalf("receipt-authentication/%s outcome %s, want %s", m.Name, result.Outcome, m.Outcome)
		}
		requireCitationOnlyOnAccept(t, "receipt-authentication/"+m.Name, result)
	}
}

// TestCEMV1092SealedCasesCarryCitationOnlyOnAccept checks the sealed public
// expectations, which TestStableS0EPublicCases compares in full: every
// non-ACCEPT case, including the drift refusals, carries citation null.
func TestCEMV1092SealedCasesCarryCitationOnlyOnAccept(t *testing.T) {
	t.Parallel()
	packet := loadS0EPacket(t)
	accepted, drift := 0, 0
	for _, c := range packet.cases {
		citation, present := c.ExpectedResult["citation"]
		if !present {
			t.Fatalf("%s: no citation member", c.ID)
		}
		accept := c.ExpectedResult["outcome"] == "ACCEPT"
		if accept != (citation != nil) {
			t.Errorf("%s: outcome %v with citation %v", c.ID, c.ExpectedResult["outcome"], citation)
		}
		if accept {
			accepted++
		}
		if c.ExpectedResult["stage"] == "drift" {
			drift++
		}
	}
	if accepted == 0 || drift == 0 {
		t.Fatalf("sealed cases: %d ACCEPT, %d drift; want both", accepted, drift)
	}
}

// TestCEMV1092RefuseAndRejectClearCitation pins the central rule: every
// failure path, including a cleanup override after a provisional ACCEPT and a
// drift rejection, clears the block.
func TestCEMV1092RefuseAndRejectClearCitation(t *testing.T) {
	t.Parallel()
	r := NewStableResult(StableOptions{})
	r.Citation = &Citation{Profile: CitationProfile}
	r.Refuse("process-cleanup", "process-cleanup-unobserved", true)
	if r.Citation != nil {
		t.Fatal("Refuse kept the citation block")
	}
	r.Citation = &Citation{Profile: CitationProfile}
	r.reject("drift", "evidence-drift")
	if r.Citation != nil {
		t.Fatal("reject kept the citation block")
	}
}
