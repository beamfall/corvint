package affected

import "strings"

// MaxValueWordDepth bounds how deep ValueWords follows a value word inside a
// value word. The shell plugin keeps only the tokens of the blanked text for
// a deeper word; the driver joins a deeper word in place
// (driverJoinedValueWords).
const MaxValueWordDepth = 8

// ValueWords finds, by brace matching, the value-supplying parameter
// expansions of text: `${N:-WORD}`, `${N-WORD}`, `${N:=WORD}`, `${N=WORD}`,
// `${N:+WORD}` and `${N+WORD}` for a name, a positional parameter, `@` or
// `*`. WORD is the value the expansion takes when the parameter is unset,
// empty or set, so a literal default such as
// `file=${1:-conformance/register.md}` is a path the script reads
// (AZU-V0-8920). The `?` form's word is an error message, not a value, and
// every other operator is no value either.
//
// words lists each outermost expansion's word; a word may hold expansions
// of its own (`${1:-${ROOT:-.}/conformance/register.md}`), which the caller
// reads by passing the word back. A word whose closing brace is missing runs
// to the end of text. joined is text with each closed expansion replaced by
// its word, the words of that word replaced in turn down to depth: the text
// as it reads when every default applies (`./conformance/register.md`).
//
// The caller adds the tokens of words and of joined to those of the blanked
// text and never replaces them: a form this scan misreads loses no token.
// It is the one lexer of these forms: the shell plugin's path tokens
// (AZU-V0-8920) and the driver's (AZU-V0-8927) both read through it.
func ValueWords(text string, depth int) (words []string, joined string) {
	var out strings.Builder
	from := 0
	for {
		offset := strings.Index(text[from:], "${")
		if offset < 0 {
			break
		}
		open := from + offset
		word := valueWordStart(text, open+2)
		if word < 0 {
			// No value form: keep the opener and read on inside it.
			out.WriteString(text[from : open+2])
			from = open + 2
			continue
		}
		end := closingBrace(text, word)
		if end < 0 {
			// Unbalanced: the word is the rest, and nothing is joined.
			words = append(words, text[word:])
			break
		}
		value := text[word:end]
		words = append(words, value)
		out.WriteString(text[from:open])
		if depth > 0 {
			_, value = ValueWords(value, depth-1)
		}
		out.WriteString(value)
		from = end + 1
	}
	out.WriteString(text[from:])
	return words, out.String()
}

// valueWordStart returns the index of the word of a value-supplying
// expansion whose parameter starts at index (just after `${`), or -1.
func valueWordStart(text string, index int) int {
	start := index
	switch {
	case index >= len(text):
		return -1
	case text[index] == '@' || text[index] == '*':
		index++
	case text[index] >= '0' && text[index] <= '9':
		for index < len(text) && text[index] >= '0' && text[index] <= '9' {
			index++
		}
	default:
		for index < len(text) && isDriverNameByte(text[index]) {
			index++
		}
	}
	if index == start {
		return -1
	}
	if index < len(text) && text[index] == ':' {
		index++
	}
	if index >= len(text) || strings.IndexByte("-=+", text[index]) < 0 {
		return -1
	}
	return index + 1
}

// closingBrace returns the index of the `}` that closes a braced expansion
// whose content starts at index, or -1: a `${` inside opens another level,
// and a backslash escapes the byte after it.
func closingBrace(text string, index int) int {
	for level := 1; index < len(text); index++ {
		switch {
		case text[index] == '\\':
			index++
		case text[index] == '$' && index+1 < len(text) && text[index+1] == '{':
			level++
			index++
		case text[index] == '}':
			if level--; level == 0 {
				return index
			}
		}
	}
	return -1
}
