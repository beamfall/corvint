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

// TestESCV0005_AttemptPinsTheAdmittedAnswers: a no-answer attempt keeps its
// bytes, pinned references round-trip exactly, and an empty, unsorted,
// duplicate or open-ended pin is refused.
func TestESCV0005_AttemptPinsTheAdmittedAnswers(t *testing.T) {
	a := accountingAttempt()
	legacy, err := a.Encode()
	if err != nil || bytes.Contains(legacy, []byte("escalationAnswers")) {
		t.Fatalf("no-answer attempt carries an answer pin: %v", err)
	}
	d := func(c string) wire.Digest { return wire.Digest(strings.Repeat(c, 64)) }
	a.EscalationAnswers = []EscalationAnswerRef{{RequestID: "q-1", OriginSha256: d("a"), HeadSha256: d("b")}, {RequestID: "q-2", OriginSha256: d("c"), HeadSha256: d("e")}}
	raw, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	b, err := DecodeAttempt(raw)
	if err != nil || len(b.EscalationAnswers) != 2 || b.EscalationAnswers[1] != a.EscalationAnswers[1] {
		t.Fatalf("pin = %+v, %v", b.EscalationAnswers, err)
	}
	if again, err := b.Encode(); err != nil || !bytes.Equal(raw, again) {
		t.Fatalf("pinned bytes changed: %v", err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set("escalationAnswers", wire.Array())
	for name, bad := range map[string][]byte{
		"empty":     wire.EncodeFile(v),
		"unsorted":  bytes.Replace(raw, []byte(`"q-2"`), []byte(`"q-0"`), 1),
		"duplicate": bytes.Replace(raw, []byte(`"q-2"`), []byte(`"q-1"`), 1),
		"open":      bytes.Replace(raw, []byte(`"requestId":"q-1"`), []byte(`"requestId":"q-1","state":"OPEN"`), 1),
		"digest":    bytes.Replace(raw, []byte(string(d("e"))), []byte("e"), 1),
	} {
		if _, err := DecodeAttempt(bad); err == nil {
			t.Fatalf("%s pin accepted", name)
		}
	}
}
