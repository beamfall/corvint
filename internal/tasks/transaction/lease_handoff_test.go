package transaction

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

// Pure predicates consume hypothetical observations; actual provenance is
// established by the full-audit store regressions, not these supplied values.
func TestCALV0044_HistoryBindingRefusesMissingOrMismatchedEvidence(t *testing.T) {
	for _, name := range []string{"ok", "missing", "interval", "head", "receipt", "generation", "policy-path", "policy-body-digest", "original-zero", "original-malformed", "first-malformed", "first-before-policy", "first-future", "first-path", "first-policy", "first-config", "first-capability", "missing-receipt", "allocation", "current-relevant"} {
		t.Run(name, func(t *testing.T) {
			rec := fixture.Ticket("AT-01")
			q, _ := wire.ParseQueueID("", rec.TicketID.QueueID())
			tickets, _ := ticket.NewInventory(q, []*ticket.Record{rec})
			original := fixture.PolicyBytes()
			v, _ := wire.Parse(original)
			v.Obj.Set("policyVersion", wire.String("2"))
			if name == "current-relevant" {
				v.Obj.Set("cemRequired", wire.Bool(true))
			}
			current, err := intent.DecodePolicy(wire.EncodeFile(v))
			if err != nil {
				t.Fatal(err)
			}
			head, receipt := []byte("hypothetical head"), []byte("hypothetical receipt")
			a := &snapshot.Attempt{AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Generation: "1", TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, PolicySha256: wire.Sum(original), ConfigSha256: wire.Sum(original), CapabilityProfileSha256: wire.Sum(original), RuntimeID: snapshot.RuntimeExternalAgent, Stage: "review", Phase: "RUNNING", ScopeCheck: "UNKNOWN", RetryAccounting: &snapshot.RetryAccounting{Disposition: "NONE"}}
			h := &HandoffPolicyObservation{AttemptID: a.AttemptID, Generation: "1", LastSeq: "5", HeadSha256: wire.Sum(head), LastReceiptSha256: wire.Sum(receipt), FinalPolicySha256: wire.Sum(current.Raw), OriginalPath: "intent/policy.json", OriginalSeq: "1", OriginalSha256: wire.Sum(original), OriginalRaw: original, OriginalReceiptSha256: wire.Sum([]byte("original receipt")), FirstAttemptPath: attemptPath(a.AttemptID), FirstAttemptSeq: "2", FirstAttemptSha256: wire.Sum([]byte("first afterimage")), FirstAttemptReceiptSha256: wire.Sum([]byte("first receipt")), FirstPolicySha256: a.PolicySha256, FirstConfigSha256: a.ConfigSha256, FirstCapabilitySha256: a.CapabilityProfileSha256, FirstAllocationSha256: wire.Sum(wire.EncodeFile(wire.Null())), Compatible: true}
			switch name {
			case "missing":
				h = nil
			case "interval":
				h.Compatible = false
			case "head":
				h.HeadSha256 = wire.Sum(nil)
			case "receipt":
				h.LastReceiptSha256 = wire.Sum(nil)
			case "generation":
				h.Generation = "2"
			case "policy-path":
				h.OriginalPath = "intent/queue.json"
			case "policy-body-digest":
				p, _ := intent.DecodePolicy(original)
				h.OriginalSha256 = p.PolicySha256()
			case "original-zero":
				h.OriginalSeq = "0"
			case "original-malformed":
				h.OriginalSeq = "01"
			case "first-malformed":
				h.FirstAttemptSeq = "02"
			case "first-before-policy":
				h.FirstAttemptSeq = "1"
			case "first-future":
				h.FirstAttemptSeq = "6"
			case "first-path":
				h.FirstAttemptPath = attemptPath("other")
			case "first-policy":
				h.FirstPolicySha256 = wire.Sum(nil)
			case "first-config":
				h.FirstConfigSha256 = wire.Sum(nil)
			case "first-capability":
				h.FirstCapabilitySha256 = wire.Sum(nil)
			case "missing-receipt":
				h.OriginalReceiptSha256 = ""
			case "allocation":
				h.FirstAllocationSha256 = wire.Sum(nil)
			}
			c := leaseContext{st: inputState{tickets: tickets, policy: current, head: &snapshot.Head{LastSeq: "5"}}, in: Input{Head: head, HeadReceipt: receipt, HandoffPolicy: h}, l: &LeaseRequest{Reason: wire.CodeHandoff, Evidence: "local:external"}}
			got := c.verifyHandoff(a)
			if name == "ok" {
				if got != nil {
					t.Fatalf("valid bound projection refused: %+v", got)
				}
			} else if got == nil || !got.result.Outcome.HasCode(wire.CodeStalePolicy) {
				t.Fatalf("unbound history gained a clean handoff: %+v", got)
			}
		})
	}
}

func TestCALV0046_ReleasePreimageCompatibility(t *testing.T) {
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	l := LeaseRequest{Verb: LeaseRelease, AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Generation: "1", Reason: wire.CodeHandoff}
	v, e := leaseValue(&l, q)
	if e != nil {
		t.Fatal(e)
	}
	// Exact pre-extension RELEASE preimage, captured from the public adf source shape.
	want := `{"attemptId":"attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","base":null,"branch":null,"generation":"1","holder":null,"leaseMinutes":null,"reason":"HANDOFF","scope":null,"ticketId":null,"verb":"RELEASE","wholeRepository":false}`
	if got := string(wire.Encode(v)); got != want {
		t.Fatalf("legacy preimage changed: %s", got)
	}
	l.Evidence = "local:review-1"
	a, e := leaseValue(&l, q)
	if e != nil {
		t.Fatal(e)
	}
	l.Evidence = "local:review-2"
	b, e := leaseValue(&l, q)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Equal(wire.Encode(a), wire.Encode(b)) {
		t.Fatal("evidence not bound")
	}
	for _, bad := range []string{strings.Repeat("x", 129), "bad\nref"} {
		l.Evidence = bad
		if _, e := leaseValue(&l, q); e == nil {
			t.Fatal("invalid evidence accepted")
		}
	}
	l.Evidence = "valid"
	l.Reason = wire.CodeGateFailed
	if _, e := leaseValue(&l, q); e == nil {
		t.Fatal("ordinary cancellation accepts evidence")
	}
}

func TestCALV0046_NoTreeEligibilityBindings(t *testing.T) {
	for _, name := range []string{"ok", "acceptance", "policy", "config", "legacy", "failed", "gates", "effects", "scope", "phase", "supervised"} {
		t.Run(name, func(t *testing.T) {
			rec := fixture.Ticket("AT-01")
			q, _ := wire.ParseQueueID("", rec.TicketID.QueueID())
			inv, e := ticket.NewInventory(q, []*ticket.Record{rec})
			if e != nil {
				t.Fatal(e)
			}
			policy, e := intent.DecodePolicy(fixture.PolicyBytes())
			if e != nil {
				t.Fatal(e)
			}
			a := &snapshot.Attempt{TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, PolicySha256: wire.Sum(policy.Raw), ConfigSha256: wire.Sum(policy.Raw), RuntimeID: snapshot.RuntimeExternalAgent, Stage: "integrate", Phase: "RUNNING", ScopeCheck: "UNKNOWN", RetryAccounting: &snapshot.RetryAccounting{Disposition: "NONE"}}
			code := wire.CodeMissingEvidence
			switch name {
			case "acceptance":
				a.TicketRevision = "2"
				code = wire.CodeStaleTicket
			case "policy":
				a.PolicySha256 = wire.Sum(nil)
				code = wire.CodeStalePolicy
			case "config":
				a.ConfigSha256 = wire.Sum(nil)
				code = wire.CodeStalePolicy
			case "legacy":
				a.RetryAccounting = nil
			case "failed":
				a.RetryAccounting.FailedOrUnknown = true
			case "gates":
				a.GateResults = []string{"passed"}
			case "effects":
				a.PendingEffects = []string{"pending"}
			case "scope":
				a.ScopeCheck = "WITHIN"
			case "phase":
				a.Phase = "CHECKING"
			case "supervised":
				a.RuntimeID = snapshot.SupervisedProfile
				code = wire.CodeTicketState
			}
			c := leaseContext{st: inputState{tickets: inv, policy: policy}, l: &LeaseRequest{Reason: wire.CodeHandoff, Evidence: "local:external"}}
			got := c.verifyHandoff(a)
			if name == "ok" {
				if got != nil {
					t.Fatalf("valid refused: %+v", got)
				}
				return
			}
			if got == nil || got.result == nil || !got.result.Outcome.HasCode(code) {
				t.Fatalf("expected %s: %+v", code, got)
			}
		})
	}
}
