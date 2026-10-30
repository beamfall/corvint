package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0011_CompletionOfferNeedsEveryRequiredGateCurrentPass: the offer
// returns the sorted heads only when every gate the policy declares, and every
// gate the ticket references, reads CURRENT PASS. A missing, STALE, UNKNOWN,
// RETURN or resubmitted gate, or a policy declaring no gate, gives no offer.
func TestERGV0011_CompletionOfferNeedsEveryRequiredGateCurrentPass(t *testing.T) {
	pass, ret := "PASS", "RETURN"
	head := func(c string) *wire.Digest { d := wire.Digest(strings.Repeat(c, 64)); return &d }
	current := func(c string) ExternalReviewView {
		return ExternalReviewView{Verdict: &pass, Generation: "1", Revision: "1", Status: "CURRENT", EvidenceSha256: head(c)}
	}
	policy := &intent.Policy{ExternalReviews: []intent.ExternalReviewDefinition{{GateID: "g2"}, {GateID: "g1"}}}
	both := map[string]ExternalReviewView{"g1": current("b"), "g2": current("a")}
	got := ExternalReviewCompletionOffer(policy, both)
	if len(got) != 2 || got[0] != *head("a") || got[1] != *head("b") {
		t.Fatalf("every gate CURRENT PASS: %v", got)
	}
	with := func(gate string, v ExternalReviewView) map[string]ExternalReviewView {
		m := map[string]ExternalReviewView{"g1": current("b"), "g2": current("a")}
		m[gate] = v
		return m
	}
	stale, unknown, returned, resubmitted, unbound := current("a"), ExternalReviewView{Generation: "0", Revision: "0", Status: "UNKNOWN"}, current("a"), current("a"), current("a")
	stale.Status = "STALE"
	returned.Verdict = &ret
	resubmitted.Verdict, resubmitted.Resubmitted = nil, true
	unbound.EvidenceSha256 = nil
	for name, views := range map[string]map[string]ExternalReviewView{
		"missing declared gate":     {"g1": current("b")},
		"STALE":                     with("g2", stale),
		"UNKNOWN":                   with("g2", unknown),
		"RETURN":                    with("g2", returned),
		"RESUBMITTED":               with("g2", resubmitted),
		"no head digest":            with("g2", unbound),
		"undeclared reference":      with("g3", unknown),
		"no views":                  {},
		"undeclared reference only": {"g3": current("c")},
	} {
		if got := ExternalReviewCompletionOffer(policy, views); got != nil {
			t.Fatalf("%s: offered %v", name, got)
		}
	}
	for name, p := range map[string]*intent.Policy{"nil policy": nil, "no definitions": {}} {
		if got := ExternalReviewCompletionOffer(p, both); got != nil {
			t.Fatalf("%s: offered %v", name, got)
		}
	}
}
