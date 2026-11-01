package appmap

import "testing"

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
