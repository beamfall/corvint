package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0003_ImportApplyNeverCarriesKnowHow: only KNOWHOW_ADD and
// KNOWHOW_RETRACT write the ledger (and only on NATIVE tickets), so
// IMPORT_APPLY refuses a new or chained imported record that adds one.
func TestKHNV0003_ImportApplyNeverCarriesKnowHow(t *testing.T) {
	in := roadmapInitialized(t)
	entry := []ticket.KnowHowEntry{{Seq: "1", Operation: ticket.KnowHowAdd, Text: "n",
		Anchors: []ticket.KnowHowAnchor{{Path: "a.go", Blob: strings.Repeat("a", 40)}}, Routes: []string{},
		Commit: strings.Repeat("c", 40), ActorID: "russell", ActorRole: "OWNER", RecordedAt: "2026-09-06T13:00:00Z"}}
	fresh := imported("BF-9", "nine")
	fresh.KnowHow = entry
	refusedWith(t, "new ticket with know-how", Model(importRequest(fresh), in), wire.CodeMalformed)

	pre := imported("BF-9", "nine")
	digest := pre.FileDigest()
	next := imported("BF-9", "nine, changed")
	next.Revision, next.AcceptanceRevision, next.PreviousRecordSha256, next.KnowHow = "2", "2", &digest, entry
	refusedWith(t, "revision adding know-how", Model(importRequest(next), withTicket(t, in, pre, pre.Encode())), wire.CodeMalformed)
	next.KnowHow = nil
	if r := Model(importRequest(next), withTicket(t, in, pre, pre.Encode())); r.Kind != "Transaction" {
		t.Fatalf("plain chained import: %+v", r)
	}
}
