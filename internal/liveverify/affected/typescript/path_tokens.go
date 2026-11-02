package typescript

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// maxPathTokens bounds one unit's distinct path tokens. A unit over it keeps none and is marked
// PathTokensBounded (AFP-V0-021, TJAA-V0-102); tests lower it.
var maxPathTokens = affected.MaxPathsPerUnit

// pathLexicon is the AFP-V0-012 path-token lexicon the Go and shell plugins use.
var pathLexicon = regexp.MustCompile(`[A-Za-z0-9._~@+/-]+`)

// pathTokens returns the sorted, unique path-shaped tokens of a module's string, template-text and
// regular-expression contents (without the expression's delimiters), each escape-decoded, and
// whether they exceed maxPathTokens (TJAA-V0-102). Comments and code contribute nothing. A module
// whose comments or literals do not tokenize is read whole instead, so a literal the lexer cannot
// place still names its paths.
func pathTokens(body string) ([]string, bool) {
	return pathTokensSkipping(body, nil)
}

// pathTokensSkipping is pathTokens without the string literals that are exactly a module
// specifier the import scan parsed at that offset and the adapter resolved to a unit
// (TJAA-V0-126). skip maps a specifier's content offset to its text; a literal is left out only
// when it is that one quoted specifier, so the same text in any other literal is still read, and
// a module read whole because it does not tokenize keeps every token. A token only literals
// starting or ending on a fixturePathMarker line hold is left out too (TJAA-V0-830).
func pathTokensSkipping(body string, skip map[int]string) ([]string, bool) {
	tokens := map[string]bool{}
	var into map[string]bool
	add := func(text string) {
		decoded, _ := decodeStringEscapes(text)
		for _, token := range pathLexicon.FindAllString(decoded, -1) {
			if pathShaped(token) {
				tokens[token] = true
				if into != nil {
					into[token] = true
				}
			}
		}
	}
	clean, err := stripComments(body, false)
	lexed, ok := playwrightLex(clean)
	var markers [][2]int
	if err == nil && ok {
		markers = fixtureLines(body, clean)
	}
	marked, unmarked := map[string]bool{}, map[string]bool{}
	switch {
	case err != nil:
		add(body)
	case !ok:
		add(clean)
	default:
		for start := 0; start < len(clean); {
			if lexed.code[start] {
				start++
				continue
			}
			end := start
			for end < len(clean) && !lexed.code[end] {
				end++
			}
			literal := clean[start:end]
			if specifier, found := skip[start+1]; found && (literal[0] == '\'' || literal[0] == '"') && literal == literal[:1]+specifier+literal[:1] {
				start = end
				continue
			}
			if literal[0] == '/' {
				// Only a regular expression's run starts at a slash: drop its delimiters.
				literal = strings.TrimSuffix(literal[1:], "/")
			}
			into = unmarked
			if onMarkedLine(markers, start) || onMarkedLine(markers, end-1) {
				into = marked
			}
			add(literal)
			start = end
		}
	}
	for token := range marked {
		if !unmarked[token] {
			delete(tokens, token)
		}
	}
	if len(tokens) > maxPathTokens {
		return nil, true
	}
	if len(tokens) == 0 {
		return nil, false
	}
	values := make([]string, 0, len(tokens))
	for token := range tokens {
		values = append(values, token)
	}
	sort.Strings(values)
	return values, false
}

// pathShaped keeps a token that carries a slash or a dot, is not a bare number, and has a letter
// or digit, as the shell plugin does.
func pathShaped(token string) bool {
	if !strings.ContainsAny(token, "/.") || strings.Trim(token, "0123456789.") == "" {
		return false
	}
	return strings.IndexFunc(token, func(c rune) bool {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
	}) >= 0
}

// fixturePathMarker is the per-literal fixture marker of ASD-V0-895 and AFP-V0-551: a literal
// starting or ending on its line names no repository read (TJAA-V0-830).
const fixturePathMarker = "//corvint:fixture-path"

// fixtureLines returns the [start, end) byte range of each line of body holding a line comment
// that is exactly fixturePathMarker: an occurrence counts only where stripComments blanked it, so
// the same text inside a literal marks nothing. clean keeps body's offsets.
func fixtureLines(body, clean string) [][2]int {
	var lines [][2]int
	for offset := 0; ; {
		index := strings.Index(body[offset:], fixturePathMarker)
		if index < 0 {
			return lines
		}
		index += offset
		offset = index + len(fixturePathMarker)
		end := offset + strings.IndexAny(body[offset:], "\r\n")
		if end < offset {
			end = len(body)
		}
		if clean[index] == ' ' && strings.TrimSpace(body[offset:end]) == "" {
			start := strings.LastIndexAny(body[:index], "\r\n") + 1
			lines = append(lines, [2]int{start, end})
		}
	}
}

// onMarkedLine reports whether offset lies on a line fixtureLines returned.
func onMarkedLine(lines [][2]int, offset int) bool {
	for _, line := range lines {
		if offset >= line[0] && offset < line[1] {
			return true
		}
	}
	return false
}
