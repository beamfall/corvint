package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0008_TicketShowRendersAnUnresolvableNoteAsUnavailable covers the
// `ticket show` fallback used when no journal audit refuses the read: a note
// reference whose event is absent or does not resolve renders as UNAVAILABLE
// with the refusal code, the reference revision and head, and no text; never
// as NONE.
func TestONV0008_TicketShowRendersAnUnresolvableNoteAsUnavailable(t *testing.T) {
	state := t.TempDir()
	if err := os.MkdirAll(filepath.Join(state, "evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	get := func(v wire.Value, k string) wire.Value { x, _ := v.Obj.Get(k); return x }
	rc := &readCtx{repo: &intent.Repository{StateDir: state}}
	head := wire.Sum([]byte("an event that was never retained"))
	rec := fixture.Ticket("NT-01")
	rec.OperatorNote = &ticket.OperatorNoteReference{Revision: "3", Current: &head, Head: head}
	if v := operatorNoteShowValue(rc, rec); string(wire.Encode(v)) != `{"code":"MISSING_EVIDENCE","head":"`+string(head)+`","revision":"3","state":"UNAVAILABLE","text":null}` {
		t.Fatalf("missing event: %s", wire.Encode(v))
	}
	if err := os.WriteFile(filepath.Join(state, "evidence", string(head)), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := operatorNoteShowValue(rc, rec)
	if get(v, "state").Str != "UNAVAILABLE" || get(v, "code").Str == "" || get(v, "text").Kind != wire.KindNull || get(v, "head").Str != string(head) {
		t.Fatalf("mismatched event: %s", wire.Encode(v))
	}
	rec.OperatorNote = nil
	if v := operatorNoteShowValue(rc, rec); get(v, "state").Str != "NONE" {
		t.Fatalf("never noted: %s", wire.Encode(v))
	}
}
