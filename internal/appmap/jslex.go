package appmap

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The application map reads JavaScript and TypeScript lexically: a token stream that tells code
// from comment and literal, never an evaluator. Anything the compiler would need to execute to
// know is reported UNKNOWN (AMAP-V0-002, AMAP-V0-005).

type tokKind int

const (
	tokIdent tokKind = iota + 1
	tokString
	tokTemplate // a template literal; parts holds its text and substitution boundaries
	tokNumber
	tokPunct
)

type token struct {
	kind tokKind
	text string // identifier, punctuation, number, or a string literal's decoded body
	line int
	// subst is true for a template literal with at least one ${} substitution; text then holds
	// the literal with every substitution replaced by substMark.
	subst bool
	// inexact marks a string or template whose escapes could not be decoded exactly (legacy
	// octal, a lone surrogate); its text is then not the program's value and never a literal.
	inexact bool
	// code is the raw source of a template literal from its first ${ to its end: the tokens never
	// show the substitutions, so a reader that must see every use of a name searches it. unsure
	// marks a template the lexer cannot bound: a substitution holding a `/` (a comment, regex or
	// division) or a backslash (an escaped identifier), or no closing backtick, so code or the
	// tokens after it may hide source. lexJS also sets it on the first token when it met source it
	// cannot place anywhere (see lexJS).
	code   string
	unsure bool
	// glued marks punctuation written directly after other punctuation (the second `+` of `++`).
	glued bool
	// regex is a regular-expression literal's source, slashes and flags included; the token is
	// otherwise an inexact empty string (AMAP-V0-092).
	regex string
	// doubtLine, on the first token only, is the line where lexJS first met source it cannot place.
	doubtLine int
}

// literal reports whether t is a string or substitution-free template whose value is exact.
func literal(t token) bool {
	return !t.inexact && (t.kind == tokString || (t.kind == tokTemplate && !t.subst))
}

// substMark stands for one ${} substitution inside a template literal's text.
const substMark = "\x00"

var (
	controlKeywords = map[string]bool{"if": true, "while": true, "for": true, "with": true}
	// regexKeywords are the reserved words after which a `/` starts a regular expression; a
	// division cannot follow them. A `/` after doubtWords may be either: contextual keywords that
	// may also be plain names (`let of = 4; of / 2`), and `debugger`, which ends a statement.
	regexKeywords = map[string]bool{"return": true, "typeof": true, "case": true, "in": true,
		"else": true, "do": true, "void": true, "delete": true, "throw": true, "new": true,
		"instanceof": true, "extends": true, "default": true, "break": true, "continue": true}
	doubtWords = map[string]bool{"of": true, "yield": true, "await": true, "debugger": true}
	// regexAfter is the closed list of punctuation after which only an expression can start, so
	// a `/` there surely starts a regular expression. `+`, `-`, `!`, `)` and `>` are decided by
	// regexStarts; after any other punctuation a `/` is doubt.
	regexAfter = map[string]bool{"(": true, ",": true, "=": true, ":": true, "[": true, "?": true,
		"~": true, "&": true, "|": true, "^": true, "*": true, "%": true, ";": true, "{": true, "/": true}
)

// regexStarts reports whether a `/` after toks starts a regular expression, and whether that is
// sure. A regular expression is sure only after a token from a closed list that can precede
// nothing but an expression (the start, regexAfter, a regexKeywords word, a prefix `!` run there,
// an odd `+`/`-` run); a division is sure only after a token that can only end an operand (a
// name, a property, a literal, `]`, a `)` closing no control head, an even `++`/`--` run). control
// tells whether a final `)` closes a control head (1), does not (0) or is not known (-1). Every
// other predecessor is doubt: `}`, `<`, a `>` not of `=>`, `.`, a label after break/continue, and
// a `!` run after an operand (a TypeScript postfix `x!`).
func regexStarts(toks []token, control int) (re, sure bool) {
	n := len(toks)
	if n == 0 {
		return true, true
	}
	p := toks[n-1]
	switch p.kind {
	case tokIdent:
		switch {
		case property(toks, n-1):
			return false, true
		case regexKeywords[p.text]:
			return true, true
		case doubtWords[p.text], word(toks, n-2, "break"), word(toks, n-2, "continue"):
			return false, false // `break label` ends a statement; a regular expression may follow
		}
		return false, true
	case tokPunct:
		switch p.text {
		case ")":
			return control == 1, control >= 0
		case "]":
			return false, true
		case ">":
			return true, p.glued && isPunct(toks[n-2], "=") // `=>`; else doubt (type arguments' end, a comparison)
		case "!":
			// A `!` run is a prefix operator only where an expression surely starts.
			k := n - 1
			for k >= 0 && isPunct(toks[k], "!") {
				k--
			}
			if re, sure := regexStarts(toks[:k+1], -1); re && sure {
				return true, true
			}
			return false, false
		case "+", "-":
			// Maximal munch: an even run ends in `++`/`--`, which no regular expression follows
			// (`n++ / 2`; `++/re/` is an early error); an odd run ends in a binary or unary operator.
			run := 1
			for k := n - 1; k > 0 && toks[k].glued && toks[k-1].text == p.text; k-- {
				run++
			}
			return run%2 == 1, true
		}
		return regexAfter[p.text], regexAfter[p.text]
	}
	return false, true // a number, string, template or regular expression
}

// lineEnd returns the index of the first line terminator (LF, CR, U+2028, U+2029) in text at or
// after i, or len(text).
func lineEnd(text string, i int) int {
	for j := i; j < len(text); j++ {
		if text[j] == '\n' || text[j] == '\r' || terminatorAt(text, j) {
			return j
		}
	}
	return len(text)
}

// oddEnd reports whether the line terminator at text[j] is one the line count does not see: a lone
// CR, U+2028 or U+2029. A comment or hashbang it ends is doubt, since a reader built on lines
// would not expect code after it on the same counted line.
func oddEnd(text string, j int) bool {
	return j < len(text) && text[j] != '\n' && !(text[j] == '\r' && (j+1 == len(text) || text[j+1] == '\n'))
}

// terminatorAt reports whether text holds U+2028 or U+2029 at i.
func terminatorAt(text string, i int) bool {
	return strings.HasPrefix(text[i:], "\u2028") || strings.HasPrefix(text[i:], "\u2029")
}

// lexJS tokenizes text and returns the tokens and a copy of text with every comment and literal
// body blanked to spaces (newlines kept), for line-shaped scans of code only. Where it cannot be
// sure the tokens show all the code, it marks the first token unsure, so a reader that must see
// every use of a name fails closed: a `/` it cannot place as a regular expression or a division
// (see regexStarts), a regular expression or string or block comment with no end, a `//` comment
// or hashbang line ended by a lone CR, U+2028 or U+2029 (it ends there, as in JavaScript), a
// non-ASCII identifier character that is not a letter, digit or mark (a Unicode space splits the
// name), or an HTML-like comment (`<!--`, `-->`). Line terminators are LF, CR, U+2028 and U+2029.

func lexJS(text string) ([]token, string) { return lexJSComments(text, nil) }

// jsComment is one comment lexJSComments met: its first line and its text, delimiters included.
type jsComment struct {
	line int
	text string
}

// lexJSComments is lexJS that also appends every comment to comments when it is not nil, so a
// reader of annotations (TSA-V0-011) sees exactly the comments the token stream left out.
func lexJSComments(text string, comments *[]jsComment) ([]token, string) {
	lx := &jsLexer{text: text, code: []byte(text), comments: comments}
	toks, _, _ := lx.lex(0, 1, 0)
	if lx.doubt && len(toks) > 0 {
		toks[0].unsure, toks[0].doubtLine = true, lx.doubtLine
	}
	return toks, string(lx.code)
}

// lexJSX is lexJS that also reads JSX (AMAP-V0-356): an element where an expression surely starts
// becomes the tokens `jsx ( e1 , e2 ... )` of its `{}` expression containers, each lexed as code,
// so its tag names, attribute strings and text never reach the token stream. An element it cannot
// bound is doubt.
func lexJSX(text string) []token {
	lx := &jsLexer{text: text, code: []byte(text), jsx: true}
	toks, _, _ := lx.lex(0, 1, 0)
	if lx.doubt && len(toks) > 0 {
		toks[0].unsure, toks[0].doubtLine = true, lx.doubtLine
	}
	return toks
}

// jsLexer is one lexJS pass over text: code is text with comments and literal bodies blanked,
// doubt is set with the line it was first met on when the lexer met source it cannot place.
type jsLexer struct {
	text      string
	code      []byte
	comments  *[]jsComment
	jsx       bool
	doubt     bool
	doubtLine int
}

// maxJSXDepth bounds nested JSX elements and expression containers.
const maxJSXDepth = 64

// lex lexes text from i, on line, to its end; at depth > 0 it lexes one JSX expression container
// and stops before the `}` that closes it. It returns the tokens, the index it stopped at and the
// line there.
func (lx *jsLexer) lex(i, line, depth int) ([]token, int, int) {
	text, toks := lx.text, []token{}
	blank := func(from, to int) {
		for k := from; k < to && k < len(lx.code); k++ {
			if lx.code[k] != '\n' {
				lx.code[k] = ' '
			}
		}
	}
	note := func(unplaced bool) {
		if unplaced && !lx.doubt {
			lx.doubt, lx.doubtLine = true, line
		}
	}
	// parens records, for each open '(', whether it opens a control-statement condition; after
	// its ')' a '/' starts a regular expression (if (x) /re/.test(s)), not a division.
	parens, closedControl, braces := []bool{}, false, 0
	if i == 0 && strings.HasPrefix(text, "#!") { // a hashbang line is a comment
		i = lineEnd(text, 0)
		note(oddEnd(text, i))
		blank(0, i)
	}
	for i < len(text) {
		c := text[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case terminatorAt(text, i): // U+2028, U+2029: line terminators the line count does not track
			i += len("\u2028")
		case c == '/' && i+1 < len(text) && text[i+1] == '/':
			j := lineEnd(text, i)
			note(oddEnd(text, j))
			if lx.comments != nil {
				*lx.comments = append(*lx.comments, jsComment{line: line, text: text[i:j]})
			}
			blank(i, j)
			i = j
		case c == '/' && i+1 < len(text) && text[i+1] == '*':
			j := strings.Index(text[i+2:], "*/")
			end := len(text)
			if j >= 0 {
				end = i + 2 + j + 2
			} else {
				note(true)
			}
			if lx.comments != nil {
				*lx.comments = append(*lx.comments, jsComment{line: line, text: text[i:end]})
			}
			line += strings.Count(text[i:end], "\n")
			blank(i, end)
			i = end
		case c == '\'' || c == '"':
			body, inexact, closed, end := readQuoted(text, i)
			note(!closed)
			toks = append(toks, token{kind: tokString, text: body, line: line, inexact: inexact})
			line += strings.Count(text[i:end], "\n") // line continuations
			blank(i+1, end-1)
			i = end
		case c == '`':
			body, subst, inexact, unsure, end := readTemplate(text, i)
			code := ""
			if subst {
				code = text[i+strings.Index(text[i:end], "${") : end]
			}
			toks = append(toks, token{kind: tokTemplate, text: body, line: line, subst: subst, inexact: inexact, code: code, unsure: unsure})
			line += strings.Count(text[i:end], "\n")
			blank(i+1, end-1)
			i = end
		case c == '/':
			// A regular-expression literal closes on its own line; one that does not was misread.
			control := 0
			if closedControl {
				control = 1
			}
			re, sure := regexStarts(toks, control)
			note(!sure)
			j := -1
			if re {
				j = regexEnd(text, i)
				note(j < 0)
			}
			if j < 0 {
				toks = append(toks, token{kind: tokPunct, text: "/", line: line, glued: glued(toks, text, i)})
				i++
				continue
			}
			blank(i+1, j-1)
			// A regex is a value but never a literal string: a pattern is not the text it matches.
			toks = append(toks, token{kind: tokString, text: "", line: line, inexact: true, regex: text[i:j]})
			i = j
		case isIdentStart(c):
			j := i + 1
			for j < len(text) && isIdentPart(text[j]) {
				j++
			}
			note(!identChars(text[i:j]))
			toks = append(toks, token{kind: tokIdent, text: text[i:j], line: line})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(text) && (isIdentPart(text[j]) || text[j] == '.') {
				j++
			}
			toks = append(toks, token{kind: tokNumber, text: text[i:j], line: line})
			i = j
		case lx.jsx && c == '<' && jsxMayStart(toks, text, i, closedControl):
			el, end, endLine, ok := lx.element(i, line, depth+1)
			if !ok {
				note(true)
				toks = append(toks, token{kind: tokPunct, text: "<", line: line, glued: glued(toks, text, i)})
				i++
				continue
			}
			toks = append(toks, el...)
			i, line = end, endLine
		case depth > 0 && c == '}' && braces == 0:
			return toks, i, line
		default:
			switch c {
			case '{':
				braces++
			case '}':
				braces--
			case '(':
				n := len(toks)
				control := n > 0 && toks[n-1].kind == tokIdent && !property(toks, n-1) &&
					(controlKeywords[toks[n-1].text] || toks[n-1].text == "await" && word(toks, n-2, "for"))
				parens = append(parens, control)
			case ')':
				closedControl = false
				if n := len(parens); n > 0 {
					closedControl, parens = parens[n-1], parens[:n-1]
				}
			}
			// An HTML-like comment (Annex B, scripts only): `<!--` anywhere, `-->` opening a line (any
			// `-->`, since a line may also start after a CR, U+2028 or U+2029).
			note(strings.HasPrefix(text[i:], "<!--") || strings.HasPrefix(text[i:], "-->"))
			toks = append(toks, token{kind: tokPunct, text: string(c), line: line, glued: glued(toks, text, i)})
			i++
		}
	}
	if depth > 0 {
		note(true) // an expression container without its closing brace
	}
	return toks, i, line
}

// jsxMayStart reports whether the `<` at text[i] starts a JSX element: an expression surely starts
// there and a tag name or a fragment's `>` follows, and it is not a TypeScript generic arrow's type
// parameter list (`<T,>`, `<T extends U>`, `<T = U>`).
func jsxMayStart(toks []token, text string, i int, closedControl bool) bool {
	if i+1 >= len(text) || !(isIdentStart(text[i+1]) || text[i+1] == '>') {
		return false
	}
	control := 0
	if closedControl {
		control = 1
	}
	if re, sure := regexStarts(toks, control); !re || !sure {
		return false
	}
	j := i + 1
	for j < len(text) && isIdentPart(text[j]) {
		j++
	}
	rest := strings.TrimLeft(text[j:], " \t")
	return !strings.HasPrefix(rest, ",") && !strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "extends ")
}

// element reads the JSX element at text[i] to its end: its tokens, the index after it and the
// line there, or ok false when it cannot be bounded.
func (lx *jsLexer) element(i, line, depth int) ([]token, int, int, bool) {
	text := lx.text
	if depth > maxJSXDepth {
		return nil, 0, 0, false
	}
	out := []token{{kind: tokIdent, text: "jsx", line: line}, {kind: tokPunct, text: "(", line: line}}
	args := 0
	adv := func(to int) {
		line += strings.Count(text[i:to], "\n")
		i = to
	}
	container := func() bool { // text[i] is '{'
		toks, end, endLine := lx.lex(i+1, line, depth+1)
		if end >= len(text) || text[end] != '}' {
			return false
		}
		if len(toks) > 0 {
			if args > 0 {
				out = append(out, token{kind: tokPunct, text: ",", line: toks[0].line})
			}
			out = append(out, toks...)
			args++
		}
		i, line = end+1, endLine
		return true
	}
	name := func() string {
		j := i
		for j < len(text) && (isIdentPart(text[j]) || text[j] == '.' || text[j] == ':' || text[j] == '-') {
			j++
		}
		n := text[i:j]
		i = j
		return n
	}
	space := func() {
		j := i
		for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n' || text[j] == '\r') {
			j++
		}
		adv(j)
	}
	i++ // '<'
	tag := name()
	for open := true; open; {
		space()
		switch {
		case i >= len(text):
			return nil, 0, 0, false
		case strings.HasPrefix(text[i:], "/>"):
			i += 2
			return append(out, token{kind: tokPunct, text: ")", line: line}), i, line, true
		case text[i] == '>':
			i++
			open = false
		case text[i] == '{':
			if !container() {
				return nil, 0, 0, false
			}
		case isIdentStart(text[i]) && tag != "":
			name()
			space()
			if i < len(text) && text[i] == '=' {
				i++
				space()
				switch {
				case i < len(text) && (text[i] == '"' || text[i] == '\''):
					k := strings.IndexByte(text[i+1:], text[i])
					if k < 0 {
						return nil, 0, 0, false
					}
					adv(i + 1 + k + 1)
				case i < len(text) && text[i] == '{':
					if !container() {
						return nil, 0, 0, false
					}
				default:
					return nil, 0, 0, false
				}
			}
		default:
			return nil, 0, 0, false
		}
	}
	for i < len(text) {
		switch {
		case strings.HasPrefix(text[i:], "</"):
			i += 2
			space()
			if name() != tag {
				return nil, 0, 0, false
			}
			space()
			if i >= len(text) || text[i] != '>' {
				return nil, 0, 0, false
			}
			i++
			return append(out, token{kind: tokPunct, text: ")", line: line}), i, line, true
		case text[i] == '<':
			el, end, endLine, ok := lx.element(i, line, depth+1)
			if !ok {
				return nil, 0, 0, false
			}
			if args > 0 {
				out = append(out, token{kind: tokPunct, text: ",", line: line})
			}
			out, args = append(out, el...), args+1
			i, line = end, endLine
		case text[i] == '{':
			if !container() {
				return nil, 0, 0, false
			}
		default:
			adv(i + 1)
		}
	}
	return nil, 0, 0, false
}

// glued reports whether the punctuation at text[i] directly follows the punctuation token last in
// toks: no literal, comment or regular expression ends in a punctuation character it could equal.
func glued(toks []token, text string, i int) bool {
	n := len(toks)
	return n > 0 && i > 0 && toks[n-1].kind == tokPunct && toks[n-1].text == text[i-1:i]
}

// identChars reports whether every non-ASCII character of an identifier the lexer read is one
// JavaScript allows there; a Unicode space or other separator would end the name instead.
func identChars(s string) bool {
	for _, r := range s {
		if r < utf8.RuneSelf {
			continue
		}
		if r == utf8.RuneError || !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) ||
			unicode.Is(unicode.Pc, r) || unicode.Is(unicode.Nl, r) || r == 0x200C || r == 0x200D) {
			return false
		}
	}
	return true
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentPart(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

// readQuoted reads a '...' or "..." literal starting at i; it ends at its closing quote or at the
// line break, so an unterminated literal cannot swallow the rest of the file, and reports whether
// the quote closed it.
func readQuoted(text string, i int) (string, bool, bool, int) {
	q := text[i]
	var b strings.Builder
	inexact := false
	j := i + 1
	for j < len(text) && text[j] != q && text[j] != '\n' && text[j] != '\r' {
		if text[j] == '\\' && j+1 < len(text) {
			var ok bool
			j, ok = unescape(&b, text, j+1)
			inexact = inexact || !ok
			continue
		}
		b.WriteByte(text[j])
		j++
	}
	if j < len(text) && text[j] == q {
		return b.String(), inexact, true, j + 1
	}
	return b.String(), inexact, false, j
}

// unescape decodes the escape sequence whose character after the backslash is at text[j], writing
// its value to b and returning the index after it. It reports false for a sequence whose value it
// cannot know exactly: a legacy octal escape or a lone UTF-16 surrogate.
func unescape(b *strings.Builder, text string, j int) (int, bool) {
	c := text[j]
	switch c {
	case 'n':
		b.WriteByte('\n')
	case 'r':
		b.WriteByte('\r')
	case 't':
		b.WriteByte('\t')
	case 'b':
		b.WriteByte('\b')
	case 'f':
		b.WriteByte('\f')
	case 'v':
		b.WriteByte('\v')
	case '\n': // line continuation
	case '\r':
		if j+1 < len(text) && text[j+1] == '\n' {
			return j + 2, true
		}
	case '0':
		if j+1 < len(text) && text[j+1] >= '0' && text[j+1] <= '9' {
			return j + 1, false
		}
		b.WriteByte(0)
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return j + 1, false
	case 'x':
		if r, ok := hexRune(text, j+1, j+3); ok {
			b.WriteRune(r)
			return j + 3, true
		}
		return j + 1, false
	case 'u':
		r, end, ok := unicodeEscape(text, j+1)
		if ok && r >= 0xD800 && r <= 0xDBFF && end+1 < len(text) && text[end] == '\\' && text[end+1] == 'u' {
			if lo, end2, ok2 := unicodeEscape(text, end+2); ok2 && lo >= 0xDC00 && lo <= 0xDFFF {
				b.WriteRune((r-0xD800)<<10 + (lo - 0xDC00) + 0x10000)
				return end2, true
			}
		}
		if !ok || (r >= 0xD800 && r <= 0xDFFF) {
			return max(end, j+1), false
		}
		b.WriteRune(r)
		return end, true
	default:
		_, size := utf8.DecodeRuneInString(text[j:])
		b.WriteString(text[j : j+size])
		return j + size, true
	}
	return j + 1, true
}

// unicodeEscape reads the HHHH or {H+} after a \u at text[j].
func unicodeEscape(text string, j int) (rune, int, bool) {
	if j < len(text) && text[j] == '{' {
		end := strings.IndexByte(text[j:], '}')
		if end < 2 || end > 7 {
			return 0, j, false
		}
		r, ok := hexRune(text, j+1, j+end)
		return r, j + end + 1, ok && r <= utf8.MaxRune
	}
	r, ok := hexRune(text, j, j+4)
	if !ok {
		return 0, j, false // never consume past a short or non-hex escape
	}
	return r, j + 4, true
}

func hexRune(text string, from, to int) (rune, bool) {
	if to > len(text) {
		return 0, false
	}
	v, err := strconv.ParseUint(text[from:to], 16, 32)
	return rune(v), err == nil
}

// readTemplate reads a template literal starting at i, replacing each ${...} with substMark. It
// reports whether a substitution holds a `/` or backslash it cannot place (see token.unsure).
func readTemplate(text string, i int) (string, bool, bool, bool, int) {
	var b strings.Builder
	subst, inexact, unsure := false, false, false
	j := i + 1
	for j < len(text) && text[j] != '`' {
		switch {
		case text[j] == '\\' && j+1 < len(text):
			var ok bool
			j, ok = unescape(&b, text, j+1)
			inexact = inexact || !ok
		case text[j] == '$' && j+1 < len(text) && text[j+1] == '{':
			subst = true
			b.WriteString(substMark)
			depth := 0
			for j < len(text) {
				if text[j] == '\'' || text[j] == '"' { // a brace in a nested literal does not count
					var closed bool
					_, _, closed, j = readQuoted(text, j)
					unsure = unsure || !closed
					continue
				}
				if text[j] == '`' {
					var inner bool
					_, _, _, inner, j = readTemplate(text, j)
					unsure = unsure || inner
					continue
				}
				slash := 0
				if text[j] == '/' {
					slash = substSlash(text, i, j)
				}
				if slash > 0 {
					if end := regexEnd(text, j); end > 0 {
						j = end
						continue
					}
				}
				unsure = unsure || text[j] == '/' && slash >= 0 || text[j] == '\\' // a comment or a slash it cannot place; an escaped identifier
				if text[j] == '{' {
					depth++
				} else if text[j] == '}' {
					depth--
					if depth == 0 {
						j++
						break
					}
				}
				j++
			}
		default:
			b.WriteByte(text[j])
			j++
		}
	}
	if j < len(text) {
		j++
	} else {
		unsure = true // unterminated: the rest of the file may be code the tokens do not show
	}
	return b.String(), subst, inexact, unsure, j
}

// substSlash classifies the `/` at j, inside a substitution of the template starting at i, by the
// source before it: 1 when only an expression may start there, so it begins a regular expression
// (after an opening punctuator, `=>`, `${` or a regexKeywords word, as in
// `${x.replace(/[:.]/g, "-")}`); -1 when it can only follow an operand, so it is a division (after
// a name that is not a keyword, a number or `]`); 0 when it is a comment or cannot be placed
// (TSA-V0-838, TSA-V0-847).
func substSlash(text string, i, j int) int {
	if j+1 < len(text) && (text[j+1] == '/' || text[j+1] == '*') {
		return 0
	}
	k := j - 1
	for k > i && (text[k] == ' ' || text[k] == '\t') {
		k--
	}
	if k <= i {
		return 0
	}
	switch c := text[k]; {
	case strings.IndexByte("(,=:[!&|?;{~^*%", c) >= 0:
		return 1
	case c == '>' && text[k-1] == '=':
		return 1
	case c == ']':
		return -1
	case c == '_' || c == '$' || c >= '0' && c <= '9' || c|0x20 >= 'a' && c|0x20 <= 'z':
		start := k
		for start > i && (text[start-1] == '_' || text[start-1] == '$' || text[start-1] >= '0' && text[start-1] <= '9' || text[start-1]|0x20 >= 'a' && text[start-1]|0x20 <= 'z') {
			start--
		}
		switch word := text[start : k+1]; {
		case regexKeywords[word]:
			return 1
		case doubtWords[word] || text[start-1] == '\\' || text[start-1] >= 0x80:
			return 0
		}
		return -1
	}
	return 0
}

func regexEnd(text string, i int) int {
	inClass := false
	for j := i + 1; j < len(text); j++ {
		if text[j] == '\n' || text[j] == '\r' || terminatorAt(text, j) {
			return -1
		}
		switch text[j] {
		case '\\':
			j++
			if j < len(text) && (text[j] == '\n' || text[j] == '\r' || terminatorAt(text, j)) {
				return -1
			}
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if !inClass {
				k := j + 1
				for k < len(text) && isIdentPart(text[k]) {
					k++
				}
				return k
			}
		}
	}
	return -1
}

// jsValue is a literal value read from an object or array literal; kind "other" is anything the
// compiler would have to evaluate.
type jsValue struct {
	kind  string // string, bool, number, object, array, member (`X.Y`, str "X.Y"), other
	str   string
	tok   int // a member's token index: which binding of `X` it reads (AMAP-V0-022)
	obj   []jsPair
	arr   []jsValue
	line  int
	subst bool
}

type jsPair struct {
	key   string
	value jsValue
}

// get returns the value of key; as in JavaScript, the last of repeated keys wins.
func (v jsValue) get(key string) (jsValue, bool) {
	for i := len(v.obj) - 1; i >= 0; i-- {
		if v.obj[i].key == key {
			return v.obj[i].value, true
		}
	}
	return jsValue{}, false
}

// stringList reads an array of plain string literals; ok is false for anything else.
func (v jsValue) stringList() ([]string, bool) {
	if v.kind != "array" {
		return nil, false
	}
	out := []string{}
	for _, e := range v.arr {
		if e.kind != "string" {
			return nil, false
		}
		out = append(out, e.str)
	}
	return out, true
}

// parseValue reads one value starting at toks[i] and returns the index after it. A value it cannot
// read literally is skipped to the next ',' or closing bracket at its own depth.
func parseValue(toks []token, i int) (jsValue, int) {
	if i >= len(toks) {
		return jsValue{kind: "other"}, i
	}
	t := toks[i]
	switch {
	case literal(t):
		if next(toks, i+1, ",", "}", "]", ")") {
			return jsValue{kind: "string", str: t.text, line: t.line}, i + 1
		}
	case t.kind == tokNumber:
		if next(toks, i+1, ",", "}", "]", ")") {
			return jsValue{kind: "number", str: t.text, line: t.line}, i + 1
		}
	case t.kind == tokIdent && (t.text == "true" || t.text == "false"):
		if next(toks, i+1, ",", "}", "]", ")") {
			return jsValue{kind: "bool", str: t.text, line: t.line}, i + 1
		}
	case t.kind == tokIdent && next(toks, i+1, ".") && i+2 < len(toks) && toks[i+2].kind == tokIdent:
		if end := assertedEnd(toks, i+3); next(toks, end, ",", "}", "]", ")") {
			return jsValue{kind: "member", str: t.text + "." + toks[i+2].text, line: t.line, tok: i}, end
		}
	case t.kind == tokPunct && t.text == "{":
		return parseObject(toks, i)
	case t.kind == tokPunct && t.text == "[":
		return parseArray(toks, i)
	}
	return jsValue{kind: "other", line: t.line, subst: t.kind == tokTemplate}, skipValue(toks, i)
}

// assertedEnd steps over TypeScript assertions after a value ending at toks[i] -- `!`, `as T`,
// `satisfies T` -- which leave the value unchanged (AMAP-V0-204), and returns the index after
// them; i when there is none, or when the type is not one skipAssertedType reads.
func assertedEnd(toks []token, i int) int {
	for {
		switch {
		case next(toks, i, "!") && !next(toks, i+1, "="):
			i++
		case word(toks, i, "as") || word(toks, i, "satisfies"):
			k := skipAssertedType(toks, i+1)
			if k < 0 {
				return i
			}
			i = k
		default:
			return i
		}
	}
}

func next(toks []token, i int, punct ...string) bool {
	if i < 0 || i >= len(toks) || toks[i].kind != tokPunct {
		return false
	}
	for _, p := range punct {
		if toks[i].text == p {
			return true
		}
	}
	return false
}

// skipValue advances past one expression to the ',' or closing bracket that ends it.
func skipValue(toks []token, i int) int {
	depth := 0
	for ; i < len(toks); i++ {
		t := toks[i]
		if t.kind != tokPunct {
			continue
		}
		switch t.text {
		case "{", "[", "(":
			depth++
		case "}", "]", ")":
			if depth == 0 {
				return i
			}
			depth--
		case ",":
			if depth == 0 {
				return i
			}
		}
	}
	return i
}

func parseObject(toks []token, i int) (jsValue, int) {
	v := jsValue{kind: "object", line: toks[i].line}
	i++
	for i < len(toks) {
		if next(toks, i, "}") {
			return v, i + 1
		}
		t := toks[i]
		if (t.kind != tokIdent && (t.kind != tokString || t.inexact)) || !next(toks, i+1, ":") {
			// spread, computed key, method or shorthand: the object is not a plain literal
			v.obj = append(v.obj, jsPair{key: "", value: jsValue{kind: "other", line: t.line}})
			i = skipValue(toks, i)
		} else {
			var val jsValue
			val, i = parseValue(toks, i+2)
			if _, dup := v.get(t.text); dup {
				// A repeated key replaces the earlier value; mark the object as not plain so a
				// reader that walks members cannot act on the superseded one (AMAP-V0-002).
				v.obj = append(v.obj, jsPair{key: "", value: jsValue{kind: "other", line: t.line}})
			}
			v.obj = append(v.obj, jsPair{key: t.text, value: val})
		}
		if next(toks, i, ",") {
			i++
			continue
		}
		if next(toks, i, "}") {
			return v, i + 1
		}
		return jsValue{kind: "other", line: v.line}, skipValue(toks, i)
	}
	return jsValue{kind: "other", line: v.line}, i
}

func parseArray(toks []token, i int) (jsValue, int) {
	v := jsValue{kind: "array", line: toks[i].line}
	i++
	for i < len(toks) {
		if next(toks, i, "]") {
			return v, i + 1
		}
		var val jsValue
		val, i = parseValue(toks, i)
		v.arr = append(v.arr, val)
		if next(toks, i, ",") {
			i++
			continue
		}
		if next(toks, i, "]") {
			return v, i + 1
		}
		return jsValue{kind: "other", line: v.line}, skipValue(toks, i)
	}
	return jsValue{kind: "other", line: v.line}, i
}

// closeParen returns the index of the ')' matching the '(' at toks[i].
func closeParen(toks []token, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		if toks[j].kind != tokPunct {
			continue
		}
		switch toks[j].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(toks) - 1
}
