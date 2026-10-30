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
// a module read whole because it does not tokenize keeps every token.
func pathTokensSkipping(body string, skip map[int]string) ([]string, bool) {
	tokens := map[string]bool{}
	add := func(text string) {
		decoded, _ := decodeStringEscapes(text)
		for _, token := range pathLexicon.FindAllString(decoded, -1) {
			if pathShaped(token) {
				tokens[token] = true
			}
		}
	}
	clean, err := stripComments(body, false)
	lexed, ok := playwrightLex(clean)
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
			add(literal)
			start = end
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
