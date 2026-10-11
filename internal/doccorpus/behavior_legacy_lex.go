package doccorpus

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// legacyToken is one significant JavaScript/TypeScript token of a legacy test file. The lexer is
// the same static, comment- and literal-aware approach the Playwright producers use: it never
// evaluates code, and anything it cannot classify refuses the file instead of guessing.
type legacyToken struct {
	kind byte // 'i' identifier, 's' static string, 't' template with substitutions, 'n' number, 'r' regex, 'p' punctuation
	text string
	raw  string
	line int
	sub  []legacyToken // the tokens of a template's substitutions, so code inside them is never invisible
}

var (
	legacyRegexAfterWord  = words("return typeof case do else in of new delete void throw instanceof yield await")
	legacyRegexAfterParen = words("if while for with")
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
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			i++
		case strings.HasPrefix(src[i:], "//"):
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				i = len(src)
			} else {
				i += end
			}
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, 0, false
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
			end := i + 1
			for end < len(src) && (legacyWordByte(src[end]) || src[end] == '.') {
				end++
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
			for _, long := range []string{"...", "=>", "?."} {
				if strings.HasPrefix(src[i:], long) {
					text = long
					break
				}
			}
			tokens = append(tokens, legacyToken{kind: 'p', text: text, raw: text, line: l.line(i)})
			i += len(text)
		}
	}
	return tokens, i, !sub
}

func legacyLineStarts(src string) []int {
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
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
		case "}":
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
		return legacyRegexAfterWord[last.text], true
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
			if !ok {
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
		value.WriteRune(r)
		return 5, true
	default:
		r, size := utf8.DecodeRuneInString(rest)
		value.WriteRune(r)
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
		default:
			value.WriteByte(src[i])
		}
	}
	return 0, "", nil, false, false
}

func legacyRegex(src string, start int) (int, bool) {
	class := false
	for i := start + 1; i < len(src); i++ {
		switch src[i] {
		case '\n', '\r':
			return 0, false
		case '\\':
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

// legacyCanonical joins tokens[lo:hi] without layout: a space only separates two word tokens, so
// reformatting never changes the text or its digest while any literal or name change does.
func legacyCanonical(tokens []legacyToken, lo, hi int) string {
	var out strings.Builder
	for i := lo; i < hi; i++ {
		if i > lo && tokens[i].kind != 'p' && tokens[i-1].kind != 'p' {
			out.WriteByte(' ')
		}
		out.WriteString(tokens[i].raw)
	}
	return out.String()
}
