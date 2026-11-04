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

// anchorWords returns the sorted, unique lexicon words of a module's string, template-text and
// regular-expression contents that are no path token (pathShaped rejects them: no slash or dot,
// or no letter or digit), and whether the plugin certifies the module's anchored tokens
// (affected.Unit.AnchorsPlaced, AFP-V0-2245). With the words, every component a literal of the
// module could join before an anchored token is known, as the Go plugin's lexicon keeps every
// word. A module is not certified when its code may root such a token at a directory the
// plugin does not record: it names `chdir` (other than as an object key, `{chdir: ...}`), a
// shell directory variable (`PWD`, `OLDPWD`, `INIT_CWD`, in code or in a literal), the
// process arguments (`argv`), or a module resolver (`require.resolve`, `import.meta.resolve`,
// `createRequire`). `process.cwd()`, `path.resolve()` and relative paths start in a runner
// directory, which the plugin records (runDirectories). A module that does not lex, or whose
// words exceed maxPathTokens, is not certified either.
func anchorWords(body string) ([]string, bool) {
	clean, err := stripComments(body, false)
	if err != nil {
		return nil, false
	}
	lexed, ok := playwrightLex(clean)
	if !ok {
		return nil, false
	}
	words := map[string]bool{}
	for start := 0; start < len(clean); {
		if lexed.code[start] {
			if identifierByte(clean[start]) && (start == 0 || !identifierByte(clean[start-1]) || !lexed.code[start-1]) {
				end := start
				for end < len(clean) && lexed.code[end] && identifierByte(clean[end]) {
					end++
				}
				if unrecordedDirectory(clean, lexed.code, start, end) {
					return nil, false
				}
				start = end
				continue
			}
			start++
			continue
		}
		end := start
		for end < len(clean) && !lexed.code[end] {
			end++
		}
		literal := clean[start:end]
		if literal[0] == '/' {
			literal = strings.TrimSuffix(literal[1:], "/")
		}
		decoded, _ := decodeStringEscapes(literal)
		if trimmed := strings.Trim(decoded, "'\"`"); trimmed == "process" || trimmed == "node:process" {
			// The process module imported by name: its cwd, chdir and argv escape the
			// allowlist of unrecordedDirectory.
			return nil, false
		}
		for _, word := range pathLexicon.FindAllString(decoded, -1) {
			if shellDirectoryNames[word] {
				return nil, false
			}
			if !pathShaped(word) {
				words[word] = true
			}
		}
		start = end
	}
	if len(words) > maxPathTokens {
		return nil, false
	}
	if len(words) == 0 {
		return nil, true
	}
	values := make([]string, 0, len(words))
	for word := range words {
		values = append(values, word)
	}
	sort.Strings(values)
	return values, true
}

// shellDirectoryNames are the environment variables that hold a directory the shell or the
// package manager chose, which need not be the runner's working directory.
var shellDirectoryNames = map[string]bool{"PWD": true, "OLDPWD": true, "INIT_CWD": true}

// processMembers are the members of `process` that reach no directory: everything else
// (`chdir`, `argv`, a computed member, the object itself passed or destructured) refuses.
var processMembers = map[string]bool{
	"platform": true, "arch": true, "version": true, "versions": true, "exit": true, "exitCode": true,
	"stdout": true, "stderr": true, "stdin": true, "on": true, "once": true, "off": true, "nextTick": true,
	"hrtime": true, "pid": true, "ppid": true, "execPath": true, "execArgv": true, "memoryUsage": true,
	"emitWarning": true, "kill": true, "release": true, "uptime": true, "title": true,
}

// unrecordedDirectory reports a code identifier at clean[start:end] through which a module may
// reach a directory the plugin does not record (anchorWords). It is an allowlist:
// `process` may appear only as `process.cwd()` (the runner's directory), `process.env.NAME` or
// `process.env['NAME']` for a NAME no shell directory variable, or a member that reaches no
// directory; `require` only as a call of a literal or a `require.resolve` of a `./` or `../`
// literal; `import` only as a static import, a dynamic import of a literal, `import.meta.url`,
// `dirname` or `filename`, or an `import.meta.resolve` of a `./` or `../` literal. A shell
// directory variable, `createRequire`, `Deno`, `Bun` and a computed `globalThis` or `global`
// member refuse.
func unrecordedDirectory(clean string, code []bool, start, end int) bool {
	name := clean[start:end]
	switch {
	case shellDirectoryNames[name], name == "createRequire", name == "Deno", name == "Bun":
		return true
	case name == "globalThis" || name == "global":
		at := nextCode(clean, code, end)
		return at < len(clean) && clean[at] == '['
	case name == "process":
		at := nextCode(clean, code, end)
		if at >= len(clean) || clean[at] != '.' {
			return true
		}
		member, after := codeIdentifierAt(clean, code, nextCode(clean, code, at+1))
		switch {
		case member == "cwd":
			next := nextCode(clean, code, after)
			return next >= len(clean) || clean[next] != '('
		case member == "env":
			next := nextCode(clean, code, after)
			if next < len(clean) && clean[next] == '.' {
				variable, _ := codeIdentifierAt(clean, code, nextCode(clean, code, next+1))
				return variable == "" || shellDirectoryNames[variable]
			}
			if next < len(clean) && clean[next] == '[' {
				literal, ok := literalAt(clean, code, nextCode(clean, code, next+1))
				return !ok || shellDirectoryNames[literal]
			}
			return true
		}
		return !processMembers[member]
	case name == "require":
		at := nextCode(clean, code, end)
		if at < len(clean) && clean[at] == '(' {
			_, ok := literalAt(clean, code, nextCode(clean, code, at+1))
			return !ok
		}
		if at < len(clean) && clean[at] == '.' {
			member, after := codeIdentifierAt(clean, code, nextCode(clean, code, at+1))
			return member != "resolve" || !moduleRelativeArgument(clean, code, after)
		}
		return true
	case name == "import":
		at := nextCode(clean, code, end)
		if at < len(clean) && clean[at] == '(' {
			_, ok := literalAt(clean, code, nextCode(clean, code, at+1))
			return !ok
		}
		if at < len(clean) && clean[at] == '.' {
			meta, after := codeIdentifierAt(clean, code, nextCode(clean, code, at+1))
			dot := nextCode(clean, code, after)
			if meta != "meta" || dot >= len(clean) || clean[dot] != '.' {
				return true
			}
			member, rest := codeIdentifierAt(clean, code, nextCode(clean, code, dot+1))
			switch member {
			case "url", "dirname", "filename":
				return false
			case "resolve":
				return !moduleRelativeArgument(clean, code, rest)
			}
			return true
		}
		return false
	}
	return false
}

// nextCode is the index of the first code byte at or after at that is no space.
func nextCode(clean string, code []bool, at int) int {
	for at < len(clean) && code[at] && strings.IndexByte(" \t\r\n", clean[at]) >= 0 {
		at++
	}
	return at
}

// codeIdentifierAt returns the code identifier starting at at, and the index after it.
func codeIdentifierAt(clean string, code []bool, at int) (string, int) {
	end := at
	for end < len(clean) && code[end] && identifierByte(clean[end]) {
		end++
	}
	return clean[at:end], end
}

// literalAt returns the decoded content of a string literal starting at at that is followed,
// past spaces, by `)` or `]`, so it is the whole argument or index.
func literalAt(clean string, code []bool, at int) (string, bool) {
	if at >= len(clean) || code[at] || strings.IndexByte(`'"`+"`", clean[at]) < 0 {
		return "", false
	}
	end := at
	for end < len(clean) && !code[end] {
		end++
	}
	literal := clean[at:end]
	if literal[0] == '`' && strings.Contains(literal, "${") {
		return "", false
	}
	next := nextCode(clean, code, end)
	if next >= len(clean) || (clean[next] != ')' && clean[next] != ']') {
		return "", false
	}
	content, _ := decodeStringEscapes(strings.Trim(literal, "'\"`"))
	return content, true
}

func identifierByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// moduleRelativeArgument reports a call at clean[at:] whose first argument is a literal that
// starts with `./` or `../`: a module resolver then resolves it against the module's own
// directory, which is already a base.
func moduleRelativeArgument(clean string, code []bool, at int) bool {
	at = nextCode(clean, code, at)
	if at >= len(clean) || !code[at] || clean[at] != '(' {
		return false
	}
	content, ok := literalAt(clean, code, nextCode(clean, code, at+1))
	return ok && (strings.HasPrefix(content, "./") || strings.HasPrefix(content, "../"))
}
