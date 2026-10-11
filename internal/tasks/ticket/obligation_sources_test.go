package ticket_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// legacyEntryBytes is an entry as ledgers wrote it before review sources
// existed (TOL-V0-003); TOL-V0-039 keeps its bytes.
const legacyEntryBytes = `{"core":true,"evidence":null,"id":"AC-2","reason":"bug","state":"DEFECT","title":"two",` +
	`"updatedAt":"2026-10-08T12:00:00Z","updatedBy":{"id":"owner","role":"OWNER"}}`

func tolSources(t *testing.T, sources string) (*ticket.ObligationEntry, error) {
	t.Helper()
	e := tolParse(t, legacyEntryBytes)
	if sources != "" {
		e.Obj.Set("sources", tolParse(t, sources))
	}
	r := wire.NewReader(e, "/entry")
	got := ticket.ReadObligationEntry(r)
	return got, r.Err()
}

// TestTOLV0037_ReviewSourceCodec: an entry's optional sources list holds
// closed {kind, ref, digest} review sources and {kind: none, query,
// searched, at} empty searches, round-trips byte for byte, and refuses an
// unknown kind, a key of the other shape, a bad digest, an empty list and an
// unsorted searched set, and refuses a list over 16 LIMIT_EXCEEDED.
func TestTOLV0037_ReviewSourceCodec(t *testing.T) {
	d := strings.Repeat("d", 64)
	good := `[{"digest":"` + d + `","kind":"legacy-case","ref":"C7"},{"digest":"` + d + `","kind":"legacy-spec","ref":"specs/old/login.md"},` +
		`{"digest":"` + d + `","kind":"manual-case","ref":"C12"},{"at":"2026-10-11T09:00:00Z","kind":"none","query":"refund totals","searched":["testrail-json:sha256:` + d + `"]}]`
	e, err := tolSources(t, good)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Sources) != 4 || e.Sources[0].Ref != "C7" || e.Sources[3].Query != "refund totals" || len(e.Sources[3].Searched) != 1 {
		t.Fatalf("sources: %+v", e.Sources)
	}
	want := strings.Replace(legacyEntryBytes, `"state":`, `"sources":`+good+`,"state":`, 1)
	if got := string(wire.Encode(e.Value())); got != want {
		t.Fatalf("round trip\n got %s\nwant %s", got, want)
	}
	// The codec admits a none with an empty query or searched set; the
	// policy refusal is preflight's (TOL-V0-040).
	if _, err := tolSources(t, `[{"at":"2026-10-11T09:00:00Z","kind":"none","query":"","searched":[]}]`); err != nil {
		t.Fatalf("empty none refused by the codec: %v", err)
	}
	many := make([]string, 17)
	for i := range many {
		many[i] = `{"digest":"` + d + `","kind":"legacy-case","ref":"C1"}`
	}
	if _, err := tolSources(t, `[`+strings.Join(many, ",")+`]`); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("seventeen sources: want LIMIT_EXCEEDED, got %v", err)
	}
	for name, s := range map[string]string{
		"empty list":        `[]`,
		"unknown kind":      `[{"digest":"` + d + `","kind":"jira","ref":"C7"}]`,
		"no kind":           `[{"digest":"` + d + `","ref":"C7"}]`,
		"query on a case":   `[{"digest":"` + d + `","kind":"legacy-case","query":"x","ref":"C7"}]`,
		"ref on a none":     `[{"at":"2026-10-11T09:00:00Z","kind":"none","query":"x","ref":"C7","searched":["s"]}]`,
		"upper-hex digest":  `[{"digest":"` + strings.Repeat("D", 64) + `","kind":"legacy-case","ref":"C7"}]`,
		"blank ref":         `[{"digest":"` + d + `","kind":"legacy-case","ref":" "}]`,
		"unsorted searched": `[{"at":"2026-10-11T09:00:00Z","kind":"none","query":"x","searched":["b","a"]}]`,
		"bad at":            `[{"at":"today","kind":"none","query":"x","searched":["a"]}]`,
		"not a list":        `{"kind":"none"}`,
	} {
		if _, err := tolSources(t, s); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
}

// TestTOLV0039_LegacyLedgerBytesUnchanged: an entry and a set change
// without sources keep their pre-#813 bytes, and a sources-only change is
// admitted.
func TestTOLV0039_LegacyLedgerBytesUnchanged(t *testing.T) {
	e, err := tolSources(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if e.Sources != nil || !bytes.Equal(wire.Encode(e.Value()), []byte(legacyEntryBytes)) {
		t.Fatalf("legacy entry bytes changed: %s", wire.Encode(e.Value()))
	}
	legacySet := `{"changes":[{"core":null,"id":"AC-1","reason":"bug","state":"DEFECT"}]}`
	read := func(s string) (*ticket.ObligationSetPayload, error) {
		r := wire.NewReader(tolParse(t, s), "")
		p := ticket.ReadObligationSetPayload(r)
		return p, r.Err()
	}
	p, err := read(legacySet)
	if err != nil || string(wire.Encode(p.Value())) != legacySet {
		t.Fatalf("legacy set payload: %v %s", err, wire.Encode(p.Value()))
	}
	sourcesOnly := `{"changes":[{"core":null,"id":"AC-1","reason":"reviewed","sources":[{"digest":"` + strings.Repeat("a", 64) + `","kind":"legacy-case","ref":"C7"}],"state":null}]}`
	if p, err := read(sourcesOnly); err != nil || string(wire.Encode(p.Value())) != sourcesOnly {
		t.Fatalf("sources-only change: %v", err)
	}
	if _, err := read(`{"changes":[{"core":null,"id":"AC-1","reason":"x","state":null}]}`); err == nil {
		t.Fatal("an empty change decoded")
	}
}
