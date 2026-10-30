package doccorpus

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// legacyToken is one significant JavaScript/TypeScript token of a legacy test file. The lexer is
// the same static, comment- and literal-aware approach the Playwright producers use: it never
// evaluates code, and anything it cannot classify refuses the file instead of guessing.
type legacyToken struct {
	kind byte // 'i' identifier, 's' static string, 't' template with substitutions, 'n' number, 'r' regex, 'p' punctuation
	text string
	raw  string
	line int           // the anchor line: LF-delimited, as the corpus anchor span counts lines
	nl   bool          // a JavaScript line terminator (LF, CR, CRLF, U+2028, U+2029) precedes the token
	sub  []legacyToken // the tokens of a template's substitutions, so code inside them is never invisible
}

var (
	legacyRegexAfterWord  = words("return typeof case do else in of new delete void throw instanceof yield await")
	legacyRegexAfterParen = words("if while for with")
	// legacyOperators are the multi-character punctuators, longest first, so operator boundaries
	// survive tokenization ("x + ++y" and "x++ + y" stay distinct).
	legacyOperators = strings.Fields(">>>= ... === !== **= <<= >>= >>> &&= ||= ??= => ?. == != <= >= && || ?? ++ -- += -= *= %= &= |= ^= ** << >>")
	// legacySeparators are the punctuators canonical text joins without a space.
	legacySeparators = words("( ) [ ] { } , ; . ?. ...")
)

// lexLegacy tokenizes src. It refuses (ok=false) invalid UTF-8, an unterminated literal or
// comment, an invalid escape, an unbalanced bracket, a '/' it cannot classify as division or
// regex, or a byte it cannot classify, so a file is either fully tokenized or unknown.
func lexLegacy(src string) ([]legacyToken, []int, bool) {
	if !utf8.ValidString(src) {
		return nil, nil, false
	}
	lexer := legacyLexer{src: src, lines: legacyLineStarts(src)}
	tokens, end, ok := lexer.run(0, false)
	if !ok || end != len(src) {
		return nil, nil, false
	}
	match, ok := legacyMatchBrackets(tokens)
	return tokens, match, ok
}

type legacyLexer struct {
	src   string
	lines []int
}

func (l *legacyLexer) line(offset int) int { return sort.SearchInts(l.lines, offset+1) }

// run lexes from i. In a template substitution (sub) it stops at the '}' that closes it and
// returns that offset.
func (l *legacyLexer) run(i int, sub bool) ([]legacyToken, int, bool) {
	src := l.src
	tokens := []legacyToken{}
	braces := 0
	// broke records a line terminator since the previous token, in layout or inside a comment,
	// which is what automatic semicolon insertion and restricted productions test.
	broke, seen := false, 0
	mark := func() {
		// Each pass appends at most one token; flag it with the breaks scanned before it.
		if len(tokens) > seen {
			tokens[len(tokens)-1].nl, broke, seen = broke, false, len(tokens)
		}
	}
	for i < len(src) {
		mark()
		c := src[i]
		if size := legacyLineTerminator(src, i); size > 0 {
			broke = true
			i += size
			continue
		}
		switch {
		case c == ' ' || c == '\t' || c == '\f' || c == '\v':
			i++
		case strings.HasPrefix(src[i:], "//"):
			// A single-line comment ends at the first JavaScript line terminator, which is not
			// part of it.
			i += 2
			for i < len(src) && legacyLineTerminator(src, i) == 0 {
				i++
			}
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, 0, false
			}
			for k := i + 2; k < i+2+end; k++ {
				if legacyLineTerminator(src, k) > 0 {
					broke = true
				}
			}
			i += end + 4
		case c == '\'' || c == '"':
			end, value, ok := legacyQuoted(src, i)
			if !ok {
				return nil, 0, false
			}
			tokens = append(tokens, legacyToken{kind: 's', text: value, raw: src[i:end], line: l.line(i)})
			i = end
		case c == '`':
			end, value, inner, static, ok := l.template(i)
			if !ok {
				return nil, 0, false
			}
			kind := byte('t')
			if static {
				kind = 's'
			}
			tokens = append(tokens, legacyToken{kind: kind, text: value, raw: src[i:end], line: l.line(i), sub: inner})
			i = end
		case c == '/':
			regex, known := legacyRegexAllowed(tokens)
			if !known {
				return nil, 0, false
			}
			if !regex {
				tokens = append(tokens, legacyToken{kind: 'p', text: "/", raw: "/", line: l.line(i)})
				i++
				continue
			}
			end, ok := legacyRegex(src, i)
			if !ok {
				return nil, 0, false
			}
			tokens = append(tokens, legacyToken{kind: 'r', text: src[i:end], raw: src[i:end], line: l.line(i)})
			i = end
		case c >= '0' && c <= '9' || c == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9':
			end, ok := legacyNumber(src, i)
			if !ok {
				return nil, 0, false
			}
			tokens = append(tokens, legacyToken{kind: 'n', text: src[i:end], raw: src[i:end], line: l.line(i)})
			i = end
		default:
			r, size := utf8.DecodeRuneInString(src[i:])
			if r == '_' || r == '$' || unicode.IsLetter(r) {
				end := i + size
				for end < len(src) {
					next, width := utf8.DecodeRuneInString(src[end:])
					if next != '_' && next != '$' && !unicode.IsLetter(next) && !unicode.IsDigit(next) {
						break
					}
					end += width
				}
				tokens = append(tokens, legacyToken{kind: 'i', text: src[i:end], raw: src[i:end], line: l.line(i)})
				i = end
				continue
			}
			if r > unicode.MaxASCII && !unicode.IsSpace(r) {
				return nil, 0, false
			}
			if unicode.IsSpace(r) {
				i += size
				continue
			}
			if sub && c == '}' && braces == 0 {
				return tokens, i, true
			}
			if c == '{' {
				braces++
			} else if c == '}' {
				braces--
			}
			text := string(c)
			for _, long := range legacyOperators {
				if strings.HasPrefix(src[i:], long) {
					text = long
					break
				}
			}
			tokens = append(tokens, legacyToken{kind: 'p', text: text, raw: text, line: l.line(i)})
			i += len(text)
		}
	}
	mark()
	return tokens, i, !sub
}

// legacyLineTerminator returns the byte length of the JavaScript line terminator at src[i] (LF,
// CR, CRLF, U+2028 or U+2029), or 0. It is the one definition comments, automatic semicolon
// insertion and canonical text use.
func legacyLineTerminator(src string, i int) int {
	switch {
	case src[i] == '\n':
		return 1
	case src[i] == '\r':
		if i+1 < len(src) && src[i+1] == '\n' {
			return 2
		}
		return 1
	case strings.HasPrefix(src[i:], "\u2028") || strings.HasPrefix(src[i:], "\u2029"):
		return 3
	}
	return 0
}

// legacyLineStarts indexes anchor lines. Anchors use the corpus-wide LF line convention (span), so
// JavaScript line semantics are carried by each token's nl flag instead.
func legacyLineStarts(src string) []int {
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// legacyNumber scans one JavaScript numeric literal at start: a 0x/0o/0b integer, or digits with
// at most one fraction and one exponent, each with optional _ separators and a BigInt n suffix.
// A literal followed directly by an identifier character (1.toString, 3in) is a syntax error, so
// the file is refused; 1..should keeps the second dot as a member access.
func legacyNumber(src string, start int) (int, bool) {
	i := start
	digits := func(ok func(byte) bool) {
		for i < len(src) && (ok(src[i]) || src[i] == '_') {
			i++
		}
	}
	decimal := func(c byte) bool { return c >= '0' && c <= '9' }
	if src[i] == '0' && i+1 < len(src) && strings.ContainsRune("xXoObB", rune(src[i+1])) {
		i += 2
		digits(func(c byte) bool { return decimal(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' })
	} else {
		digits(decimal)
		if i < len(src) && src[i] == '.' {
			i++
			digits(decimal)
		}
		if i < len(src) && (src[i] == 'e' || src[i] == 'E') {
			i++
			if i < len(src) && (src[i] == '+' || src[i] == '-') {
				i++
			}
			digits(decimal)
		}
	}
	if i < len(src) && src[i] == 'n' {
		i++
	}
	if i < len(src) {
		if r, _ := utf8.DecodeRuneInString(src[i:]); legacyWordByte(src[i]) || r == '\\' || r > unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return 0, false
		}
	}
	return i, true
}

func legacyWordByte(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// legacyRegexAllowed decides whether a '/' starts a regex. known is false where JavaScript's
// answer depends on statement context the static reader does not track (after '}'), so the file
// is refused rather than tokenized one way or the other by guess.
func legacyRegexAllowed(tokens []legacyToken) (regex, known bool) {
	if len(tokens) == 0 {
		return true, true
	}
	last := tokens[len(tokens)-1]
	switch last.kind {
	case 'p':
		switch last.text {
		case "]":
			return false, true
		case "}", "++", "--":
			return false, false
		case ")":
			depth := 0
			for k := len(tokens) - 1; k >= 0; k-- {
				if tokens[k].kind != 'p' {
					continue
				}
				switch tokens[k].text {
				case ")":
					depth++
				case "(":
					if depth--; depth == 0 {
						return k > 0 && tokens[k-1].kind == 'i' && legacyRegexAfterParen[tokens[k-1].text] && !legacyChained(tokens, k-1), true
					}
				}
			}
			return false, false
		}
		return true, true
	case 'i':
		// A keyword used as a property name (obj.return) ends an expression.
		return legacyRegexAfterWord[last.text] && !legacyChained(tokens, len(tokens)-1), true
	}
	return false, true
}

// legacyQuoted scans a single- or double-quoted string and returns its decoded value.
func legacyQuoted(src string, start int) (int, string, bool) {
	quote := src[start]
	var value strings.Builder
	for i := start + 1; i < len(src); i++ {
		switch src[i] {
		case quote:
			return i + 1, value.String(), true
		case '\n', '\r':
			return 0, "", false
		case '\\':
			used, ok := legacyEscape(src[i+1:], &value)
			if !ok {
				return 0, "", false
			}
			i += used
		default:
			value.WriteByte(src[i])
		}
	}
	return 0, "", false
}

// legacyEscape decodes the JavaScript escape after a backslash into value and returns the bytes
// consumed. An escape whose meaning is not a single defined character (a malformed \x or \u, or a
// legacy octal escape) refuses the literal instead of guessing its value.
func legacyEscape(rest string, value *strings.Builder) (int, bool) {
	if rest == "" {
		return 0, false
	}
	hex := func(digits string) (rune, bool) {
		code, err := strconv.ParseUint(digits, 16, 32)
		return rune(code), err == nil && code <= unicode.MaxRune
	}
	switch rest[0] {
	case 'n':
		value.WriteByte('\n')
	case 't':
		value.WriteByte('\t')
	case 'r':
		value.WriteByte('\r')
	case 'b':
		value.WriteByte('\b')
	case 'f':
		value.WriteByte('\f')
	case 'v':
		value.WriteByte('\v')
	case '\n':
	case '\r':
		if len(rest) > 1 && rest[1] == '\n' {
			return 2, true
		}
	case '0':
		if len(rest) > 1 && rest[1] >= '0' && rest[1] <= '9' {
			return 0, false
		}
		value.WriteByte(0)
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return 0, false
	case 'x':
		if len(rest) < 3 {
			return 0, false
		}
		r, ok := hex(rest[1:3])
		if !ok || strings.ContainsAny(rest[1:3], "+-") {
			return 0, false
		}
		value.WriteRune(r)
		return 3, true
	case 'u':
		if strings.HasPrefix(rest, "u{") {
			end := strings.IndexByte(rest, '}')
			if end < 3 || end > 9 || strings.ContainsAny(rest[2:end], "+-") {
				return 0, false
			}
			r, ok := hex(rest[2:end])
			if !ok || utf16.IsSurrogate(r) {
				return 0, false
			}
			value.WriteRune(r)
			return end + 1, true
		}
		if len(rest) < 5 || strings.ContainsAny(rest[1:5], "+-") {
			return 0, false
		}
		r, ok := hex(rest[1:5])
		if !ok {
			return 0, false
		}
		if !utf16.IsSurrogate(r) {
			value.WriteRune(r)
			return 5, true
		}
		// Only a high surrogate followed by an escaped low surrogate is one character; a lone
		// surrogate has no UTF-8 value and refuses the literal.
		if r >= 0xDC00 || len(rest) < 11 || rest[5:7] != `\u` || strings.ContainsAny(rest[7:11], "+-") {
			return 0, false
		}
		low, ok := hex(rest[7:11])
		if !ok || low < 0xDC00 || low > 0xDFFF {
			return 0, false
		}
		value.WriteRune(utf16.DecodeRune(r, low))
		return 11, true
	default:
		r, size := utf8.DecodeRuneInString(rest)
		if r != '\u2028' && r != '\u2029' {
			// A backslash before a line or paragraph separator is a line continuation.
			value.WriteRune(r)
		}
		return size, true
	}
	return 1, true
}

// template scans a template literal. Substitution code is lexed like any other code and kept on
// the token; static is false when the literal has a substitution.
func (l *legacyLexer) template(start int) (int, string, []legacyToken, bool, bool) {
	src := l.src
	var value strings.Builder
	var inner []legacyToken
	static := true
	for i := start + 1; i < len(src); i++ {
		switch {
		case src[i] == '`':
			return i + 1, value.String(), inner, static, true
		case src[i] == '\\':
			used, ok := legacyEscape(src[i+1:], &value)
			if !ok {
				return 0, "", nil, false, false
			}
			i += used
		case strings.HasPrefix(src[i:], "${"):
			static = false
			tokens, end, ok := l.run(i+2, true)
			if !ok {
				return 0, "", nil, false, false
			}
			inner = append(inner, tokens...)
			i = end
		case src[i] == '\r':
			// A template's cooked value normalizes CR and CRLF line terminators to LF.
			value.WriteByte('\n')
			if i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
		default:
			value.WriteByte(src[i])
		}
	}
	return 0, "", nil, false, false
}

func legacyRegex(src string, start int) (int, bool) {
	class := false
	for i := start + 1; i < len(src); i++ {
		if legacyLineTerminator(src, i) > 0 {
			return 0, false
		}
		switch src[i] {
		case '\\':
			if i+1 < len(src) && legacyLineTerminator(src, i+1) > 0 {
				return 0, false
			}
			i++
		case '[':
			class = true
		case ']':
			class = false
		case '/':
			if !class {
				end := i + 1
				for end < len(src) && legacyWordByte(src[end]) {
					end++
				}
				return end, true
			}
		}
	}
	return 0, false
}

// legacyMatchBrackets maps each bracket token to its partner, or refuses an unbalanced file.
func legacyMatchBrackets(tokens []legacyToken) ([]int, bool) {
	match := make([]int, len(tokens))
	stack := []int{}
	pairs := map[string]string{")": "(", "]": "[", "}": "{"}
	for i, token := range tokens {
		match[i] = -1
		if token.kind != 'p' {
			continue
		}
		switch token.text {
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			if len(stack) == 0 || tokens[stack[len(stack)-1]].text != pairs[token.text] {
				return nil, false
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			match[open], match[i] = i, open
		}
	}
	return match, len(stack) == 0
}

// legacyCanonical joins tokens[lo:hi] without layout: a space only separates two word tokens or
// two operators, so reformatting never changes the text or its digest while any literal, name or
// operator change does. A line break where a restricted production makes ASI insert a semicolon
// (after return, break, continue or yield, or before ++ or --) is written as that semicolon.
func legacyCanonical(tokens []legacyToken, lo, hi int) string {
	var out strings.Builder
	operator := func(token legacyToken) bool { return token.kind == 'p' && !legacySeparators[token.text] }
	for i := lo; i < hi; i++ {
		if i > lo && tokens[i].nl && legacyRestrictedBreak(tokens, i) {
			out.WriteByte(';')
			out.WriteString(tokens[i].raw)
			continue
		}
		if i > lo && (tokens[i].kind != 'p' && tokens[i-1].kind != 'p' || operator(tokens[i]) && operator(tokens[i-1])) {
			out.WriteByte(' ')
		}
		out.WriteString(tokens[i].raw)
	}
	return out.String()
}

// legacyRestrictedBreak reports whether a line break before tokens[i] ends a restricted production.
func legacyRestrictedBreak(tokens []legacyToken, i int) bool {
	previous := tokens[i-1]
	if previous.kind == 'i' && (i < 2 || tokens[i-2].kind != 'p' || tokens[i-2].text != "." && tokens[i-2].text != "?.") {
		switch previous.text {
		case "return", "break", "continue", "yield":
			return true
		}
	}
	return tokens[i].kind == 'p' && (tokens[i].text == "++" || tokens[i].text == "--")
}
