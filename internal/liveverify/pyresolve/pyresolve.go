// Package pyresolve answers, from Python source bytes alone, whether an
// import statement that begins on a given line imports a given dotted
// module. It backs the `reference-resolves` falsifier's Python arm, mirroring
// what goImportsAtLine (cmd/corvint/prove.go) does for Go with go/parser.
//
// Tokenization reuses the repository's closed-subset Python lexer
// (internal/pythongrammar.LexPython312), which already handles comments,
// prefixed and triple-quoted strings, and backslash line continuation. The
// statement- and import-grammar walk above the token stream is this
// package's own: pythongrammar.Token's line/column fields are unexported (so
// line numbers here are derived independently from byte offsets), and the
// grammar's own from-import fact emits only the imported module, not each
// member's fully-qualified name, which this package needs.
package pyresolve

import (
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/pythongrammar"
)

// Import is one import statement: the 1-based line it begins on, and every
// dotted name it binds. `from a.b import c` yields "a.b.c" and "a.b"; `import
// a.b` yields "a.b" only (no prefix packages); `from a import *` yields "a"; a
// relative import yields names starting with one or more dots, e.g. ".a.b" and
// ".a" for `from .a import b`, and ".c" and "." for `from . import c`.
type Import struct {
	Line  int
	Names []string
}

// Imports lists every import binding in the file in source order. It returns
// an error if the source does not tokenize (including an unterminated
// string); a statement that tokenizes but is not a well-formed import is
// silently skipped rather than treated as a file-wide failure, since this
// package does not attempt full Python parsing.
func Imports(source []byte) ([]Import, error) {
	tokens, reason := pythongrammar.LexPython312(source)
	if reason != "" {
		return nil, fmt.Errorf("pyresolve: %s", reason)
	}

	var imports []Import
	lines := lineIndex{source: source, line: 1}
	index := 0
	for index < len(tokens) {
		switch tokens[index].Kind {
		case pythongrammar.KindNewline, pythongrammar.KindIndent, pythongrammar.KindDedent:
			index++
			continue
		}
		end := statementEnd(tokens, source, index)
		statement := trimNewline(tokens[index:end])
		if names := importNames(statement, source); len(names) > 0 {
			imports = append(imports, Import{Line: lines.lineOf(statement[0].Start), Names: names})
		}
		index = end
		if index < len(tokens) && isSymbol(tokens[index], source, ";") {
			index++
		}
	}
	return imports, nil
}

// ImportsAtLine reports whether a Python import statement that begins on
// `line` (1-based) imports the dotted module `imported`. For a relative
// import it compares literally against the dotted-with-leading-dots spelling
// Imports produces (e.g. ".a.b") — it has no package context to resolve a
// relative import to an absolute name, so it returns false unless `imported`
// already matches that literal relative form or the resolved form a caller
// computed with ResolveRelative. It returns false if the source does not
// tokenize.
func ImportsAtLine(source []byte, imported string, line int) bool {
	imports, err := Imports(source)
	if err != nil {
		return false
	}
	for _, statementImport := range imports {
		if statementImport.Line != line {
			continue
		}
		for _, name := range statementImport.Names {
			if name == imported {
				return true
			}
		}
	}
	return false
}

// ResolveRelative resolves a relative import's dots and optional dotted
// suffix into an absolute dotted module name, given the dotted package the
// importing file resolves relative imports from (its own dotted name for an
// __init__.py, otherwise its enclosing package). dots is the number of
// leading dots from the relative import (>=1) and module is the optional
// dotted suffix after them (empty for `from . import x`). It returns "" if
// there are not enough package components to climb past.
func ResolveRelative(importingPackage string, dots int, module string) string {
	if dots <= 0 {
		return ""
	}
	var parts []string
	if importingPackage != "" {
		parts = strings.Split(importingPackage, ".")
	}
	climb := dots - 1
	if climb > len(parts) {
		return ""
	}
	base := strings.Join(parts[:len(parts)-climb], ".")
	switch {
	case module == "":
		return base
	case base == "":
		return module
	default:
		return base + "." + module
	}
}

// lineIndex maps an ascending sequence of byte offsets to 1-based line
// numbers in one forward sweep over source, rather than rescanning from the
// start for every query.
type lineIndex struct {
	source []byte
	pos    int
	line   int
}

func (l *lineIndex) lineOf(offset int) int {
	for l.pos < offset && l.pos < len(l.source) {
		if l.source[l.pos] == '\n' {
			l.line++
		}
		l.pos++
	}
	return l.line
}

func text(source []byte, t pythongrammar.Token) string {
	return string(source[t.Start:t.End])
}

func isName(t pythongrammar.Token, source []byte, want string) bool {
	return t.Kind == pythongrammar.KindName && text(source, t) == want
}

func isSymbol(t pythongrammar.Token, source []byte, want string) bool {
	return t.Kind == pythongrammar.KindSymbol && text(source, t) == want
}

func trimNewline(statement []pythongrammar.Token) []pythongrammar.Token {
	if len(statement) > 0 && statement[len(statement)-1].Kind == pythongrammar.KindNewline {
		return statement[:len(statement)-1]
	}
	return statement
}

// statementEnd finds the end of the logical statement starting at start:
// the token past a top-level NEWLINE, or the top-level ';' itself. It mirrors
// pythongrammar.Parser.StatementEnd, reimplemented here because that method
// hangs off a Parser this package has no clean way to construct (several of
// its fields, needed by other methods, are unexported).
func statementEnd(tokens []pythongrammar.Token, source []byte, start int) int {
	depth := 0
	for index := start; index < len(tokens); index++ {
		token := tokens[index]
		if token.Kind == pythongrammar.KindNewline && depth == 0 {
			return index + 1
		}
		if token.Kind != pythongrammar.KindSymbol {
			continue
		}
		switch text(source, token) {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case ";":
			if depth == 0 {
				return index
			}
		}
	}
	return len(tokens)
}

// parseDottedName reads a NAME ("." NAME)* sequence starting at index.
func parseDottedName(tokens []pythongrammar.Token, source []byte, index int) (name string, next int, ok bool) {
	if index >= len(tokens) || tokens[index].Kind != pythongrammar.KindName {
		return "", index, false
	}
	name = text(source, tokens[index])
	next = index + 1
	for next+1 < len(tokens) && isSymbol(tokens[next], source, ".") && tokens[next+1].Kind == pythongrammar.KindName {
		name += "." + text(source, tokens[next+1])
		next += 2
	}
	return name, next, true
}

// skipOptionalAs consumes a trailing "as NAME", if present.
func skipOptionalAs(tokens []pythongrammar.Token, source []byte, index int) int {
	if index < len(tokens) && isName(tokens[index], source, "as") {
		index++
		if index < len(tokens) && tokens[index].Kind == pythongrammar.KindName {
			index++
		}
	}
	return index
}

// importNames recognizes a `import ...` or `from ... import ...` statement
// and returns the dotted names it binds. Any other statement, or one that
// does not fit the shapes this package supports, yields nil.
func importNames(statement []pythongrammar.Token, source []byte) []string {
	if len(statement) == 0 {
		return nil
	}
	if isName(statement[0], source, "import") {
		return plainImportNames(statement, source)
	}
	if isName(statement[0], source, "from") {
		return fromImportNames(statement, source)
	}
	return nil
}

// plainImportNames handles `import a.b.c [as x][, ...]`.
func plainImportNames(tokens []pythongrammar.Token, source []byte) []string {
	var names []string
	index := 1
	for {
		name, next, ok := parseDottedName(tokens, source, index)
		if !ok || name == "" {
			return names
		}
		index = skipOptionalAs(tokens, source, next)
		names = append(names, name)
		if index < len(tokens) && isSymbol(tokens[index], source, ",") {
			index++
			continue
		}
		return names
	}
}

// fromImportNames handles `from [.[.[...]]][pkg.mod] import name[, ...]`,
// including a parenthesized, possibly multi-line, name list and `*`. Each
// member yields both its full dotted spelling and, once, the base module
// alone (skipped for a bare-dot base with no module, e.g. `from . import x`).
func fromImportNames(tokens []pythongrammar.Token, source []byte) []string {
	index := 1
	dots := 0
	for index < len(tokens) && isSymbol(tokens[index], source, ".") {
		dots++
		index++
	}
	module := ""
	if index < len(tokens) && tokens[index].Kind == pythongrammar.KindName && !isName(tokens[index], source, "import") {
		name, next, ok := parseDottedName(tokens, source, index)
		if !ok {
			return nil
		}
		module, index = name, next
	}
	if dots == 0 && module == "" {
		return nil
	}
	if index >= len(tokens) || !isName(tokens[index], source, "import") {
		return nil
	}
	index++

	base := module
	if dots > 0 {
		base = strings.Repeat(".", dots) + module
	}

	paren := false
	if index < len(tokens) && isSymbol(tokens[index], source, "(") {
		paren, index = true, index+1
	}

	var names []string
	haveBinding := false
	for index < len(tokens) {
		if paren && isSymbol(tokens[index], source, ")") {
			index++
			break
		}
		switch {
		case isSymbol(tokens[index], source, "*"):
			haveBinding = true
			index++
		case tokens[index].Kind == pythongrammar.KindName:
			member := text(source, tokens[index])
			index = skipOptionalAs(tokens, source, index+1)
			haveBinding = true
			separator := "."
			if strings.HasSuffix(base, ".") {
				separator = ""
			}
			names = append(names, base+separator+member)
		default:
			index = len(tokens)
		}
		if index < len(tokens) && isSymbol(tokens[index], source, ",") {
			index++
			continue
		}
		break
	}
	if haveBinding {
		names = append(names, base)
	}
	return names
}
