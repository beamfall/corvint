package ticket_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TestKHNV0010_CodecWorkerEntry: the Tasks codec admits a WORKER entry only
// as a non-superseding ADD that records its attempt and generation.
func TestKHNV0010_CodecWorkerEntry(t *testing.T) {
	decode := func(edit func([]ticket.KnowHowEntry)) error {
		rec := fixture.Ticket("AT-01")
		rec.KnowHow = issue502KnowHow()
		edit(rec.KnowHow)
		_, err := ticket.Decode(rec.Encode())
		return err
	}
	if err := decode(func(k []ticket.KnowHowEntry) { k[2].ActorRole = "WORKER" }); err != nil {
		t.Fatalf("WORKER ADD with attempt and generation: %v", err)
	}
	for name, edit := range map[string]func([]ticket.KnowHowEntry){
		"superseding ADD": func(k []ticket.KnowHowEntry) { k[1].ActorRole = "WORKER" },
		"RETRACT":         func(k []ticket.KnowHowEntry) { k[3].ActorRole = "WORKER" },
		"no attempt":      func(k []ticket.KnowHowEntry) { k[2].ActorRole, k[2].Attempt = "WORKER", nil },
		"no generation":   func(k []ticket.KnowHowEntry) { k[2].ActorRole, k[2].Generation = "WORKER", nil },
		"other role":      func(k []ticket.KnowHowEntry) { k[2].ActorRole = "REVIEWER" },
	} {
		t.Run(name, func(t *testing.T) {
			if err := decode(edit); err == nil {
				t.Fatal("decoded a WORKER entry outside KHN-V0-010")
			}
		})
	}
}
