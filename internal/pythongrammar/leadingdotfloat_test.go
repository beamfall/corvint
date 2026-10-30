package pythongrammar

import "testing"

type discardFacts struct{}

func (discardFacts) Add(_, _, _, _ string, _, _ int) Failure { return "" }

// TestLeadingDotFloatIsOneNumberToken pins the lexical half of the fix. CPython
// 3.12's tokenizer emits `.1` as a single NUMBER; emitting `.` and `1` made the
// dot an operand-position delimiter, which no expression rule admits.
func TestLeadingDotFloatIsOneNumberToken(t *testing.T) {
	tokens, reason := LexPython312([]byte("f(.1)\n"))
	if reason != "" {
		t.Fatalf("lex .1: %s", reason)
	}
	var numbers []string
	for _, token := range tokens {
		if token.Kind == KindNumber {
			numbers = append(numbers, string([]byte("f(.1)\n")[token.Start:token.End]))
		}
	}
	if len(numbers) != 1 || numbers[0] != ".1" {
		t.Fatalf("number tokens = %q, want exactly [\".1\"]", numbers)
	}
}

// TestLeadingDotFloatAccepted pins the reported false positive together with the
// siblings that already passed, so a future lexer change cannot regress one
// without the others. Every case here is accepted by CPython 3.12 ast.parse.
func TestLeadingDotFloatAccepted(t *testing.T) {
	for _, source := range []string{
		"f(.1)\n",     // the reported false positive
		"f(0.1)\n",    // passing sibling: explicit leading zero
		"x = .1\n",    // passing sibling: assignment right-hand side
		"f(k=.1)\n",   // passing sibling: keyword argument
		"[.1]\n",      // passing sibling: list display
		"f(1, .2)\n",  // trailing positional
		"f(.1, 2)\n",  // leading positional followed by more
		"f((.1))\n",   // parenthesized inside a call
		"f(.1e3)\n",   // exponent suffix
		"f(.1j)\n",    // imaginary suffix
		"print(.5)\n", // the shape that cost test_soak.py its symbols
	} {
		if reason := ParsePython312Subset("probe.py", []byte(source), discardFacts{}); reason != "" {
			t.Errorf("ParsePython312Subset(%q) = %q, want accepted", source, reason)
		}
	}
}

// TestDotWithoutDigitStaysADelimiter guards the other side of the guard: a dot
// that does not open a float must keep its delimiter meaning, or attribute
// access, ellipsis and relative imports all break.
func TestDotWithoutDigitStaysADelimiter(t *testing.T) {
	for _, source := range []string{
		"x = a.b\n",
		"x = ...\n",
		"from . import alpha\n",
		"from ..pkg import beta\n",
		"x = a.b.c(1).d\n",
	} {
		if reason := ParsePython312Subset("probe.py", []byte(source), discardFacts{}); reason != "" {
			t.Errorf("ParsePython312Subset(%q) = %q, want accepted", source, reason)
		}
	}
}
