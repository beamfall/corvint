package ticket_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TestKHNV0025_CodecRepositoryEntry: an ADD may name a repository alias
// whose "<alias>/" prefixes every anchor path; the key round-trips, is
// omitted when absent, and the legacy fixture carries none.
func TestKHNV0025_CodecRepositoryEntry(t *testing.T) {
	raw, err := os.ReadFile(issue502RecordFixture)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"repository"`)) {
		t.Fatal("legacy fixture carries a repository key")
	}
	qualify := func(k []ticket.KnowHowEntry, i int, alias string) {
		anchors := append([]ticket.KnowHowAnchor{}, k[i].Anchors...)
		for j := range anchors {
			anchors[j].Path = alias + "/" + anchors[j].Path
		}
		k[i].Anchors, k[i].Repository = anchors, alias
	}
	decode := func(edit func([]ticket.KnowHowEntry)) (*ticket.Record, []byte, error) {
		rec := fixture.Ticket("AT-01")
		rec.KnowHow = issue502KnowHow()
		edit(rec.KnowHow)
		b := rec.Encode()
		got, err := ticket.Decode(b)
		return got, b, err
	}
	got, b, err := decode(func(k []ticket.KnowHowEntry) { qualify(k, 0, "e2e") })
	if err != nil {
		t.Fatalf("repository ADD: %v", err)
	}
	if got.KnowHow[0].Repository != "e2e" || got.KnowHow[1].Repository != "" {
		t.Fatalf("repository did not round-trip: %+v", got.KnowHow)
	}
	if !bytes.Equal(got.Encode(), b) || bytes.Count(b, []byte(`"repository":"e2e"`)) != 1 {
		t.Fatalf("repository bytes changed or repeated:\n%s", b)
	}
	for name, edit := range map[string]func([]ticket.KnowHowEntry){
		"unprefixed anchor": func(k []ticket.KnowHowEntry) { k[0].Repository = "e2e" },
		"other prefix": func(k []ticket.KnowHowEntry) {
			qualify(k, 0, "work")
			k[0].Repository = "e2e"
		},
		"one anchor unprefixed": func(k []ticket.KnowHowEntry) {
			anchors := append([]ticket.KnowHowAnchor{}, k[1].Anchors...)
			anchors[0].Path = "e2e/" + anchors[0].Path
			k[1].Anchors, k[1].Repository = anchors, "e2e"
		},
		"alias not a token": func(k []ticket.KnowHowEntry) { qualify(k, 0, "_e2e") },
		"alias too long":    func(k []ticket.KnowHowEntry) { qualify(k, 0, strings.Repeat("a", 65)) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := decode(edit); err == nil {
				t.Fatal("decoded a repository entry outside KHN-V0-025")
			}
		})
	}
}
