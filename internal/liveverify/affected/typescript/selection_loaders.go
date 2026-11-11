// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

// FrontierMockTarget reports a jest.mock, jest.doMock, vi.mock or vi.doMock
// call whose target is not a string literal (SLF-V1-004): the module it
// replaces, and so which tests observe the mock, is not visible in source
// text. It is DISCOVERY at the calling file.
const FrontierMockTarget = "typescript:mock-target-unresolved"

// globExpansionBound is the plan's path budget for one glob call (SLF-V1-003):
// a call matching more owned files is treated as computed.
const globExpansionBound = 1024

// Loader call kinds an import scan records beside plain specifiers.
const (
	loaderGlob    = "glob"    // import.meta.glob or import.meta.globEager
	loaderContext = "context" // require.context
	loaderMock    = "mock"    // jest.mock, jest.doMock, vi.mock, vi.doMock
)

// loaderCall is one glob or mock call an import scan found in code position:
// its kind, the offset of its callee, its top-level argument texts, and
// whether its arguments could not be read at all.
type loaderCall struct {
	kind     string
	offset   int
	args     []string
	unparsed bool
}

// metaGlobAt reports an `import.meta.glob` or `import.meta.globEager` callee
// at index, which holds the `import` keyword, and the offset past its name.
func metaGlobAt(body string, index int) (int, bool) {
	at := index + len("import")
	for _, word := range []string{".", "meta", ".", "glob"} {
		at = skipSpace(body, at)
		if !strings.HasPrefix(body[at:], word) {
			return 0, false
		}
		at += len(word)
	}
	if strings.HasPrefix(body[at:], "Eager") {
		at += len("Eager")
	}
	if at < len(body) && isIdentifier(body[at]) {
		return 0, false
	}
	return at, true
}

// memberCallAt reports `object.member` at index for object (not preceded by an
// identifier byte or `.`) and one of members, and the offset past the member.
func memberCallAt(body string, index int, object string, members ...string) (int, bool) {
	if !strings.HasPrefix(body[index:], object) || index > 0 && (isIdentifier(body[index-1]) || body[index-1] == '.') {
		return 0, false
	}
	at := skipSpace(body, index+len(object))
	if at == len(body) || body[at] != '.' {
		return 0, false
	}
	at = skipSpace(body, at+1)
	for _, member := range members {
		end := at + len(member)
		if strings.HasPrefix(body[at:], member) && (end == len(body) || !isIdentifier(body[end])) {
			return end, true
		}
	}
	return 0, false
}

// loaderCallee reports a `require.context` callee or a `jest.mock`,
// `jest.doMock`, `vi.mock` or `vi.doMock` callee at index, its kind and the
// offset past it.
func loaderCallee(body string, index int) (int, string, bool) {
	switch body[index] {
	case 'r':
		if end, found := memberCallAt(body, index, "require", "context"); found {
			return end, loaderContext, true
		}
	case 'j':
		if end, found := memberCallAt(body, index, "jest", "mock", "doMock"); found {
			return end, loaderMock, true
		}
	case 'v':
		if end, found := memberCallAt(body, index, "vi", "mock", "doMock"); found {
			return end, loaderMock, true
		}
	}
	return 0, "", false
}

// loaderCallAt reads the call that follows a loader callee ending at end: an
// optional TypeScript type argument list, then the parenthesised arguments.
// A callee that is not called, or whose arguments do not close, is unparsed.
func loaderCallAt(body string, kind string, offset, end int) loaderCall {
	call := loaderCall{kind: kind, offset: offset, unparsed: true}
	at := skipSpace(body, end)
	if at < len(body) && body[at] == '<' {
		closing := strings.IndexByte(body[at:], '>')
		if closing < 0 {
			return call
		}
		at = skipSpace(body, at+closing+1)
	}
	if at == len(body) || body[at] != '(' {
		return call
	}
	args, ok := callArguments(body, at)
	if !ok {
		return call
	}
	call.args, call.unparsed = args, false
	return call
}

// callArguments splits the arguments of the call whose `(` is at open into
// their trimmed top-level texts. Strings, template literals and regular
// expression literals are skipped whole; a template's substitutions are kept
// in its text, so a reader sees it as computed.
func callArguments(body string, open int) ([]string, bool) {
	var args []string
	depth, start := 0, open+1
	for index := open + 1; index < len(body); {
		switch character := body[index]; {
		case character == '\'' || character == '"':
			index = quotedEnd(body, index)
			if index > len(body) {
				return nil, false
			}
			continue
		case character == '`':
			index = templateEnd(body, index)
			if index > len(body) {
				return nil, false
			}
			continue
		case character == '/':
			if end, regex := regexEnd(body, index); regex {
				index = end
				continue
			}
		case character == '(' || character == '[' || character == '{':
			depth++
		case character == ')' || character == ']' || character == '}':
			if depth == 0 {
				if character != ')' {
					return nil, false
				}
				if last := strings.TrimSpace(body[start:index]); last != "" || len(args) != 0 {
					args = append(args, last)
				}
				return args, true
			}
			depth--
		case character == ',' && depth == 0:
			args = append(args, strings.TrimSpace(body[start:index]))
			start = index + 1
		}
		index++
	}
	return nil, false
}

// stringLiteral reads a whole argument that is one quoted string, or one
// template literal without a substitution, and returns its value.
func stringLiteral(argument string) (string, bool) {
	if len(argument) < 2 {
		return "", false
	}
	quote := argument[0]
	if quote != '\'' && quote != '"' && quote != '`' || argument[len(argument)-1] != quote {
		return "", false
	}
	if quote == '`' {
		if strings.Contains(argument, "${") {
			return "", false
		}
	} else if quotedEnd(argument, 0) != len(argument) {
		return "", false
	}
	inner := argument[1 : len(argument)-1]
	var value strings.Builder
	for index := 0; index < len(inner); index++ {
		if inner[index] != '\\' {
			value.WriteByte(inner[index])
			continue
		}
		index++
		if index == len(inner) {
			return "", false
		}
		switch inner[index] {
		case 'n', 'r', 't', 'b', 'f', 'v', '0', 'x', 'u':
			// An escape that encodes another character is not read.
			return "", false
		default:
			value.WriteByte(inner[index])
		}
	}
	return value.String(), true
}

// stringList reads an argument that is one string literal or an array of
// string literals.
func stringList(argument string) ([]string, bool) {
	if value, ok := stringLiteral(argument); ok {
		return []string{value}, true
	}
	if !strings.HasPrefix(argument, "[") || !strings.HasSuffix(argument, "]") {
		return nil, false
	}
	items, ok := callArguments("("+argument[1:len(argument)-1]+")", 0)
	if !ok {
		return nil, false
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		value, literal := stringLiteral(item)
		if !literal {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
}

// regexLiteral reads an argument that is one regular-expression literal and
// returns it as a Go expression; only the `i` and `g` flags are read.
func regexLiteral(argument string) (*regexp.Regexp, bool) {
	if !strings.HasPrefix(argument, "/") {
		return nil, false
	}
	end, ok := regexLiteralEnd(argument, 0)
	if !ok || end != len(argument) {
		return nil, false
	}
	closing := strings.LastIndexByte(argument, '/')
	source, flags := argument[1:closing], argument[closing+1:]
	prefix := ""
	for _, flag := range flags {
		switch flag {
		case 'i':
			prefix = "(?i)"
		case 'g':
		default:
			return nil, false
		}
	}
	expression, err := regexp.Compile(prefix + source)
	if err != nil {
		return nil, false
	}
	return expression, true
}

// globRegexp translates a glob in the picomatch dialect Vite and Vitest use
// (`*`, `**`, `?`, classes, braces and the `?()`, `*()`, `+()` and `@()`
// extglobs) into an anchored expression. A negated `!()` extglob, or any
// other form it cannot translate, reports false.
func globRegexp(glob string) (*regexp.Regexp, bool) {
	source, ok := globSource(glob)
	if !ok {
		return nil, false
	}
	expression, err := regexp.Compile("^" + source + "$")
	return expression, err == nil
}

func globSource(glob string) (string, bool) {
	var out strings.Builder
	for index := 0; index < len(glob); index++ {
		character := glob[index]
		next := byte(0)
		if index+1 < len(glob) {
			next = glob[index+1]
		}
		switch {
		case strings.IndexByte("?*+@!", character) >= 0 && next == '(':
			closing := matchingClose(glob, index+1, '(', ')')
			if closing < 0 || character == '!' {
				return "", false
			}
			var alternatives []string
			for _, alternative := range splitTopLevel(glob[index+2:closing], '|') {
				translated, ok := globSource(alternative)
				if !ok {
					return "", false
				}
				alternatives = append(alternatives, translated)
			}
			out.WriteString("(?:" + strings.Join(alternatives, "|") + ")")
			switch character {
			case '?':
				out.WriteByte('?')
			case '*':
				out.WriteByte('*')
			case '+':
				out.WriteByte('+')
			}
			index = closing
		case character == '*' && next == '*':
			for index+1 < len(glob) && glob[index+1] == '*' {
				index++
			}
			if index+1 < len(glob) && glob[index+1] == '/' {
				index++
				out.WriteString("(?:.*/)?")
			} else {
				out.WriteString(".*")
			}
		case character == '*':
			out.WriteString("[^/]*")
		case character == '?':
			out.WriteString("[^/]")
		case character == '[':
			closing := strings.IndexByte(glob[index+1:], ']')
			if closing < 0 {
				return "", false
			}
			class := glob[index+1 : index+1+closing]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			out.WriteString("[" + class + "]")
			index += closing + 1
		case character == '{':
			closing := matchingClose(glob, index, '{', '}')
			if closing < 0 {
				return "", false
			}
			var alternatives []string
			for _, alternative := range splitTopLevel(glob[index+1:closing], ',') {
				translated, ok := globSource(alternative)
				if !ok {
					return "", false
				}
				alternatives = append(alternatives, translated)
			}
			out.WriteString("(?:" + strings.Join(alternatives, "|") + ")")
			index = closing
		case character == '\\' && index+1 < len(glob):
			index++
			out.WriteString(regexp.QuoteMeta(glob[index : index+1]))
		default:
			out.WriteString(regexp.QuoteMeta(string(character)))
		}
	}
	return out.String(), true
}

// matchingClose returns the index of the close byte matching the open byte at
// index, or -1.
func matchingClose(text string, index int, open, closing byte) int {
	depth := 0
	for position := index; position < len(text); position++ {
		switch text[position] {
		case open:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return position
			}
		}
	}
	return -1
}

// splitTopLevel splits text at separator outside parentheses, braces and
// classes.
func splitTopLevel(text string, separator byte) []string {
	var parts []string
	depth, start := 0, 0
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
		case separator:
			if depth == 0 {
				parts = append(parts, text[start:index])
				start = index + 1
			}
		}
	}
	return append(parts, text[start:])
}

// globExpansion is what a literal glob call expands to (SLF-V1-003): the
// owned files it matches, as specifiers relative to the importing file, and
// the anchored membership globs the importing unit reads.
type globExpansion struct {
	refs  []string
	globs []string
}

// expandLoader expands one literal glob call of relative against the observed
// files, or reports that the call is computed: an argument that is not a
// literal, a pattern that is not relative or that climbs out of the
// repository, a `base` option, a regular expression or flag the planner
// cannot read, or more matches than globExpansionBound.
func expandLoader(relative string, call loaderCall, files []string, owns func(string) bool) (globExpansion, bool) {
	if call.unparsed || len(call.args) == 0 {
		return globExpansion{}, false
	}
	directory := path.Dir(relative)
	var match func(string) bool
	var globs []string
	switch call.kind {
	case loaderGlob:
		patterns, literal := stringList(call.args[0])
		if !literal || len(patterns) == 0 {
			return globExpansion{}, false
		}
		for _, option := range call.args[1:] {
			if regexp.MustCompile(`\bbase\b`).MatchString(option) {
				return globExpansion{}, false
			}
		}
		var expressions []*regexp.Regexp
		for _, pattern := range patterns {
			if strings.HasPrefix(pattern, "!") {
				// A negated pattern only removes matches; reading none of
				// them keeps the expansion a superset.
				continue
			}
			if !strings.HasPrefix(pattern, "./") && !strings.HasPrefix(pattern, "../") {
				return globExpansion{}, false
			}
			joined, inside := repositoryJoin(directory, pattern)
			if !inside {
				return globExpansion{}, false
			}
			expression, ok := globRegexp(joined)
			if !ok {
				return globExpansion{}, false
			}
			expressions = append(expressions, expression)
			globs = append(globs, membershipGlobs(joined)...)
		}
		match = func(file string) bool {
			for _, expression := range expressions {
				if expression.MatchString(file) {
					return true
				}
			}
			return false
		}
	case loaderContext:
		base, literal := stringLiteral(call.args[0])
		if !literal || len(call.args) > 3 {
			return globExpansion{}, false
		}
		recursive := true
		if len(call.args) > 1 {
			switch call.args[1] {
			case "true":
			case "false":
				recursive = false
			default:
				return globExpansion{}, false
			}
		}
		expression := regexp.MustCompile(`^\./.*$`)
		if len(call.args) > 2 {
			var ok bool
			if expression, ok = regexLiteral(call.args[2]); !ok {
				return globExpansion{}, false
			}
		}
		root, inside := repositoryJoin(directory, base)
		if !inside || strings.ContainsAny(root, "*?[{") {
			return globExpansion{}, false
		}
		if recursive {
			globs = append(globs, anchoredDirectory(root))
		} else {
			globs = append(globs, anchoredDirectory(root)+"/*")
		}
		match = func(file string) bool {
			if !inside || root != "." && !strings.HasPrefix(file, root+"/") {
				return false
			}
			rest := strings.TrimPrefix(file, root+"/")
			if !recursive && strings.Contains(rest, "/") {
				return false
			}
			return expression.MatchString("./" + rest)
		}
	default:
		return globExpansion{}, false
	}
	expansion := globExpansion{}
	climb := strings.Repeat("../", strings.Count(directory, "/")+1)
	if directory == "." {
		climb = ""
	}
	for _, file := range files {
		if file == relative || !owns(file) || !match(file) {
			continue
		}
		if len(expansion.refs) == globExpansionBound {
			return globExpansion{}, false
		}
		expansion.refs = append(expansion.refs, "./"+climb+file)
	}
	expansion.globs = globs
	return expansion, true
}

// repositoryJoin joins a relative specifier to directory and reports whether
// the result stays inside the repository.
func repositoryJoin(directory, specifier string) (string, bool) {
	joined := path.Join(directory, specifier)
	return joined, joined != ".." && !strings.HasPrefix(joined, "../") && !strings.HasPrefix(joined, "/")
}

// membershipGlobs returns the anchored PathGlobs entries that read the
// membership of a repository-relative glob: the glob itself when path.Match
// can read every component (braces expanded), else its literal directory,
// whose every descendant then reselects the importer.
func membershipGlobs(glob string) []string {
	var expanded []string
	if opening := strings.IndexByte(glob, '{'); opening >= 0 {
		closing := matchingClose(glob, opening, '{', '}')
		if closing < 0 {
			return []string{anchoredDirectory(literalDirectory(glob))}
		}
		for _, alternative := range splitTopLevel(glob[opening+1:closing], ',') {
			expanded = append(expanded, membershipGlobs(glob[:opening]+alternative+glob[closing+1:])...)
		}
		return expanded
	}
	if strings.Contains(glob, "**") || strings.ContainsAny(glob, "()") {
		return []string{anchoredDirectory(literalDirectory(glob))}
	}
	for _, component := range strings.Split(glob, "/") {
		if _, err := path.Match(component, ""); err != nil {
			return []string{anchoredDirectory(literalDirectory(glob))}
		}
	}
	return []string{"/" + glob}
}

// literalDirectory is the leading run of glob components with no wildcard.
func literalDirectory(glob string) string {
	var literal []string
	for _, component := range strings.Split(glob, "/") {
		if strings.ContainsAny(component, "*?[{(!") {
			break
		}
		literal = append(literal, component)
	}
	if len(literal) == len(strings.Split(glob, "/")) {
		literal = literal[:len(literal)-1]
	}
	if len(literal) == 0 {
		return "."
	}
	return strings.Join(literal, "/")
}

// anchoredDirectory is the anchored glob for a directory's subtree; for the
// repository root it is `/*`, which every path matches.
func anchoredDirectory(directory string) string {
	if directory == "." || directory == "" {
		return "/*"
	}
	return "/" + directory
}

// mockIndex maps manual mocks to the modules they replace (SLF-V1-004): adjacent
// maps each source unit to the units of the files under the `__mocks__`
// directory beside it, and packages maps a bare specifier to the units of the
// files a `__mocks__` directory holds under that name, with the directory that
// holds that `__mocks__` directory.
type mockIndex struct {
	adjacent map[string][]string
	packages map[string][]packageMock
}

type packageMock struct {
	directory string
	id        string
}

// manualMocks indexes every observed file under a `__mocks__` directory.
func manualMocks(pathToID map[string]string) mockIndex {
	index := mockIndex{adjacent: map[string][]string{}, packages: map[string][]packageMock{}}
	paths := make([]string, 0, len(pathToID))
	for relative := range pathToID {
		paths = append(paths, relative)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		components := strings.Split(relative, "/")
		at := -1
		for position, component := range components[:len(components)-1] {
			if component == "__mocks__" {
				at = position
			}
		}
		if at < 0 {
			continue
		}
		mock := pathToID[relative]
		directory := strings.Join(components[:at], "/")
		if directory == "" {
			directory = "."
		}
		name := strings.Join(components[at+1:], "/")
		stem := strings.TrimSuffix(name, path.Ext(name))
		module := path.Join(directory, stem)
		for _, candidate := range moduleCandidates(module) {
			if target := pathToID[candidate]; target != "" && target != mock {
				index.adjacent[target] = append(index.adjacent[target], mock)
			}
		}
		packageName := strings.TrimSuffix(stem, "/index")
		index.packages[packageName] = append(index.packages[packageName], packageMock{directory: directory, id: mock})
	}
	return index
}

// add adds to edges, for a unit whose file is owner and whose specifiers are
// refs, the manual mocks of every module it imports and of every bare package
// it names whose `__mocks__` directory lies in owner's directory or an
// ancestor of it.
func (index mockIndex) add(owner string, refs map[string]bool, edges map[string]bool, self string) {
	if len(index.adjacent) == 0 && len(index.packages) == 0 {
		return
	}
	for _, target := range sortedKeys(edges) {
		for _, mock := range index.adjacent[target] {
			if mock != self {
				edges[mock] = true
			}
		}
	}
	for ref := range refs {
		if strings.HasPrefix(ref, ".") || strings.HasPrefix(ref, "/") {
			continue
		}
		name := strings.TrimPrefix(ref, "node:")
		for _, mock := range index.packages[name] {
			if mock.id != self && (mock.directory == "." || inside(owner, mock.directory)) {
				edges[mock.id] = true
			}
		}
	}
}

// manualMockPath reports a file under a `__mocks__` directory.
func manualMockPath(relative string) bool {
	return strings.HasPrefix(relative, "__mocks__/") || strings.Contains(relative, "/__mocks__/")
}

// mockSiteToken is the token a computed mock target names.
func mockSiteToken(call loaderCall) string {
	if len(call.args) == 0 {
		return ""
	}
	return sourceToken(call.args[0])
}
