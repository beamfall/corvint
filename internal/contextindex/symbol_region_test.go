package contextindex

import (
	"reflect"
	"testing"
)

func normalizedTexts(t *testing.T, name, source string) []NormalizedLine {
	t.Helper()
	lines, ok := NormalizedCodeLines(name, []byte(source))
	if !ok {
		t.Fatalf("%s: normalizer refused", name)
	}
	return lines
}

func TestNormalizedCodeLines(t *testing.T) {
	t.Run("DCP-V1-140 ruby drops comments and indentation but keeps literals", func(t *testing.T) {
		got := normalizedTexts(t, "a.rb", "def total  # sum\n\n    # note\n  x  =  \"a  # b\"\n  <<~TXT\n    keep  me\n  TXT\nend\n")
		want := []NormalizedLine{{1, "def total"}, {4, "x = \"a  # b\""}, {5, "<<~TXT"}, {6, "    keep  me"}, {7, "  TXT"}, {8, "end"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})
	t.Run("DCP-V1-140 ruby block comments are dropped", func(t *testing.T) {
		got := normalizedTexts(t, "a.rb", "=begin\nnotes\n=end\nx = 1\n")
		if want := []NormalizedLine{{4, "x = 1"}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v", got)
		}
	})
	t.Run("DCP-V1-140 web drops comments and collapses whitespace outside literals", func(t *testing.T) {
		got := normalizedTexts(t, "a.ts", "function f(a,   b) { // c\n  /* x\n  y */ return `t  ${a}`;\n}\n")
		want := []NormalizedLine{{1, "function f(a, b) {"}, {3, "return `t  ${a}`;"}, {4, "}"}}
		if len(got) != len(want) {
			t.Fatalf("got %#v", got)
		}
		for i := range want {
			if got[i].Line != want[i].Line {
				t.Fatalf("line %d: got %#v", i, got)
			}
			if got[i].Text != want[i].Text {
				t.Fatalf("text %d: got %q", i, got[i].Text)
			}
		}
		reformatted := normalizedTexts(t, "a.ts", "function f(a, b) {\n\n    return `t  ${a}`;   // r\n}\n")
		for i := range reformatted {
			if reformatted[i].Text != got[i].Text {
				t.Fatalf("reformat changed text: %#v vs %#v", reformatted, got)
			}
		}
	})
	t.Run("DCP-V1-142 unterminated web source and other languages are refused", func(t *testing.T) {
		if _, ok := NormalizedCodeLines("a.ts", []byte("const x = `open\n")); ok {
			t.Fatal("unterminated template accepted")
		}
		if _, ok := NormalizedCodeLines("a.py", []byte("x = 1\n")); ok {
			t.Fatal("python accepted")
		}
		if SymbolRegionLanguage("a.go") != "" || SymbolRegionLanguage("a.mjs") != "web" || SymbolRegionLanguage("a.rb") != "ruby" {
			t.Fatal("language family")
		}
	})
}
