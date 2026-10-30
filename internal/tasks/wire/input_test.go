package wire

import (
	"strings"
	"testing"
)

// TestV10750_ParseInputCanonicalizesFramingOnly is V1-0750 at the codec: a
// caller-supplied payload may carry whitespace, any key order and any valid
// escape form, and re-encodes to the one canonical body. Array order is
// never changed by the parser.
func TestV10750_ParseInputCanonicalizesFramingOnly(t *testing.T) {
	canonical := `{"a":"/","b":["z","a"],"c":{"x":"A","y":"` + rn(0xE9) + `"}}`
	cases := map[string]string{
		"canonical":            canonical,
		"canonical with LF":    canonical + "\n",
		"pretty-printed":       "{\n \"a\": \"/\",\n \"b\": [\n  \"z\",\n  \"a\"\n ],\n \"c\": {\n  \"x\": \"A\",\n  \"y\": \"" + rn(0xE9) + "\"\n }\n}\n",
		"folded pretty-print":  `{ "a": "/", "b": [ "z", "a" ], "c": { "x": "A", "y": "` + rn(0xE9) + `" } }`,
		"default separators":   `{"a": "/", "b": ["z", "a"], "c": {"x": "A", "y": "` + rn(0xE9) + `"}}`,
		"reversed keys":        `{"c":{"y":"` + rn(0xE9) + `","x":"A"},"b":["z","a"],"a":"/"}`,
		"escape forms":         `{"a":"\/","b":["z","a"],"c":{"x":"` + uesc("0041") + `","y":"` + uesc("00e9") + `"}}`,
		"CR LF TAB whitespace": "\t{\r\n\"a\"\t:\"/\",\"b\":[\"z\",\"a\"],\"c\":{\"x\":\"A\",\"y\":\"" + rn(0xE9) + "\"}}\r\n",
	}
	for name, in := range cases {
		v, err := ParseInput([]byte(in))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got := string(Encode(v)); got != canonical {
			t.Errorf("%s: canonical %s, want %s", name, got, canonical)
		}
	}
}

// TestV10750_ParseInputKeepsTheValueRules: leniency covers framing only.
func TestV10750_ParseInputKeepsTheValueRules(t *testing.T) {
	cases := map[string]string{
		"duplicate key":   `{"a": "1", "a": "2"}`,
		"number":          `{"a": 1}`,
		"BOM":             "\xEF\xBB\xBF{}",
		"lone surrogate":  `["` + uesc("d800") + `"]`,
		"hostile escape":  `["` + uesc("001f") + `"]`,
		"trailing bytes":  `{} x`,
		"two documents":   `{} {}`,
		"empty":           "",
		"only whitespace": " \n ",
		"form feed":       "{\f}",
	}
	for name, in := range cases {
		if _, err := ParseInput([]byte(in)); codeOf(t, err) != CodeMalformed {
			t.Errorf("%s: err %v, want MALFORMED", name, err)
		}
	}
	deep := strings.Repeat("[ ", 25) + strings.Repeat("] ", 25)
	if _, err := ParseInput([]byte(deep)); codeOf(t, err) != CodeLimitExceeded {
		t.Errorf("depth 25: err %v, want LIMIT_EXCEEDED", err)
	}
}

// TestV10750_StrictParseNamesTheExactDefect keeps canonical input a wire rule
// for stored documents while every refusal names the transformation that
// fixes it; pretty-printed JSON never reports "expected object key".
func TestV10750_StrictParseNamesTheExactDefect(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"pretty-printed object", "{\n \"a\": \"1\"\n}\n", "whitespace"},
		{"folded pretty-print", `{ "a": "1" }` + "\n", "whitespace"},
		{"space before colon", `{"a" :"1"}` + "\n", "whitespace"},
		{"space after comma", `{"a":"1", "b":"2"}` + "\n", "whitespace"},
		{"space before brace", `{"a":"1" }` + "\n", "whitespace"},
		{"space in array", `["a" ,"b"]` + "\n", "whitespace"},
		{"empty object with space", "{ }\n", "whitespace"},
		{"reversed top-level keys", `{"b":"1","a":"2"}` + "\n", `key order: the keys of the object at "/"`},
		{"reversed nested keys", `{"a":{"z":"1","y":"2"}}` + "\n", `key order: the keys of the object at "/a"`},
		{"solidus escape", `["\/"]` + "\n", "escape form"},
		{"optional escape", `["` + uesc("0041") + `"]` + "\n", "escape form"},
	}
	for _, c := range cases {
		_, err := Parse([]byte(c.in))
		if err == nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, c.want) {
			t.Errorf("%s: %q does not name %q", c.name, msg, c.want)
		}
		if strings.Contains(msg, "expected object key") && !strings.Contains(msg, "whitespace") {
			t.Errorf("%s: %q blames the key, not the whitespace", c.name, msg)
		}
	}
}

// TestV10750_SetSortingReaderSortsOnlySets: the CLI reader sorts an array a
// decoder reads as a set, in place, and never touches an ordered array.
func TestV10750_SetSortingReaderSortsOnlySets(t *testing.T) {
	v, err := ParseInput([]byte(`{"argv":["run","-x","a"],"labels":["zeta","alpha"]}`))
	if err != nil {
		t.Fatal(err)
	}
	r := NewSetSortingReader(v, "/payload")
	r.Field("labels").Strings(-1, false, (*Reader).Label)
	r.Field("argv").Strings(-1, true, (*Reader).String)
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	if got, want := string(Encode(v)), `{"argv":["run","-x","a"],"labels":["alpha","zeta"]}`; got != want {
		t.Errorf("canonicalized %s, want %s", got, want)
	}

	dup, _ := ParseInput([]byte(`{"labels":["b","a","b"]}`))
	d := NewSetSortingReader(dup, "/payload")
	d.Field("labels").Strings(-1, false, (*Reader).Label)
	if err := d.Err(); err == nil || !strings.Contains(err.Error(), "/payload/labels") || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate set element: err %v, want a /payload/labels duplicate refusal", err)
	}

	unsorted, _ := ParseInput([]byte(`["b","a"]`))
	strict := NewReader(unsorted, "/payload/labels")
	strict.Strings(-1, false, (*Reader).Label)
	if err := strict.Err(); err == nil || !strings.Contains(err.Error(), "/payload/labels") || !strings.Contains(err.Error(), "sort its elements") {
		t.Errorf("strict unsorted set: err %v, want the path and the sort fix", err)
	}
}

// TestV10750_StrictParseRefusesWhitespaceBeforeTheReencode pins the
// assumption behind ParseWith's fallback message: once every object's keys
// are sorted, a re-encode mismatch is reported as an escape form, which is
// only true because strict parsing refuses insignificant whitespace first.
// Every structural gap of a sorted-key canonical document, filled with each
// JSON whitespace byte, must be refused by the parser itself; if strict mode
// ever let whitespace through, the re-encode comparison would mislabel it as
// an escape form and this test fails.
func TestV10750_StrictParseRefusesWhitespaceBeforeTheReencode(t *testing.T) {
	body := `{"a":["x",{"b":"y"},[]],"c":{},"d":"z"}`
	if _, err := Parse([]byte(body + "\n")); err != nil {
		t.Fatalf("canonical document refused: %v", err)
	}
	inString := false
	for i := 0; i <= len(body); i++ {
		if i > 0 && body[i-1] == '"' {
			inString = !inString
		}
		if inString {
			continue
		}
		for _, ws := range []string{" ", "\t", "\n", "\r"} {
			in := body[:i] + ws + body[i:] + "\n"
			_, err := Parse([]byte(in))
			if err == nil {
				t.Errorf("gap %d %q: strict parse accepted whitespace", i, ws)
				continue
			}
			if msg := err.Error(); strings.Contains(msg, "non-canonical string escape form") || strings.Contains(msg, "non-canonical key order") {
				t.Errorf("gap %d %q: whitespace reached the re-encode comparison: %q", i, ws, msg)
			}
		}
	}
	// With whitespace excluded, a sorted-key mismatch is an escape form.
	if _, err := Parse([]byte(`{"a":"\/","b":"1"}` + "\n")); err == nil || !strings.Contains(err.Error(), "non-canonical string escape form") {
		t.Errorf("sorted keys with a solidus escape: %v, want the escape-form message", err)
	}
}
