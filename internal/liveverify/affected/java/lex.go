package java

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// sourceText blanks comments and the contents of string, character and text
// block literals while keeping identifiers, quotes and newlines. It also
// returns each literal Class.forName target, which is a type reference the
// blanked text no longer shows. An unterminated construct is not guessed.
// Unicode escapes are translated first, as javac does, so an escaped line
// terminator ends a line comment.
func sourceText(text string) (string, []string, bool) {
	text, ok := translateUnicodeEscapes(text)
	if !ok {
		return "", nil, false
	}
	const (
		code = iota
		lineComment
		blockComment
		quoted
		textBlock
		character
	)
	state := code
	escaped := false
	start := 0
	literals := map[int]string{}
	var output strings.Builder
	output.Grow(len(text))
	for index := 0; index < len(text); index++ {
		current := text[index]
		next := byte(0)
		if index+1 < len(text) {
			next = text[index+1]
		}
		switch state {
		case code:
			switch {
			case current == '/' && next == '/':
				state = lineComment
				output.WriteString("  ")
				index++
			case current == '/' && next == '*':
				state = blockComment
				output.WriteString("  ")
				index++
			case strings.HasPrefix(text[index:], `"""`):
				state = textBlock
				output.WriteString(`"""`)
				index += 2
			case current == '"':
				state = quoted
				start = index
				output.WriteByte('"')
			case current == '\'':
				state = character
				output.WriteByte('\'')
			default:
				output.WriteByte(current)
			}
		case lineComment:
			if current == '\n' || current == '\r' {
				state = code
				output.WriteByte(current)
			} else {
				output.WriteByte(' ')
			}
		case blockComment:
			switch {
			case current == '*' && next == '/':
				state = code
				output.WriteString("  ")
				index++
			case current == '\n' || current == '\r':
				output.WriteByte(current)
			default:
				output.WriteByte(' ')
			}
		case quoted, character:
			if current == '\n' || current == '\r' {
				return "", nil, false
			}
			if escaped {
				escaped = false
				output.WriteByte(' ')
				continue
			}
			switch {
			case current == '\\':
				escaped = true
				output.WriteByte(' ')
			case state == quoted && current == '"':
				literals[start] = text[start+1 : index]
				state = code
				output.WriteByte('"')
			case state == character && current == '\'':
				state = code
				output.WriteByte('\'')
			default:
				output.WriteByte(' ')
			}
		case textBlock:
			switch {
			case escaped:
				escaped = false
				output.WriteByte(' ')
			case current == '\\':
				escaped = true
				output.WriteByte(' ')
			case strings.HasPrefix(text[index:], `"""`):
				state = code
				output.WriteString(`"""`)
				index += 2
			case current == '\n' || current == '\r':
				output.WriteByte(current)
			default:
				output.WriteByte(' ')
			}
		}
	}
	if state != code && state != lineComment {
		return "", nil, false
	}
	clean := output.String()
	return clean, forNameTargets(clean, literals), true
}

// forNameTargets returns the literal argument of each forName call whose only
// or first argument is one string literal naming a dotted type.
func forNameTargets(clean string, literals map[int]string) []string {
	targets := make([]string, 0)
	for offset := 0; ; {
		found := strings.Index(clean[offset:], "forName")
		if found < 0 {
			break
		}
		at := offset + found + len("forName")
		offset = at
		at = skipSpace(clean, at)
		if at >= len(clean) || clean[at] != '(' {
			continue
		}
		at = skipSpace(clean, at+1)
		value, ok := literals[at]
		if !ok || dottedName(value) == "" {
			continue
		}
		end := skipSpace(clean, at+len(value)+2)
		if end < len(clean) && (clean[end] == ')' || clean[end] == ',') {
			targets = append(targets, value)
		}
	}
	return targets
}

func skipSpace(text string, index int) int {
	for index < len(text) && (text[index] == ' ' || text[index] == '\t' || text[index] == '\n' || text[index] == '\r') {
		index++
	}
	return index
}

// translateUnicodeEscapes applies JLS 3.3: a backslash preceded by an even
// number of backslashes, then one or more u and four hex digits, is the
// character it names. A malformed escape is a compile error, so it fails.
func translateUnicodeEscapes(text string) (string, bool) {
	if !strings.Contains(text, "\\u") {
		return text, true
	}
	var output strings.Builder
	output.Grow(len(text))
	backslashes := 0
	for index := 0; index < len(text); index++ {
		current := text[index]
		if current != '\\' || backslashes%2 == 1 || index+1 >= len(text) || text[index+1] != 'u' {
			if current == '\\' {
				backslashes++
			} else {
				backslashes = 0
			}
			output.WriteByte(current)
			continue
		}
		at := index + 1
		for at < len(text) && text[at] == 'u' {
			at++
		}
		if at+4 > len(text) {
			return "", false
		}
		value, err := strconv.ParseUint(text[at:at+4], 16, 32)
		if err != nil {
			return "", false
		}
		decoded := rune(value)
		if !utf8.ValidRune(decoded) {
			// A lone surrogate half cannot be a code point; it can only sit in a
			// literal or comment, so any non-ASCII placeholder keeps the lexing.
			decoded = utf8.RuneError
		}
		output.WriteRune(decoded)
		// An escaped backslash does not start another escape.
		backslashes = 0
		index = at + 3
	}
	return output.String(), true
}
