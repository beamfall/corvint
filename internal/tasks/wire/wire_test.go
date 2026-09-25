package wire

import (
	"strings"
	"testing"
)

func codeOf(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("error is not a *wire.Error: %v", err)
	}
	return e.Code
}

// uesc renders a JSON `\uXXXX` escape from its four hex digits. The escape
// is assembled at run time so that no `\u` sequence, NUL or byte order mark
// ever appears literally in this source file (the Go lexer refuses NUL and
// a BOM, and an editor may silently translate `\u` escapes).
func uesc(hex4 string) string { return "\\" + "u" + hex4 }

// rn renders a rune as raw UTF-8.
func rn(cp rune) string { return string(cp) }

// TestTMV0002_AS10_CountBoundaries covers 0, max, max+1, leading zero and
// sign for Count (SPEC §2, AS-10).
func TestTMV0002_AS10_CountBoundaries(t *testing.T) {
	cases := []struct {
		in   string
		want string // "" = accepted
	}{
		{"0", ""},
		{"1", ""},
		{"2147483647", ""},
		{"2147483648", CodeMalformed},
		{"01", CodeMalformed},
		{"-1", CodeMalformed},
		{"+1", CodeMalformed},
		{"", CodeMalformed},
		{"1 ", CodeMalformed},
		{"1e3", CodeMalformed},
		{"18446744073709551615", CodeMalformed},
	}
	for _, c := range cases {
		_, err := ParseCount("/x", c.in)
		if got := codeOf(t, err); got != c.want {
			t.Errorf("ParseCount(%q): code %q, want %q (err %v)", c.in, got, c.want, err)
		}
	}
}

// TestTMV0002_AS10_SizeBoundaries covers the uint64 Size primitive.
func TestTMV0002_AS10_SizeBoundaries(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0", ""},
		{"2147483648", ""},
		{"18446744073709551615", ""},
		{"18446744073709551616", CodeMalformed},
		{"99999999999999999999", CodeMalformed},
		{"00", CodeMalformed},
		{"-0", CodeMalformed},
		{"+5", CodeMalformed},
		{"", CodeMalformed},
	}
	for _, c := range cases {
		_, err := ParseSize("/x", c.in)
		if got := codeOf(t, err); got != c.want {
			t.Errorf("ParseSize(%q): code %q, want %q", c.in, got, c.want)
		}
	}
	if Size("18446744073709551615").Uint64() != ^uint64(0) {
		t.Errorf("Size max does not round-trip through Uint64")
	}
}

// TestTMV0002_AS01_ParseFraming covers canonical framing: trailing LF,
// whitespace, BOM, numbers, key order, duplicate keys and depth.
func TestTMV0002_AS01_ParseFraming(t *testing.T) {
	deep := strings.Repeat("[", 25) + strings.Repeat("]", 25) + "\n"
	ok24 := strings.Repeat("[", 24) + strings.Repeat("]", 24) + "\n"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"canonical object", `{"a":"1","b":null}` + "\n", ""},
		{"empty object", "{}\n", ""},
		{"empty array", "[]\n", ""},
		{"missing LF", `{"a":"1"}`, CodeMalformed},
		{"two LF", `{"a":"1"}` + "\n\n", CodeMalformed},
		{"CRLF", `{"a":"1"}` + "\r\n", CodeMalformed},
		{"whitespace", `{"a": "1"}` + "\n", CodeMalformed},
		{"unsorted keys", `{"b":"1","a":"1"}` + "\n", CodeMalformed},
		{"duplicate key", `{"a":"1","a":"2"}` + "\n", CodeMalformed},
		{"number", `{"a":1}` + "\n", CodeMalformed},
		{"negative number", "[-1]\n", CodeMalformed},
		{"BOM", "\xEF\xBB\xBF{}\n", CodeMalformed},
		{"empty", "", CodeMalformed},
		{"trailing bytes", "{}x\n", CodeMalformed},
		{"optional escape", `["` + uesc("0041") + `"]` + "\n", CodeMalformed},
		{"solidus escape", `["\/"]` + "\n", CodeMalformed},
		{"uppercase hex escape", `["` + uesc("001F") + `"]` + "\n", CodeMalformed},
		{"lowercase control escape is still hostile", `["` + uesc("001f") + `"]` + "\n", CodeMalformed},
		{"DEL is raw, not escaped", "[\"\x7f\"]\n", ""},
		{"DEL escaped is non-canonical", `["` + uesc("007f") + `"]` + "\n", CodeMalformed},
		{"short escapes", `["\t\n\r\"\\"]` + "\n", ""},
		{"depth 24", ok24, ""},
		{"depth 25", deep, CodeLimitExceeded},
		{"invalid utf8", "[\"\xff\"]\n", CodeMalformed},
		{"raw control", "[\"\x01\"]\n", CodeMalformed},
		{"literal true", "[true,false,null]\n", ""},
		{"bad literal", "[tru]\n", CodeMalformed},
	}
	for _, c := range cases {
		_, err := Parse([]byte(c.in))
		if got := codeOf(t, err); got != c.want {
			t.Errorf("%s: code %q, want %q (err %v)", c.name, got, c.want, err)
		}
	}
}

// TestTMV0002_AS01_HostileCodePoints covers NUL, C1 controls, bidi
// controls, BOM inside strings and lone surrogates, each as the raw UTF-8
// code point and as its `\u` escape. Every hostile byte is constructed
// (uesc, r, byte slices), never written literally in the source.
func TestTMV0002_AS01_HostileCodePoints(t *testing.T) {
	quoted := func(s string) string { return "[\"" + s + "\"]\n" }
	cases := []struct {
		name string
		in   string
	}{
		{"NUL escape", quoted(uesc("0000"))},
		{"NUL raw", quoted(string([]byte{0x00}))},
		{"C1 control raw (NEL)", quoted(rn(0x85))},
		{"C1 control escape", quoted(uesc("0085"))},
		{"C1 control raw (APC)", quoted(rn(0x9F))},
		{"LRM raw", quoted("a" + rn(0x200E) + "b")},
		{"LRM escape", quoted("a" + uesc("200e") + "b")},
		{"RLM raw", quoted("a" + rn(0x200F) + "b")},
		{"RLO raw", quoted("a" + rn(0x202E) + "b")},
		{"RLO escape", quoted("a" + uesc("202e") + "b")},
		{"isolate raw", quoted("a" + rn(0x2066) + "b")},
		{"isolate escape", quoted("a" + uesc("2069") + "b")},
		{"ALM raw", quoted(rn(0x061C))},
		{"BOM inside raw", quoted(rn(0xFEFF))},
		{"BOM inside escape", quoted(uesc("feff"))},
		{"BOM inside raw bytes", quoted(string([]byte{0xEF, 0xBB, 0xBF}))},
		{"lone high surrogate", quoted(uesc("d800"))},
		{"lone low surrogate", quoted(uesc("dc00"))},
		{"high without low", quoted(uesc("d800") + "A")},
		{"surrogate as raw bytes", quoted(string([]byte{0xED, 0xA0, 0x80}))},
	}
	for _, c := range cases {
		if _, err := Parse([]byte(c.in)); codeOf(t, err) != CodeMalformed {
			t.Errorf("%s: accepted hostile input (err %v)", c.name, err)
		}
	}
	if _, err := Parse([]byte(quoted(rn(0x1F600) + " caf" + rn(0xE9)))); err != nil {
		t.Errorf("valid non-ASCII refused: %v", err)
	}
	if _, err := Parse([]byte(quoted(uesc("d83d") + uesc("de00")))); codeOf(t, err) != CodeMalformed {
		t.Errorf("escaped surrogate pair is non-canonical (raw UTF-8 required) but was accepted")
	}
}

// TestTMV0002_AS01_EncodeRoundTrip proves Encode(Parse(x)) == x for
// canonical documents and that Encode sorts keys and escapes canonically.
func TestTMV0002_AS01_EncodeRoundTrip(t *testing.T) {
	// Keys sort by UTF-8 bytes (§2): "z" (0x7A) precedes "é" (0xC3 0xA9),
	// so the canonical witness puts the ASCII key first.
	docs := []string{
		"{}\n", "[]\n", `{"a":[{"b":"x"}],"c":"\t\n\r\"\\"}` + "\n",
		`{"z":true,"` + rn(0xE9) + `":"unicode key"}` + "\n",
	}
	for _, d := range docs {
		v, err := Parse([]byte(d))
		if err != nil {
			t.Fatalf("parse %q: %v", d, err)
		}
		if string(EncodeFile(v)) != d {
			t.Errorf("round trip differs: %q -> %q", d, EncodeFile(v))
		}
	}
	// The same object with the non-ASCII key first is byte-unsorted and
	// therefore non-canonical; the negative witness stays MALFORMED.
	if _, err := Parse([]byte(`{"` + rn(0xE9) + `":"unicode key","z":true}` + "\n")); codeOf(t, err) != CodeMalformed {
		t.Errorf("unicode key before ASCII key is unsorted by UTF-8 bytes and must be MALFORMED: %v", err)
	}
	o := NewObject().Set("zeta", String("1")).Set("alpha", String("2"))
	if got := string(Encode(ObjectValue(o))); got != `{"alpha":"2","zeta":"1"}` {
		t.Errorf("keys not sorted: %s", got)
	}
	if got := string(Encode(String("a/b\x7f"))); got != "\"a/b\x7f\"" {
		t.Errorf("solidus or DEL escaped unexpectedly: %s", got)
	}
	// A control byte the encoder must escape is rendered as lowercase \u00xx.
	if got := string(Encode(String("\x01"))); got != `"`+uesc("0001")+`"` {
		t.Errorf("control byte not escaped canonically: %s", got)
	}
}

// TestTMV0002_AS01_ClosedObjectsAndTypes covers the Reader: unknown and
// missing keys, wrong types, Count where Size is declared and the reverse.
func TestTMV0002_AS01_ClosedObjectsAndTypes(t *testing.T) {
	doc := `{"count":"3","extra":"x","size":"18446744073709551615"}` + "\n"
	v, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(v, "/")
	r.Closed("count", "size")
	if err := r.Err(); codeOf(t, err) != CodeMalformed || !strings.Contains(err.Error(), "extra") {
		t.Errorf("unknown key not reported: %v", err)
	}
	r = NewReader(v, "/")
	r.Closed("count", "size", "extra", "missing")
	if err := r.Err(); codeOf(t, err) != CodeMalformed || !strings.Contains(err.Error(), "missing") {
		t.Errorf("missing key not reported: %v", err)
	}
	r = NewReader(v, "/")
	r.Closed("count", "size", "extra")
	r.Field("count").Size() // Count-shaped value accepted as Size (a Size admits every Count)
	if err := r.Err(); err != nil {
		t.Errorf("Count-shaped value as Size: %v", err)
	}
	r = NewReader(v, "/")
	r.Closed("count", "size", "extra")
	r.Field("size").Count()
	if err := r.Err(); codeOf(t, err) != CodeMalformed || !strings.Contains(err.Error(), "/size") {
		t.Errorf("Size where a Count is declared must fail at /size: %v", err)
	}
	r = NewReader(v, "/")
	r.Closed("count", "size", "extra")
	r.Field("count").Bool()
	if err := r.Err(); codeOf(t, err) != CodeMalformed {
		t.Errorf("wrong type not reported: %v", err)
	}
	arr, _ := Parse([]byte(`["b","a"]` + "\n"))
	if err := CheckSortedUnique("/", arr.Arr); codeOf(t, err) != CodeMalformed {
		t.Errorf("unsorted non-semantic array accepted")
	}
	dup, _ := Parse([]byte(`["a","a"]` + "\n"))
	if err := CheckSortedUnique("/", dup.Arr); codeOf(t, err) != CodeMalformed {
		t.Errorf("duplicate element accepted")
	}
}

// TestTMV0002_AS01_ProfileVersions covers UNSUPPORTED_VERSION for every
// other version of a known profile and MALFORMED for a foreign profile.
func TestTMV0002_AS01_ProfileVersions(t *testing.T) {
	profiles := []string{
		"taskman-queue/0", "taskman-policy/0", "taskman-ticket/0", "taskman-mutation/0",
		"taskman-outcome/0", "taskman-receipt/0", "taskman-journal-head/0", "taskman-attempt/0",
		"taskman-reservation-set/0", "taskman-effect/0", "taskman-plan/0", "taskman-gate-result/0",
		"taskman-claim-disposition/0", "taskman-completion-manifest/0", "taskman-capability-profile/0",
		"taskman-import-plan/0", "taskman-import-map/0", "taskman-archive/0", "taskman-barrier/0",
		"taskman-command-result/0", "taskman-perf-baseline/0",
	}
	for _, p := range profiles {
		if err := CheckProfile("/profile", p, p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
		name := p[:len(p)-2]
		for _, bad := range []string{name + "/1", name + "/00", name + "/"} {
			if codeOf(t, CheckProfile("/profile", bad, p)) != CodeUnsupportedVersion {
				t.Errorf("%s: %q must be UNSUPPORTED_VERSION", p, bad)
			}
		}
		if codeOf(t, CheckProfile("/profile", "other/0", p)) != CodeMalformed {
			t.Errorf("%s: foreign profile must be MALFORMED", p)
		}
	}
}

// TestTMV0002_AS01_Identifiers covers the ID grammars and text kinds.
func TestTMV0002_AS01_Identifiers(t *testing.T) {
	if _, err := ParseTicketID("/", "ticket:acme:main:AT-07"); err != nil {
		t.Errorf("valid ticket id refused: %v", err)
	}
	bad := []string{"ticket:acme:main", "ticket:acme:main:", "ticket:acme:main:-x", "ticket:acme:main:a b",
		"queue:acme:main:x", "ticket:acme:main:" + strings.Repeat("a", 65), "ticket:acme:ma" + rn(0xEF) + "n:x"}
	for _, b := range bad {
		if _, err := ParseTicketID("/", b); err == nil {
			t.Errorf("accepted bad ticket id %q", b)
		}
	}
	if _, err := ParseQueueID("/", "queue:acme:main"); err != nil {
		t.Errorf("valid queue id refused: %v", err)
	}
	if _, err := ParseRepoID("/", "repo:acme"); err != nil {
		t.Errorf("valid repo id refused: %v", err)
	}
	if _, err := ParseIdentifier("/", strings.Repeat("x", 128)); err != nil {
		t.Errorf("128-byte identifier refused: %v", err)
	}
	if _, err := ParseIdentifier("/", strings.Repeat("x", 129)); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("129-byte identifier must be LIMIT_EXCEEDED: %v", err)
	}
	if _, err := ParseIdentifier("/", ""); codeOf(t, err) != CodeMalformed {
		t.Errorf("empty identifier must be MALFORMED")
	}
	if _, err := ParseIdentifier("/", "a\tb"); codeOf(t, err) != CodeMalformed {
		t.Errorf("TAB in identifier must be MALFORMED")
	}
	if _, err := ParseProse("/", "a\tb\nc", 1, 64); err != nil {
		t.Errorf("TAB/LF in prose refused: %v", err)
	}
	if _, err := ParseLabel("/", strings.Repeat("l", 65)); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("65-byte label must be LIMIT_EXCEEDED")
	}
	for _, p := range []string{"a/b", "a/b/", "dir/"} {
		if _, err := ParsePath("/", p); err != nil {
			t.Errorf("valid path %q refused: %v", p, err)
		}
	}
	for _, p := range []string{"/a", "a//b", "./a", "a/../b", "a\\b", "", "a/./b"} {
		if _, err := ParsePath("/", p); err == nil {
			t.Errorf("accepted bad path %q", p)
		}
	}
	if _, err := ParseTimestamp("/", "2026-09-06T12:00:00Z"); err != nil {
		t.Errorf("valid timestamp refused: %v", err)
	}
	for _, ts := range []string{"2026-09-06T12:00:00+00:00", "2026-13-06T12:00:00Z", "2026-09-06T12:00:00.000Z", "2026-02-30T00:00:00Z"} {
		if _, err := ParseTimestamp("/", ts); err == nil {
			t.Errorf("accepted bad timestamp %q", ts)
		}
	}
	if _, err := ParseDigest("/", strings.Repeat("A", 64)); err == nil {
		t.Errorf("uppercase digest accepted")
	}
	if FoldToken("AT-07") != "at-07" {
		t.Errorf("FoldToken is not ASCII lower-casing")
	}
}

// TestTMV0002_AS10_PathTextBoundaries covers the PathText primitive (§2,
// B2 resolution): 1..4096 UTF-8 bytes, absolute, hostile code points and
// TAB/LF/CR refused, independent of the 128-byte Identifier bound.
func TestTMV0002_AS10_PathTextBoundaries(t *testing.T) {
	long := "/" + strings.Repeat("d", MaxPathTextBytes-1)
	if len(long) != MaxPathTextBytes {
		t.Fatalf("fixture length %d", len(long))
	}
	if _, err := ParsePathText("/", long); err != nil {
		t.Errorf("%d-byte path refused: %v", MaxPathTextBytes, err)
	}
	if _, err := ParsePathText("/", long+"d"); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("%d-byte path must be LIMIT_EXCEEDED: %v", MaxPathTextBytes+1, err)
	}
	if _, err := ParsePathText("/", "/"+strings.Repeat("p", 200)); err != nil {
		t.Errorf("a path over the Identifier bound but under the PathText bound refused: %v", err)
	}
	if _, err := ParseIdentifier("/", "/"+strings.Repeat("p", 200)); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("Identifier bound must stay at %d bytes", MaxIdentifierBytes)
	}
	for _, p := range []string{"", "relative/path", "/a\tb", "/a\nb", "/a\rb", "/a" + string([]byte{0x00}) + "b", "/a" + rn(0x202E) + "b", "/a\xffb"} {
		if _, err := ParsePathText("/", p); codeOf(t, err) != CodeMalformed {
			t.Errorf("accepted bad path text %q: %v", p, err)
		}
	}
	if _, err := ParsePathText("/", "/tmp/caf"+rn(0xE9)+"/repo"); err != nil {
		t.Errorf("non-ASCII path text refused: %v", err)
	}
	v, _ := Parse([]byte(`{"p":"/x/y"}` + "\n"))
	rd := NewReader(v, "/")
	rd.Closed("p")
	if got := rd.Field("p").PathText(); got != "/x/y" || rd.Err() != nil {
		t.Errorf("Reader.PathText: %q %v", got, rd.Err())
	}
}

// TestTMV0002_AS01_CommandResultEnvelope covers the taskman-command-result/0
// round trip, closed codes and the outcome enum.
func TestTMV0002_AS01_CommandResultEnvelope(t *testing.T) {
	seq := Size("7")
	d := Digest(strings.Repeat("ab", 32))
	res := &Result{
		Command:   []string{"ticket", "list"},
		Outcome:   OutcomeOK,
		Codes:     []string{CodeRedoPending, CodeCycle, CodeCycle},
		Snapshot:  &Snapshot{HeadSeq: &seq, HeadReceiptSha256: &d, IntentTreeSha256: &d, PrimaryWorktreeSha256: &d, Barrier: &BarrierRef{Scope: "ALL", Reason: "DRAIN"}},
		Items:     []Value{ObjectValue(NewObject().Set("title", String("<untrusted>")))},
		Page:      &Page{Offset: "0", Limit: "100", Truncated: true},
		Untrusted: true,
		Warnings:  []string{"w2", "w1"},
	}
	data, err := res.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.HasPrefix(string(data), `{"codes":["CYCLE","REDO_PENDING"],"command":["ticket","list"]`) {
		t.Errorf("codes not sorted/deduplicated or key order wrong: %s", data)
	}
	back, err := DecodeResult(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !back.Untrusted || back.Page == nil || !back.Page.Truncated || back.Snapshot == nil || back.Snapshot.Barrier == nil || back.Snapshot.Barrier.Scope != "ALL" {
		t.Errorf("round trip lost fields: %+v", back)
	}
	if string(Encode(back.Value())) != string(Encode(res.Value())) {
		t.Errorf("re-encoded envelope differs")
	}
	bad := strings.Replace(string(data), `"REDO_PENDING"`, `"NOT_A_CODE"`, 1)
	if _, err := DecodeResult([]byte(bad)); codeOf(t, err) != CodeMalformed {
		t.Errorf("unknown code accepted: %v", err)
	}
	bad = strings.Replace(string(data), `"outcome":"OK"`, `"outcome":"MAYBE"`, 1)
	if _, err := DecodeResult([]byte(bad)); codeOf(t, err) != CodeMalformed {
		t.Errorf("unknown outcome accepted: %v", err)
	}
	res.Codes = []string{"BOGUS"}
	if _, err := res.Encode(); codeOf(t, err) != CodeMalformed {
		t.Errorf("Encode must refuse a code outside §11: %v", err)
	}
	if Errorf("NOT_A_CODE", "", "x").Code != CodeMalformed {
		t.Errorf("Errorf must never leak an unknown code")
	}
	for _, c := range Codes {
		if !IsCode(c) {
			t.Errorf("code %s missing from the set", c)
		}
	}
	if len(Codes) != 69 {
		t.Errorf("§11 lists 69 codes, table has %d", len(Codes))
	}
}

// TestTMV0002_AS10_DecodeBounds covers the array-element and node bounds.
func TestTMV0002_AS10_DecodeBounds(t *testing.T) {
	arr := "[" + strings.TrimSuffix(strings.Repeat(`"",`, MaxJSONArrayElements), ",") + "]\n"
	if _, err := Parse([]byte(arr)); err != nil {
		t.Errorf("array at the 10,000 bound refused: %v", err)
	}
	arr = "[" + strings.TrimSuffix(strings.Repeat(`"",`, MaxJSONArrayElements+1), ",") + "]\n"
	if _, err := Parse([]byte(arr)); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("array over the bound must be LIMIT_EXCEEDED: %v", err)
	}
	// 250,001 nodes: nested arrays of 10,000 elements each, 26 of them.
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < 26; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("[" + strings.TrimSuffix(strings.Repeat(`"",`, 9999), ",") + "]")
	}
	b.WriteString("]\n")
	if _, err := Parse([]byte(b.String())); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("aggregate node bound not enforced: %v", err)
	}
}

// TestTMV0022_AS10_ParseWithWidensOnlyTheNamedArray proves the opt-in
// archive parser entry (§3.5, B4): ParseWith widens exactly the top-level
// `files` array and the node cap; every other array at any depth, the
// depth bound and the default Parse are unchanged. Small documents only.
func TestTMV0022_AS10_ParseWithWidensOnlyTheNamedArray(t *testing.T) {
	elems := func(n int) string { return strings.TrimSuffix(strings.Repeat(`"",`, n), ",") }
	over := MaxJSONArrayElements + 1
	wide := ParseOptions{WideArrayKey: "files", WideArrayMax: over, MaxNodes: 4*over + 14}
	// files over the ordinary bound: refused by Parse, accepted by ParseWith.
	doc := `{"files":[` + elems(over) + `]}` + "\n"
	if _, err := Parse([]byte(doc)); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("default parser must keep the 10,000 bound on files: %v", err)
	}
	if _, err := ParseWith([]byte(doc), wide); err != nil {
		t.Errorf("wide parser refused files at %d: %v", over, err)
	}
	// One beyond the wide bound is still LIMIT_EXCEEDED.
	if _, err := ParseWith([]byte(`{"files":[`+elems(over+1)+`]}`+"\n"), wide); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("wide bound must be hard: %v", err)
	}
	// A sibling array, a nested array inside files, a `files` key below the
	// top level and a top-level array document all keep the ordinary bound.
	for name, d := range map[string]string{
		"sibling":      `{"files":[],"other":[` + elems(over) + `]}` + "\n",
		"nested":       `{"files":[[` + elems(over) + `]]}` + "\n",
		"deeper files": `{"a":{"files":[` + elems(over) + `]}}` + "\n",
		"root array":   `[` + elems(over) + `]` + "\n",
	} {
		if _, err := ParseWith([]byte(d), wide); codeOf(t, err) != CodeLimitExceeded {
			t.Errorf("%s: only the top-level files array may widen: %v", name, err)
		}
	}
	// The node cap is the option, checked before materialization completes.
	small := ParseOptions{WideArrayKey: "files", WideArrayMax: over, MaxNodes: 5}
	if _, err := ParseWith([]byte(`{"files":["","","",""]}`+"\n"), small); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("node cap option not enforced: %v", err)
	}
	if _, err := ParseWith([]byte(`{"files":["","",""]}`+"\n"), small); err != nil {
		t.Errorf("five nodes under a cap of five refused: %v", err)
	}
	// Depth is never widened.
	deep := `{"files":` + strings.Repeat("[", 24) + strings.Repeat("]", 24) + `}` + "\n"
	if _, err := ParseWith([]byte(deep), wide); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("depth bound must survive ParseWith: %v", err)
	}
	// Zero options are exactly Parse.
	if _, err := ParseWith([]byte(doc), ParseOptions{}); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("ParseOptions{} must equal Parse: %v", err)
	}
	// Frozen archive numbers (§1): the node cap covers a full manifest.
	if MaxArchiveFiles != 2100000 || MaxArchiveManifestBytes != 768*MiB || MaxArchiveScanEntries != MaxArchiveFiles+4096 {
		t.Errorf("archive capacity constants differ from the §1 freeze")
	}
	if MaxJSONArrayElements != 10000 || MaxJSONNodes != 250000 || MaxJSONDepth != 24 {
		t.Errorf("ordinary decode bounds must be unchanged by the archive freeze")
	}
}
