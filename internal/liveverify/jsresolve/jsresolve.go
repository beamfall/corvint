// Package jsresolve answers, from JavaScript or TypeScript source bytes
// alone, the three questions the `reference-resolves` falsifier asks of a
// web citation (FPK-V0-018): does an import on the cited line carry a given
// specifier, does an identifier sit on the cited line, and does a blob
// declare a name at top level. It mirrors what goImportsAtLine,
// goIdentifierAtLine, and goDeclaresAtTopLevel (cmd/corvint/prove.go) do
// for Go with go/parser, and what internal/liveverify/pyresolve does for
// Python.
//
// The tokenizer is a deliberate, line-tracking copy of the lexer in
// internal/contextindex/webimports.go, which the index uses to make the
// claim: comments produce no tokens, a `'`/`"` literal ends at its line, and a
// template literal is one token with its substitutions consumed. One lexing
// rule goes past the index: a `/` after a regexOpeners punctuator that closes
// on its line reads as a regular-expression literal, so a backtick or quote
// inside it opens nothing; every other `/` reads as division. That lexer is unexported and carries no line
// numbers, and the falsifier must never call the index that made the row, so
// the copy is the smaller dependency. The one addition over the index's
// specifier walk is `require("x")`, which the index never records; accepting
// it here only widens what a row can PASS on, never what the index claims.
//
// No parser is involved. The accepted false positives are the lexer's own for
// a regular-expression literal read as division (after a word, `)`, `]`, `}`,
// `+`, `-`, `!`, `<`, or `>`): a quote inside it opens a string that was never
// there (bounded to its line), a backtick opens a template, and an identifier
// inside it counts as an occurrence.
package jsresolve

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Import is one module-naming statement: the 1-based line its keyword sits
// on, the line its specifier literal sits on (the same line unless the clause
// wraps), and the specifier as written.
type Import struct {
	Line          int
	SpecifierLine int
	Specifier     string
}

// Imports lists every specifier-carrying statement in source order: ESM
// `import ... from "x"`, `import "x"`, `export ... from "x"`, dynamic
// `import("x")`, and CommonJS `require("x")`. A dynamic import or require
// whose argument is not a closed plain-quoted literal names nothing.
func Imports(source []byte) []Import {
	tokens := tokenize(source)
	imports := []Import{}
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if token.kind != kindWord || precededByDot(tokens, index) {
			continue
		}
		if found, ok := importAt(tokens, index); ok {
			imports = append(imports, found)
		}
	}
	return imports
}

// ImportsAtLine reports whether a statement whose specifier is exactly
// `specifier` spans `line`: the keyword sits on or before it and the
// specifier literal on or after it.
func ImportsAtLine(source []byte, specifier string, line int) bool {
	for _, found := range Imports(source) {
		if found.Specifier == specifier && found.Line <= line && line <= found.SpecifierLine {
			return true
		}
	}
	return false
}

// IdentifierAtLine reports whether a word token spelled `name` sits on
// `line`. It is occurrence, not binding: a property access or a same-named
// local counts, a comment or string does not.
func IdentifierAtLine(source []byte, name string, line int) bool {
	for _, token := range tokenize(source) {
		if token.kind == kindWord && token.text == name && token.line == line {
			return true
		}
	}
	return false
}

// DeclaresAtTopLevel reports whether the source declares `name` outside every
// brace, bracket, and parenthesis as `function name`, `class name`,
// `const|let|var name`, any of those after `export` or `export default`, or
// a name listed in a top-level `export { ... }` clause. A declaration keyword
// counts only at a statement start: after nothing, `;`, `}`, a line terminator, or
// `export`/`default`/`async`, so `const x = function name() {}` declares `x`
// and not `name`.
func DeclaresAtTopLevel(source []byte, name string) bool {
	tokens := tokenize(source)
	depth := 0
	for index, token := range tokens {
		if token.kind == kindPunct {
			depth = adjustDepth(depth, token.text)
		}
		if depth != 0 {
			continue
		}
		if declaredName(tokens, index) == name {
			return true
		}
		if exportListNames(tokens, index)[name] {
			return true
		}
	}
	return false
}

func adjustDepth(depth int, punct string) int {
	switch punct {
	case "{", "(", "[":
		return depth + 1
	case "}", ")", "]":
		if depth > 0 {
			return depth - 1
		}
	}
	return depth
}

// declarationKeywords opens a named declaration; the name is the next word,
// skipping a generator's `*`.
var declarationKeywords = map[string]bool{"function": true, "class": true, "const": true, "let": true, "var": true}

// statementOpeners are the words after which a declaration keyword still
// starts a statement.
var statementOpeners = map[string]bool{"export": true, "default": true, "async": true}

func declaredName(tokens []token, index int) string {
	keyword := tokens[index]
	if keyword.kind != kindWord || !declarationKeywords[keyword.text] || !startsStatement(tokens, index) {
		return ""
	}
	next := index + 1
	if next < len(tokens) && tokens[next].kind == kindPunct && tokens[next].text == "*" {
		next++
	}
	if next < len(tokens) && tokens[next].kind == kindWord {
		return tokens[next].text
	}
	return ""
}

func startsStatement(tokens []token, index int) bool {
	if index == 0 || tokens[index].newlineBefore {
		return true
	}
	previous := tokens[index-1]
	if previous.kind == kindPunct {
		return previous.text == ";" || previous.text == "}"
	}
	return previous.kind == kindWord && statementOpeners[previous.text]
}

// exportListNames collects every word inside `export { ... }` at `index`,
// both the local name and an `as` alias, and nothing when the token is not
// that clause.
func exportListNames(tokens []token, index int) map[string]bool {
	if tokens[index].kind != kindWord || tokens[index].text != "export" {
		return nil
	}
	if index+1 >= len(tokens) || tokens[index+1].kind != kindPunct || tokens[index+1].text != "{" {
		return nil
	}
	names := map[string]bool{}
	for _, token := range tokens[index+2:] {
		if token.kind == kindPunct && token.text == "}" {
			return names
		}
		if token.kind == kindWord && token.text != "as" {
			names[token.text] = true
		}
	}
	return names
}

func precededByDot(tokens []token, index int) bool {
	return index > 0 && tokens[index-1].kind == kindPunct && tokens[index-1].text == "."
}

// importAt reads one statement head at `index` and reports the specifier it
// carries. The positions are the index's three, plus require: a literal after
// `import`, inside `import(` or `require(`, or after the `from` that ends an
// import or export clause.
func importAt(tokens []token, index int) (Import, bool) {
	keyword := tokens[index]
	if index+1 >= len(tokens) {
		return Import{}, false
	}
	next := tokens[index+1]
	switch keyword.text {
	case "require":
		return callSpecifier(tokens, index)
	case "import":
		if next.kind == kindString {
			return specifierImport(keyword, next)
		}
		if next.kind == kindPunct && next.text == "(" {
			return callSpecifier(tokens, index)
		}
		return clauseSpecifier(tokens, index)
	case "export":
		return clauseSpecifier(tokens, index)
	}
	return Import{}, false
}

func callSpecifier(tokens []token, index int) (Import, bool) {
	if index+2 >= len(tokens) || tokens[index+1].kind != kindPunct || tokens[index+1].text != "(" {
		return Import{}, false
	}
	return specifierImport(tokens[index], tokens[index+2])
}

// clauseSpecifier walks an import or export clause to its `from`. A clause is
// only words, braces, commas, and `*`; any other token means the keyword was
// not opening a module statement.
func clauseSpecifier(tokens []token, index int) (Import, bool) {
	for cursor := index + 1; cursor < len(tokens); cursor++ {
		token := tokens[cursor]
		if token.kind == kindWord && token.text == "from" {
			if cursor+1 >= len(tokens) {
				return Import{}, false
			}
			return specifierImport(tokens[index], tokens[cursor+1])
		}
		if !clauseToken(token) {
			return Import{}, false
		}
	}
	return Import{}, false
}

func clauseToken(token token) bool {
	if token.kind == kindWord {
		return true
	}
	return token.kind == kindPunct && (token.text == "{" || token.text == "}" || token.text == "," || token.text == "*")
}

// specifierImport accepts a closed `'`/`"` literal with a body as the
// specifier, exactly what the index's quotedSpecifier admits.
func specifierImport(keyword, literal token) (Import, bool) {
	if literal.kind != kindString || !literal.quoted || literal.text == "" {
		return Import{}, false
	}
	return Import{Line: keyword.line, SpecifierLine: literal.line, Specifier: literal.text}, true
}

type tokenKind int

const (
	kindPunct tokenKind = iota
	kindWord
	kindString
)

// token is one lexical unit of code with the line it starts on and whether
// a line break separated it from the previous token.
type token struct {
	kind          tokenKind
	text          string
	quoted        bool
	line          int
	newlineBefore bool
}

type lexer struct {
	text          string
	position      int
	line          int
	newlineBefore bool
	// previous is the last emitted token, the zero punctuation before the
	// first one. It decides whether a `/` may open a regular expression.
	previous token
}

func tokenize(source []byte) []token {
	lexer := &lexer{text: string(source), line: 1}
	tokens := []token{}
	for {
		token, ok := lexer.next()
		if !ok {
			return tokens
		}
		tokens = append(tokens, token)
	}
}

func (lexer *lexer) next() (token, bool) {
	for lexer.position < len(lexer.text) {
		character, width := utf8.DecodeRuneInString(lexer.text[lexer.position:])
		switch {
		case character == '\n':
			lexer.position += width
			lexer.line++
			lexer.newlineBefore = true
		case strings.ContainsRune(lineTerminators, character):
			lexer.position += width
			lexer.newlineBefore = true
		case unicode.IsSpace(character):
			lexer.position += width
		case lexer.hasAt("//"):
			lexer.skipLineComment()
		case lexer.hasAt("/*"):
			lexer.skipBlockComment()
		case character == '"' || character == '\'':
			return lexer.emit(lexer.readQuoted(byte(character))), true
		case character == '`':
			return lexer.emit(lexer.readTemplate()), true
		case isWordStart(character):
			return lexer.emit(lexer.readWord()), true
		default:
			if regex, ok := lexer.readRegex(); ok {
				return lexer.emit(regex), true
			}
			return lexer.emit(lexer.readPunct()), true
		}
	}
	return token{}, false
}

// emit stamps the token with the line it began on and the line-break flag,
// then clears the flag for the next token. Readers that cross lines advance
// lexer.line themselves after the start line is captured, so the stamp is the
// start line.
func (lexer *lexer) emit(value token) token {
	value.newlineBefore = lexer.newlineBefore
	lexer.newlineBefore = false
	lexer.previous = value
	return value
}

func (lexer *lexer) hasAt(prefix string) bool {
	return strings.HasPrefix(lexer.text[lexer.position:], prefix)
}

// lineTerminators is the ECMAScript LineTerminator set. Each one ends a `//`
// comment and separates statements for DeclaresAtTopLevel, but only `\n`
// counts a line, so citation line numbers stay `\n`-counted.
const lineTerminators = "\n\r\u2028\u2029"

// skipLineComment consumes a `//` comment up to the line terminator that ends
// it, as the index lexer does. next meets the terminator and sets the
// line-break flag.
func (lexer *lexer) skipLineComment() {
	if end := strings.IndexAny(lexer.text[lexer.position:], lineTerminators); end >= 0 {
		lexer.position += end
		return
	}
	lexer.position = len(lexer.text)
}

// skipBlockComment consumes a `/* ... */` comment, or the rest of the text
// when it is unterminated. A comment whose body holds any line terminator is a
// line terminator in JavaScript, so it sets the line-break flag as `\n` does.
func (lexer *lexer) skipBlockComment() {
	body := lexer.text[lexer.position+2:]
	end := strings.Index(body, "*/")
	if end < 0 {
		lexer.countCommentLines(body)
		lexer.position = len(lexer.text)
		return
	}
	lexer.countCommentLines(body[:end])
	lexer.position += 2 + end + 2
}

func (lexer *lexer) countCommentLines(body string) {
	lexer.line += strings.Count(body, "\n")
	if strings.ContainsAny(body, lineTerminators) {
		lexer.newlineBefore = true
	}
}

// readQuoted consumes a `'`/`"` literal, which ends at its closing quote or
// at a `\n` or `\r`, the terminators a literal may not hold raw, so a
// mis-lexed quote stays on its line. U+2028 and U+2029 are legal inside a
// literal and do not end it. An escaped line break continues the literal and
// is counted as a line; `\` before `\r\n` is one escaped line break.
func (lexer *lexer) readQuoted(quote byte) token {
	line := lexer.line
	start := lexer.position + 1
	position := start
	for position < len(lexer.text) {
		switch lexer.text[position] {
		case '\\':
			position = lexer.escapeEnd(position)
			continue
		case '\n', '\r':
			lexer.position = position
			return token{kind: kindString, text: lexer.text[start:position], line: line}
		case quote:
			lexer.position = position + 1
			return token{kind: kindString, text: lexer.text[start:position], quoted: true, line: line}
		}
		position++
	}
	lexer.position = len(lexer.text)
	return token{kind: kindString, text: lexer.text[start:], line: line}
}

// escapeEnd returns the offset just past the escape a `\` at `position`
// opens, counting the line it continues. A `\` before `\r\n` escapes both
// bytes as one line break, so a quoted literal does not end at the `\n`.
func (lexer *lexer) escapeEnd(position int) int {
	rest := lexer.text[position+1:]
	if strings.HasPrefix(rest, "\r\n") {
		lexer.line++
		return position + 3
	}
	if strings.HasPrefix(rest, "\n") {
		lexer.line++
	}
	return position + 2
}

// readTemplate consumes a template literal with its `${...}` substitutions
// and any template nested inside one, counting the lines it spans. The
// substitution bodies are lexed through next, whose emits clear the
// line-break flag, so the flag this token carries is restored first.
func (lexer *lexer) readTemplate() token {
	line := lexer.line
	newlineBefore := lexer.newlineBefore
	lexer.position++
	lexer.consumeTemplateBody()
	lexer.newlineBefore = newlineBefore
	return token{kind: kindString, line: line}
}

func (lexer *lexer) consumeTemplateBody() {
	for lexer.position < len(lexer.text) {
		switch {
		case lexer.text[lexer.position] == '\\':
			lexer.position = lexer.escapeEnd(lexer.position)
		case lexer.text[lexer.position] == '\n':
			lexer.line++
			lexer.position++
		case lexer.hasAt("${"):
			lexer.position += 2
			lexer.skipSubstitution()
		case lexer.text[lexer.position] == '`':
			lexer.position++
			return
		default:
			lexer.position++
		}
	}
}

// skipSubstitution consumes a `${...}` body through its closing `}`, as the
// index lexer's skipSubstitution does (decision 0208). The body is code, so
// it is tokenised rather than scanned byte by byte: a `}` or backtick inside
// one of its string literals, a comment, or a nested template is not a
// boundary, and only unbalanced `}` punctuation closes it. next counts the
// lines the body spans, and the tokens it yields are discarded.
func (lexer *lexer) skipSubstitution() {
	depth := 1
	lexer.previous = token{kind: kindPunct, text: "{"}
	for {
		token, ok := lexer.next()
		if !ok {
			return
		}
		if token.kind != kindPunct {
			continue
		}
		switch token.text {
		case "{":
			depth++
		case "}":
			depth--
		}
		if depth == 0 {
			return
		}
	}
}

// regexOpeners are the punctuators after which an expression must begin, so a
// `/` there opens a regular expression and cannot be division. The zero
// previous token (start of text) counts as one. Every other predecessor keeps
// `/` as division: a word, a literal, `)`, `]`, and `}` need grammar state
// (`if (a) /x/` against `(a) / b`), `+` and `-` may end `++` or `--`, `!` may
// be a TypeScript non-null assertion, and `<` and `>` may be JSX or type
// arguments.
const regexOpeners = "(,=:[&|?{;~^%*"

// readRegex consumes a regular-expression literal and its flags as one token
// when a `/` follows a regexOpeners punctuator and the literal closes before
// any line terminator, which none may hold. Otherwise it consumes nothing and
// the `/` stays division.
func (lexer *lexer) readRegex() (token, bool) {
	if lexer.text[lexer.position] != '/' {
		return token{}, false
	}
	if lexer.previous.kind != kindPunct {
		return token{}, false
	}
	if !strings.Contains(regexOpeners, lexer.previous.text) {
		return token{}, false
	}
	start := lexer.position
	end, ok := regexBodyEnd(lexer.text, start+1)
	if !ok {
		return token{}, false
	}
	lexer.position = end
	lexer.readWord()
	return token{kind: kindString, text: lexer.text[start:lexer.position], line: lexer.line}, true
}

// regexBodyEnd returns the offset just past the `/` that closes a regular
// expression body starting at `position`: an escaped character and a `/`
// inside a `[...]` class do not close it, and a line terminator refuses it.
func regexBodyEnd(text string, position int) (int, bool) {
	escaped, inClass := false, false
	for position < len(text) {
		character, width := utf8.DecodeRuneInString(text[position:])
		position += width
		switch {
		case strings.ContainsRune(lineTerminators, character):
			return 0, false
		case escaped:
			escaped = false
		case character == '\\':
			escaped = true
		case inClass:
			inClass = character != ']'
		case character == '[':
			inClass = true
		case character == '/':
			return position, true
		}
	}
	return 0, false
}

func (lexer *lexer) readWord() token {
	start := lexer.position
	for lexer.position < len(lexer.text) {
		value, width := utf8.DecodeRuneInString(lexer.text[lexer.position:])
		if !isWordPart(value) {
			break
		}
		lexer.position += width
	}
	return token{kind: kindWord, text: lexer.text[start:lexer.position], line: lexer.line}
}

func (lexer *lexer) readPunct() token {
	start := lexer.position
	_, width := utf8.DecodeRuneInString(lexer.text[lexer.position:])
	lexer.position += width
	return token{kind: kindPunct, text: lexer.text[start:lexer.position], line: lexer.line}
}

func isWordStart(character rune) bool {
	return character == '_' || character == '$' || unicode.IsLetter(character)
}

func isWordPart(character rune) bool {
	return isWordStart(character) || unicode.IsDigit(character)
}
