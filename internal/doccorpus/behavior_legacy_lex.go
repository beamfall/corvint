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
}

var legacyRegexAfterWord = words("return typeof case do else in of new delete void throw instanceof yield await")

// lexLegacy tokenizes src. It refuses (ok=false) an unterminated literal or comment, an
// unbalanced bracket or a byte it cannot classify, so a file is either fully tokenized or unknown.
func lexLegacy(src string) ([]legacyToken, []int, bool) {
	tokens := []legacyToken{}
	lines := legacyLineStarts(src)
	lineOf := func(offset int) int { return sort.SearchInts(lines, offset+1) }
	for i := 0; i < len(src); {
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
				return nil, nil, false
			}
			i += end + 4
		case c == '\'' || c == '"':
			end, value, ok := legacyQuoted(src, i)
			if !ok {
				return nil, nil, false
			}
			tokens = append(tokens, legacyToken{kind: 's', text: value, raw: src[i:end], line: lineOf(i)})
			i = end
		case c == '`':
			end, value, static, ok := legacyTemplate(src, i)
			if !ok {
				return nil, nil, false
			}
			kind := byte('t')
			if static {
				kind = 's'
			}
			tokens = append(tokens, legacyToken{kind: kind, text: value, raw: src[i:end], line: lineOf(i)})
			i = end
		case c == '/' && legacyRegexAllowed(tokens):
			end, ok := legacyRegex(src, i)
			if !ok {
				return nil, nil, false
			}
			tokens = append(tokens, legacyToken{kind: 'r', text: src[i:end], raw: src[i:end], line: lineOf(i)})
			i = end
		case c >= '0' && c <= '9' || c == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9':
			end := i + 1
			for end < len(src) && (legacyWordByte(src[end]) || src[end] == '.') {
				end++
			}
			tokens = append(tokens, legacyToken{kind: 'n', text: src[i:end], raw: src[i:end], line: lineOf(i)})
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
				tokens = append(tokens, legacyToken{kind: 'i', text: src[i:end], raw: src[i:end], line: lineOf(i)})
				i = end
				continue
			}
			if r == utf8.RuneError || r > unicode.MaxASCII && !unicode.IsSpace(r) {
				return nil, nil, false
			}
			if unicode.IsSpace(r) {
				i += size
				continue
			}
			text := string(c)
			for _, long := range []string{"...", "=>", "?."} {
				if strings.HasPrefix(src[i:], long) {
					text = long
					break
				}
			}
			tokens = append(tokens, legacyToken{kind: 'p', text: text, raw: text, line: lineOf(i)})
			i += len(text)
		}
	}
	match, ok := legacyMatchBrackets(tokens)
	return tokens, match, ok
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

func legacyRegexAllowed(tokens []legacyToken) bool {
	if len(tokens) == 0 {
		return true
	}
	last := tokens[len(tokens)-1]
	switch last.kind {
	case 'p':
		return last.text != ")" && last.text != "]" && last.text != "}"
	case 'i':
		return legacyRegexAfterWord[last.text]
	}
	return false
}

// legacyQuoted scans a single- or double-quoted string and returns its decoded value.
func legacyQuoted(src string, start int) (int, string, bool) {
	quote := src[start]
	var value strings.Builder
	for i := start + 1; i < len(src); i++ {
		switch src[i] {
		case quote:
			return i + 1, value.String(), true
		case '\n':
			return 0, "", false
		case '\\':
			if i+1 >= len(src) {
				return 0, "", false
			}
			i += legacyEscape(src[i+1:], &value)
		default:
			value.WriteByte(src[i])
		}
	}
	return 0, "", false
}

// legacyEscape decodes the escape after a backslash into value and returns the bytes consumed.
func legacyEscape(rest string, value *strings.Builder) int {
	switch rest[0] {
	case 'n':
		value.WriteByte('\n')
	case 't':
		value.WriteByte('\t')
	case 'r':
		value.WriteByte('\r')
	case '\n':
	case 'u':
		if len(rest) >= 5 {
			if code, err := strconv.ParseUint(rest[1:5], 16, 32); err == nil {
				value.WriteRune(rune(code))
				return 5
			}
		}
		value.WriteByte('u')
	default:
		r, size := utf8.DecodeRuneInString(rest)
		value.WriteRune(r)
		return size
	}
	return 1
}

// legacyTemplate scans a template literal. static is false when it has a substitution, whose code
// is skipped with nested literals and braces balanced.
func legacyTemplate(src string, start int) (int, string, bool, bool) {
	var value strings.Builder
	static := true
	for i := start + 1; i < len(src); i++ {
		switch {
		case src[i] == '`':
			return i + 1, value.String(), static, true
		case src[i] == '\\':
			if i+1 >= len(src) {
				return 0, "", false, false
			}
			i += legacyEscape(src[i+1:], &value)
		case strings.HasPrefix(src[i:], "${"):
			static = false
			end, ok := legacySubstitutionEnd(src, i+2)
			if !ok {
				return 0, "", false, false
			}
			i = end
		default:
			value.WriteByte(src[i])
		}
	}
	return 0, "", false, false
}

func legacySubstitutionEnd(src string, i int) (int, bool) {
	depth := 1
	for ; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i, true
			}
		case '\'', '"':
			end, _, ok := legacyQuoted(src, i)
			if !ok {
				return 0, false
			}
			i = end - 1
		case '`':
			end, _, _, ok := legacyTemplate(src, i)
			if !ok {
				return 0, false
			}
			i = end - 1
		}
	}
	return 0, false
}

func legacyRegex(src string, start int) (int, bool) {
	class := false
	for i := start + 1; i < len(src); i++ {
		switch src[i] {
		case '\n':
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
