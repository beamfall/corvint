package snapshot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0007_AttemptPinsTheAdmittedNote: an attempt without a note keeps
// its legacy bytes, and a pinned reference round-trips exactly.
func TestONV0007_AttemptPinsTheAdmittedNote(t *testing.T) {
	a := accountingAttempt()
	legacy, err := a.Encode()
	if err != nil || bytes.Contains(legacy, []byte("operatorNote")) {
		t.Fatalf("legacy attempt carries a note pin: %v", err)
	}
	head := wire.Digest(strings.Repeat("c", 64))
	a.OperatorNote = &ticket.OperatorNoteReference{Revision: "2", Head: head}
	raw, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	b, err := DecodeAttempt(raw)
	if err != nil || b.OperatorNote == nil || b.OperatorNote.Revision != "2" || b.OperatorNote.Head != head || b.OperatorNote.Current != nil {
		t.Fatalf("pin = %+v, %v", b.OperatorNote, err)
	}
	if again, err := b.Encode(); err != nil || !bytes.Equal(raw, again) {
		t.Fatalf("pinned bytes changed: %v", err)
	}
	if _, err := DecodeAttempt(bytes.Replace(raw, []byte(`"revision":"2"`), []byte(`"revision":"x"`), 1)); err == nil {
		t.Fatal("malformed pin accepted")
	}
}
