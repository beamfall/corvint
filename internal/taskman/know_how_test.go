package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestKHNV0002_ReaderKnowHow: Core's ticket reader admits the shared
// fixture's knowHow ledger and refuses each malformed or inconsistent form,
// so the optional key stays fail-closed and mirrors the Tasks codec.
func TestKHNV0002_ReaderKnowHow(t *testing.T) {
	entry := func(v wire.Value, i int) wire.Value { return value(v, "knowHow").Arr[i] }
	anchor := func(v wire.Value) wire.Value { return value(entry(v, 0), "anchors").Arr[0] }
	arr := func(vs ...wire.Value) wire.Value { return wire.Value{Kind: wire.KindArray, Arr: vs} }
	cases := []struct {
		name string
		edit func(wire.Value)
	}{
		{"empty list", func(v wire.Value) { setMember(v, "knowHow", arr()) }},
		{"not an array", func(v wire.Value) { setMember(v, "knowHow", str502("x")) }},
		{"unknown entry key", func(v wire.Value) { setMember(entry(v, 0), "extra", str502("x")) }},
		{"unknown operation", func(v wire.Value) { setMember(entry(v, 0), "operation", str502("EDIT")) }},
		{"missing text", func(v wire.Value) { dropMember(entry(v, 0), "text") }},
		{"blank text", func(v wire.Value) { setMember(entry(v, 0), "text", str502("  ")) }},
		{"long text", func(v wire.Value) { setMember(entry(v, 0), "text", str502(strings.Repeat("t", 1025))) }},
		{"seq gap", func(v wire.Value) { setMember(entry(v, 0), "seq", str502("2")) }},
		{"bad commit", func(v wire.Value) { setMember(entry(v, 0), "commit", str502("HEAD")) }},
		{"no anchors", func(v wire.Value) { setMember(entry(v, 0), "anchors", arr()) }},
		{"absolute anchor", func(v wire.Value) { setMember(anchor(v), "path", str502("/etc/passwd")) }},
		{"dotdot anchor", func(v wire.Value) { setMember(anchor(v), "path", str502("../x.go")) }},
		{"directory anchor", func(v wire.Value) { setMember(anchor(v), "path", str502("internal/")) }},
		{"bad blob", func(v wire.Value) { setMember(anchor(v), "blob", str502("00")) }},
		{"duplicate anchors", func(v wire.Value) {
			e := value(entry(v, 1), "anchors")
			setMember(entry(v, 1), "anchors", arr(e.Arr[0], e.Arr[0]))
		}},
		{"unsorted routes", func(v wire.Value) {
			r := value(entry(v, 1), "routes")
			setMember(entry(v, 1), "routes", arr(r.Arr[1], r.Arr[0]))
		}},
		{"supersede without reason", func(v wire.Value) { setMember(entry(v, 1), "reason", wire.Value{Kind: wire.KindNull}) }},
		{"reason without supersede", func(v wire.Value) { setMember(entry(v, 0), "reason", str502("why")) }},
		{"retract of superseded note", func(v wire.Value) { setMember(entry(v, 3), "note", str502("1")) }},
		{"forward target", func(v wire.Value) { setMember(entry(v, 1), "supersedes", str502("4")) }},
		{"worker actor", func(v wire.Value) { setMember(value(entry(v, 0), "actor"), "role", str502("WORKER")) }},
		{"bad time", func(v wire.Value) { setMember(entry(v, 3), "recordedAt", str502("yesterday")) }},
		{"retract text", func(v wire.Value) { setMember(entry(v, 3), "text", str502("x")) }},
		{"retract without reason", func(v wire.Value) { setMember(entry(v, 3), "reason", wire.Value{Kind: wire.KindNull}) }},
	}
	if _, e := decodeTicket(issue502Record(t)); e != nil {
		t.Fatalf("fixture: %v", e)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := issue502Record(t)
			c.edit(v)
			if _, e := decodeTicket(v); e == nil {
				t.Fatal("decoded a malformed knowHow member")
			}
		})
	}
}

// TestKHNV0023_ReaderWorkerEntry: Core's reader admits a WORKER entry only as
// a non-superseding ADD that records its attempt and generation, mirroring
// the Tasks codec.
func TestKHNV0023_ReaderWorkerEntry(t *testing.T) {
	entry := func(v wire.Value, i int) wire.Value { return value(v, "knowHow").Arr[i] }
	worker := func(v wire.Value, i int) { setMember(value(entry(v, i), "actor"), "role", str502("WORKER")) }
	v := issue502Record(t)
	worker(v, 2)
	if _, e := decodeTicket(v); e != nil {
		t.Fatalf("WORKER ADD with attempt and generation: %v", e)
	}
	for name, edit := range map[string]func(wire.Value){
		"superseding ADD": func(v wire.Value) { worker(v, 1) },
		"RETRACT":         func(v wire.Value) { worker(v, 3) },
		"no attempt": func(v wire.Value) {
			worker(v, 2)
			setMember(entry(v, 2), "attempt", wire.Value{Kind: wire.KindNull})
		},
		"no generation": func(v wire.Value) {
			worker(v, 2)
			setMember(entry(v, 2), "generation", wire.Value{Kind: wire.KindNull})
		},
	} {
		t.Run(name, func(t *testing.T) {
			v := issue502Record(t)
			edit(v)
			if _, e := decodeTicket(v); e == nil {
				t.Fatal("decoded a WORKER entry outside KHN-V0-023")
			}
		})
	}
}

// TestKHNV0020_ReaderKnowHowSymbolsAndReconfirm: Core's reader admits a
// symbol anchor and a RECONFIRM that re-pins an active note, and refuses the
// same forms the Tasks codec refuses (KHN-V0-016, KHN-V0-018, KHN-V0-019), so
// a record the new codec writes never reads differently in Core.
func TestKHNV0020_ReaderKnowHowSymbolsAndReconfirm(t *testing.T) {
	const (
		blobA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		blobD = "dddddddddddddddddddddddddddddddddddddddd"
		sha1  = "1111111111111111111111111111111111111111111111111111111111111111"
		sha2  = "2222222222222222222222222222222222222222222222222222222222222222"
		ctx   = `"actor":{"id":"russell","role":"OWNER"},"attempt":null,"commit":"cccccccccccccccccccccccccccccccccccccccc","generation":null`
	)
	symAdd := `{` + ctx + `,"anchors":[{"blob":"` + blobA + `","path":"internal/a.go","symbol":"T.M","symbolSha256":"` + sha1 + `"}],"evidencePath":null,"operation":"ADD","reason":null,"recordedAt":"2026-09-07T14:00:00Z","routes":[],"seq":"5","supersedes":null,"text":"M owns the lock"}`
	reconfirm := func(seq, note, anchors string) string {
		return `{` + ctx + `,"anchors":[` + anchors + `],"note":"` + note + `","operation":"RECONFIRM","recordedAt":"2026-09-08T14:00:00Z","seq":"` + seq + `"}`
	}
	symbol := func(blob, sym, digest string) string {
		return `{"blob":"` + blob + `","path":"internal/a.go","symbol":"` + sym + `","symbolSha256":"` + digest + `"}`
	}
	fileA := func(blob string) string { return `{"blob":"` + blob + `","path":"internal/a.go"}` }
	fileB := `{"blob":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","path":"internal/b.go"}`
	record := func(t *testing.T, extra ...string) wire.Value {
		v := issue502Record(t)
		ledger := value(v, "knowHow")
		for _, raw := range extra {
			e, err := wire.Parse([]byte(raw))
			if err != nil {
				t.Fatalf("entry %s: %v", raw, err)
			}
			ledger.Arr = append(ledger.Arr, e)
		}
		setMember(v, "knowHow", ledger)
		return v
	}
	for name, extra := range map[string][]string{
		"symbol add":                {symAdd},
		"reconfirm a symbol digest": {symAdd, reconfirm("6", "5", symbol(blobD, "T.M", sha2))},
		"reconfirm a file anchor":   {symAdd, reconfirm("6", "2", fileA(blobD)+","+fileB)},
		"reconfirm twice":           {reconfirm("5", "2", fileA(blobD)+","+fileB), reconfirm("6", "2", fileA(blobA)+","+fileB)},
		"supersede after reconfirm": {reconfirm("5", "2", fileA(blobD)+","+fileB), strings.Replace(strings.Replace(strings.Replace(symAdd, `"supersedes":null`, `"supersedes":"2"`, 1), `"reason":null`, `"reason":"moved"`, 1), `"seq":"5"`, `"seq":"6"`, 1)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := decodeTicket(record(t, extra...)); e != nil {
				t.Fatalf("refused: %v", e)
			}
		})
	}
	for name, extra := range map[string][]string{
		"unchanged pins":         {reconfirm("5", "2", fileA(blobA)+","+fileB)},
		"unchanged symbol pin":   {symAdd, reconfirm("6", "5", symbol(blobA, "T.M", sha1))},
		"only the blob changed":  {symAdd, reconfirm("6", "5", symbol(blobD, "T.M", sha1))},
		"dropped anchor":         {reconfirm("5", "2", fileA(blobD))},
		"renamed symbol":         {symAdd, reconfirm("6", "5", symbol(blobD, "T.N", sha2))},
		"symbol became a file":   {symAdd, reconfirm("6", "5", fileA(blobD))},
		"forward note":           {reconfirm("5", "6", fileA(blobD)+","+fileB)},
		"superseded note":        {reconfirm("5", "1", fileA(blobD))},
		"retracted note":         {reconfirm("5", "3", fileA(blobD))},
		"reason key":             {strings.Replace(reconfirm("5", "2", fileA(blobD)+","+fileB), `"recordedAt"`, `"reason":"x","recordedAt"`, 1)},
		"symbol with a space":    {strings.Replace(symAdd, "T.M", "T M", 1)},
		"symbol with a hash":     {strings.Replace(symAdd, "T.M", "T#M", 1)},
		"symbol without digest":  {strings.Replace(symAdd, `,"symbolSha256":"`+sha1+`"`, "", 1)},
		"bad digest":             {strings.Replace(symAdd, sha1, "abc", 1)},
		"two blobs in one path":  {strings.Replace(symAdd, `"anchors":[`, `"anchors":[`+fileA(blobD)+",", 1)},
		"symbol before the file": {strings.Replace(symAdd, `"anchors":[`+symbol(blobA, "T.M", sha1), `"anchors":[`+symbol(blobA, "T.M", sha1)+","+fileA(blobA), 1)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := decodeTicket(record(t, extra...)); e == nil {
				t.Fatal("decoded a malformed knowHow member")
			}
		})
	}
}
