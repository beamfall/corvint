package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTEAV0001_ImportApplyNeverCarriesAttachedEvidence: only ATTACH_EVIDENCE
// writes attachments (and only on NATIVE tickets), so IMPORT_APPLY refuses a
// new or chained imported record that adds, rewrites or drops one.
func TestTEAV0001_ImportApplyNeverCarriesAttachedEvidence(t *testing.T) {
	in := roadmapInitialized(t)
	entry := []ticket.AttachedEvidence{{AcceptanceRevision: "1", Actor: "russell", Evidence: []wire.Digest{wire.Sum([]byte("log"))}, Reason: "r", RecordedAt: "2026-09-06T13:00:00Z"}}
	fresh := imported("BF-9", "nine")
	fresh.AttachedEvidence = entry
	refusedWith(t, "new ticket with attachments", Model(importRequest(fresh), in), wire.CodeMalformed)

	pre := imported("BF-9", "nine")
	digest := pre.FileDigest()
	next := imported("BF-9", "nine, changed")
	next.Revision, next.AcceptanceRevision, next.PreviousRecordSha256, next.AttachedEvidence = "2", "2", &digest, entry
	refusedWith(t, "revision adding attachments", Model(importRequest(next), withTicket(t, in, pre, pre.Encode())), wire.CodeMalformed)
	next.AttachedEvidence = nil
	if r := Model(importRequest(next), withTicket(t, in, pre, pre.Encode())); r.Kind != "Transaction" {
		t.Fatalf("plain chained import: %+v", r)
	}
}
