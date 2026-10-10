package typescript

import (
	"strings"
	"testing"
)

// TJAA-V0-018: the side-effect-free subset admitted for non-identity `use` values. A coercing
// operator, template substitution or computed key admits only operands proven primitive, because
// coercing an object can call its toString, valueOf, Symbol.toPrimitive or Symbol.hasInstance.
func TestPlaywrightPureExpression(t *testing.T) {
	for _, raw := range []string{
		"'x'", `"x"`, "1", "-1", "1e-5", "0x1F", "10n", ".5", "true", "null", "undefined", "authFile",
		"process.env.BASE_URL", "a.b.c?.['d']", "a?.b", "x.default", "a[0]['c']", "process.env['X']",
		"1 / 2", "1 >>> 1", "a === b", "a !== b && !c", "typeof a === 'string'", "void 0", "void a", "!a",
		"x ? y : z", "(a ?? b) || c", "[1, , 2]", "{ a }", "{ 'a': 1, ['k']: v, get: 1, 2: x }",
		"`plain`", "`a${'b'}c${1 + 2}`", "`a${`b${1}`}`", "`${typeof a}`", "'a' + '/' + 1", "-1 * 2",
		"typeof a + 'x'", "(true || 'x') + 1", "(null ?? 0) - 1", "process.env.CI ? 1 : 2", "a?.5:1",
		"process.env.A ?? 'x'", "process.env.A || process.env.B", "process.env.A === 'x'",
		// the ECMAScript numeric grammar
		"0", "1_000", "1_000.5_5e1_0", "0b101", "0B1_1", "0o17", "0x1F_FF", "0xffn", "10n", "1.", "1.e3",
		"1.5e+3", "2E-3", ".5e1", "1..payload", "1.0.x", "1 .x", ".5.x", "-1_0 + 2", "- -1",
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
		// process.env.NAME is not proven primitive: a getter, Proxy or replaced process.env can
		// return an object, so only non-coercing operators take it
		"process.env.X + ''", "`${process.env.X}`", "`a${process.env.B ?? 'f'}`", "-process.env.X",
		"process.env.PORT * 2", "(process.env.A || 'x') + 1", "process.env['X'] + ''", "{ [process.env.K]: 1 }",
		"a[process.env.K]", "'x'.length + 1",
		// a member read on a number is not primitive, and a malformed number is refused
		"1..payload + ''", "1.0.x + ''", ".5.x * 2", "1 .x + 1", "1.toString", "1abc", "1_", "1__0", "1._5",
		"0x", "0xg", "0b2", "0o8", "0_1", "01", "08", "1e", "1e+", "1.5n", "1e3n", "08n", "1n2", "1$", "0x1F.x + 1",
		// update operators are never a pair of unary signs
		"++process.env.COUNTER", "--process.env.COUNTER", "++a", "1--1", "1++1", "a--", "a ++",
	} {
		if playwrightPureExpression(raw) {
			t.Errorf("admitted %q", raw)
		}
	}
	// Right-associative exponentiation and every other binary nesting share the depth bound.
	for _, raw := range []string{strings.Repeat("1**", 100000) + "1", strings.Repeat("1**(", 200) + "1" + strings.Repeat(")", 200)} {
		if playwrightPureExpression(raw) {
			t.Errorf("admitted %d-byte nested exponentiation", len(raw))
		}
	}
	if !playwrightPureExpression(strings.Repeat("1**", 20)+"1") || !playwrightPureExpression(strings.Repeat("1+", 100000)+"1") {
		t.Error("refused a shallow exponentiation chain or a long left-associative sum")
	}
}
