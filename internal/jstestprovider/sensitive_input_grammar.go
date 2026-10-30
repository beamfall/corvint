package jstestprovider

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

func boundaryWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r)
}

func boundaryDelimiter(r rune) bool { return !boundaryWordRune(r) }

func boundaryWords(value string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !boundaryWordRune(r) }), " ")
}

func boundaryPatternEnd(title []rune, start int, pattern string) (int, bool) {
	words := strings.Fields(boundaryWords(pattern))
	if len(words) == 0 {
		return start, false
	}
	for i, word := range words {
		if i > 0 {
			for start < len(title) && boundaryDelimiter(title[start]) {
				start++
			}
		}
		end := start
		for end < len(title) && boundaryWordRune(title[end]) {
			end++
		}
		if strings.ToLower(string(title[start:end])) != word {
			return start, false
		}
		start = end
	}
	return start, true
}

// Only the leading action position and a contiguous receiver chain are searched.
// Indices always address the original rune sequence, never case-folded bytes.
func boundaryActionMatch(title string, policy normalizedSensitivePolicy) (display, tail string) {
	runes := []rune(title)
	start := 0
	for start < len(runes) && boundaryDelimiter(runes[start]) {
		start++
	}
	for start < len(runes) {
		for _, action := range boundaryDefaultActions {
			for _, pattern := range action.patterns {
				if end, ok := boundaryPatternEnd(runes, start, pattern); ok {
					return action.display, string(runes[end:])
				}
			}
		}
		for _, pattern := range policy.actions[len(defaultSensitiveActionPatterns):] {
			if end, ok := boundaryPatternEnd(runes, start, pattern); ok {
				return boundaryWords(pattern), string(runes[end:])
			}
		}
		end := start
		for end < len(runes) && (boundaryWordRune(runes[end]) || runes[end] == '_' || runes[end] == '$') {
			end++
		}
		if end == start {
			break
		}
		separator := end
		for separator < len(runes) && unicode.IsSpace(runes[separator]) {
			separator++
		}
		if separator == len(runes) || (runes[separator] != '.' && !(runes[separator] == '/' && separator == end)) {
			break
		}
		start = separator + 1
		for start < len(runes) && boundaryDelimiter(runes[start]) {
			start++
		}
	}
	return "", ""
}

func boundaryActionDisplay(title string, policy normalizedSensitivePolicy) (string, bool) {
	display, _ := boundaryActionMatch(title, policy)
	return display, display != ""
}

// This is a bounded lexical refusal, not a receiver-call parser. Prose without
// a leading receiver expression and quoted non-property text stay out of scope.
func boundaryUnsupportedSensitiveAction(title string, policy normalizedSensitivePolicy) bool {
	if display, _ := boundaryActionMatch(title, policy); display != "" {
		return false
	}
	runes := []rune(title)
	start := 0
	for start < len(runes) && boundaryDelimiter(runes[start]) {
		start++
	}
	end := start
	for end < len(runes) && (boundaryWordRune(runes[end]) || runes[end] == '_' || runes[end] == '$') {
		end++
	}
	if end == start {
		return false
	}
	for end < len(runes) && unicode.IsSpace(runes[end]) {
		end++
	}
	if end == len(runes) || !strings.ContainsRune(".([", runes[end]) {
		return false
	}
	var quote rune
	quoteStart := 0
	property, escaped := false, false
	for i := end; i < len(runes); i++ {
		r := runes[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				if property && boundarySensitiveProperty(string(runes[quoteStart:i]), policy) {
					return true
				}
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' || r == '`' {
			quote, quoteStart = r, i+1
			previous := i - 1
			for previous >= end && unicode.IsSpace(runes[previous]) {
				previous--
			}
			property = previous >= end && runes[previous] == '['
			continue
		}
		if boundaryWordRune(r) && (i == 0 || !boundaryWordRune(runes[i-1])) {
			for _, pattern := range policy.actions {
				if _, ok := boundaryPatternEnd(runes, i, pattern); ok {
					return true
				}
			}
		}
	}
	return quote != 0
}

// An undecodable property name fails closed: it may spell a sensitive action
// the classifier cannot see (V1-3197).
func boundarySensitiveProperty(raw string, policy normalizedSensitivePolicy) bool {
	raw, ok := boundaryDecodeJSString(raw)
	if !ok {
		return true
	}
	for _, pattern := range policy.actions {
		if boundaryWords(raw) == boundaryWords(pattern) {
			return true
		}
	}
	return false
}

// Candidates are transient. Split only top-level arguments; commas/parentheses
// inside quoted strings or nested expressions are part of their argument.
func boundaryTailCandidates(tail string, add func(string)) {
	tail = strings.TrimSpace(tail)
	add(tail)
	add(strings.TrimFunc(tail, boundaryDelimiter))
	tail = strings.TrimLeftFunc(tail, func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune("):=-_", r) })
	if strings.HasPrefix(tail, "(") {
		tail = strings.TrimPrefix(tail, "(")
		tail = strings.TrimSuffix(tail, ")")
	}
	add(tail)
	runes := []rune(tail)
	start, quoteStart, depth := 0, 0, 0
	var quote rune
	escaped := false
	for i, r := range runes {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				raw := string(runes[quoteStart:i])
				add(raw)
				if decoded, ok := boundaryDecodeJSString(raw); ok {
					add(decoded)
				}
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'':
			quote, quoteStart = r, i+1
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				add(string(runes[start:i]))
				start = i + 1
			}
		default:
			if depth == 0 && boundaryWordRune(r) && (i == 0 || !boundaryWordRune(runes[i-1])) {
				end := i
				for end < len(runes) && boundaryWordRune(runes[end]) {
					end++
				}
				word := strings.ToLower(string(runes[i:end]))
				if word == "with" || word == "value" || word == "text" {
					add(string(runes[end:]))
				}
			}
		}
	}
	add(string(runes[start:]))
}

// boundaryDecodeJSString decodes the body of a JavaScript string or template
// literal with the full ECMAScript escape grammar: single-character escapes,
// \0, \xHH, \uHHHH (surrogate pairs combined), \u{H..}, line continuations
// and identity escapes. A trailing backslash, a malformed or out-of-range
// escape, a lone surrogate or a legacy octal escape reports false.
func boundaryDecodeJSString(raw string) (string, bool) {
	if !strings.ContainsRune(raw, '\\') {
		return raw, true
	}
	var units []uint16
	runes := []rune(raw)
	hex := func(from, to int) (rune, bool) {
		if to > len(runes) || from >= to {
			return 0, false
		}
		v, err := strconv.ParseUint(string(runes[from:to]), 16, 32)
		if err != nil || v > unicode.MaxRune {
			return 0, false
		}
		return rune(v), true
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != '\\' {
			units = utf16.AppendRune(units, r)
			continue
		}
		i++
		if i == len(runes) {
			return "", false
		}
		switch r = runes[i]; r {
		case 'b':
			units = append(units, '\b')
		case 'f':
			units = append(units, '\f')
		case 'n':
			units = append(units, '\n')
		case 'r':
			units = append(units, '\r')
		case '\r':
			if i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
		case '\n', '\u2028', '\u2029':
		case 't':
			units = append(units, '\t')
		case 'v':
			units = append(units, '\v')
		case '0':
			if i+1 < len(runes) && runes[i+1] >= '0' && runes[i+1] <= '9' {
				return "", false
			}
			units = append(units, 0)
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return "", false
		case 'x':
			v, ok := hex(i+1, i+3)
			if !ok {
				return "", false
			}
			units, i = append(units, uint16(v)), i+2
		case 'u':
			if i+1 < len(runes) && runes[i+1] == '{' {
				end := i + 2
				for end < len(runes) && runes[end] != '}' {
					end++
				}
				v, ok := hex(i+2, end)
				if !ok || end == len(runes) {
					return "", false
				}
				// A BMP code point stays one code unit, so a surrogate written as
				// \u{D83D} pairs or stays lone exactly as \uD83D does.
				if v <= 0xFFFF {
					units, i = append(units, uint16(v)), end
				} else {
					units, i = utf16.AppendRune(units, v), end
				}
				continue
			}
			v, ok := hex(i+1, i+5)
			if !ok {
				return "", false
			}
			units, i = append(units, uint16(v)), i+4
		default:
			units = utf16.AppendRune(units, r)
		}
	}
	for i := 0; i < len(units); i++ {
		if utf16.IsSurrogate(rune(units[i])) {
			if i+1 == len(units) || utf16.DecodeRune(rune(units[i]), rune(units[i+1])) == unicode.ReplacementChar {
				return "", false
			}
			i++
		}
	}
	return string(utf16.Decode(units)), true
}
