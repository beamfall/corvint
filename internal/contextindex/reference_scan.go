package contextindex

import (
	"unicode"
	"unicode/utf8"
)

// The two scanners below replace regexes that dominated the whole-checkout
// build's regexp time (V1-2746). Each returns exactly what its regex's
// FindAll returned; TestReferenceScannersMatchTheirRegexes holds the regexes
// as the oracle over the checkout and edge fixtures.

// isReferenceByte is documentReference's segment class [A-Za-z0-9_.-]. It is
// ASCII, so a byte scan sees the same characters as the regex's rune scan.
func isReferenceByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-'
}

// documentReferences is documentReference.FindAllString(text, -1) for
// `(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+`: in each run of segment bytes and
// slashes, every maximal chain of two or more non-empty segments joined by
// single slashes is one match. An empty segment (a doubled, leading or
// trailing slash) ends a chain, which is where the leftmost-first greedy
// regex stops and restarts.
func documentReferences(text string) []string {
	var matches []string
	for i := 0; i < len(text); {
		if !isReferenceByte(text[i]) {
			i++
			continue
		}
		start, segments, end := i, 1, i
		for {
			for end < len(text) && isReferenceByte(text[end]) {
				end++
			}
			if end+1 < len(text) && text[end] == '/' && isReferenceByte(text[end+1]) {
				segments++
				end++
				continue
			}
			break
		}
		if segments >= 2 {
			matches = append(matches, text[start:end])
		}
		i = end
	}
	return matches
}

// genericImportSpecifiers is, for every genericImport match
// `(?:from\s+|import\s*(?:\(\s*)?)["']([^"']+)["']` (with the oracle's
// space class), its captured specifier, skipping a match preceded by a word
// rune exactly as sourceImports did. A skipped match still consumes its span,
// as FindAll's non-overlapping scan did.
func genericImportSpecifiers(text string, result map[string]struct{}) {
	for i := 0; i < len(text); {
		end, body, bodyEnd := genericImportAt(text, i)
		if end < 0 {
			i++
			continue
		}
		if i == 0 {
			result[text[body:bodyEnd]] = struct{}{}
		} else if previous, _ := utf8.DecodeLastRuneInString(text[:i]); !isPythonWordRune(previous) {
			result[text[body:bodyEnd]] = struct{}{}
		}
		i = end
	}
}

// genericImportAt returns the end of a genericImport match starting at i and
// its capture bounds, or -1. Both quantified spaces are greedy and a quote or
// "(" is never a space, so the first parse is the only one that can match.
func genericImportAt(text string, i int) (int, int, int) {
	var next int
	switch {
	case len(text)-i >= 4 && text[i:i+4] == "from":
		after, spaces := skipGenericImportSpaces(text, i+4)
		if spaces == 0 {
			return -1, 0, 0
		}
		next = after
	case len(text)-i >= 6 && text[i:i+6] == "import":
		next, _ = skipGenericImportSpaces(text, i+6)
		if next < len(text) && text[next] == '(' {
			next, _ = skipGenericImportSpaces(text, next+1)
		}
	default:
		return -1, 0, 0
	}
	if next >= len(text) || text[next] != '"' && text[next] != '\'' {
		return -1, 0, 0
	}
	body := next + 1
	bodyEnd := body
	for bodyEnd < len(text) && text[bodyEnd] != '"' && text[bodyEnd] != '\'' {
		bodyEnd++
	}
	if bodyEnd == body || bodyEnd == len(text) {
		return -1, 0, 0
	}
	return bodyEnd + 1, body, bodyEnd
}

// skipGenericImportSpaces advances past the runes genericImport's space class
// admits, returning the new offset and how many runes it skipped.
func skipGenericImportSpaces(text string, offset int) (int, int) {
	count := 0
	for offset < len(text) {
		r, size := utf8.DecodeRuneInString(text[offset:])
		if !(r >= '\t' && r <= '\r' || r == ' ' || r >= 0x1c && r <= 0x1f || r == 0x85 || unicode.Is(unicode.Z, r)) {
			break
		}
		offset += size
		count++
	}
	return offset, count
}
