package ticket_test

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TestKHNV0012_LegacyKnowHowReadsExactlyAsBefore: a record written before
// symbol anchors and RECONFIRM existed (the shared fixture Core also reads)
// decodes and re-encodes to the same bytes, gains no symbol or reconfirm
// key, and its effective notes are its active notes with no overlay.
func TestKHNV0012_LegacyKnowHowReadsExactlyAsBefore(t *testing.T) {
	raw, err := os.ReadFile(issue502RecordFixture)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := ticket.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rec.Encode(), raw) {
		t.Fatalf("legacy record bytes changed:\n%s\n%s", raw, rec.Encode())
	}
	for _, key := range []string{`"symbol"`, `"symbolSha256"`, `"RECONFIRM"`, `"reconfirmed"`} {
		if bytes.Contains(raw, []byte(key)) {
			t.Fatalf("legacy fixture carries %s", key)
		}
	}
	active := ticket.ActiveKnowHow(rec.KnowHow)
	effective := ticket.EffectiveKnowHow(rec.KnowHow)
	if len(active) == 0 || !reflect.DeepEqual(active, effective) {
		t.Fatalf("effective notes differ from active notes:\n%+v\n%+v", active, effective)
	}
	for _, n := range effective {
		if n.Reconfirmed != nil {
			t.Fatalf("legacy note gained a reconfirm overlay: %+v", n)
		}
		for _, a := range n.Anchors {
			if a.Symbol != "" || a.SymbolSha256 != "" {
				t.Fatalf("legacy anchor gained a symbol: %+v", a)
			}
		}
	}
}
