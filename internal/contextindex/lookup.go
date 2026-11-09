package contextindex

import (
	"strings"
)

// lookup.go is the structural query surface of the `context` verb
// (TCP-V0-017, proposed): three read-only sub-modes over the compiled index
// so an agent iterates against the snapshot instead of grep. Every mode reads
// only the tables the packet reads (Symbols, the term table, the import
// graph, Sources), so an excluded path can never appear: it is absent from
// all of them.

const (
	// LookupDefaultLimit is the result cap when the invocation names none.
	LookupDefaultLimit = 20
	// LookupIdentifierErrorCode is the typed refusal for an identifier or term
	// that is empty or over maxLookupIdentifierBytes.
	LookupIdentifierErrorCode = "unsupported-context-lookup-identifier"
)

// countWholeWord counts the occurrences of word in text bounded by non-word
// bytes on both sides, and reports the 1-based line of the first.
func countWholeWord(text, word string) (int, int) {
	count, firstLine, line, from := 0, 0, 1, 0
	for {
		at := strings.Index(text[from:], word)
		if at < 0 {
			return count, firstLine
		}
		start := from + at
		end := start + len(word)
		line += strings.Count(text[from:start], "\n")
		from = start + 1
		if start > 0 && isWordByte(text[start-1]) {
			continue
		}
		if end < len(text) && isWordByte(text[end]) {
			continue
		}
		count++
		if firstLine == 0 {
			firstLine = line
		}
	}
}
