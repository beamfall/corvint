package ticket_test

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const tolRef = `{"counts":{"coreTotal":"2","coreWitnessed":"1","deferred":"1","total":"3","witnessed":"2"},` +
	`"head":"` + "e1a8b6ae3d8b80dff469b369f9fcf79fb98bbb45d6f20150c817bc6b4066df62" + `","highWater":{"acceptanceRevision":"1","witnessed":"2"},` +
	`"lastRaise":{"acceptanceRevision":"1","attempt":"attempt:acme:main:b30a2bde9265683313779c2922bdca29","generation":"1"},"prefix":"AC","revision":"4"}`

func tolParse(t *testing.T, s string) wire.Value {
	t.Helper()
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	v, err := wire.Parse([]byte(s))
	if err != nil {
		t.Fatalf("%v: %s", err, s)
	}
	return v
}

// tolRecord is the shared optional-key fixture with an obligations member
// edited by edit.
func tolRecord(t *testing.T, edit func(rec, ref wire.Value)) []byte {
	t.Helper()
	raw, err := os.ReadFile(issue502RecordFixture)
	if err != nil {
		t.Fatal(err)
	}
	v := tolParse(t, string(raw))
	ref := tolParse(t, tolRef)
	v.Obj.Set("obligations", ref)
	if edit != nil {
		edit(v, ref)
	}
	return wire.EncodeFile(v)
}

// TestTOLV0001_RecordMemberRoundTrip: a NATIVE record carries the closed
// obligations reference and re-encodes to the same bytes; a record without
// it keeps its legacy bytes; each malformed or inconsistent reference, and
// the member on a non-NATIVE record, refuses.
func TestTOLV0001_RecordMemberRoundTrip(t *testing.T) {
	fv := tolParse(t, string(tolRecord(t, nil)))
	delete(fv.Obj.Vals, "obligations")
	fv.Obj.Keys = slices.DeleteFunc(fv.Obj.Keys, func(k string) bool { return k == "obligations" })
	legacy := wire.EncodeFile(fv)
	if rec, err := ticket.Decode(legacy); err != nil || rec.ObligationsRef != nil || !bytes.Equal(rec.Encode(), legacy) {
		t.Fatalf("legacy record: %v", err)
	}
	raw := tolRecord(t, nil)
	rec, err := ticket.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if o := rec.ObligationsRef; o == nil || o.Prefix != "AC" || o.Revision != "4" || o.Counts.Witnessed != 2 || o.LastRaise == nil {
		t.Fatalf("reference: %+v", rec.ObligationsRef)
	}
	if !bytes.Equal(rec.Encode(), raw) {
		t.Fatalf("round trip:\n%s\n%s", raw, rec.Encode())
	}
	set := func(v wire.Value, path string, x wire.Value) {
		keys := strings.Split(path, ".")
		for _, k := range keys[:len(keys)-1] {
			v, _ = v.Obj.Get(k)
		}
		v.Obj.Set(keys[len(keys)-1], x)
	}
	s := wire.String
	cases := map[string]func(rec, ref wire.Value){
		"unknown key":             func(_, ref wire.Value) { set(ref, "extra", s("x")) },
		"lowercase prefix":        func(_, ref wire.Value) { set(ref, "prefix", s("ac")) },
		"digit-first prefix":      func(_, ref wire.Value) { set(ref, "prefix", s("1A")) },
		"long prefix":             func(_, ref wire.Value) { set(ref, "prefix", s("ABCDEFGHIJKLMNOPQ")) },
		"zero revision":           func(_, ref wire.Value) { set(ref, "revision", s("0")) },
		"revision over 1024":      func(_, ref wire.Value) { set(ref, "revision", s("1025")) },
		"uppercase head":          func(_, ref wire.Value) { set(ref, "head", s(strings.ToUpper(strings.Repeat("e1", 32)))) },
		"non-canonical count":     func(_, ref wire.Value) { set(ref, "counts.total", s("03")) },
		"witnessed over total":    func(_, ref wire.Value) { set(ref, "counts.witnessed", s("4")) },
		"core over total":         func(_, ref wire.Value) { set(ref, "counts.coreTotal", s("4")) },
		"over 256 entries":        func(_, ref wire.Value) { set(ref, "counts.deferred", s("254")) },
		"high water below count":  func(_, ref wire.Value) { set(ref, "highWater.witnessed", s("1")) },
		"high water postdates":    func(_, ref wire.Value) { set(ref, "highWater.acceptanceRevision", s("2")) },
		"last raise postdates":    func(_, ref wire.Value) { set(ref, "lastRaise.acceptanceRevision", s("2")) },
		"last raise missing key":  func(_, ref wire.Value) { ref.Obj.Set("lastRaise", tolParse(t, `{"attempt":"a","generation":"1"}`)) },
		"non-native record":       func(rec, _ wire.Value) { set(rec, "source.kind", s("IMPORTED")) },
		"obligations is not null": func(_, ref wire.Value) { ref.Obj.Set("counts", wire.Null()) },
	}
	for name, edit := range cases {
		if _, err := ticket.Decode(tolRecord(t, edit)); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
	nullRaise := tolRecord(t, func(_, ref wire.Value) { ref.Obj.Set("lastRaise", wire.Null()) })
	if rec, err := ticket.Decode(nullRaise); err != nil || rec.ObligationsRef.LastRaise != nil || !bytes.Equal(rec.Encode(), nullRaise) {
		t.Fatalf("null lastRaise: %v", err)
	}
}

// tolRequest is a canonical retained OBLIGATIONS_* envelope.
func tolRequest(t *testing.T, req, op, expected, payload string) []byte {
	t.Helper()
	return wire.EncodeFile(tolParse(t, `{"actor":{"id":"owner","role":"OWNER"},"expectedRevision":"`+expected+`","issuedAt":"2026-10-08T12:00:00Z",`+
		`"operation":"`+op+`","payload":`+payload+`,"profile":"taskman-mutation/0","queueId":"queue:acme:main","requestId":"`+req+`",`+
		`"targetId":"ticket:acme:main:AT-01"}`))
}

const tolSeed = `{"obligations":[{"core":true,"id":"AC-1","title":"one"},{"core":false,"id":"AC-2","title":"two"}],"prefix":"AC"}`

func tolEvent(t *testing.T, rev int64, previous *wire.Digest, request []byte) []byte {
	t.Helper()
	id, err := wire.ParseTicketID("/ticketId", "ticket:acme:main:AT-01")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ticket.ObligationEvent{TicketID: id, Revision: wire.CountOf(rev), Previous: previous, RequestSha256: wire.Sum(request), Request: request}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestTOLV0002_EventCanonicalAndChained: an event is the closed canonical
// {profile, ticketId, revision, previous, requestSha256, request} retaining
// the whole request; a chain folds from the first event to head and refuses
// a broken link, a missing event or a wrong head.
func TestTOLV0002_EventCanonicalAndChained(t *testing.T) {
	seed := tolRequest(t, "seed-1", ticket.OpObligationsSeed, "1", tolSeed)
	first := tolEvent(t, 1, nil, seed)
	ev, err := ticket.DecodeObligationEvent(first)
	if err != nil || !bytes.Equal(ev.Request, seed) || ev.Decoded.Seed == nil || ev.Decoded.IssuedAt != "2026-10-08T12:00:00Z" {
		t.Fatalf("first event: %v", err)
	}
	if !bytes.Equal(wire.EncodeFile(tolParse(t, string(first))), first) || !strings.HasPrefix(string(first), `{"previous":null,"profile":"`+ticket.ObligationEventProfile+`"`) {
		t.Fatalf("event is not canonical: %s", first)
	}
	h1 := wire.Sum(first)
	set := tolRequest(t, "set-1", ticket.OpObligationsSet, "2", `{"changes":[{"core":null,"id":"AC-2","reason":"later","state":"DEFERRED"}]}`)
	second := tolEvent(t, 2, &h1, set)
	h2 := wire.Sum(second)
	blobs := map[wire.Digest][]byte{h1: first, h2: second}
	read := func(d wire.Digest) ([]byte, error) { return blobs[d], nil }
	id, _ := wire.ParseTicketID("/ticketId", "ticket:acme:main:AT-01")
	ref := ticket.ObligationsReference{Prefix: "AC", Revision: "2", Head: h2, Counts: ticket.ObligationCounts{Total: 1, Deferred: 1, CoreTotal: 1}}
	l, err := ticket.FoldObligationChain(id, ref, read)
	if err != nil {
		t.Fatal(err)
	}
	if c := l.Counts(); c.Total != 1 || c.Deferred != 1 || c.CoreTotal != 1 || l.Entry("AC-2").State != ticket.ObligationDeferred || l.Revision != 2 {
		t.Fatalf("fold: %+v %s", c, wire.Encode(l.EntriesValue()))
	}
	for name, mutate := range map[string]func(){
		"wrong revision": func() { ref.Revision = "3" },
		"missing event":  func() { delete(blobs, h1) },
		"unknown head":   func() { ref.Head = wire.Sum([]byte("x")) },
		"wrong counts":   func() { ref.Counts.Total = 2 },
		"wrong prefix":   func() { ref.Prefix = "AB" },
	} {
		saved, savedRef := map[wire.Digest][]byte{h1: first, h2: second}, ref
		mutate()
		if _, err := ticket.FoldObligationChain(id, ref, read); err == nil || !strings.Contains(err.Error(), ticket.ObligationChainDetail) {
			t.Errorf("%s: %v", name, err)
		}
		blobs, ref = saved, savedRef
	}
	// A corrupt store whose event points back at its own storage key must not
	// loop: collection stops at the revisit and the fold refuses.
	loopKey := wire.Sum([]byte("loop"))
	loop := map[wire.Digest][]byte{loopKey: tolEvent(t, 2, &loopKey, set)}
	loopRead := func(d wire.Digest) ([]byte, error) { return loop[d], nil }
	loopRef := ticket.ObligationsReference{Prefix: "AC", Revision: "2", Head: loopKey}
	if got, err := ticket.ObligationChainEvents(loopRead, &loopRef); err != nil || len(got) != 1 {
		t.Fatalf("cyclic collection: %d %v", len(got), err)
	}
	if _, err := ticket.FoldObligationChain(id, loopRef, loopRead); err == nil || !strings.Contains(err.Error(), ticket.ObligationChainDetail) {
		t.Fatalf("cyclic fold: %v", err)
	}
	edit := func(raw []byte, f func(v wire.Value)) []byte {
		v := tolParse(t, string(raw))
		f(v)
		return wire.EncodeFile(v)
	}
	for name, raw := range map[string][]byte{
		"non-canonical":       append([]byte(" "), first...),
		"unknown key":         edit(first, func(v wire.Value) { v.Obj.Set("extra", wire.Null()) }),
		"digest mismatch":     edit(first, func(v wire.Value) { v.Obj.Set("requestSha256", wire.String(string(wire.Sum(set)))) }),
		"second without link": edit(second, func(v wire.Value) { v.Obj.Set("previous", wire.Null()) }),
		"first with link":     edit(first, func(v wire.Value) { v.Obj.Set("previous", wire.String(string(h2))) }),
		"other ticket":        edit(first, func(v wire.Value) { v.Obj.Set("ticketId", wire.String("ticket:acme:main:AT-02")) }),
		"not an obligation":   edit(first, func(v wire.Value) { r, _ := v.Obj.Get("request"); r.Obj.Set("operation", wire.String("REFINE")) }),
	} {
		if _, err := ticket.DecodeObligationEvent(raw); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
}

// TestTOLV0002_EventSlotBound: an event of exactly 65,536 bytes decodes and
// one byte more refuses LIMIT_EXCEEDED OBLIGATION_EVENT_TOO_LARGE:.
func TestTOLV0002_EventSlotBound(t *testing.T) {
	base := tolEvent(t, 1, nil, tolRequest(t, "seed-1", ticket.OpObligationsSeed, "1", tolSeed))
	// Titles are bounded at 256 bytes, so the size is reached through a
	// many-entry seed instead of one long title.
	var items []string
	for i := 1; i <= 256; i++ {
		items = append(items, `{"core":false,"id":"AC-`+itoa(i)+`","title":"`+strings.Repeat("x", 230)+`"}`)
	}
	sortIDs(items)
	over := tolRequest(t, "seed-1", ticket.OpObligationsSeed, "1", `{"obligations":[`+strings.Join(items, ",")+`],"prefix":"AC"}`)
	id, _ := wire.ParseTicketID("/ticketId", "ticket:acme:main:AT-01")
	_, err := ticket.ObligationEvent{TicketID: id, Revision: "1", RequestSha256: wire.Sum(over), Request: over}.Encode()
	if wire.CodeOf(err) != wire.CodeLimitExceeded || !strings.Contains(err.Error(), ticket.ObligationEventTooLargeDetail) {
		t.Fatalf("oversized event: %v", err)
	}
	filler := ticket.MaxObligationEventBytes - len(base)
	padded := append([]byte(strings.Repeat(" ", filler)), base...)
	if len(padded) != ticket.MaxObligationEventBytes {
		t.Fatalf("filler arithmetic: %d", len(padded))
	}
	// A 65,536-byte input reaches the shape checks (and is refused there as
	// non-canonical); 65,537 bytes is refused by size before parsing.
	if _, err := ticket.DecodeObligationEvent(padded); err == nil || strings.Contains(err.Error(), ticket.ObligationEventTooLargeDetail) {
		t.Fatalf("65,536 bytes: %v", err)
	}
	if _, err := ticket.DecodeObligationEvent(append([]byte(" "), padded...)); wire.CodeOf(err) != wire.CodeLimitExceeded ||
		!strings.Contains(err.Error(), ticket.ObligationEventTooLargeDetail) {
		t.Fatalf("65,537 bytes: %v", err)
	}
}

func sortIDs(items []string) {
	// Ids sort as strings: AC-1, AC-10, AC-100, ...
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j] < items[j-1]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func tolEntry(t *testing.T, edit func(e, ev wire.Value)) (*ticket.ObligationEntry, error) {
	t.Helper()
	e := tolParse(t, `{"core":true,"evidence":{"commit":"`+strings.Repeat("c", 40)+`","declaration":null,"eventSha256":"`+strings.Repeat("e", 64)+`",`+
		`"matches":[{"path":"e2e/login.spec.ts","stepPath":["sign in"],"stepTitle":"sign in","testId":"s1@chromium","titlePath":["login","AC-1 works"]}],`+
		`"playwrightVersion":"1.63.0","reportSha256":"`+strings.Repeat("a", 64)+`","source":"PLAYWRIGHT_REPORT"},"id":"AC-1","reason":null,`+
		`"state":"WITNESSED","title":"one","updatedAt":"2026-10-08T12:00:00Z","updatedBy":{"id":"owner","role":"OWNER"}}`)
	ev, _ := e.Obj.Get("evidence")
	if edit != nil {
		edit(e, ev)
	}
	r := wire.NewReader(e, "/entry")
	got := ticket.ReadObligationEntry(r)
	return got, r.Err()
}

// TestTOLV0003_EntryCodecRefusals: the closed entry round-trips and each
// out-of-shape id, title, state, reason, evidence pairing or actor refuses.
func TestTOLV0003_EntryCodecRefusals(t *testing.T) {
	e, err := tolEntry(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := wire.Encode(tolParse(t, string(wire.Encode(e.Value()))))
	if back, _ := tolEntry(t, nil); !bytes.Equal(wire.Encode(back.Value()), want) || e.UpdatedAt != "2026-10-08T12:00:00Z" {
		t.Fatal("entry does not round-trip")
	}
	s := wire.String
	for name, edit := range map[string]func(e, ev wire.Value){
		"unknown key":          func(e, _ wire.Value) { e.Obj.Set("extra", wire.Null()) },
		"leading-zero id":      func(e, _ wire.Value) { e.Obj.Set("id", s("AC-01")) },
		"seven-digit id":       func(e, _ wire.Value) { e.Obj.Set("id", s("AC-1000000")) },
		"lowercase id":         func(e, _ wire.Value) { e.Obj.Set("id", s("ac-1")) },
		"blank title":          func(e, _ wire.Value) { e.Obj.Set("title", s("  ")) },
		"long title":           func(e, _ wire.Value) { e.Obj.Set("title", s(strings.Repeat("t", 257))) },
		"string core":          func(e, _ wire.Value) { e.Obj.Set("core", s("true")) },
		"unknown state":        func(e, _ wire.Value) { e.Obj.Set("state", s("DONE")) },
		"long reason":          func(e, _ wire.Value) { e.Obj.Set("reason", s(strings.Repeat("r", 513))) },
		"evidence while OPEN":  func(e, _ wire.Value) { e.Obj.Set("state", s("OPEN")) },
		"WITNESSED without":    func(e, _ wire.Value) { e.Obj.Set("evidence", wire.Null()) },
		"bad time":             func(e, _ wire.Value) { e.Obj.Set("updatedAt", s("yesterday")) },
		"reviewer actor":       func(e, _ wire.Value) { by, _ := e.Obj.Get("updatedBy"); by.Obj.Set("role", s("REVIEWER")) },
		"missing updatedBy id": func(e, _ wire.Value) { e.Obj.Set("updatedBy", tolParse(t, `{"role":"OWNER"}`)) },
	} {
		if _, err := tolEntry(t, edit); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
	for _, ok := range []func(e, ev wire.Value){
		func(e, _ wire.Value) { e.Obj.Set("id", s("AC-999999")) },
		func(e, _ wire.Value) { e.Obj.Set("title", s(strings.Repeat("t", 256))) },
		func(e, _ wire.Value) {
			e.Obj.Set("state", s("DEFECT"))
			e.Obj.Set("evidence", wire.Null())
			e.Obj.Set("reason", s(strings.Repeat("r", 512)))
		},
	} {
		if _, err := tolEntry(t, ok); err != nil {
			t.Errorf("boundary refused: %v", err)
		}
	}
}

// TestTOLV0004_EvidenceCodec: report evidence carries report digest,
// version and ordered matches with a null declaration; DECLARED evidence
// carries only the declaration; mixing, a bad commit or an empty match list
// refuses.
func TestTOLV0004_EvidenceCodec(t *testing.T) {
	s := wire.String
	declared := func(_, ev wire.Value) {
		ev.Obj.Set("source", s("DECLARED")).Set("reportSha256", wire.Null()).Set("playwrightVersion", wire.Null()).Set("matches", wire.Null()).
			Set("declaration", tolParse(t, `{"manifestSha256":"`+strings.Repeat("d", 64)+`","reason":"checked by hand","testId":"manual"}`))
	}
	if e, err := tolEntry(t, declared); err != nil || e.Evidence.Declaration == nil || e.Evidence.Matches != nil {
		t.Fatalf("declared evidence: %v", err)
	}
	e, err := tolEntry(t, nil)
	if err != nil || e.Evidence.Matches[0].StepTitle == nil || strings.Join(e.Evidence.Matches[0].TitlePath, "/") != "login/AC-1 works" {
		t.Fatalf("report evidence: %v", err)
	}
	unsorted := func(_, ev wire.Value) {
		m, _ := ev.Obj.Get("matches")
		m.Arr[0].Obj.Set("titlePath", tolParse(t, `["zeta","alpha"]`)).Set("stepTitle", wire.Null()).Set("stepPath", tolParse(t, `[]`))
	}
	if e, err := tolEntry(t, unsorted); err != nil || e.Evidence.Matches[0].TitlePath[0] != "zeta" {
		t.Fatalf("semantic title order or test-level match: %v", err)
	}
	if _, err := tolEntry(t, func(_, ev wire.Value) { ev.Obj.Set("commit", s(strings.Repeat("c", 64))) }); err != nil {
		t.Fatalf("64-hex commit: %v", err)
	}
	for name, edit := range map[string]func(e, ev wire.Value){
		"unknown source":   func(_, ev wire.Value) { ev.Obj.Set("source", s("CI")) },
		"short commit":     func(_, ev wire.Value) { ev.Obj.Set("commit", s("abc123")) },
		"symbolic commit":  func(_, ev wire.Value) { ev.Obj.Set("commit", s("HEAD")) },
		"bad event digest": func(_, ev wire.Value) { ev.Obj.Set("eventSha256", s("x")) },
		"report with decl": func(_, ev wire.Value) { declared(wire.Value{}, ev); ev.Obj.Set("source", s("PLAYWRIGHT_REPORT")) },
		"declared with report": func(_, ev wire.Value) {
			declared(wire.Value{}, ev)
			ev.Obj.Set("reportSha256", s(strings.Repeat("a", 64)))
		},
		"report without version": func(_, ev wire.Value) { ev.Obj.Set("playwrightVersion", wire.Null()) },
		"empty matches":          func(_, ev wire.Value) { ev.Obj.Set("matches", tolParse(t, `[]`)) },
		"match unknown key": func(_, ev wire.Value) {
			m, _ := ev.Obj.Get("matches")
			m.Arr[0].Obj.Set("extra", wire.Null())
		},
		"absolute match path": func(_, ev wire.Value) {
			m, _ := ev.Obj.Get("matches")
			m.Arr[0].Obj.Set("path", s("/etc/passwd"))
		},
		"unknown key": func(_, ev wire.Value) { ev.Obj.Set("extra", wire.Null()) },
	} {
		if _, err := tolEntry(t, edit); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
}
