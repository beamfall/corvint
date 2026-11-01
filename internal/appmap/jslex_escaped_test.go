package appmap

import (
	"strings"
	"testing"
)

// AMAP-V0-880: an identifier written with a \u escape is one tokIdent token
// marked escaped whose text is the raw source; a malformed escape stays a
// backslash punctuation token.
func TestAMAPV0880EscapedIdentifierToken(t *testing.T) {
	toks, _ := lexJS("const \\u0061pp = a\\u{62}c + plain + \\x;")
	type want struct {
		kind    tokKind
		text    string
		escaped bool
	}
	expected := []want{
		{tokIdent, "const", false}, {tokIdent, "\\u0061pp", true}, {tokPunct, "=", false},
		{tokIdent, "a\\u{62}c", true}, {tokPunct, "+", false}, {tokIdent, "plain", false},
		{tokPunct, "+", false}, {tokPunct, `\`, false}, {tokIdent, "x", false}, {tokPunct, ";", false},
	}
	if len(toks) != len(expected) {
		t.Fatalf("tokens=%+v", toks)
	}
	for k, w := range expected {
		if got := toks[k]; got.kind != w.kind || got.text != w.text || got.escaped != w.escaped {
			t.Fatalf("token %d = %+v, want %+v", k, got, w)
		}
	}
}

// AMAP-V0-880: the shared object reader treats an escaped identifier key as
// doubt (a non-plain member), never as the key it decodes to, so no keyed
// reader has to look for backslashes itself.
func TestAMAPV0880EscapedIdentifierKeyIsDoubt(t *testing.T) {
	for _, source := range []string{"x = { \\u0061lias: 'a', b: 'b' }", "x = { al\\u{69}as: 'a', b: 'b' }"} {
		toks, _ := lexJS(source)
		v, _ := parseValue(toks, 2)
		if v.kind != "object" {
			t.Fatalf("%s: value %+v", source, v)
		}
		if _, ok := v.get("alias"); ok {
			t.Fatalf("%s: escaped key read as alias: %+v", source, v)
		}
		if len(v.obj) != 2 || v.obj[0].key != "" || v.obj[0].value.kind != "other" || v.obj[1].key != "b" {
			t.Fatalf("%s: escaped key not doubt: %+v", source, v.obj)
		}
	}
	toks, _ := lexJS(`x = { alias: "a" }`)
	if v, _ := parseValue(toks, 2); v.obj[0].key != "alias" {
		t.Fatalf("plain key: %+v", v.obj)
	}
}

// AMAP-V0-880: an endpoint config key written with an escape is doubt. An
// escaped method never reads as the default verb, and an escaped baseURL never
// passes as a config without one. (\x5c is a backslash.)
func TestAMAPV0880EscapedEndpointConfigKey(t *testing.T) {
	for src, want := range map[string]string{
		"axios({ url: '/a', \x5cu006dethod: 'DELETE' });":  "! non-literal-config",
		"$http({ url: '/a', m\x5cu{65}thod: 'DELETE' });":  "! non-literal-config",
		"axios({ url: '/a', \x5cu0062aseURL: '/b' });":     "! non-literal-config",
		"axios('/a', { \x5cu0062aseURL: '/b' });":          "! non-literal-config",
		"axios.get('/a', { \x5cu0062aseURL: '/b' });":      "! non-literal-config",
		"fetch('/a', { \x5cu006dethod: 'POST' });":         "* /a",
		"axios({ url: '/a', method: 'DELETE' });":          "DELETE /a",
		"axios.get('/a', { headers: {}, baseURL: '/b' });": "! config-base-url",
	} {
		calls, notes := scanCalls("f.js", []byte(src))
		got := []string{}
		for _, c := range calls {
			got = append(got, c.Verb+" "+c.URL)
		}
		for _, n := range notes {
			got = append(got, "! "+n.Reason)
		}
		if strings.Join(got, ";") != want {
			t.Errorf("%s: got %q, want %q", src, got, want)
		}
	}
}

// AMAP-V0-880: a braced escape takes any number of hex digits, leading zeros
// included, while its value is at most 0x10FFFF; a larger value is malformed
// and stays a backslash punctuation token.
func TestAMAPV0880BracedEscapeDigits(t *testing.T) {
	bs := "\x5c"
	for _, src := range []string{
		bs + "u{0000061}b", bs + "u{00000000000000000061}b", bs + "u{10FFFF}b", bs + "u{0010FFFF}b",
	} {
		toks, _ := lexJS(src + ";")
		if len(toks) != 2 || toks[0].kind != tokIdent || toks[0].text != src || !toks[0].escaped {
			t.Errorf("%s: tokens %+v", src, toks)
		}
	}
	for _, src := range []string{bs + "u{110000}b", bs + "u{000110000}b", bs + "u{}b", bs + "u{FFFFFFFFF}b"} {
		toks, _ := lexJS(src + ";")
		if len(toks) == 0 || toks[0].kind != tokPunct || toks[0].text != bs {
			t.Errorf("%s: tokens %+v", src, toks)
		}
	}
}
