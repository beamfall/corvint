package ticket_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// verdictPathsRecordFixture is shared with Core's reader test
// (internal/taskman), so both readers decode the bytes this codec writes.
const verdictPathsRecordFixture = "testdata/verdict-paths-record.json"

// verdictPathsCarrier declares a full write set as touchPaths and a narrower
// verdict predicate domain beneath it (TCB-V0-035).
func verdictPathsCarrier() *ticket.Record {
	rec := fixture.Ticket("AT-01")
	rec.Effects.TouchPaths = []string{"docs/specs/", "internal/taskman/", "internal/tasks/store/lease_gate.go"}
	rec.Effects.VerdictPaths = []string{"docs/specs/README.md", "docs/specs/REQUIREMENTS.tsv"}
	return rec
}

// TestTCBV0035_RecordVerdictPathsKey: a record without verdictPaths keeps its
// exact bytes, a carrier round trips byte-identically and deterministically
// with the member in canonical key order, and the shared fixture is exactly
// this codec's encoding.
func TestTCBV0035_RecordVerdictPathsKey(t *testing.T) {
	legacy := fixture.Ticket("AT-01")
	raw := legacy.Encode()
	if bytes.Contains(raw, []byte(`"verdictPaths"`)) {
		t.Fatalf("a record without verdict paths gained the key: %s", raw)
	}
	back, err := ticket.Decode(raw)
	if err != nil || back.Effects.VerdictPaths != nil || !bytes.Equal(back.Encode(), raw) {
		t.Fatalf("legacy round trip: %v", err)
	}

	rec := verdictPathsCarrier()
	raw = rec.Encode()
	if !bytes.Equal(raw, verdictPathsCarrier().Encode()) {
		t.Fatal("encoding is not deterministic")
	}
	if !bytes.Contains(raw, []byte(`"touchPaths":["docs/specs/","internal/taskman/","internal/tasks/store/lease_gate.go"],"verdictPaths":["docs/specs/README.md","docs/specs/REQUIREMENTS.tsv"]}`)) {
		t.Fatalf("verdictPaths is not the last effects member in canonical order: %s", raw)
	}
	back, err = ticket.Decode(raw)
	if err != nil {
		t.Fatalf("decode carrier: %v", err)
	}
	if !bytes.Equal(back.Encode(), raw) || back.FileDigest() != wire.Sum(raw) || len(back.Effects.VerdictPaths) != 2 {
		t.Fatalf("carrier round trip differs:\n%s\n%s", raw, back.Encode())
	}
	file, err := os.ReadFile(verdictPathsRecordFixture)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(file, raw) {
		t.Fatalf("%s is not this codec's encoding:\n%s", verdictPathsRecordFixture, raw)
	}
}

// TestTCBV0036_RecordVerdictPathsRefusals: the codec refuses an empty list,
// a path no touchPaths key covers under CAL-V0-021 (a key without a trailing
// "/" covers only itself), a null, an unsorted list and an unknown effects
// member, and admits a path covered by equality or by a directory key.
func TestTCBV0036_RecordVerdictPathsRefusals(t *testing.T) {
	encode := func(touch []string, verdict wire.Value) []byte {
		v, err := wire.Parse(verdictPathsCarrier().Encode())
		if err != nil {
			t.Fatal(err)
		}
		effects, _ := v.Obj.Get("effects")
		effects.Obj.Set("touchPaths", wire.Strings(touch))
		effects.Obj.Set("verdictPaths", verdict)
		return wire.EncodeFile(v)
	}
	touch := []string{"docs/a.md", "src", "src/"}
	for _, c := range []struct {
		name, code, detail string
		verdict            wire.Value
	}{
		{"empty", wire.CodeMalformed, ticket.VerdictPathsEmpty, wire.Strings([]string{})},
		{"outside", wire.CodeOutOfScope, ticket.VerdictPathOutsideTouchPaths, wire.Strings([]string{"lib/x.go"})},
		{"exact key covers only itself", wire.CodeOutOfScope, ticket.VerdictPathOutsideTouchPaths, wire.Strings([]string{"docs/a.md/b"})},
		{"prefix without slash", wire.CodeOutOfScope, ticket.VerdictPathOutsideTouchPaths, wire.Strings([]string{"srcx/a.go"})},
		{"null", wire.CodeMalformed, "", wire.Null()},
		{"unsorted", wire.CodeMalformed, "", wire.Array(wire.String("src/b"), wire.String("src/a"))},
	} {
		_, err := ticket.Decode(encode(touch, c.verdict))
		if wire.CodeOf(err) != c.code || err == nil || !strings.Contains(err.Error(), c.detail) {
			t.Fatalf("%s: %v, want %s %s", c.name, err, c.code, c.detail)
		}
	}
	for _, ok := range [][]string{{"docs/a.md"}, {"src"}, {"src/"}, {"src/deep/x.go"}, {"docs/a.md", "src/a.go"}} {
		if _, err := ticket.Decode(encode(touch, wire.Strings(ok))); err != nil {
			t.Fatalf("covered %v refused: %v", ok, err)
		}
	}
	v, _ := wire.Parse(verdictPathsCarrier().Encode())
	effects, _ := v.Obj.Get("effects")
	effects.Obj.Set("verdictPathz", wire.Strings([]string{"src/"}))
	if _, err := ticket.Decode(wire.EncodeFile(v)); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("unknown effects member: %v", err)
	}
}
