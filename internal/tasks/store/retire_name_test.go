package store

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// V1-3179: unretire writes only the canonical receipt name it regenerates from
// the sequence, so a traversal-shaped archive member can never name the file.
func TestV13179RetiredReceiptReturnsCanonicalName(t *testing.T) {
	want, err := snapshot.ReceiptName(7)
	if err != nil {
		t.Fatal(err)
	}
	seq, name, ok := retiredReceipt("receipts/"+want, 9)
	if !ok || seq != 7 || name != want {
		t.Fatalf("retiredReceipt(%q) = %d, %q, %v; want 7, %q, true", "receipts/"+want, seq, name, ok, want)
	}
	for _, member := range []string{
		"receipts/../00000000007.json",
		"receipts/../../escape.json",
		"receipts/0000/../0007.json",
		"receipts//00000000007.json",
		"/receipts/" + want,
		"receipts/" + want + "/",
		"receipts/000000000009.json",
		"receipts/000000000000.json",
	} {
		if seq, name, ok := retiredReceipt(member, 9); ok {
			t.Fatalf("retiredReceipt(%q) admitted %d, %q", member, seq, name)
		}
	}
}
