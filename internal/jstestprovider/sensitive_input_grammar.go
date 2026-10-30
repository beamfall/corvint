package jstestprovider

import (
	"encoding/json"
	"strings"
	"unicode"
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
				if property && boundarySensitiveProperty(string(runes[quoteStart:i]), quote, policy) {
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

func boundarySensitiveProperty(raw string, quote rune, policy normalizedSensitivePolicy) bool {
	if quote == '\'' {
		raw = strings.ReplaceAll(raw, `\'`, `'`)
	}
	var decoded string
	if json.Unmarshal([]byte(`"`+raw+`"`), &decoded) == nil {
		raw = decoded
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
				if quote == '\'' {
					raw = strings.ReplaceAll(raw, `\'`, `'`)
				}
				var decoded string
				if json.Unmarshal([]byte(`"`+raw+`"`), &decoded) == nil {
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
