package contextindex

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file gives JavaScript and TypeScript import extraction the lexical state
// the shared genericImport scanner has never had.
//
// genericImport (parse.go) matches `from "x"` and `import "x"` anywhere in the
// bytes. It is a faithful mirror of the frozen oracle's fallback
// (src/context_corvint_index.py:1213-1217) and it is wrong for these languages in
// one direction: it records import-shaped text found inside a `//` comment, a
// `/* */` comment, a quoted string, or a template literal as though the quoted
// run beside it were a module specifier. A template literal is the worst of
// them, because it spans lines: the "specifier" recorded is then a paragraph of
// program text.
//
// The fix is not a parser. It is the smallest lexer that can tell code from
// not-code -- the same shape langsymbols.go already uses for the C-family
// languages, and the same shape the oracle itself uses for web sources in
// _strip_web_noncode (src/context_corvint.py:106) but does not apply to import
// extraction. Specifier positions are unchanged from genericImport: a quoted
// string after `from`, after `import`, or inside `import(`. Nothing new is
// recognised, so every edge this produces is an edge genericImport also
// produced; the difference is only which ones it refuses to invent.
//
// require() is deliberately NOT recognised. genericImport never saw it, so
// adding it here would mix new edges into a change whose whole claim is that it
// only removes fabricated ones.
//
// A `/` opens either a regular-expression literal or a division, and telling
// them apart in general needs the preceding token's grammatical role. The lexer
// reads a regular expression only where no grammar state is needed: after
// nothing or one of webRegexOpeners, when the literal closes on its line (the
// rule jsresolve applies for FPK-V0-018). Every other `/` reads as division, so
// after a word, `)`, `]`, `}`, `+`, `-`, `!`, `<`, or `>` a quote inside a regex
// still opens a literal that was never there. That damage is bounded by the two
// rules below -- a `'` or `"` literal ends at the line break, and an
// unterminated template is reported rather than swallowed. A backtick inside
// such a regex, or inside JSX text, remains unbounded: it opens a template, and
// when a later backtick closes it the code between is dropped without a refusal.

// webSuffixes is the extension set the oracle names as web sources
// (src/context_corvint.py:100) and the TS/JS adapter spec claims exactly
// (docs/specs/typescript-javascript-affected-adapter-v0.md TJAA-V0-001). Any
// other extension keeps the unstructured scanner.
var webSuffixes = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
}

// webSymbolPattern is the oracle's own line-anchored heuristic for
// `_web_symbols`, not a parser: a leading (possibly exported) function, class,
// interface, type, const, let or var declaration. It used to run only on the
// query eval path (evalIndexWithSymbols, since removed); moving it here makes
// web declarations part of the index build itself, so every consumer of
// Index.Symbols -- not just EvalQuery -- sees them (bugs.md "contextindex: web
// symbols exist only on the query eval path").
var webSymbolPattern = regexp.MustCompile(`^\s*(?:export\s+(?:default\s+)?)?((?:async\s+)?function|class|interface|type|const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)

// webSymbols extracts one web source's top-level declarations with
// webSymbolPattern. It has the languageSymbols(Source, string) ([]Symbol,
// []ExtractionNote) shape so it can sit in symbolExtractors alongside the
// other lexical languages; it reports no ExtractionNote because, unlike the
// brace-tracking extractors, this heuristic never detects its own truncation.
func webSymbols(source Source, text string) ([]Symbol, []ExtractionNote) {
	var symbols []Symbol
	for offset, line := range strings.Split(text, "\n") {
		match := webSymbolPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		kind := match[1]
		if strings.Contains(kind, "function") || strings.Contains(line, "=>") {
			kind = "func"
		}
		symbols = append(symbols, Symbol{
			Kind: kind, Name: match[2], Path: source.Path,
			BlobHash: source.BlobHash, Line: offset + 1,
		})
	}
	return symbols, nil
}

// webTokenKind separates the three things the statement walk has to tell apart.
// Everything that is neither a name nor a completed string literal is
// punctuation, because nothing below needs to distinguish one operator from
// another.
type webTokenKind int

const (
	webPunct webTokenKind = iota
	webWord
	webString
)

// webToken is one lexical unit of code. Comments produce none at all, which is
// what makes a commented-out import unreachable rather than merely unlikely.
type webToken struct {
	kind webTokenKind
	text string
	// quoted marks a `'` or `"` literal. A template literal is a string too,
	// but never a specifier: genericImport's character class admits only the
	// two plain quotes, so accepting backticks would add edges.
	quoted bool
}

// webLexer walks the source once, carrying the state that spans lines. The
// state is exactly the two constructs that can: a block comment and a template
// literal. A file that ends inside either one ended mid-construct, and every
// import after the opener was swallowed -- so that ending is reported rather
// than silently returning the shorter edge set.
type webLexer struct {
	text     string
	position int
	// previous is the last token next yielded, the zero punctuation at the
	// start of the text; it decides whether a `/` may open a regex.
	previous webToken
	// unterminated names the construct the file ended inside, empty when the
	// scan completed. It is the refusal the caller files in Index.Unparsed.
	unterminated string
}

// next yields the next code token, reporting false at end of input.
func (lexer *webLexer) next() (webToken, bool) {
	token, ok := lexer.scan()
	lexer.previous = token
	return token, ok
}

func (lexer *webLexer) scan() (webToken, bool) {
	for lexer.position < len(lexer.text) {
		character, width := utf8.DecodeRuneInString(lexer.text[lexer.position:])
		switch {
		// Unicode space, not just ASCII: the scanner this replaces separated
		// `import` from its specifier with `\p{Z}`, so a no-break space there
		// must keep separating them rather than fusing into the keyword.
		case unicode.IsSpace(character):
			lexer.position += width
		case lexer.hasAt("//"):
			lexer.skipLineComment()
		case lexer.hasAt("/*"):
			lexer.skipBlockComment()
		case character == '/' && lexer.readRegex():
			return webToken{kind: webString}, true
		case character == '"' || character == '\'':
			return lexer.readQuoted(byte(character)), true
		case character == '`':
			return lexer.readTemplate(), true
		case isWebWordStart(character):
			return lexer.readWord(), true
		default:
			return lexer.readPunct(), true
		}
	}
	return webToken{}, false
}

func (lexer *webLexer) hasAt(prefix string) bool {
	return strings.HasPrefix(lexer.text[lexer.position:], prefix)
}

// skipLineComment consumes a `//` comment up to, not through, the line
// terminator that ends it. JavaScript ends one at `\n`, `\r`, U+2028, or
// U+2029, so code after a lone `\r` or a U+2028 on the same `\n`-line is
// code; next consumes the terminator itself as space.
func (lexer *webLexer) skipLineComment() {
	if end := strings.IndexAny(lexer.text[lexer.position:], lineCommentTerminators); end >= 0 {
		lexer.position += end
		return
	}
	lexer.position = len(lexer.text)
}

// lineCommentTerminators is the ECMAScript LineTerminator set, the characters
// that end a `//` comment.
const lineCommentTerminators = "\n\r\u2028\u2029"

// skipBlockComment consumes through the first `*/`. JavaScript block comments
// do not nest, so the first close ends it whatever appears inside.
func (lexer *webLexer) skipBlockComment() {
	if end := strings.Index(lexer.text[lexer.position+2:], "*/"); end >= 0 {
		lexer.position += 2 + end + 2
		return
	}
	lexer.position = len(lexer.text)
	lexer.note(noteUnterminatedComment)
}

// readQuoted consumes a `'` or `"` literal. The grammar forbids a raw `\n` or
// `\r` inside one, so an unclosed literal ends at either instead of swallowing
// the rest of the file: the damage of a mis-lex stays on its line. U+2028 and
// U+2029 are legal inside a literal and do not end it.
func (lexer *webLexer) readQuoted(quote byte) webToken {
	start := lexer.position + 1
	position := start
	for position < len(lexer.text) {
		switch lexer.text[position] {
		case '\\':
			position = webEscapeEnd(lexer.text, position)
			continue
		case '\n', '\r':
			lexer.position = position
			return webToken{kind: webString, text: lexer.text[start:position]}
		case quote:
			lexer.position = position + 1
			return webToken{kind: webString, text: lexer.text[start:position], quoted: true}
		}
		position++
	}
	lexer.position = len(lexer.text)
	return webToken{kind: webString, text: lexer.text[start:]}
}

// webEscapeEnd returns the offset just past the escape a `\` at `position`
// opens. A `\` before `\r\n` escapes both bytes as one line continuation, so
// a quoted literal does not end at the `\n`.
func webEscapeEnd(text string, position int) int {
	if strings.HasPrefix(text[position+1:], "\r\n") {
		return position + 3
	}
	return position + 2
}

// readTemplate consumes a template literal, including its `${...}`
// substitutions and any template nested inside one. Nothing inside a template
// can be a specifier under genericImport's quote set, so it yields one token.
func (lexer *webLexer) readTemplate() webToken {
	lexer.position++
	for lexer.position < len(lexer.text) {
		switch {
		case lexer.text[lexer.position] == '\\':
			lexer.position += 2
		case lexer.hasAt("${"):
			lexer.position += 2
			lexer.previous = webToken{kind: webPunct, text: "{"}
			lexer.skipSubstitution()
		case lexer.text[lexer.position] == '`':
			lexer.position++
			return webToken{kind: webString}
		default:
			lexer.position++
		}
	}
	lexer.note(noteUnterminatedString)
	return webToken{kind: webString}
}

// webRegexOpeners are the punctuators after which an expression must begin, so
// a `/` there cannot be division (FPK-V0-018's set).
const webRegexOpeners = "(,=:[&|?{;~^%*"

// readRegex consumes a regular-expression literal and its flags when the `/`
// follows nothing or a webRegexOpeners punctuator and the literal closes before
// any line terminator. Otherwise it consumes nothing and the `/` stays division.
func (lexer *webLexer) readRegex() bool {
	if lexer.previous.kind != webPunct {
		return false
	}
	if !strings.Contains(webRegexOpeners, lexer.previous.text) {
		return false
	}
	end, ok := webRegexBodyEnd(lexer.text, lexer.position+1)
	if !ok {
		return false
	}
	lexer.position = end
	lexer.readWord()
	return true
}

// webRegexBodyEnd returns the offset just past the `/` that closes a regular
// expression body starting at `position`: an escaped character and a `/`
// inside a `[...]` class do not close it, and a line terminator refuses it.
func webRegexBodyEnd(text string, position int) (int, bool) {
	escaped, inClass := false, false
	for position < len(text) {
		character, width := utf8.DecodeRuneInString(text[position:])
		position += width
		switch {
		case strings.ContainsRune(lineCommentTerminators, character):
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

// skipSubstitution consumes a `${...}` body through its closing `}`. The body
// is code, so it is tokenised rather than scanned byte by byte: a `}` or
// backtick inside one of its string literals, or a comment, is not a boundary,
// and only unbalanced `}` punctuation closes it. Ending the input inside the
// body leaves the enclosing template to report itself unterminated.
func (lexer *webLexer) skipSubstitution() {
	depth := 1
	for {
		token, ok := lexer.next()
		if !ok {
			return
		}
		if token.kind != webPunct {
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

func (lexer *webLexer) readWord() webToken {
	start := lexer.position
	for lexer.position < len(lexer.text) {
		value, width := utf8.DecodeRuneInString(lexer.text[lexer.position:])
		if !isWebWordPart(value) {
			break
		}
		lexer.position += width
	}
	return webToken{kind: webWord, text: lexer.text[start:lexer.position]}
}

func (lexer *webLexer) readPunct() webToken {
	start := lexer.position
	_, width := utf8.DecodeRuneInString(lexer.text[lexer.position:])
	lexer.position += width
	return webToken{kind: webPunct, text: lexer.text[start:lexer.position]}
}

// note records the first unterminated construct only. A file ends inside at
// most one, and the first one reported is the one that swallowed the tail.
func (lexer *webLexer) note(reason string) {
	if lexer.unterminated == "" {
		lexer.unterminated = reason
	}
}

// isWebWordStart and isWebWordPart follow the identifier shape the extraction
// needs: enough to tell `import` from `imports`, from `éimport`, and from
// `.import`. The full ECMAScript identifier tables are not needed, because no
// keyword matched here is anything but ASCII -- a non-ASCII letter only has to
// keep a run going so that it cannot split one.
func isWebWordStart(character rune) bool {
	return character == '_' || character == '$' || unicode.IsLetter(character)
}

func isWebWordPart(character rune) bool {
	return isWebWordStart(character) || unicode.IsDigit(character)
}

// webClauseToken reports whether a token may appear between `import`/`export`
// and the `from` that carries the specifier. An import clause is only names,
// braces, commas and `*`; anything else means this `import` word was not
// opening a module statement, and the walk abandons it rather than searching
// on for a `from` that belongs to something else.
func webClauseToken(token webToken) bool {
	if token.kind == webWord {
		return true
	}
	return token.kind == webPunct && (token.text == "{" || token.text == "}" || token.text == "," || token.text == "*")
}

// webImports extracts the module specifiers of a JavaScript or TypeScript
// source, together with the reason the scan could not complete, empty when it
// did. The specifier positions are exactly genericImport's three: a quoted
// string after `from`, directly after `import`, and inside `import(`.
func webImports(text string) (map[string]struct{}, string) {
	lexer := &webLexer{text: text}
	result := make(map[string]struct{})
	previous := webToken{}
	for {
		token, ok := lexer.next()
		if !ok {
			break
		}
		if token.kind != webWord || (previous.kind == webPunct && previous.text == ".") || (token.text != "import" && token.text != "export") {
			previous = token
			continue
		}
		specifier, last := webSpecifier(lexer, token.text)
		if specifier != "" {
			result[specifier] = struct{}{}
		}
		previous = last
	}
	return result, lexer.unterminated
}

// webSpecifier consumes one `import` or `export` statement's head and reports
// the specifier it carries, empty when it carries none. It also reports the
// last token it consumed, so the caller's property-access guard stays aligned.
func webSpecifier(lexer *webLexer, keyword string) (string, webToken) {
	token, ok := lexer.next()
	if !ok {
		return "", webToken{}
	}
	if keyword == "import" {
		// `import "x"` is the side-effect form; `import("x")` the dynamic one.
		// A dynamic import whose argument is not a literal names no module the
		// index can record, and contributes nothing rather than guessing.
		if token.kind == webString {
			return quotedSpecifier(token), token
		}
		if token.kind == webPunct && token.text == "(" {
			argument, present := lexer.next()
			if !present {
				return "", webToken{}
			}
			return quotedSpecifier(argument), argument
		}
	}
	for {
		if token.kind == webWord && token.text == "from" {
			specifier, present := lexer.next()
			if !present {
				return "", webToken{}
			}
			if specifier.kind == webString {
				return quotedSpecifier(specifier), specifier
			}
			// `import { from }`: a binding spelled from is a clause word.
			token = specifier
			continue
		}
		if !webClauseToken(token) {
			return "", token
		}
		next, present := lexer.next()
		if !present {
			return "", token
		}
		token = next
	}
}

// quotedSpecifier accepts a token as a module specifier only when it is a
// closed `'` or `"` literal with a body, which is exactly what genericImport's
// `['\"]([^'\"]+)['\"]` admits.
func quotedSpecifier(token webToken) string {
	if token.kind != webString || !token.quoted || token.text == "" {
		return ""
	}
	return token.text
}
