package tcq

import (
	"regexp"
	"strings"
)

var selectorWord = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*`)

// selectorFragment mirrors the selector grammar of OCM extractor
// `corvint-test-claim/1`. TCQ-V0-004 forbids trusting a caller-supplied selector,
// so the fragment is re-derived here from the target blob's own case literal and
// compared, never read out of the OCM row.
func selectorFragment(value string) string {
	words := selectorWord.FindAllString(expandWordBoundaries(value), -1)
	for index, word := range words {
		words[index] = singularize(strings.ToLower(word))
	}
	fragment := strings.Trim(strings.Join(words, "-"), "-")
	if len(fragment) > 96 {
		fragment = strings.Trim(fragment[:96], "-")
	}
	if fragment == "" {
		return "unnamed"
	}
	return fragment
}

func expandWordBoundaries(value string) string {
	var expanded strings.Builder
	characters := []rune(value)
	for index, character := range characters {
		previous := rune(0)
		if index > 0 {
			previous = characters[index-1]
		}
		if isLowerOrDigit(previous) && character >= 'A' && character <= 'Z' {
			expanded.WriteByte(' ')
		}
		if character == '_' || character == '-' {
			expanded.WriteByte(' ')
			continue
		}
		expanded.WriteRune(character)
	}
	return expanded.String()
}

func isLowerOrDigit(character rune) bool {
	return (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')
}

func singularize(word string) string {
	if strings.HasSuffix(word, "ies") && len(word) > 5 {
		return strings.TrimSuffix(word, "ies") + "y"
	}
	if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") && len(word) > 4 {
		return strings.TrimSuffix(word, "s")
	}
	return word
}
