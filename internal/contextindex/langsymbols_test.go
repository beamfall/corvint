package contextindex

import (
	"fmt"
	"strings"
	"testing"
)

// symbolPair renders a symbol as "kind name@line", which is short enough to
// write expectations in without a struct literal per case.
func symbolPair(symbol Symbol) string {
	return fmt.Sprintf("%s %s@%d", symbol.Kind, symbol.Name, symbol.Line)
}

func extracted(t *testing.T, extractor func(Source, string) ([]Symbol, []ExtractionNote), text string) ([]string, []string) {
	t.Helper()
	symbols, notes := extractor(Source{Path: "a", BlobHash: "h"}, text)
	rendered := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		rendered = append(rendered, symbolPair(symbol))
	}
	reasons := make([]string, 0, len(notes))
	for _, note := range notes {
		if note.Path != "a" {
			t.Fatalf("note path = %q, want %q", note.Path, "a")
		}
		reasons = append(reasons, note.Reason)
	}
	return rendered, reasons
}

func assertSymbols(t *testing.T, name string, got, want []string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%s:\n got  = %v\n want = %v", name, got, want)
	}
}

func TestRustSymbols(t *testing.T) {
	for _, testCase := range []struct {
		name, source string
		want         []string
	}{
		{"functions", "fn plain() {}\npub fn public() {}\npub(crate) fn scoped() {}\n",
			[]string{"func plain@1", "func public@2", "func scoped@3"}},
		{"async and const and unsafe", "pub async fn fetch() {}\nconst fn sized() {}\npub unsafe extern \"C\" fn raw() {}\n",
			[]string{"func fetch@1", "func sized@2", "func raw@3"}},
		{"types", "struct Point;\npub enum Color {}\ntrait Draw {}\ntype Alias = u8;\nunion Bits {}\n",
			[]string{"type Point@1", "type Color@2", "type Draw@3", "type Alias@4", "type Bits@5"}},
		{"module and macro", "mod parser;\nmacro_rules! shout {}\n",
			[]string{"module parser@1", "macro shout@2"}},
		{"consts at top level", "const MAX: u8 = 9;\nstatic NAME: &str = \"x\";\n",
			[]string{"var MAX@1", "var NAME@2"}},

		// Negative cases: the scanner must not read declarations out of
		// comments or string literals.
		{"line comment", "// fn commented() {}\nfn real() {}\n", []string{"func real@2"}},
		{"block comment", "/* fn a() {}\n   struct B;\n*/\nfn real() {}\n", []string{"func real@4"}},
		{"nested block comment", "/* outer /* inner */ still comment\nfn hidden() {}\n*/\nfn real() {}\n",
			[]string{"func real@4"}},
		{"string literal", "let q = \"fn fake() {}\";\nfn real() {}\n", []string{"func real@2"}},
		{"raw string", "let q = r#\"fn fake() {} struct S;\"#;\nfn real() {}\n", []string{"func real@2"}},
		{"raw string spanning lines", "let q = r#\"\nfn fake() {}\n\"#;\nfn real() {}\n", []string{"func real@4"}},

		// Lifetimes are the Rust-specific lexing trap: `'a` must not open a
		// character literal and swallow the rest of the signature.
		{"lifetime does not open a char", "pub fn borrow<'a>(v: &'a str) -> &'a str {}\nfn after() {}\n",
			[]string{"func borrow@1", "func after@2"}},
		{"char literal still lexes", "fn tick() { let c = '\\''; }\nfn after() {}\n",
			[]string{"func tick@1", "func after@2"}},

		// A local `const` inside a function body is not a file-level symbol,
		// but an associated const in an `impl` block is. Both sit at brace
		// depth 1, so only the function-body flag separates them.
		{"function locals suppressed", "fn outer() {\n    const LOCAL: u8 = 1;\n}\n",
			[]string{"func outer@1"}},
		{"associated const survives", "impl Point {\n    const ORIGIN: u8 = 0;\n    fn new() {}\n}\n",
			[]string{"var ORIGIN@2", "func new@3"}},
		{"const after a function body reappears", "fn outer() {\n    let x = 1;\n}\nconst TOP: u8 = 2;\n",
			[]string{"func outer@1", "var TOP@4"}},

		// `impl` anchors methods but declares no new symbol of its own.
		{"impl emits nothing itself", "impl Point {\n    pub fn new() -> Self {}\n}\n",
			[]string{"func new@2"}},
		{"identifier prefix is not a keyword", "fnord();\nstructure();\nfn real() {}\n",
			[]string{"func real@3"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, notes := extracted(t, rustSymbols, testCase.source)
			assertSymbols(t, testCase.name, got, testCase.want)
			if len(notes) != 0 {
				t.Errorf("unexpected notes %v", notes)
			}
		})
	}
}

func TestSwiftSymbols(t *testing.T) {
	for _, testCase := range []struct {
		name, source string
		want         []string
	}{
		{"functions and types", "func plain() {}\npublic final class Store {}\nstruct Point {}\nenum Color {}\nprotocol Drawable {}\nactor Worker {}\n",
			[]string{"func plain@1", "type Store@2", "type Point@3", "type Color@4", "type Drawable@5", "type Worker@6"}},
		{"members", "class A {\n    var count = 0\n    let name = \"x\"\n    func go() {}\n    init() {}\n    deinit {}\n}\n",
			[]string{"type A@1", "var count@2", "var name@3", "func go@4", "func init@5", "func deinit@6"}},
		{"attributes are skipped", "@objc public func tapped() {}\n@available(iOS 13, *) struct New {}\n",
			[]string{"func tapped@1", "type New@2"}},
		{"extension and typealias", "extension String {}\ntypealias Handler = () -> Void\n",
			[]string{"type String@1", "type Handler@2"}},
		{"backticked identifier", "func `default`() {}\n", []string{"func default@1"}},

		// Locals inside a function body must not be reported as members.
		{"locals suppressed", "class A {\n    func go() {\n        let local = 1\n        var other = 2\n    }\n}\n",
			[]string{"type A@1", "func go@2"}},
		// A local inside a TOP-LEVEL function sits at depth 1, the same depth
		// as a type member, so the depth bound alone would have emitted it.
		{"top level function locals suppressed", "func go() {\n    let local = 1\n}\nlet global = 2\n",
			[]string{"func go@1", "var global@4"}},
		{"line comment", "// func fake() {}\nfunc real() {}\n", []string{"func real@2"}},
		{"nested block comment", "/* /* func hidden() {} */ still */\nfunc real() {}\n", []string{"func real@2"}},
		{"triple quoted string", "let q = \"\"\"\nfunc fake() {}\nclass Fake {}\n\"\"\"\nfunc real() {}\n",
			[]string{"var q@1", "func real@5"}},
		{"raw string", "let q = #\"func fake() {}\"#\nfunc real() {}\n", []string{"var q@1", "func real@2"}},
		{"apostrophe is not a char literal", "let s = \"it's fine\"\nfunc real() {}\n",
			[]string{"var s@1", "func real@2"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, notes := extracted(t, swiftSymbols, testCase.source)
			assertSymbols(t, testCase.name, got, testCase.want)
			if len(notes) != 0 {
				t.Errorf("unexpected notes %v", notes)
			}
		})
	}
}

func TestKotlinSymbols(t *testing.T) {
	for _, testCase := range []struct {
		name, source string
		want         []string
	}{
		{"functions and types", "fun plain() {}\nclass Store {}\ninterface Drawable {}\nobject Single {}\ndata class Point(val x: Int)\nenum class Color {}\n",
			[]string{"func plain@1", "type Store@2", "type Drawable@3", "type Single@4", "type Point@5", "type Color@6"}},
		{"members", "class A {\n    val name = \"x\"\n    var count = 0\n    suspend fun go() {}\n}\n",
			[]string{"type A@1", "var name@2", "var count@3", "func go@4"}},
		{"locals suppressed", "class A {\n    fun go() {\n        val local = 1\n    }\n}\n",
			[]string{"type A@1", "func go@2"}},
		{"top level function locals suppressed", "fun go() {\n    val local = 1\n}\nval global = 2\n",
			[]string{"func go@1", "var global@4"}},
		{"typealias", "typealias Handler = () -> Unit\n", []string{"type Handler@1"}},

		// An extension function's dotted prefix is the receiver type, not part
		// of the declared name.
		{"extension function names itself not its receiver",
			"private fun RecordingBridge.loadfileCount() = 0\n",
			[]string{"func loadfileCount@1"}},
		{"extension property", "val Foo.isEmpty: Boolean get() = true\n",
			[]string{"var isEmpty@1"}},
		{"line comment", "// fun fake() {}\nfun real() {}\n", []string{"func real@2"}},
		{"triple quoted", "val q = \"\"\"\nfun fake() {}\n\"\"\"\nfun real() {}\n",
			[]string{"var q@1", "func real@4"}},
		{"nested block comment", "/* /* fun hidden() {} */ x */\nfun real() {}\n", []string{"func real@2"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, notes := extracted(t, kotlinSymbols, testCase.source)
			assertSymbols(t, testCase.name, got, testCase.want)
			if len(notes) != 0 {
				t.Errorf("unexpected notes %v", notes)
			}
		})
	}
}

func TestCSharpSymbols(t *testing.T) {
	for _, testCase := range []struct {
		name, source string
		want         []string
	}{
		{"types", "namespace App;\npublic class Store {}\ninternal struct Point {}\npublic interface IDraw {}\npublic enum Color {}\npublic record Item(int Id);\n",
			[]string{"module App@1", "type Store@2", "type Point@3", "type IDraw@4", "type Color@5", "type Item@6"}},
		{"methods", "public class A {\n    public void Go() {}\n    private static int Count(int x) { return x; }\n    public async Task<int> FetchAsync() {}\n}\n",
			[]string{"type A@1", "func Go@2", "func Count@3", "func FetchAsync@4"}},
		{"constructor matches enclosing type", "public class Store {\n    public Store() {}\n}\n",
			[]string{"type Store@1", "func Store@2"}},
		{"properties", "public class A {\n    public int Total { get; set; }\n    private string Name { get; }\n}\n",
			[]string{"type A@1", "var Total@2", "var Name@3"}},

		// Precision guards: a call site and an unmodified local must not read
		// as declarations.
		{"call site is not a declaration", "public class A {\n    public void Go() {\n        Helper(1);\n        var x = Compute();\n    }\n}\n",
			[]string{"type A@1", "func Go@2"}},
		// The `=` before the parenthesis is what separates this from a method
		// declaration: it is a field with a computed initialiser.
		{"assignment is a field not a method", "public class A {\n    private int count = Compute();\n}\n",
			[]string{"type A@1", "var count@2"}},
		{"control flow is not a method", "public class A {\n    public void Go() {\n        if (x) { }\n        while (y) { }\n        foreach (var i in list) { }\n    }\n}\n",
			[]string{"type A@1", "func Go@2"}},
		{"line comment", "// public class Fake {}\npublic class Real {}\n", []string{"type Real@2"}},

		// C# block comments do NOT nest: the first close ends the comment, so
		// the code after it on that line is live.
		{"block comment does not nest", "/* /* */ public class Real {}\n", []string{"type Real@1"}},
		{"verbatim string", "public class A {\n    string q = @\"public class Fake {}\";\n}\n",
			[]string{"type A@1"}},
		{"verbatim escaped quote", "public class A {\n    string q = @\"say \"\"public class Fake {}\"\" done\";\n}\n",
			[]string{"type A@1"}},
		{"attributes skipped", "[Serializable]\npublic class Real {}\n", []string{"type Real@2"}},

		// KNOWN GAP, asserted so it cannot regress silently. Interface members
		// carry no access modifier, and the modifier prelude is what keeps
		// every method CALL inside a body from reading as a declaration.
		// Recovering these members needs enclosing-block awareness this line
		// scanner does not have, so the interface is indexed and its members
		// are not.
		{"interface members are not extracted",
			"public interface IStore {\n    void Save(int id);\n    int Count { get; }\n}\n",
			[]string{"type IStore@1"}},

		// A delegate is spelled like a signature, so the identifier after the
		// keyword is the RETURN type; the name sits before the parameter list.
		{"delegate names itself not its return type",
			"internal delegate bool DpapiTransform(ref DataBlob input);\n",
			[]string{"type DpapiTransform@1"}},
		{"delegate with void return", "public delegate void Handler(int x);\n",
			[]string{"type Handler@1"}},

		// An event's name trails its delegate type, exactly like a field's.
		{"event names itself not its delegate type",
			"public class A {\n    public event Action<string>? Log;\n}\n",
			[]string{"type A@1", "var Log@2"}},

		// Fields: a type, a name and a terminator, with no parens and no brace.
		{"fields", "public class A {\n    private readonly int count;\n    public const string Name = \"x\";\n}\n",
			[]string{"type A@1", "var count@2", "var Name@3"}},
		{"field with constructed initialiser",
			"public class A {\n    public static readonly Foo Bar = new Foo();\n}\n",
			[]string{"type A@1", "var Bar@2"}},
		// A local without a modifier must still not read as a field.
		{"unmodified local is not a field",
			"public class A {\n    public void Go() {\n        int local = 1;\n        string other;\n    }\n}\n",
			[]string{"type A@1", "func Go@2"}},

		// A dotted name denotes one namespace, not its first segment.
		{"qualified namespace", "namespace Serilog.Tests.Core;\n",
			[]string{"module Serilog.Tests.Core@1"}},
		{"file scoped namespace then class", "namespace A.B;\npublic class C {}\n",
			[]string{"module A.B@1", "type C@2"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, notes := extracted(t, csharpSymbols, testCase.source)
			assertSymbols(t, testCase.name, got, testCase.want)
			if len(notes) != 0 {
				t.Errorf("unexpected notes %v", notes)
			}
		})
	}
}

// TestExtractionNotesAreReported is the regression guard for the defect this
// work exists to avoid: an extractor that stops early and leaves no trace. Each
// case drives one give-up path and asserts the note reaches the caller.
func TestExtractionNotesAreReported(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		extractor  func(Source, string) ([]Symbol, []ExtractionNote)
		source     string
		wantNote   string
		wantCounts int
	}{
		{"rust unterminated block comment", rustSymbols,
			"fn real() {}\n/* opened and never closed\nfn hidden() {}\n", noteUnterminatedComment, 1},
		{"swift unterminated block comment", swiftSymbols,
			"func real() {}\n/* never closed\n", noteUnterminatedComment, 1},
		{"rust unterminated raw string", rustSymbols,
			"fn real() {}\nlet q = r#\"never closed\n", noteUnterminatedString, 1},
		{"swift unterminated triple quote", swiftSymbols,
			"func real() {}\nlet q = \"\"\"\nstill inside\n", noteUnterminatedString, 1},
		{"kotlin unterminated triple quote", kotlinSymbols,
			"fun real() {}\nval q = \"\"\"\nstill inside\n", noteUnterminatedString, 1},
		{"csharp unterminated verbatim string", csharpSymbols,
			"public class A {\n    string q = @\"never closed\n", noteUnterminatedString, 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			symbols, notes := testCase.extractor(Source{Path: "p"}, testCase.source)
			if len(notes) != testCase.wantCounts {
				t.Fatalf("notes = %v, want %d", notes, testCase.wantCounts)
			}
			if notes[0].Reason != testCase.wantNote {
				t.Errorf("reason = %q, want %q", notes[0].Reason, testCase.wantNote)
			}
			if notes[0].Path != "p" {
				t.Errorf("path = %q, want %q", notes[0].Path, "p")
			}
			// The symbols found before the break are still returned: a note
			// narrows the claim, it does not discard the evidence.
			if len(symbols) == 0 {
				t.Errorf("expected the declarations before the break to survive")
			}
		})
	}
}

func TestSymbolCapIsReported(t *testing.T) {
	var builder strings.Builder
	for index := 0; index < maxSymbolsPerSource+50; index++ {
		fmt.Fprintf(&builder, "fn f%d() {}\n", index)
	}
	symbols, notes := rustSymbols(Source{Path: "big.rs"}, builder.String())
	if len(symbols) != maxSymbolsPerSource {
		t.Fatalf("symbols = %d, want %d", len(symbols), maxSymbolsPerSource)
	}
	if len(notes) != 1 || notes[0].Reason != noteSymbolCap {
		t.Fatalf("notes = %v, want one %q", notes, noteSymbolCap)
	}
}

func TestLineCapIsReported(t *testing.T) {
	var builder strings.Builder
	for index := 0; index < maxSourceLines+10; index++ {
		builder.WriteString("// filler\n")
	}
	symbols, notes := rustSymbols(Source{Path: "long.rs"}, builder.String())
	if len(symbols) != 0 {
		t.Fatalf("symbols = %d, want 0", len(symbols))
	}
	if len(notes) != 1 || notes[0].Reason != noteLineCap {
		t.Fatalf("notes = %v, want one %q", notes, noteLineCap)
	}
}

// TestCleanFileHasNoNotes states the contract the notes give a caller: a file
// the scanner walked to the end reports nothing, so a note always means
// something real.
func TestCleanFileHasNoNotes(t *testing.T) {
	for name, testCase := range map[string]struct {
		extractor func(Source, string) ([]Symbol, []ExtractionNote)
		source    string
	}{
		"rust":   {rustSymbols, "/* closed */\nfn a() {}\nlet q = r#\"x\"#;\n"},
		"swift":  {swiftSymbols, "let q = \"\"\"\ntext\n\"\"\"\nfunc a() {}\n"},
		"kotlin": {kotlinSymbols, "/* closed */\nfun a() {}\n"},
		"csharp": {csharpSymbols, "public class A {\n    string q = @\"closed\";\n}\n"},
	} {
		t.Run(name, func(t *testing.T) {
			_, notes := testCase.extractor(Source{Path: "p"}, testCase.source)
			if len(notes) != 0 {
				t.Errorf("notes = %v, want none", notes)
			}
		})
	}
}

// TestLanguageSymbolsDispatch checks that an unsupported suffix is reported as
// unhandled rather than as a file with nothing to declare. The two are
// different facts and the caller distinguishes them.
func TestLanguageSymbolsDispatch(t *testing.T) {
	for _, sourcePath := range []string{"a.rs", "a.cs", "a.swift", "a.kt", "a.kts", "a.rb"} {
		if _, _, handled := languageSymbols(Source{Path: sourcePath}, ""); !handled {
			t.Errorf("%s: handled = false, want true", sourcePath)
		}
	}
	for _, sourcePath := range []string{"a.go", "a.py", "a.sql", "a.m", "a.txt", "noext"} {
		if _, _, handled := languageSymbols(Source{Path: sourcePath}, ""); handled {
			t.Errorf("%s: handled = true, want false", sourcePath)
		}
	}
}

// TestLineEndingsAreNormalized covers the one input shape that produced a
// silently truncated walk: a classic-Mac source whose lines end in a bare CR
// arrives as a single line, so the scanner would find one declaration, miss the
// rest, and report no note because it did reach the end.
func TestLineEndingsAreNormalized(t *testing.T) {
	for name, text := range map[string]string{
		"crlf": "public class A {\r\n    public void Go() {}\r\n}\r\n",
		"cr":   "public class A {\r    public void Go() {}\r}\r",
		"lf":   "public class A {\n    public void Go() {}\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			symbols, notes, handled := languageSymbols(Source{Path: "a.cs"}, text)
			if !handled {
				t.Fatal("handled = false")
			}
			if len(symbols) != 2 {
				t.Fatalf("symbols = %d, want 2: %v", len(symbols), symbols)
			}
			if symbols[0].Name != "A" || symbols[1].Name != "Go" {
				t.Errorf("names = %q, %q", symbols[0].Name, symbols[1].Name)
			}
			if symbols[1].Line != 2 {
				t.Errorf("second symbol line = %d, want 2", symbols[1].Line)
			}
			if len(notes) != 0 {
				t.Errorf("notes = %v, want none", notes)
			}
		})
	}
}
