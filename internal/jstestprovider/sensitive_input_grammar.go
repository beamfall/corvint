package jstestprovider

import (
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// Candidates are transient. Split only top-level arguments; commas/parentheses
// inside quoted strings or nested expressions are part of their argument.
func boundaryTailCandidates(tail string, add func(string)) {
	tail = strings.TrimSpace(tail)
	add(tail)
	add(strings.TrimFunc(tail, jstestreceipt.BoundaryDelimiter))
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
				if decoded, ok := jstestreceipt.BoundaryDecodeJSString(raw); ok {
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
			if depth == 0 && jstestreceipt.BoundaryWordRune(r) && (i == 0 || !jstestreceipt.BoundaryWordRune(runes[i-1])) {
				end := i
				for end < len(runes) && jstestreceipt.BoundaryWordRune(runes[end]) {
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
