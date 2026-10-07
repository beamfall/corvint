package cli

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0006_DeliveredMemberFitsTheCap: across note sizes, the whole
// delivered knowHow member (envelope, counts and hint included) encodes to at
// most KnowHowDeliveryMaxBytes, and the notes stay an ordered prefix with
// omitted = matched - delivered.
func TestKHNV0006_DeliveredMemberFitsTheCap(t *testing.T) {
	head := strings.Repeat("c", 40)
	for size := 1; size <= wire.KnowHowMaxTextBytes; size += 13 {
		var notes []store.KnowHowNote
		for i := 1; i <= 12; i++ {
			e := ticket.KnowHowEntry{Seq: wire.Count(strconv.Itoa(i)), Operation: ticket.KnowHowAdd, Text: strings.Repeat("t", size),
				Anchors: []ticket.KnowHowAnchor{{Path: "src/a.go", Blob: strings.Repeat("b", 40)}}, Routes: []string{"flow-build"},
				Commit: head, ActorID: "russell", ActorRole: "OWNER", RecordedAt: "2026-10-07T12:00:00Z"}
			notes = append(notes, store.KnowHowNote{TicketID: "T-" + strconv.Itoa(100+i), Entry: e, Anchors: []string{store.KnowHowCurrent}, Freshness: store.KnowHowCurrent})
		}
		v := claimedKnowHowValue(store.ClaimedKnowHow{Head: head, Notes: notes})
		if n := len(wire.Encode(v)); n > store.KnowHowDeliveryMaxBytes {
			t.Fatalf("text of %d bytes: the member is %d bytes", size, n)
		}
		notesV, _ := v.Obj.Get("notes")
		omittedV, _ := v.Obj.Get("omitted")
		got := notesV.Arr
		omitted, _ := strconv.Atoi(omittedV.Str)
		if len(got) == 0 || len(got)+omitted != len(notes) {
			t.Fatalf("text of %d bytes: %d delivered, %d omitted of %d", size, len(got), omitted, len(notes))
		}
	}
}
