package typescript

import "testing"

// TJAA-V0-018: the side-effect-free subset admitted for non-identity `use` values.
func TestPlaywrightPureExpression(t *testing.T) {
	for _, raw := range []string{
		"'x'", `"x"`, "1", "-1", "1e-5", "0x1F", "10n", ".5", "true", "null", "undefined", "authFile",
		"process.env.BASE_URL", "a.b.c?.[d]", "a?.b", "x.default", "a[b]['c']",
		"a / b", "a >>> 1", "a == b", "a !== b", "a <= b && !c", "typeof a === 'string'", "void 0", "'k' in env",
		"x ? y : z", "(a ?? b) || c", "[1, , 2]", "{ a }", "{ 'a': 1, [k]: v, get: 1 }",
		"`plain`", "`a${b}c${d.e ?? 'f'}`", "`a${`b${c}`}`", "`${ { a: 1 }.a }`",
	} {
		if !playwrightPureExpression(raw) {
			t.Errorf("refused side-effect-free %s", raw)
		}
	}
	for _, raw := range []string{
		"", "a = b", "a => b", "a += 1", "a **= 2", "a >>>= 1", "a ??= b", "a++", "--a", "f()", "a.b()", "a?.()",
		"a?.b()", "tag`x`", "`${f()}`", "`${`${f()}`}`", "new A", "delete a.b", "await a", "import('x')", "import.meta",
		"function () {}", "class {}", "async () => 1", "{ get a() { return 1 } }", "{ a() {} }", "{ ...b }", "[...b]",
		"a, b", "(a, b)", "/x/", "/x/.source", "a.#b", "a; b", "a b", "'unterminated", "`${a`", "(a", "a)", "\\u0061",
	} {
		if playwrightPureExpression(raw) {
			t.Errorf("admitted %q", raw)
		}
	}
}
