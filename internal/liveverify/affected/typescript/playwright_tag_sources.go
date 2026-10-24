package typescript

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// playwrightTagSources decides whether the tags a listing reports for a test file are exactly
// the tags of its grep titles, from the config, the test file and their repository-relative static
// import closure. Playwright reports a config or describe tag only as its `@\S+` runs, so the
// listing alone cannot show a tag that contains whitespace or is `@` alone; when every tag in the
// closure is a literal `@\S+` string the listing hides nothing. The literal tags are returned so the
// caller can also require each reported tag to be one of them (AFP-V0-409, AFP-V0-410).
type playwrightTagSources struct {
	root  string
	files map[string]playwrightTagFile
}

type playwrightTagFile struct {
	exact   bool
	imports []string
	tags    []string
}

func newPlaywrightTagSources(root string) *playwrightTagSources {
	return &playwrightTagSources{root: root, files: map[string]playwrightTagFile{}}
}

// exact returns the literal tags of every file reachable from entries when each passes
// playwrightTagFileExact, or nil when one does not.
func (sources *playwrightTagSources) exact(entries ...string) map[string]bool {
	tags := map[string]bool{}
	seen := map[string]bool{}
	queue := slices.Clone(entries)
	for len(queue) != 0 {
		file := queue[0]
		queue = queue[1:]
		if seen[file] {
			continue
		}
		seen[file] = true
		checked, ok := sources.files[file]
		if !ok {
			checked = playwrightTagFileExact(sources.root, file)
			sources.files[file] = checked
		}
		if !checked.exact {
			return nil
		}
		for _, tag := range checked.tags {
			tags[tag] = true
		}
		queue = append(queue, checked.imports...)
	}
	return tags
}

// playwrightTagNeutralModules are bare modules whose import gives a case no tag.
var playwrightTagNeutralModules = map[string]bool{"@playwright/test": true, "dotenv": true, "dotenv/config": true, "path": true, "os": true, "url": true, "fs": true}

// playwrightTagIndirection are globals through which a module can build or rename a property at
// run time, so a tag could exist without the text `tag` in source.
var playwrightTagIndirection = []string{"Object", "Reflect", "JSON", "Proxy", "eval", "Function", "globalThis", "structuredClone"}

// playwrightTagFileExact checks one file. Its static loads must all resolve: a bare module other
// than @playwright/test, dotenv or a neutral Node built-in, an unresolved relative import, or a
// dynamic load is not exact. Every `tag` in its code must be an object-literal property key whose
// value is a literal string or an array of literal strings, each `@` followed by one or more
// non-whitespace characters without an escape. A string literal "tag", a computed object key, a
// computed member assignment or access or a property-building global is not exact (AFP-V0-409).
func playwrightTagFileExact(root, relative string) playwrightTagFile {
	body, err := affected.ReadSource(root, relative)
	if err != nil || !utf8.Valid(body) {
		return playwrightTagFile{}
	}
	scan := scanImportSites(relative, string(body))
	if scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 || scan.syntax >= 0 {
		return playwrightTagFile{}
	}
	result := playwrightTagFile{exact: true}
	for _, ref := range scan.refs {
		if playwrightTagNeutralModules[ref] || strings.HasPrefix(ref, "node:") {
			continue
		}
		resolved, ok := playwrightResolveRelative(root, relative, ref)
		if !ok {
			return playwrightTagFile{}
		}
		result.imports = append(result.imports, resolved)
	}
	clean, err := stripComments(string(body), jsxSource(relative))
	if err != nil {
		return playwrightTagFile{}
	}
	if result.tags, result.exact = playwrightTagTextLiteral(clean); !result.exact {
		return playwrightTagFile{}
	}
	return result
}

// playwrightResolveRelative resolves a relative static import of owner to an existing source file
// inside root, trying the same candidates the import graph does.
func playwrightResolveRelative(root, owner, ref string) (string, bool) {
	clean := strings.SplitN(strings.SplitN(ref, "?", 2)[0], "#", 2)[0]
	if !strings.HasPrefix(clean, ".") {
		return "", false
	}
	base := path.Clean(path.Join(path.Dir(owner), clean))
	if base == "." || base == ".." || strings.HasPrefix(base, "../") {
		return "", false
	}
	for _, candidate := range moduleCandidates(base) {
		if !affected.ValidRelativePath(candidate) {
			continue
		}
		_, err := affected.ReadSource(root, candidate)
		if err == nil {
			return candidate, true
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", false
		}
	}
	return "", false
}

// playwrightTagTextLiteral applies the per-file text rules of playwrightTagFileExact to a
// comment-stripped module and returns its literal tags.
func playwrightTagTextLiteral(clean string) ([]string, bool) {
	lexed, ok := playwrightLex(clean)
	if !ok {
		return nil, false
	}
	for _, name := range playwrightTagIndirection {
		for _, at := range playwrightTokenOffsets(clean, name) {
			if lexed.code[at] {
				return nil, false
			}
		}
	}
	for _, quoted := range []string{"'tag'", `"tag"`, "`tag`"} {
		if strings.Contains(clean, quoted) {
			return nil, false
		}
	}
	for index := 0; index < len(clean); index++ {
		if !lexed.code[index] {
			continue
		}
		switch clean[index] {
		case '[':
			previous := playwrightPreviousCode(clean, index)
			closeAt, closed := lexed.closes[index]
			if !closed {
				return nil, false
			}
			next := playwrightNextCode(clean, closeAt+1)
			if previous >= 0 && (clean[previous] == '{' || clean[previous] == ',') && next < len(clean) && (clean[next] == ':' || clean[next] == '(') {
				return nil, false // computed object key or method
			}
			if playwrightMemberAccessAt(clean, previous) && !playwrightLiteralIndex(clean[index+1:closeAt]) {
				return nil, false // computed member access, which can reach a tag array without naming it
			}
		case ']':
			// `T[] =` is a type annotation; any other `] =` writes a computed member.
			if previous := playwrightPreviousCode(clean, index); (previous < 0 || clean[previous] != '[') && playwrightAssignmentAt(clean, playwrightNextCode(clean, index+1)) {
				return nil, false // computed member write
			}
		}
	}
	tags := []string{}
	for _, at := range playwrightTokenOffsets(clean, "tag") {
		if !lexed.code[at] {
			continue
		}
		literal, ok := playwrightLiteralTagProperty(clean, lexed, at)
		if !ok {
			return nil, false
		}
		tags = append(tags, literal...)
	}
	return tags, true
}

// playwrightMemberAccessAt reports whether a `[` whose previous code byte is at previous opens a
// member access rather than an array literal: it follows an identifier other than a keyword that
// precedes an expression, a `)`, a `]` or the `.` of `?.`.
func playwrightMemberAccessAt(clean string, previous int) bool {
	if previous < 0 {
		return false
	}
	switch character := clean[previous]; {
	case character == ')' || character == ']' || character == '.':
		return true
	case !playwrightIdentifierByte(character):
		return false
	}
	start := previous
	for start > 0 && playwrightIdentifierByte(clean[start-1]) {
		start--
	}
	return !slices.Contains(playwrightExpressionKeywords, clean[start:previous+1])
}

// playwrightExpressionKeywords are the keywords after which `[` starts an array literal.
var playwrightExpressionKeywords = []string{"return", "of", "in", "typeof", "instanceof", "case", "yield", "await", "else", "do", "void", "delete", "throw", "new", "default", "extends"}

func playwrightIdentifierByte(character byte) bool {
	return character == '_' || character == '$' || character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= 0x80
}

// playwrightLiteralIndex reports whether a member access index is empty (a type such as
// `string[]`), a decimal integer or one escape-free string literal.
func playwrightLiteralIndex(raw string) bool {
	raw = strings.Trim(raw, " \t\r\n")
	if raw == "" || strings.Trim(raw, "0123456789") == "" {
		return true
	}
	_, literal := playwrightString(raw)
	return literal && !strings.ContainsRune(raw, '\\')
}

// playwrightNextCode returns the index of the first non-space byte at or after index.
func playwrightNextCode(raw string, index int) int {
	for ; index < len(raw) && strings.IndexByte(" \t\r\n", raw[index]) >= 0; index++ {
	}
	return index
}

// playwrightAssignmentAt reports whether an assignment operator (=, +=, ||=, ??=, &&= and the
// other compound forms) starts at index.
func playwrightAssignmentAt(raw string, index int) bool {
	rest := raw[min(index, len(raw)):]
	for _, operator := range []string{"**=", "<<=", ">>>=", ">>=", "||=", "&&=", "??=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^="} {
		if strings.HasPrefix(rest, operator) {
			return true
		}
	}
	return strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==") && !strings.HasPrefix(rest, "=>")
}

// playwrightLiteralTagProperty reports whether the `tag` token at offset is a key of its enclosing
// object literal whose value is one or more literal `@\S+` strings, and returns them.
func playwrightLiteralTagProperty(clean string, lexed playwrightLexed, at int) ([]string, bool) {
	previous := playwrightPreviousCode(clean, at)
	next := playwrightNextCode(clean, at+len("tag"))
	if previous < 0 || clean[previous] != '{' && clean[previous] != ',' || next >= len(clean) || clean[next] != ':' {
		return nil, false
	}
	open := lexed.parent[at]
	if open < 0 || clean[open] != '{' {
		return nil, false
	}
	closeAt, closed := lexed.closes[open]
	if !closed {
		return nil, false
	}
	properties, ok := playwrightObjectProperties(clean[open : closeAt+1])
	value, found := properties["tag"]
	if !ok || !found {
		return nil, false
	}
	items, array := playwrightArrayItems(value)
	if !array {
		items = []string{value}
	}
	tags := make([]string, 0, len(items))
	for _, item := range items {
		tag, literal := playwrightString(item)
		if !literal || strings.ContainsRune(item, '\\') || len(tag) < 2 || tag[0] != '@' || strings.ContainsFunc(tag, playwrightJSSpace) {
			return nil, false
		}
		tags = append(tags, tag)
	}
	return tags, len(tags) != 0
}
