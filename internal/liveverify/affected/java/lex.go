package java

import (
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected/java/javalex"
)

// sourceText blanks Java source text with javalex.Blank and returns each
// literal Class.forName target, which is a type reference the blanked text no
// longer shows.
func sourceText(text string) (string, []string, bool) {
	clean, literals, ok := javalex.Blank(text)
	if !ok {
		return "", nil, false
	}
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
