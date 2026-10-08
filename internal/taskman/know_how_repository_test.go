package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestKHNV0025_ReaderRepositoryEntry: Core's reader admits an ADD naming a
// repository alias whose "<alias>/" prefixes every anchor path, and refuses
// the forms the Tasks codec refuses, so a record a newer writer produces
// never reads differently in Core.
func TestKHNV0025_ReaderRepositoryEntry(t *testing.T) {
	const ctx = `"actor":{"id":"russell","role":"OWNER"},"attempt":null,"commit":"cccccccccccccccccccccccccccccccccccccccc","generation":null`
	add := func(repository string, paths ...string) string {
		var anchors []string
		for _, p := range paths {
			anchors = append(anchors, `{"blob":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","path":"`+p+`"}`)
		}
		return `{` + ctx + `,"anchors":[` + strings.Join(anchors, ",") + `],"evidencePath":null,"operation":"ADD","reason":null,"recordedAt":"2026-09-07T14:00:00Z",` + repository + `"routes":[],"seq":"5","supersedes":null,"text":"start the fixture server first"}`
	}
	record := func(t *testing.T, raw string) wire.Value {
		v := issue502Record(t)
		ledger := value(v, "knowHow")
		e, err := wire.Parse([]byte(raw))
		if err != nil {
			t.Fatalf("entry %s: %v", raw, err)
		}
		ledger.Arr = append(ledger.Arr, e)
		setMember(v, "knowHow", ledger)
		return v
	}
	for name, raw := range map[string]string{
		"repository add":    add(`"repository":"e2e",`, "e2e/src/a.go", "e2e/src/b.go"),
		"no repository add": add(``, "src/a.go"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := decodeTicket(record(t, raw)); e != nil {
				t.Fatalf("decode: %v", e)
			}
		})
	}
	for name, raw := range map[string]string{
		"unprefixed anchor":  add(`"repository":"e2e",`, "src/a.go"),
		"one anchor outside": add(`"repository":"e2e",`, "e2e/a.go", "work/b.go"),
		"alias not a token":  add(`"repository":"_e2e",`, "_e2e/a.go"),
		"alias too long":     add(`"repository":"`+strings.Repeat("a", 65)+`",`, strings.Repeat("a", 65)+"/a.go"),
		"null alias":         add(`"repository":null,`, "a.go"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := decodeTicket(record(t, raw)); e == nil {
				t.Fatal("decoded a repository entry outside KHN-V0-025")
			}
		})
	}
}
