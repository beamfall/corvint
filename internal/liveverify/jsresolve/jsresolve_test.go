package jsresolve

import "testing"

func TestImportsAtLineVerdicts(t *testing.T) {
	esm := []byte("import fs from 'fs'\nimport { compute } from './core.js'\nexport * from './core'\nexport { a } from \"../lib\"\nimport './side-effect'\n")
	wrapped := []byte("import {\n  compute,\n  total,\n} from './core'\n")
	dynamic := []byte("const core = await import('./core.js')\nconst lib = require('./lib')\nconst dyn = import(name)\n")
	noise := []byte("// import x from './core'\nconst s = 'import y from \"./core\"'\nconst t = `import z from './core'`\nobj.import('./core')\n/* import w from './core' */ const u = 1\n")
	typed := []byte("import type { Shape } from './shapes'\nimport { type Shape, draw } from './draw'\n")
	for _, test := range []struct {
		name      string
		content   []byte
		specifier string
		line      int
		want      bool
	}{
		{"default import", esm, "fs", 1, true},
		{"named import with extension", esm, "./core.js", 2, true},
		{"export star from", esm, "./core", 3, true},
		{"export list from double quotes", esm, "../lib", 4, true},
		{"side-effect import", esm, "./side-effect", 5, true},
		{"wrong line", esm, "./core.js", 1, false},
		{"other specifier", esm, "./other", 2, false},
		{"wrapped clause spans keyword line", wrapped, "./core", 1, true},
		{"wrapped clause spans middle line", wrapped, "./core", 3, true},
		{"wrapped clause spans from line", wrapped, "./core", 4, true},
		{"wrapped clause not past from", wrapped, "./core", 5, false},
		{"dynamic import", dynamic, "./core.js", 1, true},
		{"require", dynamic, "./lib", 2, true},
		{"dynamic import of a variable", dynamic, "name", 3, false},
		{"line comment is not an import", noise, "./core", 1, false},
		{"string is not an import", noise, "./core", 2, false},
		{"template is not an import", noise, "./core", 3, false},
		{"property access is not an import", noise, "./core", 4, false},
		{"block comment is not an import", noise, "./core", 5, false},
		{"type-only import", typed, "./shapes", 1, true},
		{"inline type modifier", typed, "./draw", 2, true},
	} {
		if got := ImportsAtLine(test.content, test.specifier, test.line); got != test.want {
			t.Errorf("%s: got %v want %v", test.name, got, test.want)
		}
	}
}

func TestImportsListsStatementsInOrderWithLines(t *testing.T) {
	source := []byte("/* header\n spans */ import a from './a'\nconst x = `${\n1}`\nexport {\n b } from './b'\n")
	got := Imports(source)
	want := []Import{{Line: 2, SpecifierLine: 2, Specifier: "./a"}, {Line: 5, SpecifierLine: 6, Specifier: "./b"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("import %d: got %+v want %+v", index, got[index], want[index])
		}
	}
}

func TestIdentifierAtLine(t *testing.T) {
	source := []byte("import { compute } from './core'\n// compute here\nconst s = 'compute'\nconst n = core.compute(1)\nconst computed = 2\n")
	for _, test := range []struct {
		name string
		line int
		want bool
	}{
		{"import clause", 1, true},
		{"comment", 2, false},
		{"string", 3, false},
		{"property access", 4, true},
		{"longer word", 5, false},
	} {
		if got := IdentifierAtLine(source, "compute", test.line); got != test.want {
			t.Errorf("%s: got %v want %v", test.name, got, test.want)
		}
	}
}

func TestDeclaresAtTopLevel(t *testing.T) {
	source := []byte(`function plain() {}
async function later() {}
function* gen() {}
class Widget { method() { const inner = 1 } }
const first = 1; let second = 2
var third = 3
export function exported() {}
export default function main() {}
export class Model {}
export const setting = 1
export { plain as renamed, second }
const named = function expression() {}
const Boxed = class Hidden {}
if (true) { function nested() {} }
const s = 'function fake() {}'
// function commented() {}
`)
	for _, test := range []struct {
		name string
		want bool
	}{
		{"plain", true}, {"later", true}, {"gen", true}, {"Widget", true},
		{"first", true}, {"second", true}, {"third", true},
		{"exported", true}, {"main", true}, {"Model", true}, {"setting", true},
		{"renamed", true}, {"named", true}, {"Boxed", true},
		{"method", false}, {"inner", false}, {"expression", false}, {"Hidden", false},
		{"nested", false}, {"fake", false}, {"commented", false}, {"missing", false},
	} {
		if got := DeclaresAtTopLevel(source, test.name); got != test.want {
			t.Errorf("%s: got %v want %v", test.name, got, test.want)
		}
	}
}

func TestSubstitutionBodiesLexAsCode(t *testing.T) {
	ghost := []byte("const a = `${\"}\" + `${import(\"./ghost\")}`}`\n")
	hidden := []byte("const b = `${\"`\"}`\nimport { real } from \"./real\"\n")
	object := []byte("const c = `${ {a: 1}.a + `${import(\"./ghost\")}` }`\n")
	multiline := []byte("const d = `${\n  \"`\" +\n  // }\n  1\n}`\nimport z from './after'\n")
	for _, test := range []struct {
		name      string
		content   []byte
		specifier string
		line      int
		want      bool
	}{
		{"brace in a string does not close the substitution", ghost, "./ghost", 1, false},
		{"backtick in a string does not hide a later import", hidden, "./real", 2, true},
		{"object literal brace does not close the substitution", object, "./ghost", 1, false},
		{"import after a multi-line substitution keeps its line", multiline, "./after", 6, true},
		{"multi-line substitution does not shift the import up", multiline, "./after", 5, false},
	} {
		if got := ImportsAtLine(test.content, test.specifier, test.line); got != test.want {
			t.Errorf("%s: got %v want %v", test.name, got, test.want)
		}
	}
}

func TestEscapedLineBreakInLiteralCountsALine(t *testing.T) {
	quoted := []byte("const a = 'x\\\ny'\nimport z from './z'\n")
	template := []byte("const a = `x\\\ny`\nimport z from './z'\n")
	quotedCRLF := []byte("const a = 'x\\\r\ny'\r\nimport z from './z'\r\n")
	templateCRLF := []byte("const a = `x\\\r\ny`\r\nimport z from './z'\r\n")
	identifier := []byte("const a = \"x\\\ny\"; compute()\n")
	for _, test := range []struct {
		name    string
		content []byte
		line    int
		want    bool
	}{
		{"quoted continuation keeps the import on its line", quoted, 3, true},
		{"quoted continuation does not shift the import up", quoted, 2, false},
		{"template continuation keeps the import on its line", template, 3, true},
		{"template continuation does not shift the import up", template, 2, false},
		{"quoted CRLF continuation keeps the import on its line", quotedCRLF, 3, true},
		{"template CRLF continuation keeps the import on its line", templateCRLF, 3, true},
	} {
		if got := ImportsAtLine(test.content, "./z", test.line); got != test.want {
			t.Errorf("%s: got %v want %v", test.name, got, test.want)
		}
	}
	if got := Imports(quoted); len(got) != 1 || got[0].Line != 3 {
		t.Errorf("quoted continuation imports: got %+v want one on line 3", got)
	}
	if !IdentifierAtLine(identifier, "compute", 2) || IdentifierAtLine(identifier, "compute", 1) {
		t.Errorf("identifier after a quoted continuation is not on line 2")
	}
}

// TestQuotedCRLFContinuationStaysInsideTheLiteral pins that a `\` before
// `\r\n` continues a quoted literal through both bytes, so an import spelled
// in the literal's remainder is not lexed as code.
func TestQuotedCRLFContinuationStaysInsideTheLiteral(t *testing.T) {
	content := []byte("const a = 'x\\\r\ny; import z from \"./z\"'\r\nimport w from './w'\r\n")
	if got := Imports(content); len(got) != 1 || got[0].Specifier != "./w" || got[0].Line != 3 {
		t.Errorf("got %+v want only ./w on line 3", got)
	}
}

func TestLineCommentEndsAtEveryLineTerminator(t *testing.T) {
	for _, terminator := range []string{"\r", "\u2028", "\u2029"} {
		source := []byte("// note" + terminator + "import z from './z'\n// import g from './g'\nuse(z)\n")
		if got := Imports(source); len(got) != 1 || got[0].Specifier != "./z" || got[0].Line != 1 {
			t.Errorf("%q: imports got %+v want ./z on line 1", terminator, got)
		}
		if !IdentifierAtLine(source, "use", 3) {
			t.Errorf("%q: the terminator shifted the later line count", terminator)
		}
	}
}

func TestMultiLineBlockCommentIsAStatementBoundary(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   bool
	}{
		{"multi-line comment separates declarations", "const a = 1 /*\n*/ const b = 2\n", true},
		{"CRLF multi-line comment separates declarations", "const a = 1 /*\r\n*/ const b = 2\r\n", true},
		{"single-line comment is not a boundary", "const a = 1 /* x */ const b = 2\n", false},
	} {
		if got := DeclaresAtTopLevel([]byte(test.source), "b"); got != test.want {
			t.Errorf("%s: got %v want %v", test.name, got, test.want)
		}
	}
}

func TestEveryLineTerminatorIsAStatementBoundary(t *testing.T) {
	for _, terminator := range []string{"\r", " ", " "} {
		bare := []byte("x()" + terminator + "function b() {}\nuse(b)\n")
		comment := []byte("const a = 1 /*" + terminator + "*/ const b = 2\nuse(b)\n")
		for name, source := range map[string][]byte{"bare": bare, "block comment": comment} {
			if !DeclaresAtTopLevel(source, "b") {
				t.Errorf("%s %q: b is not declared after the terminator", name, terminator)
			}
			if !IdentifierAtLine(source, "use", 2) {
				t.Errorf("%s %q: the terminator shifted the later line count", name, terminator)
			}
		}
	}
}

func TestUnclosedQuoteEndsAtCarriageReturnOnly(t *testing.T) {
	carriage := []byte("'abc\rfunction b() {}\nuse(b)\n")
	if !DeclaresAtTopLevel(carriage, "b") || !IdentifierAtLine(carriage, "use", 2) {
		t.Errorf("an unclosed quote swallowed the code after a bare \\r")
	}
	for _, separator := range []string{" ", " "} {
		source := []byte("const s = 'a" + separator + "b'; function c() {}\nuse(c)\n")
		if !DeclaresAtTopLevel(source, "c") || !IdentifierAtLine(source, "c", 1) {
			t.Errorf("%q: the quoted literal ended at the separator", separator)
		}
	}
}

func TestBacktickInRegexLiteralOpensNoTemplate(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{"after =", "const r = /`/g\nfunction b() {}\nconst q = /`/\nuse(b)\n"},
		{"inside a class after (", "s.replace(/[/`]/, '')\nfunction b() {}\nt.split(/`/)\nuse(b)\n"},
		{"division between words stays division", "const r = x / y / z; function b() {}\nconst q = `\n`\nuse(b)\n"},
	} {
		source := []byte(test.source)
		if !DeclaresAtTopLevel(source, "b") || !IdentifierAtLine(source, "use", 4) {
			t.Errorf("%s: a backtick swallowed the declaration or shifted the line", test.name)
		}
	}
}

func TestDivisionAfterClosingBraceStaysCode(t *testing.T) {
	source := []byte("const value = {a: 1} / b / c\n")
	if !IdentifierAtLine(source, "b", 1) {
		t.Fatal("division after a closing brace hid its operand")
	}
}
