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
