package typescript

import "testing"

// TJAA-V0-018: the side-effect-free subset admitted for non-identity `use` values. A coercing
// operator, template substitution or computed key admits only operands proven primitive, because
// coercing an object can call its toString, valueOf, Symbol.toPrimitive or Symbol.hasInstance.
func TestPlaywrightPureExpression(t *testing.T) {
	for _, raw := range []string{
		"'x'", `"x"`, "1", "-1", "1e-5", "0x1F", "10n", ".5", "true", "null", "undefined", "authFile",
		"process.env.BASE_URL", "a.b.c?.['d']", "a?.b", "x.default", "a[0]['c']", "process.env['X']",
		"1 / 2", "1 >>> 1", "a === b", "a !== b && !c", "typeof a === 'string'", "void 0", "void a", "!a",
		"x ? y : z", "(a ?? b) || c", "[1, , 2]", "{ a }", "{ 'a': 1, ['k']: v, get: 1, 2: x }",
		"`plain`", "`a${process.env.B}c${process.env.D ?? 'f'}`", "`a${`b${1}`}`", "`${typeof a}`",
		"process.env.PORT * 2", "-process.env.X", "process.env.A + '/' + process.env.B", "typeof a + 'x'",
		"(process.env.A || 'x') + 1", "process.env.CI ? 1 : 2", "a?.5:1",
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
		// implicit coercion of a value not proven primitive
		"`${url}`", "`${a.b}`", "`${a ?? 'x'}`", "`${{ a: 1 }.a}`", "a * 2", "a / b", "+a", "-a", "~a", "a + 'x'",
		"'x' + a", "a < 1", "a >= b", "a == 1", "a != null", "a >>> 1", "a | 0", "'k' in env", "a instanceof B",
		"a[b]", "a?.[b]", "a[b.c]", "{ [k]: 1 }", "(a ?? 'x') + 'y'", "(x ? 1 : a) * 2", "process.env + ''",
		"process.env.X.toString + ''", "process + 1", "undefined + 1",
	} {
		if playwrightPureExpression(raw) {
			t.Errorf("admitted %q", raw)
		}
	}
}
