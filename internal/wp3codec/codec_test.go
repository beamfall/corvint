package wp3codec

import (
	"errors"
	"strings"
	"testing"
)

// Every expectation below is authored from the commitment-codec paragraph in
// docs/specs/lexical-relevance-floor-v0.md, restated by CF-V0-019 in
// docs/specs/change-frontier-v0.md — never from this package's own output.

func TestEncodeEscapeProfile(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"quote", `a"b`, `"a\"b"`},
		{"backslash", `a\b`, `"a\\b"`},
		{"tab takes the long escape, not \\t", "a\tb", `"a\u0009b"`},
		{"newline takes the long escape, not \\n", "a\nb", `"a\u000ab"`},
		{"nul", "a\x00b", `"a\u0000b"`},
		{"unit separator", "a\x1fb", `"a\u001fb"`},
		{"slash is never escaped", "a/b", `"a/b"`},
		{"non-ascii stays raw utf-8", "café", `"café"`},
		{"no unicode normalization", "é", "\"é\""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			encoded, err := Encode(String(testCase.input))
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}
			if string(encoded) != testCase.want {
				t.Fatalf("encoded %s, want %s", encoded, testCase.want)
			}
		})
	}
}

// Object keys sort by their raw UTF-8 bytes, not by any collation: "Z" sorts
// before "a" because 0x5A < 0x61.
func TestObjectKeysSortByRawUTF8Bytes(t *testing.T) {
	encoded, err := Encode(Object(
		Member{Key: "a", Value: String("1")},
		Member{Key: "Z", Value: String("2")},
		Member{Key: "é", Value: String("3")},
		Member{Key: "A", Value: String("4")},
	))
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	want := `{"A":"4","Z":"2","a":"1","é":"3"}`
	if string(encoded) != want {
		t.Fatalf("encoded %s, want %s", encoded, want)
	}
}

// Arrays preserve declared order; only objects are reordered.
func TestArraysPreserveDeclaredOrder(t *testing.T) {
	encoded, err := Encode(Strings([]string{"b", "a", "c"}))
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if string(encoded) != `["b","a","c"]` {
		t.Fatalf("encoded %s, want the declared order", encoded)
	}
}

func TestEncodeLiteralsAndEmptyContainers(t *testing.T) {
	cases := []struct {
		value Value
		want  string
	}{
		{Null(), "null"},
		{Bool(true), "true"},
		{Bool(false), "false"},
		{Array(), "[]"},
		{Object(), "{}"},
	}
	for _, testCase := range cases {
		encoded, err := Encode(testCase.value)
		if err != nil {
			t.Fatalf("encode failed: %v", err)
		}
		if string(encoded) != testCase.want {
			t.Fatalf("encoded %s, want %s", encoded, testCase.want)
		}
	}
}

// Counts, ordinals and offsets are base-10 strings with no sign and no leading
// zero except "0"; a negative value has no canonical form.
func TestDecimalForm(t *testing.T) {
	for input, want := range map[int64]string{0: `"0"`, 7: `"7"`, 1024: `"1024"`} {
		value, err := Decimal(input)
		if err != nil {
			t.Fatalf("decimal %d failed: %v", input, err)
		}
		encoded, _ := Encode(value)
		if string(encoded) != want {
			t.Fatalf("encoded %s, want %s", encoded, want)
		}
	}
	if _, err := Decimal(-1); !errors.Is(err, ErrInvalid) {
		t.Fatal("a negative decimal must be rejected, not signed")
	}
}

// The clause names a CLOSED set of invalid inputs: a duplicate object key,
// invalid UTF-8, a BOM, an unpaired surrogate, or a forbidden value. Parse
// rejects exactly those (plus malformed JSON); everything merely non-canonical
// is Verify's business, asserted separately below.
func TestParseRejectsForbiddenInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"json number", `{"a":1}`},
		{"negative number", `-1`},
		{"duplicate key", `{"a":"1","a":"2"}`},
		{"byte order mark", "\ufeff{}"},
		{"unpaired high surrogate", `"\ud800"`},
		{"unpaired low surrogate", `"\udc00"`},
		{"raw control byte", "\"a\x01b\""},
		{"trailing bytes", `{}{}`},
		{"unterminated string", `"abc`},
		{"unterminated object", `{"a":"b"`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := Parse([]byte(testCase.input)); err == nil {
				t.Fatalf("Parse accepted forbidden input %q", testCase.input)
			}
		})
	}
	if _, err := Parse([]byte{'"', 0xff, '"'}); err == nil {
		t.Fatal("Parse accepted invalid UTF-8")
	}
}

// A verifier must parse, reserialize, and require byte equality before hashing.
// That single rule rejects mis-ordered keys and non-minimal escapes.
func TestVerifyRequiresByteEqualSerialization(t *testing.T) {
	if err := Verify([]byte(`{"a":"1","b":"2"}`)); err != nil {
		t.Fatalf("canonical bytes were rejected: %v", err)
	}
	rejected := []string{
		`{"b":"2","a":"1"}`, // keys not in raw-byte order
		`{"a": "1"}`,        // insignificant whitespace between tokens
		`["a", "b"]`,        // insignificant whitespace between elements
		` {"a":"1"}`,        // leading whitespace
		`"a\nb"`,            // a short escape where \u000a is canonical
		`"a\/b"`,            // an optional solidus escape
		`"\u000A"`,          // uppercase hex where lowercase is canonical
		`"\u0041"`,          // an escape where the raw scalar is canonical
		`{"a":"1"}` + "\n",  // a terminal LF is not part of a codec value
		`"\ud83d\ude00"`,    // a surrogate pair where raw UTF-8 is canonical
	}
	for _, input := range rejected {
		if err := Verify([]byte(input)); err == nil {
			t.Fatalf("Verify accepted non-canonical bytes %q", input)
		}
	}
}

func TestEncodeRejectsInvalidScalars(t *testing.T) {
	if _, err := Encode(String(string([]byte{0xff}))); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid UTF-8 must be rejected at encode time")
	}
	if _, err := Encode(String("\xed\xa0\x80")); !errors.Is(err, ErrInvalid) {
		t.Fatal("an unpaired surrogate must be rejected at encode time")
	}
	duplicate := Value{kind: KindObject, members: []Member{
		{Key: "a", Value: Null()}, {Key: "a", Value: Null()},
	}}
	if _, err := Encode(duplicate); !errors.Is(err, ErrInvalid) {
		t.Fatal("a duplicate object key must be rejected at encode time")
	}
}

// Serialization emits no whitespace and no terminal LF, at any nesting depth.
func TestEncodeEmitsNoWhitespaceOrTerminalLF(t *testing.T) {
	encoded, err := Encode(Object(
		Member{Key: "outer", Value: Array(Object(Member{Key: "inner", Value: Bool(true)}))},
	))
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if strings.ContainsAny(string(encoded), " \t\r\n") {
		t.Fatalf("encoded %s contains whitespace", encoded)
	}
}

func TestDepthBound(t *testing.T) {
	deep := Null()
	for index := 0; index <= maxDepth+1; index++ {
		deep = Array(deep)
	}
	if _, err := Encode(deep); !errors.Is(err, ErrInvalid) {
		t.Fatal("nesting past the depth bound must be rejected")
	}
	if _, err := Parse([]byte(strings.Repeat("[", maxDepth+2) + strings.Repeat("]", maxDepth+2))); err == nil {
		t.Fatal("parsing past the depth bound must be rejected")
	}
}

// A surrogate pair is accepted by Parse and decodes to its scalar; it is Verify
// that refuses it, because the canonical form is the raw UTF-8 bytes.
func TestSurrogatePairDecodes(t *testing.T) {
	value, err := Parse([]byte(`"\ud83d\ude00"`))
	if err != nil {
		t.Fatalf("a well-formed surrogate pair must parse: %v", err)
	}
	if value.Text() != "\U0001F600" {
		t.Fatalf("decoded %q, want the astral scalar", value.Text())
	}
}

// CF-V0-019 splits the work: Parse admits the broader JSON subset and decodes
// it to scalars, Encode emits the one canonical form. That split is what lets
// an independent consumer canonicalize input (CF-V0-028) instead of only
// checking bytes it was already handed in canonical form.
func TestParseThenEncodeCanonicalizes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"stripped whitespace", "{\n  \"b\" : \"2\",\n  \"a\" : [ \"x\", \"y\" ]\n}", `{"a":["x","y"],"b":"2"}`},
		{"short escapes become long", `"a\tb\nc"`, `"a\u0009b\u000ac"`},
		{"uppercase hex becomes lowercase", `"\u001F"`, `"\u001f"`},
		{"optional escapes are dropped", `"a\/b\u0041"`, `"a/bA"`},
		{"surrogate pair becomes raw utf-8", `"\ud83d\ude00"`, "\"\U0001F600\""},
		{"trailing lf is not part of the value", `{"a":"1"}` + "\n", `{"a":"1"}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, err := Parse([]byte(testCase.input))
			if err != nil {
				t.Fatalf("Parse rejected admissible input %q: %v", testCase.input, err)
			}
			encoded, err := Encode(parsed)
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}
			if string(encoded) != testCase.want {
				t.Fatalf("canonicalized %s, want %s", encoded, testCase.want)
			}
		})
	}
}
