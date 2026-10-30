package ticket_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0009_TicketReviewReferencesCodec: a ticket without references keeps
// its legacy bytes, a ticket with references round-trips canonically, and an
// empty or malformed map is refused.
func TestERGV0009_TicketReviewReferencesCodec(t *testing.T) {
	rec := fixture.Ticket("AT-0001")
	legacy := rec.Encode()
	if bytes.Contains(legacy, []byte("externalReviews")) {
		t.Fatal("a ticket without reviews encoded the field")
	}
	head := wire.Digest(strings.Repeat("a", 64))
	rec.ExternalReviews = map[string]ticket.ExternalReviewRef{"g2": {Generation: "1", Revision: "1", Head: head}, "g1": {Generation: "2", Revision: "3", Head: head}}
	raw := rec.Encode()
	got, err := ticket.Decode(raw)
	if err != nil || !bytes.Equal(got.Encode(), raw) || got.ExternalReviews["g1"].Revision != "3" {
		t.Fatalf("round trip: %v %s", err, raw)
	}
	if bytes.Index(raw, []byte(`"g1"`)) > bytes.Index(raw, []byte(`"g2"`)) {
		t.Fatal("gate keys are not sorted")
	}
	for name, bad := range map[string]string{
		"empty":          `{}`,
		"gen > revision": `{"g1":{"generation":"2","head":"` + string(head) + `","revision":"1"}}`,
		"extra":          `{"g1":{"extra":"1","generation":"1","head":"` + string(head) + `","revision":"1"}}`,
		"over capacity":  `{"g1":{"generation":"1","head":"` + string(head) + `","revision":"4097"}}`,
	} {
		v, err := wire.Parse([]byte(bad + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ticket.ExternalReviewsFromValue(v); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
