package contextindex

import (
	"regexp"
	"slices"
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

// webMemberModifiers are the words that may precede a class member's name. A
// modifier directly followed by `(`, `<`, `?`, `=`, `:`, `;`, `!` or `}` is
// the member's own name instead, as in `get() {}`.
var webMemberModifiers = map[string]bool{
	"public": true, "private": true, "protected": true, "static": true, "readonly": true,
	"abstract": true, "async": true, "override": true, "declare": true, "accessor": true,
	"get": true, "set": true,
}

// webContinuingWords are the words after which a line break cannot end a class
// member, because the expression they begin continues on the next line.
var webContinuingWords = map[string]bool{
	"new": true, "typeof": true, "instanceof": true, "in": true, "of": true, "keyof": true,
	"extends": true, "return": true, "await": true, "yield": true, "as": true,
	"satisfies": true, "delete": true,
}

// WebSource reports whether path has one of the suffixes whose declarations
// webSymbols extracts, the ones WebClassMembers walks.
func WebSource(path string) bool {
	return slices.Contains(webSuffixOrder, pathExt(path))
}

type webLineToken struct {
	webToken
	line int
	// start is the token's byte offset; webStatementEnds reads it to tell
	// `++` from `+ +`.
	start int
}

// WebClassMembers names the methods declared directly in each class body of a
// TypeScript or JavaScript source (DCS-V1-571), which webSymbolPattern's
// line-anchored match never sees. Each is a Symbol of kind "method" at the
// line of its name, with Path set and BlobHash empty; a `#private` method keeps
// its `#`. Constructors, properties (an arrow-function property included),
// computed and string keys, interface members and object-literal methods are
// not named. A path outside the web suffixes yields none. ok is false, with no
// members, when the source ends inside a comment, string, template or bracket,
// or closes a bracket it never opened, because a member walk over it could
// place a body's tokens at class depth.
func WebClassMembers(path, text string) (members []Symbol, ok bool) {
	if !WebSource(path) {
		return nil, true
	}
	tokens, ok := webLineTokens(text)
	if !ok {
		return nil, false
	}
	// stack holds one entry per open bracket. A class's body is the first `{`
	// at the bracket depth of its `class` keyword outside its heritage
	// clause's type arguments, so `extends Base<{ a(): void }>` opens none.
	type frame struct {
		open byte
		body bool
	}
	var stack []frame
	closes := map[string]byte{"}": '{', ")": '(', "]": '['}
	pendingClass, classNext, atMember := false, false, false
	classDepth, angles := 0, 0
	for i := 0; i < len(tokens); i++ {
		inBody := len(stack) > 0 && stack[len(stack)-1].body
		if inBody && !atMember && i > 0 && tokens[i].line > tokens[i-1].line && webEndsMember(tokens, i-1) {
			atMember = true
		}
		if inBody && atMember {
			atMember = false
			next, name := webMemberHeader(tokens, i)
			if name >= 0 {
				members = append(members, Symbol{Kind: "method", Name: webMemberName(tokens, name), Path: path, Line: tokens[name].line})
			}
			if i = next; i >= len(tokens) {
				break
			}
		}
		token := tokens[i]
		if token.kind == webWord && token.text == "class" && !(i > 0 && webIsPunct(tokens[i-1].webToken, ".")) {
			pendingClass, classNext, classDepth, angles = true, true, len(stack), 0
			continue
		}
		if classNext {
			classNext = false
			if token.kind != webWord && !webIsPunct(token.webToken, "{") {
				pendingClass = false
			}
		}
		if token.kind != webPunct {
			continue
		}
		heritage := pendingClass && len(stack) == classDepth
		switch token.text {
		case "<":
			if heritage {
				angles++
			}
		case ">":
			if heritage && angles > 0 {
				angles--
			}
		case "{":
			body := heritage && angles == 0
			stack = append(stack, frame{'{', body})
			atMember = body
			if body {
				pendingClass = false
			}
		case "(", "[":
			stack = append(stack, frame{token.text[0], false})
		case "}", ")", "]":
			if len(stack) == 0 || stack[len(stack)-1].open != closes[token.text] {
				return nil, false
			}
			stack = stack[:len(stack)-1]
			atMember = token.text == "}" && len(stack) > 0 && stack[len(stack)-1].body
		case ";":
			atMember = inBody
			if heritage {
				pendingClass = false
			}
		}
	}
	if len(stack) != 0 {
		return nil, false
	}
	return members, true
}

// webLineTokens lexes a web source into tokens with their lines. ok is false
// when the source ends inside a comment, string or template.
func webLineTokens(text string) ([]webLineToken, bool) {
	lexer := &webLexer{text: text}
	var tokens []webLineToken
	line, last := 1, 0
	for {
		token, more := lexer.next()
		if !more {
			break
		}
		if token.open {
			return nil, false
		}
		line += strings.Count(text[last:lexer.start], "\n")
		last = lexer.start
		tokens = append(tokens, webLineToken{token, line, lexer.start})
	}
	return tokens, lexer.unterminated == ""
}

// Bounds of WebExportedObjectKeys (DCS-V1-583): keys named per exported
// object, per file, and the bytes of one key.
const (
	webObjectKeysPerObject = 128
	webObjectKeysPerFile   = 256
	webObjectKeyBytes      = 128
)

// WebExportedObjectKeys names the top-level keys of each object literal a
// TypeScript or JavaScript source exports (DCS-V1-583): `export default {`,
// `export default defineConfig({`, `export const NAME = {` with an optional
// type annotation or wrapping call, `module.exports = {`, and `export default
// NAME` of a top-level `const`, `let` or `var` NAME initialized that way. Each
// key is a Symbol of kind "property" at its line, with Path set and BlobHash
// empty: a name or quoted string key followed by `:`, `,`, `}`, `(` or `?`,
// such as a property, a shorthand or a method. Keys of nested objects, spreads
// and computed keys are not named; a string key must be identifier characters,
// `-` and `.`, at most webObjectKeyBytes long. At most webObjectKeysPerObject
// keys of one object and webObjectKeysPerFile of a file are named. ok is false,
// with no keys, on the sources WebClassMembers refuses.
func WebExportedObjectKeys(path, text string) (keys []Symbol, ok bool) {
	if !WebSource(path) {
		return nil, true
	}
	tokens, ok := webLineTokens(text)
	if !ok {
		return nil, false
	}
	at := func(j int, text string) bool { return j < len(tokens) && webIsPunct(tokens[j].webToken, text) }
	word := func(j int, text string) bool {
		return j < len(tokens) && tokens[j].kind == webWord && tokens[j].text == text
	}
	closes := map[string]string{"}": "{", ")": "(", "]": "["}
	var stack []string
	var objects []int
	declared := map[string]int{}
	var defaults []string
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if len(stack) == 0 && token.kind == webWord && !(i > 0 && at(i-1, ".")) {
			switch {
			case token.text == "export" && word(i+1, "default"):
				if open := webObjectAfter(tokens, i+2); open >= 0 {
					objects = append(objects, open)
				} else if i+2 < len(tokens) && tokens[i+2].kind == webWord && !webDeclarationWords[tokens[i+2].text] && webStatementEnds(tokens, i+3) {
					defaults = append(defaults, tokens[i+2].text)
				}
			case token.text == "export" && i+2 < len(tokens) && webBindingWords[tokens[i+1].text] && tokens[i+2].kind == webWord:
				if open := webInitializer(tokens, i+3); open >= 0 {
					objects = append(objects, open)
				}
			case webBindingWords[token.text] && i+1 < len(tokens) && tokens[i+1].kind == webWord && !(i > 0 && word(i-1, "export")):
				if open := webInitializer(tokens, i+2); open >= 0 {
					declared[tokens[i+1].text] = open
				}
			case token.text == "module" && at(i+1, ".") && word(i+2, "exports") && at(i+3, "="):
				if open := webObjectAfter(tokens, i+4); open >= 0 {
					objects = append(objects, open)
				}
			}
		}
		if token.kind != webPunct {
			continue
		}
		switch token.text {
		case "{", "(", "[":
			stack = append(stack, token.text)
		case "}", ")", "]":
			if len(stack) == 0 || stack[len(stack)-1] != closes[token.text] {
				return nil, false
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return nil, false
	}
	for _, name := range defaults {
		if open, found := declared[name]; found {
			objects = append(objects, open)
		}
	}
	slices.Sort(objects)
	objects = slices.Compact(objects)
	for _, open := range objects {
		keys = append(keys, webObjectKeys(tokens, open, path, min(webObjectKeysPerObject, webObjectKeysPerFile-len(keys)))...)
		if len(keys) >= webObjectKeysPerFile {
			break
		}
	}
	return keys, true
}

// webBindingWords open a variable declaration; webDeclarationWords follow
// `export default` when it exports a declaration, not a name.
var (
	webBindingWords     = map[string]bool{"const": true, "let": true, "var": true}
	webDeclarationWords = map[string]bool{"function": true, "class": true, "async": true, "abstract": true, "interface": true, "enum": true, "new": true}
)

// webObjectAfter returns the index of the `{` that opens an object literal at
// tokens[j], directly or as the first argument of a call such as
// `defineConfig(` or `a.b(`, or -1.
func webObjectAfter(tokens []webLineToken, j int) int {
	at := func(k int, text string) bool { return k < len(tokens) && webIsPunct(tokens[k].webToken, text) }
	if at(j, "{") {
		return j
	}
	if j >= len(tokens) || tokens[j].kind != webWord {
		return -1
	}
	for j++; at(j, ".") && j+1 < len(tokens) && tokens[j+1].kind == webWord; j += 2 {
	}
	if at(j, "(") && at(j+1, "{") {
		return j + 1
	}
	return -1
}

// webStatementEnds reports whether the statement ends before tokens[j]: at end
// of file, at ';', or, by ASI, at a later line that starts with a word other
// than a continuation word, a `{`, a `!` that does not open `!=` or `!==`, or a
// `++`/`--` punctuation pair. A bare `export default NAME` exports NAME's
// object only then; `NAME ? a : b`, `NAME || b`, `NAME\n!== null`, a tagged
// template and similar export another value (DCS-V1-583, V1-3075, V1-3076,
// V1-3089).
func webStatementEnds(tokens []webLineToken, j int) bool {
	if j >= len(tokens) {
		return true
	}
	next := tokens[j]
	if next.text == ";" && next.kind == webPunct {
		return true
	}
	if next.line == tokens[j-1].line {
		return false
	}
	switch {
	// After a name a `/` divides, so an unquoted string here is a template
	// literal, with or without a substitution: a tagged call.
	case next.kind == webString:
		return next.quoted
	case next.kind == webWord:
		return !webContinuationWords[next.text]
	case next.kind != webPunct:
		return false
	case next.text == "{":
		return true
	case next.text == "!":
		// The lexer splits `!=` and `!==`: a comparison continues the value.
		return !webAdjacentPunct(tokens, j, "=")
	case next.text == "+" || next.text == "-":
		return webAdjacentPunct(tokens, j, next.text)
	}
	return false
}

// webAdjacentPunct reports whether tokens[j+1] is the punctuation text written
// directly after tokens[j], so the two spell one operator.
func webAdjacentPunct(tokens []webLineToken, j int, text string) bool {
	return j+1 < len(tokens) && webIsPunct(tokens[j+1].webToken, text) && tokens[j+1].line == tokens[j].line && tokens[j+1].start == tokens[j].start+1
}

// webContinuationWords continue an expression onto the next line.
var webContinuationWords = map[string]bool{"in": true, "instanceof": true, "as": true, "satisfies": true}

// webInitializer returns webObjectAfter of a declaration's initializer: the
// tokens after the first `=` that follows the declared name outside the
// brackets of its type annotation, or -1 when a `;` or a closing bracket
// comes first.
func webInitializer(tokens []webLineToken, j int) int {
	depth := 0
	for ; j < len(tokens); j++ {
		token := tokens[j]
		if token.kind != webPunct {
			continue
		}
		switch token.text {
		case "{", "(", "[", "<":
			depth++
		case "}", ")", "]", ">":
			if depth--; depth < 0 {
				return -1
			}
		case ";", ",":
			if depth == 0 {
				return -1
			}
		case "=":
			if depth == 0 {
				if j+1 < len(tokens) && webIsPunct(tokens[j+1].webToken, ">") {
					return -1
				}
				return webObjectAfter(tokens, j+1)
			}
		}
	}
	return -1
}

// webObjectKeys names at most limit keys of the object literal opened at
// tokens[open], at its own depth only.
func webObjectKeys(tokens []webLineToken, open int, path string, limit int) []Symbol {
	at := func(k int, text string) bool { return k < len(tokens) && webIsPunct(tokens[k].webToken, text) }
	var keys []Symbol
	depth := 0
	for i := open; i < len(tokens) && len(keys) < limit; i++ {
		token := tokens[i]
		if token.kind == webPunct {
			switch token.text {
			case "{", "(", "[":
				depth++
			case "}", ")", "]":
				depth--
			}
			if depth == 0 {
				break
			}
			if depth != 1 || !(token.text == "{" && i == open || token.text == ",") {
				continue
			}
			k := i + 1
			for k+1 < len(tokens) && (at(k, "*") || tokens[k].kind == webWord && webKeyModifiers[tokens[k].text] && (tokens[k+1].kind != webPunct || at(k+1, "*"))) {
				k++
			}
			if name, ok := webObjectKey(tokens, k); ok && (at(k+1, ":") || at(k+1, ",") || at(k+1, "}") || at(k+1, "(") || at(k+1, "?")) {
				keys = append(keys, Symbol{Kind: "property", Name: name, Path: path, Line: tokens[k].line})
			}
		}
	}
	return keys
}

// webKeyModifiers may precede an object literal method's key.
var webKeyModifiers = map[string]bool{"async": true, "get": true, "set": true}

// webObjectKey reads a name or quoted string key at tokens[k].
func webObjectKey(tokens []webLineToken, k int) (string, bool) {
	if k >= len(tokens) {
		return "", false
	}
	token := tokens[k]
	switch {
	case token.kind == webWord:
		return token.text, len(token.text) <= webObjectKeyBytes
	case token.kind == webString && token.quoted:
		if token.text == "" || len(token.text) > webObjectKeyBytes {
			return "", false
		}
		for _, r := range token.text {
			if !isWebWordPart(r) && r != '-' && r != '.' {
				return "", false
			}
		}
		return token.text, true
	}
	return "", false
}

// webMemberHeader reads the decorators, modifiers and name that open a class
// member at tokens[i]. It returns the index of the first token it did not
// consume and the index of the name token when the member is a method other
// than the constructor, or -1. A `#` name's index is the `#` itself.
func webMemberHeader(tokens []webLineToken, i int) (next, name int) {
	at := func(j int, text string) bool { return j < len(tokens) && webIsPunct(tokens[j].webToken, text) }
	for i < len(tokens) {
		token := tokens[i]
		switch {
		case webIsPunct(token.webToken, "@"):
			i++
			if i < len(tokens) && tokens[i].kind == webWord {
				i++
				for at(i, ".") && i+1 < len(tokens) && tokens[i+1].kind == webWord {
					i += 2
				}
			}
			if at(i, "(") {
				for depth := 0; i < len(tokens); i++ {
					if at(i, "(") {
						depth++
					} else if at(i, ")") {
						if depth--; depth == 0 {
							i++
							break
						}
					}
				}
			}
		case webIsPunct(token.webToken, "*"):
			i++
		case webIsPunct(token.webToken, "#") && i+1 < len(tokens) && tokens[i+1].kind == webWord:
			if webMethodFollows(tokens, i+2) {
				return i + 2, i
			}
			return i + 2, -1
		case token.kind == webWord:
			if webMemberModifiers[token.text] && i+1 < len(tokens) && !(tokens[i+1].kind == webPunct && strings.Contains("(<?=:;!}", tokens[i+1].text)) {
				i++
				continue
			}
			if token.text != "constructor" && webMethodFollows(tokens, i+1) {
				return i + 1, i
			}
			return i + 1, -1
		default:
			return i, -1
		}
	}
	return i, -1
}

// webMethodFollows reports whether the tokens from j on open a method's
// parameters or type parameters: `(`, `<`, or an optional `?(`.
func webMethodFollows(tokens []webLineToken, j int) bool {
	at := func(k int, text string) bool { return k < len(tokens) && webIsPunct(tokens[k].webToken, text) }
	return at(j, "(") || at(j, "<") || at(j, "?") && at(j+1, "(")
}

func webMemberName(tokens []webLineToken, i int) string {
	if webIsPunct(tokens[i].webToken, "#") {
		return "#" + tokens[i+1].text
	}
	return tokens[i].text
}

// webEndsMember reports whether a line break after tokens[j] may end a class
// member: after a name other than a continuing keyword, a literal, a closing
// bracket, or a `>` that does not finish an arrow.
func webEndsMember(tokens []webLineToken, j int) bool {
	token := tokens[j]
	switch token.kind {
	case webWord:
		return !webContinuingWords[token.text]
	case webString:
		return true
	}
	if token.text != "" && token.text[0] >= '0' && token.text[0] <= '9' {
		return true
	}
	switch token.text {
	case ")", "]", "}":
		return true
	case ">":
		return j == 0 || !webIsPunct(tokens[j-1].webToken, "=")
	}
	return false
}

func webIsPunct(token webToken, text string) bool {
	return token.kind == webPunct && token.text == text
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
	// template marks a closed template literal without a `${...}`
	// substitution; text is then its raw body. It is never a specifier.
	template bool
	// open marks a `'` or `"` literal the line or the text ended before its
	// closing quote; only the class member walk reads it.
	open bool
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
	// start is the offset of the last token scan yielded; only the class
	// member walk reads it, to place each token on its line.
	start int
	// unterminated names the construct the file ended inside, empty when the
	// scan completed. It is the refusal the caller files in Index.Unparsed.
	unterminated string
	// substitutions, when set, receives every token of each `${...}` body,
	// without the closing `}`, with the offset just past it, each template
	// ahead of its own substitutions (NGI-V0-065). Only the registry dialect's call-site reader sets it.
	substitutions *[]webPlacedToken
}

// webPlacedToken is a token and the offset just past it.
type webPlacedToken struct {
	token    webToken
	position int
}

// next yields the next code token, reporting false at end of input.
func (lexer *webLexer) next() (webToken, bool) {
	token, ok := lexer.scan()
	lexer.previous = token
	return token, ok
}

func (lexer *webLexer) scan() (webToken, bool) {
	for lexer.position < len(lexer.text) {
		lexer.start = lexer.position
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
			return webToken{kind: webString, text: lexer.text[start:position], open: true}
		case quote:
			lexer.position = position + 1
			return webToken{kind: webString, text: lexer.text[start:position], quoted: true}
		}
		position++
	}
	lexer.position = len(lexer.text)
	return webToken{kind: webString, text: lexer.text[start:], open: true}
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
// A template without a substitution keeps its raw body, which only the
// string-keyed registry dialect reads as a literal (NGI-V0-008).
func (lexer *webLexer) readTemplate() webToken {
	lexer.position++
	start, substituted := lexer.position, false
	for lexer.position < len(lexer.text) {
		switch {
		case lexer.text[lexer.position] == '\\':
			lexer.position += 2
		case lexer.hasAt("${"):
			substituted = true
			lexer.position += 2
			lexer.previous = webToken{kind: webPunct, text: "{"}
			lexer.skipSubstitution()
		case lexer.text[lexer.position] == '`':
			body := lexer.text[start:lexer.position]
			lexer.position++
			if substituted {
				return webToken{kind: webString}
			}
			return webToken{kind: webString, text: body, template: true}
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
		// A nested template appends its own substitution tokens while it is
		// read; it is inserted ahead of them, so a template always precedes
		// the tokens of its substitutions.
		mark := 0
		if lexer.substitutions != nil {
			mark = len(*lexer.substitutions)
		}
		token, ok := lexer.next()
		if !ok {
			return
		}
		if token.kind == webPunct {
			switch token.text {
			case "{":
				depth++
			case "}":
				depth--
			}
		}
		if depth == 0 {
			return
		}
		if lexer.substitutions != nil {
			*lexer.substitutions = slices.Insert(*lexer.substitutions, mark, webPlacedToken{token, lexer.position})
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
	result := make(map[string]struct{})
	unterminated := walkWebImports(text, func(specifier string, _ bool) {
		result[specifier] = struct{}{}
	})
	return result, unterminated
}

// WebImportKinds reports every module specifier of a JavaScript or TypeScript
// source with whether it is type-only: true only when every statement naming
// it is an `import type` or `export type` statement, which TypeScript erases
// and which therefore creates no runtime dependency (GPK-V0-079, proposed).
// A specifier named once as a value and once as a type is a value edge. The
// second result is the unterminated-construct reason webImports reports.
func WebImportKinds(text string) (map[string]bool, string) {
	result := make(map[string]bool)
	unterminated := walkWebImports(text, func(specifier string, typeOnly bool) {
		if previous, seen := result[specifier]; seen {
			result[specifier] = previous && typeOnly
			return
		}
		result[specifier] = typeOnly
	})
	return result, unterminated
}

// walkWebImports visits each specifier-carrying statement in source order.
func walkWebImports(text string, visit func(specifier string, typeOnly bool)) string {
	lexer := &webLexer{text: text}
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
		specifier, typeOnly, last := webSpecifier(lexer, token.text)
		if specifier != "" {
			visit(specifier, typeOnly)
		}
		previous = last
	}
	return lexer.unterminated
}

// webSpecifier consumes one `import` or `export` statement's head and reports
// the specifier it carries, empty when it carries none, and whether the
// statement is type-only. It also reports the last token it consumed, so the
// caller's property-access guard stays aligned.
//
// A statement is type-only when its first clause word is `type` and the next
// token is neither `from`, `,` nor `=`: `import type X from`, `import type {
// X } from`, `import type * as N from`, `export type { X } from` and `export
// type * from` are, while `import type from "x"` and `import type, { x } from
// "x"` bind a value named type. Inline `import { type X }` stays a value edge,
// which can only overstate a dependency, never hide one.
func webSpecifier(lexer *webLexer, keyword string) (string, bool, webToken) {
	token, ok := lexer.next()
	if !ok {
		return "", false, webToken{}
	}
	if keyword == "import" {
		// `import "x"` is the side-effect form; `import("x")` the dynamic one.
		// A dynamic import whose argument is not a literal names no module the
		// index can record, and contributes nothing rather than guessing.
		if token.kind == webString {
			return quotedSpecifier(token), false, token
		}
		if token.kind == webPunct && token.text == "(" {
			argument, present := lexer.next()
			if !present {
				return "", false, webToken{}
			}
			return quotedSpecifier(argument), false, argument
		}
	}
	typeOnly, first := false, true
	for {
		if token.kind == webWord && token.text == "from" {
			specifier, present := lexer.next()
			if !present {
				return "", false, webToken{}
			}
			if specifier.kind == webString {
				return quotedSpecifier(specifier), typeOnly, specifier
			}
			// `import { from }`: a binding spelled from is a clause word.
			token = specifier
			continue
		}
		if !webClauseToken(token) {
			return "", false, token
		}
		next, present := lexer.next()
		if !present {
			return "", false, token
		}
		if first && token.kind == webWord && token.text == "type" {
			typeOnly = !(next.kind == webWord && next.text == "from") && !(next.kind == webPunct && (next.text == "," || next.text == "="))
		}
		first = false
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
