package breakagemap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// jsDeclarationExtensions are the JavaScript and TypeScript files whose
// top-level declarations the token extractor resolves (BKM-V0-011). JSX and
// TSX stay file-level: element text is not JavaScript and can hide braces from
// the shared lexer.
var jsDeclarationExtensions = map[string]bool{".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".mts": true, ".cts": true}

// declarationSpan returns the 1-based physical line span of the single
// top-level declaration named name in the file at p, for Go (a receiverless
// function) and for JavaScript/TypeScript (BKM-V0-011). ok is false whenever
// the declaration is absent, ambiguous or the file cannot be read with
// confidence; callers keep that as an unresolved unknown.
func declarationSpan(p string, b []byte, name string) (start, end int, ok bool) {
	switch ext := path.Ext(p); {
	case ext == ".go":
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "source.go", b, 0)
		if err != nil {
			return 0, 0, false
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
				return fset.PositionFor(fn.Pos(), false).Line, fset.PositionFor(fn.End(), false).Line, true
			}
		}
		return 0, 0, false
	case jsDeclarationExtensions[ext]:
		return jsDeclaration(string(b), name)
	}
	return 0, 0, false
}

// declarationText is the change-comparison identity of a selected
// declaration: Go keeps the exact declaration bytes (BKM-V0-009); JS/TS uses
// the declaration's whole physical lines, so any edit inside the declaration
// changes it.
func declarationText(p string, b []byte, name string) (string, bool) {
	if path.Ext(p) == ".go" {
		return declarationBytes(b, name)
	}
	start, end, ok := declarationSpan(p, b, name)
	if !ok {
		return "", false
	}
	return strings.Join(strings.Split(string(b), "\n")[start-1:end], "\n"), true
}

// jsSource is a lexed JS/TS file with bracket matches and nesting depth.
type jsSource struct {
	tokens []appmap.JSToken
	match  []int
	depth  []int
}

var jsModifiers = map[string]bool{"export": true, "default": true, "async": true, "declare": true, "abstract": true}

// jsExpressionLead are identifiers after which a function or class keyword
// starts an expression, not a declaration.
var jsExpressionLead = map[string]bool{"return": true, "yield": true, "await": true, "typeof": true, "void": true, "new": true, "delete": true, "in": true, "of": true, "instanceof": true, "extends": true, "case": true, "throw": true, "else": true, "do": true}

// jsTypeOperators precede a type, never a body brace.
var jsTypeOperators = map[string]bool{"extends": true, "implements": true, "keyof": true, "typeof": true, "in": true, "is": true, "as": true, "satisfies": true}

// jsStatementWords end an arrow's return-type scan: reaching one means the
// binding was not an arrow function.
var jsStatementWords = map[string]bool{"const": true, "let": true, "var": true, "function": true, "class": true, "export": true, "import": true, "return": true}

// jsDeclaration resolves one top-level `function NAME`, `class NAME`,
// `interface|enum|namespace NAME {…}`, or `const|let|var NAME =
// function…{…}` / `= (…) => {…}` / `= x => {…}` declaration. It fails closed
// on lexer uncertainty, unbalanced brackets, more than one top-level
// declaration of the name (TypeScript overloads included), any assignment to
// the name, a decorated class, an expression-bodied arrow, a type-annotated
// binding, or a body brace it cannot tell from a type literal.
func jsDeclaration(text, name string) (int, int, bool) {
	s, ok := lexJSSource(text)
	if !ok {
		return 0, 0, false
	}
	found, start, end := 0, 0, 0
	for i, t := range s.tokens {
		if t.Kind != appmap.JSIdent || t.Text != name || s.punct(i-1, ".") {
			continue
		}
		if s.assigned(i) {
			return 0, 0, false
		}
		if s.depth[i] != 0 || i == 0 {
			continue
		}
		kw := i - 1
		if s.punct(kw, "*") && kw > 0 {
			kw--
		}
		if s.tokens[kw].Kind != appmap.JSIdent {
			continue
		}
		switch s.tokens[kw].Text {
		case "function", "class", "const", "let", "var", "interface", "enum", "type", "namespace", "module":
		default:
			continue
		}
		if s.tokens[kw].Text != "function" && kw != i-1 {
			continue
		}
		found++
		first, last, resolved := s.declaration(kw, i)
		if !resolved {
			return 0, 0, false
		}
		start, end = first, last
	}
	if found != 1 {
		return 0, 0, false
	}
	return start, end, true
}

// assigned reports whether the name at token i is the target of an
// assignment (`=`, a compound `op=`, `++` or `--`) other than its own
// `const|let|var NAME =` binding.
func (s jsSource) assigned(i int) bool {
	if i > 0 && s.tokens[i-1].Kind == appmap.JSIdent && (s.tokens[i-1].Text == "const" || s.tokens[i-1].Text == "let" || s.tokens[i-1].Text == "var") {
		return false
	}
	next := i + 1
	if s.punct(next, "=") {
		return !(s.punct(next+1, "=") && s.tokens[next+1].Glued || s.punct(next+1, ">") && s.tokens[next+1].Glued)
	}
	if next >= len(s.tokens) || s.tokens[next].Kind != appmap.JSPunct {
		return false
	}
	switch op := s.tokens[next].Text; op {
	case "+", "-":
		if s.punct(next+1, op) && s.tokens[next+1].Glued {
			return true
		}
		fallthrough
	case "*", "/", "%", "&", "|", "^", "?":
		return s.punct(next+1, "=") && s.tokens[next+1].Glued && !(s.punct(next+2, "=") && s.tokens[next+2].Glued)
	}
	return false
}

func lexJSSource(text string) (jsSource, bool) {
	tokens, _, unsure := appmap.LexJS(text)
	if unsure {
		return jsSource{}, false
	}
	s := jsSource{tokens: tokens, match: make([]int, len(tokens)), depth: make([]int, len(tokens))}
	stack := []int{}
	for i, t := range tokens {
		s.match[i] = -1
		s.depth[i] = len(stack)
		if t.Kind != appmap.JSPunct {
			continue
		}
		switch t.Text {
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			if len(stack) == 0 || closer(tokens[stack[len(stack)-1]].Text) != t.Text {
				return jsSource{}, false
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			s.match[open], s.match[i] = i, open
			s.depth[i] = len(stack)
		}
	}
	return s, len(stack) == 0
}

func closer(open string) string {
	return map[string]string{"(": ")", "[": "]", "{": "}"}[open]
}

func (s jsSource) punct(i int, text string) bool {
	return i >= 0 && i < len(s.tokens) && s.tokens[i].Kind == appmap.JSPunct && s.tokens[i].Text == text
}

func (s jsSource) ident(i int, text string) bool {
	return i >= 0 && i < len(s.tokens) && s.tokens[i].Kind == appmap.JSIdent && s.tokens[i].Text == text
}

// declaration returns the line span of the top-level declaration whose
// keyword is at token kw and whose name is at token name.
func (s jsSource) declaration(kw, name int) (int, int, bool) {
	lead := kw
	for lead > 0 && s.tokens[lead-1].Kind == appmap.JSIdent && jsModifiers[s.tokens[lead-1].Text] {
		lead--
	}
	if !s.statementStart(lead) {
		return 0, 0, false
	}
	closing := -1
	switch s.tokens[kw].Text {
	case "function":
		closing = s.functionBody(name + 1)
	case "class":
		for i := range s.tokens {
			if s.punct(i, "@") {
				return 0, 0, false
			}
		}
		closing = s.bodyBrace(name + 1)
	case "interface", "enum", "namespace", "module":
		closing = s.bodyBrace(name + 1)
	case "const", "let", "var":
		if !s.punct(name+1, "=") {
			return 0, 0, false
		}
		at := name + 2
		if s.ident(at, "async") {
			at++
		}
		switch {
		case s.ident(at, "function"):
			at++
			if s.punct(at, "*") {
				at++
			}
			if at < len(s.tokens) && s.tokens[at].Kind == appmap.JSIdent {
				at++ // a named function expression
			}
			closing = s.functionBody(at)
		case s.punct(at, "(") && s.match[at] > at:
			closing = s.arrowBody(s.match[at] + 1)
		case at < len(s.tokens) && s.tokens[at].Kind == appmap.JSIdent:
			closing = s.arrowBody(at + 1)
		}
		if closing >= 0 && !s.expressionEnds(closing) {
			return 0, 0, false
		}
	}
	if closing < 0 {
		return 0, 0, false
	}
	return s.tokens[lead].Line, s.tokens[closing].Line, true
}

// statementStart reports whether token i begins a statement rather than
// continuing an expression.
func (s jsSource) statementStart(i int) bool {
	if i == 0 || s.punct(i-1, ";") || s.punct(i-1, "}") {
		return true
	}
	previous := s.tokens[i-1]
	if previous.Line == s.tokens[i].Line {
		return false
	}
	if previous.Kind == appmap.JSPunct {
		return previous.Text == ")" || previous.Text == "]"
	}
	return !(previous.Kind == appmap.JSIdent && jsExpressionLead[previous.Text])
}

// functionBody resolves `[<…>] (…) [: T] {…}` from token at, which follows
// the function name, and returns the body's closing brace or -1.
func (s jsSource) functionBody(at int) int {
	open := s.firstAtTop(at, "(")
	if open < 0 {
		return -1
	}
	return s.bodyBrace(s.match[open] + 1)
}

// arrowBody resolves `[: T] => {…}` from token at, which follows the
// parameters, and returns the body's closing brace or -1. An expression body
// cannot be bounded without a parser, so it fails closed.
func (s jsSource) arrowBody(at int) int {
	if !s.punct(at, "=") && !s.punct(at, ":") {
		return -1
	}
	for j := at; j < len(s.tokens); j++ {
		if s.depth[j] != 0 {
			continue
		}
		if s.punct(j, "=") && s.punct(j+1, ">") && s.tokens[j+1].Glued {
			if !s.punct(j+2, "{") {
				return -1
			}
			return s.match[j+2]
		}
		if s.punct(j, ";") || s.punct(j, "{") || s.punct(j, "=") || s.tokens[j].Kind == appmap.JSIdent && jsStatementWords[s.tokens[j].Text] {
			return -1
		}
	}
	return -1
}

// bodyBrace finds the top-level body brace from token at and returns its
// closing brace, or -1. A brace after a type operator (`:`, `|`, `&`, `<`,
// `,`, `=`, `=>`, `extends`, …) may be an object type, so it fails closed.
func (s jsSource) bodyBrace(at int) int {
	brace := s.firstAtTop(at, "{")
	if brace <= 0 {
		return -1
	}
	previous := s.tokens[brace-1]
	if previous.Kind == appmap.JSPunct && (strings.Contains(":|&<,=", previous.Text) || previous.Text == ">" && previous.Glued) || previous.Kind == appmap.JSIdent && jsTypeOperators[previous.Text] {
		return -1
	}
	return s.match[brace]
}

// firstAtTop returns the first top-level token at or after at with the given
// punctuation, or -1 when a top-level `;` ends the statement first.
func (s jsSource) firstAtTop(at int, text string) int {
	for j := at; j < len(s.tokens); j++ {
		if s.depth[j] != 0 {
			continue
		}
		if s.punct(j, text) {
			return j
		}
		if s.punct(j, ";") {
			return -1
		}
	}
	return -1
}

// expressionEnds reports whether a function or arrow expression closed at
// token closing also ends its binding, so no call or operator extends it.
func (s jsSource) expressionEnds(closing int) bool {
	next := closing + 1
	if next >= len(s.tokens) || s.punct(next, ";") {
		return true
	}
	return s.tokens[next].Line > s.tokens[closing].Line && s.tokens[next].Kind == appmap.JSIdent
}
