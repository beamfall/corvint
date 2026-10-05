package journal

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestESCV0010_EscalationEventsEarnCoverage: a typed escalation event is a
// covered evidence profile (its receipt binding is store-tested), while an
// unknown evidence blob stays UNKNOWN (ESC-V0-010).
func TestESCV0010_EscalationEventsEarnCoverage(t *testing.T) {
	d := wire.Sum([]byte("admission"))
	src := ticket.EscalationSource{QueueID: "queue:test:main", TicketID: "ticket:test:main:one", AttemptID: "attempt:test:main:0123456789abcdef0123456789abcdef", Generation: "7", Holder: "worker-1", AcceptanceRevision: "1", ReceiptSequence: "42", ReceiptSha256: d, PostAttemptSha256: d, TicketRecordSha256: d}
	r := ticket.EscalationRequest{Profile: ticket.EscalationRequestProfile, QueueID: src.QueueID, TicketID: src.TicketID, RequestID: "question-1", Actor: "worker-1", ActorRole: "OWNER", Operation: "OPEN", Open: &ticket.EscalationOpen{Source: src, Kind: "infrastructure", Question: "Runner lost its network?", Options: []string{}}}
	rb, err := ticket.EncodeEscalationRequest(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ticket.EncodeEscalationEvent(ticket.EscalationEvent{Profile: ticket.EscalationEventProfile, QueueID: r.QueueID, TicketID: r.TicketID, EscalationID: r.RequestID, Revision: "1", Operation: "OPEN", Source: src, Actor: r.Actor, ActorRole: r.ActorRole, RecordedAt: "2026-10-05T00:00:00Z", OriginalRequest: r, RequestSha256: wire.Sum(rb), ResolvedRequestID: r.RequestID, ResolvedPreviousRevision: "0"})
	if err != nil {
		t.Fatal(err)
	}
	rc := &snapshot.Receipt{Seq: "43", Kind: "TRANSITION"}
	for name, c := range map[string]struct {
		raw  []byte
		want bool
	}{"escalation event": {raw, true}, "unknown blob": {[]byte("opaque gate output\n"), false}} {
		got, _, err := (Reader{}).validateRecord("evidence/"+string(wire.Sum(c.raw)), c.raw, rc)
		if err != nil || got != c.want {
			t.Errorf("%s: coverage %v, %v; want %v", name, got, err, c.want)
		}
	}
}
