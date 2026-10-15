// Package javalex blanks Java source text the way javac lexes it, so both the
// Java plugin and the Kotlin plugin's scan of Java files read the same header.
// It has no dependencies on either plugin.
package javalex

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Blank blanks comments and the contents of string, character and text block
// literals while keeping identifiers, quotes and newlines. It also returns the
// contents of each string literal keyed by the offset of its opening quote in
// the blanked text. An unterminated construct is not guessed. Unicode escapes
// are translated first, as javac does, so an escaped line terminator ends a
// line comment; offsets are into the translated text.
func Blank(text string) (string, map[int]string, bool) {
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
	return output.String(), literals, true
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
