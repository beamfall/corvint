package pythongrammar

import (
	"strings"
	"testing"
)

type recordedFacts []string

func (facts *recordedFacts) Add(kind, _, _, value string, _, _ int) Failure {
	*facts = append(*facts, kind+" "+value)
	return ""
}

// assertMalformed checks the PNC-004 closure: every source here is rejected by
// CPython 3.12 ast.parse, so the grammar must reject it rather than report a
// parse whose facts describe code Python never runs.
func assertMalformed(t *testing.T, sources []string) {
	t.Helper()
	for _, source := range sources {
		if reason := ParsePython312Subset("probe.py", []byte(source), discardFacts{}); reason != failureMalformed {
			t.Errorf("ParsePython312Subset(%q) = %q, want %q", source, reason, failureMalformed)
		}
	}
}

// TestPNC004InconsistentTabsReject pins CPython's TabError: indentation whose
// ordering differs between tab size 8 and tab size 1 is not Python 3.12.
func TestPNC004InconsistentTabsReject(t *testing.T) {
	assertMalformed(t, []string{
		"if x:\n        pass\n\tpass\n",
		"if x:\n\tpass\n  \tpass\n",
		"if x:\n  \tif y:\n\t pass\n",
		"if x:\n\tif y:\n\t\tpass\n        pass\n",
	})
	for _, source := range []string{"if x:\n\tif y:\n\t\tpass\n\tpass\npass\n", "if x:\n    \tpass\n    \tpass\n"} {
		if reason := ParsePython312Subset("probe.py", []byte(source), discardFacts{}); reason != "" {
			t.Errorf("ParsePython312Subset(%q) = %q, want accepted", source, reason)
		}
	}
}

// TestPNC004DecoratorDoesNotCrossDedent: a decorator left at the end of a
// suite was attached to the next definition in the parent suite.
func TestPNC004DecoratorDoesNotCrossDedent(t *testing.T) {
	assertMalformed(t, []string{"if x:\n    @d\ndef f():\n    pass\n"})
}

// TestPNC004CompoundStatementAfterSemicolonRejects: CPython admits only simple
// statements after ';', so no definition fact may come from that position.
func TestPNC004CompoundStatementAfterSemicolonRejects(t *testing.T) {
	assertMalformed(t, []string{
		"class A: pass; def f(): pass\n",
		"x = 1; def f(): pass\n",
		"@d; def f(): pass\n",
		"x = 1; if y: f()\n",
	})
	for _, source := range []string{"x = 1; y = 2; f()\n", "if x: pass; y()\n", "x = 1; match: int = 2\n"} {
		if reason := ParsePython312Subset("probe.py", []byte(source), discardFacts{}); reason != "" {
			t.Errorf("ParsePython312Subset(%q) = %q, want accepted", source, reason)
		}
	}
}

// TestPNC004CompoundHeaderAfterInlineColonRejects: CPython admits only a
// simple statement after a compound header's same-line colon, so a nested
// definition or compound header there is not Python 3.12 either, the same
// closure TestPNC004CompoundStatementAfterSemicolonRejects pins for ';'.
func TestPNC004CompoundHeaderAfterInlineColonRejects(t *testing.T) {
	assertMalformed(t, []string{
		"if x: def f(): pass\n",
		"if x: class C: pass\n",
		"while x: def f(): pass\n",
		"if x: if y: pass\n",
		"if x: for i in y: pass\n",
		"if x: async def f(): pass\n",
	})
	for _, source := range []string{"if x: pass\n", "if x: return 1\n", "while x: break\n"} {
		if reason := ParsePython312Subset("probe.py", []byte(source), discardFacts{}); reason != "" {
			t.Errorf("ParsePython312Subset(%q) = %q, want accepted", source, reason)
		}
	}
}

// TestPNC004StarAndEmptyFromImportsReject: `*` is only the sole bare target and
// an empty parenthesized list names nothing; each emitted an import fact.
func TestPNC004StarAndEmptyFromImportsReject(t *testing.T) {
	assertMalformed(t, []string{
		"from a import ()\n",
		"from a import *, b\n",
		"from a import b, *\n",
		"from a import * as x\n",
	})
}

// TestPNC003ParenthesizedImportListWithoutTrailingComma: the closing
// parenthesis directly after the last name rejected the whole source.
func TestPNC003ParenthesizedImportListWithoutTrailingComma(t *testing.T) {
	for _, source := range []string{"from a import (b, c)\n", "from .a import (b as c, d)\n", "from a import (b)\n", "from a import (\n    b,\n    c\n)\n"} {
		facts := recordedFacts{}
		if reason := ParsePython312Subset("probe.py", []byte(source), &facts); reason != "" {
			t.Errorf("ParsePython312Subset(%q) = %q, want accepted", source, reason)
			continue
		}
		if len(facts) != 1 || !strings.HasPrefix(facts[0], "python.import.static ") {
			t.Errorf("ParsePython312Subset(%q) facts = %q, want one import", source, facts)
		}
	}
	assertMalformed(t, []string{"from a import (b) c\n", "from a import (b c)\n"})
}
