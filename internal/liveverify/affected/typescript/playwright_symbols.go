package typescript

import (
	"errors"
	"io/fs"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Changed-symbol states (TJAA-V0-033).
const (
	PlaywrightSymbolsResolved   = "RESOLVED"
	PlaywrightSymbolsUnresolved = "UNRESOLVED"

	playwrightSymbolMaxBytes = 1 << 20
	playwrightSymbolMaxPaths = 64
)

// PlaywrightBaseReader returns a repository-relative path's content at the comparison base;
// exists is false when the path is absent there.
type PlaywrightBaseReader func(relative string) (body []byte, exists bool, err error)

// PlaywrightSymbolChange is a syntactic comparison of one dirty module's top-level statements.
// AdditiveOnly says the current statements are the base statements with only new named export
// declarations inserted; it says nothing about the load-time effects of those declarations and
// never narrows a selection (TJAA-V0-033).
type PlaywrightSymbolChange struct {
	Added        []string `json:"added"`
	AdditiveOnly bool     `json:"additiveOnly"`
	Detail       string   `json:"detail,omitzero"`
	Modified     []string `json:"modified"`
	OtherChanged bool     `json:"otherChanged"`
	Path         string   `json:"path"`
	Removed      []string `json:"removed"`
	State        string   `json:"state"`
}

// playwrightChangedSymbols compares every owned dirty path, at most playwrightSymbolMaxPaths of
// them; the rest and every path without a readable base are UNRESOLVED.
func playwrightChangedSymbols(root string, dirty []string, before PlaywrightBaseReader) []PlaywrightSymbolChange {
	changes := []PlaywrightSymbolChange{}
	language := New()
	for _, relative := range dirty {
		if !language.Owns(relative) {
			continue
		}
		change := unresolvedSymbols(relative, "")
		switch {
		case len(changes) >= playwrightSymbolMaxPaths:
			change.Detail = "more owned dirty paths than the symbol bound"
		case before == nil:
			change.Detail = "no comparison base was read"
		default:
			change = readChangedSymbols(root, relative, before)
		}
		changes = append(changes, change)
	}
	return changes
}

func readChangedSymbols(root, relative string, before PlaywrightBaseReader) PlaywrightSymbolChange {
	old, oldExists, err := before(relative)
	if err != nil {
		return unresolvedSymbols(relative, "comparison base content is unreadable")
	}
	current, err := affected.ReadSource(root, relative)
	currentExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return unresolvedSymbols(relative, "current content is unreadable")
	}
	return ChangedSymbols(relative, old, oldExists, current, currentExists)
}

func unresolvedSymbols(relative, detail string) PlaywrightSymbolChange {
	return PlaywrightSymbolChange{Added: []string{}, Detail: detail, Modified: []string{}, Path: relative, Removed: []string{}, State: PlaywrightSymbolsUnresolved}
}

// ChangedSymbols compares two versions of one module by top-level statement. A statement starts
// at a top-level line beginning with a declaration, import or export keyword; a named export
// statement is keyed by the names it exports (TJAA-V0-033).
func ChangedSymbols(relative string, before []byte, beforeExists bool, after []byte, afterExists bool) PlaywrightSymbolChange {
	if jsxSource(relative) {
		return unresolvedSymbols(relative, "JSX syntax is outside the statement segmenter")
	}
	if len(before) > playwrightSymbolMaxBytes || len(after) > playwrightSymbolMaxBytes {
		return unresolvedSymbols(relative, "content exceeds the symbol byte bound")
	}
	var old, current []symbolStatement
	if beforeExists {
		statements, ok := symbolStatements(before)
		if !ok {
			return unresolvedSymbols(relative, "comparison base content does not tokenize")
		}
		old = statements
	}
	if afterExists {
		statements, ok := symbolStatements(after)
		if !ok {
			return unresolvedSymbols(relative, "current content does not tokenize")
		}
		current = statements
	}
	oldNames, newNames := exportedStatements(old), exportedStatements(current)
	change := PlaywrightSymbolChange{Added: []string{}, Modified: []string{}, Path: relative, Removed: []string{}, State: PlaywrightSymbolsResolved}
	added := map[string]bool{}
	for _, name := range sortedKeys(newNames) {
		previous, existed := oldNames[name]
		switch {
		case !existed:
			change.Added = append(change.Added, name)
			added[name] = true
		case !slices.Equal(previous, newNames[name]):
			change.Modified = append(change.Modified, name)
		}
	}
	for _, name := range sortedKeys(oldNames) {
		if _, kept := newNames[name]; !kept {
			change.Removed = append(change.Removed, name)
		}
	}
	change.OtherChanged = !slices.Equal(otherStatements(old), otherStatements(current))
	remaining := []string{}
	for _, statement := range current {
		if len(statement.names) != 0 && allIn(statement.names, added) {
			continue
		}
		remaining = append(remaining, statement.text)
	}
	change.AdditiveOnly = slices.Equal(statementTexts(old), remaining)
	return change
}

type symbolStatement struct {
	text  string
	names []string
	// bound holds, for a variable statement binding several names, each
	// name's own declarator text, so a change to one binding changes only
	// that name (TJAA-V0-761).
	bound map[string]string
}

var symbolStatementKeywords = map[string]bool{
	"abstract": true, "async": true, "class": true, "const": true, "declare": true, "enum": true,
	"export": true, "function": true, "import": true, "interface": true, "let": true,
	"module": true, "namespace": true, "type": true, "var": true,
}

// symbolStatements splits a module at top-level lines that begin with a statement keyword. Code
// whitespace collapses to one space, so comment and layout edits compare equal; string and
// template content is kept exactly.
func symbolStatements(body []byte) ([]symbolStatement, bool) {
	if !utf8.Valid(body) {
		return nil, false
	}
	clean, err := stripComments(string(body), false)
	if err != nil {
		return nil, false
	}
	lexed, ok := playwrightLex(clean)
	if !ok {
		return nil, false
	}
	starts := []int{0}
	lineStart := true
	for index := 0; index < len(clean); index++ {
		character := clean[index]
		if character == '\n' && lexed.code[index] {
			lineStart = true
			continue
		}
		if character == ' ' || character == '\t' || character == '\r' {
			continue
		}
		if lineStart && index > 0 && lexed.topLevel(index) && symbolStatementKeywords[identifierAt(clean, index)] {
			starts = append(starts, index)
		}
		lineStart = false
	}
	starts = append(starts, len(clean))
	statements := []symbolStatement{}
	for index := 0; index+1 < len(starts); index++ {
		text := normalizedStatement(clean, lexed, starts[index], starts[index+1])
		if text == "" {
			continue
		}
		statement := symbolStatement{text: text, names: exportNames(text)}
		if bound := variableBindings(text); len(bound) > 1 {
			statement.names, statement.bound = sortedKeys(bound), bound
		}
		statements = append(statements, statement)
	}
	return statements, true
}

func normalizedStatement(clean string, lexed playwrightLexed, start, end int) string {
	var builder strings.Builder
	space := false
	for index := start; index < end; index++ {
		character := clean[index]
		if lexed.code[index] && (character == ' ' || character == '\t' || character == '\r' || character == '\n') {
			space = true
			continue
		}
		if space && builder.Len() != 0 {
			builder.WriteByte(' ')
		}
		space = false
		builder.WriteByte(character)
	}
	return builder.String()
}

func identifierAt(text string, index int) string {
	end := index
	for end < len(text) && isIdentifier(text[end]) {
		end++
	}
	return text[index:end]
}

// exportNames returns the names a named export statement binds, or none for any other statement
// and for an export form it does not model (`export =`, destructuring, `export {}`).
func exportNames(text string) []string {
	if identifierAt(text, 0) != "export" {
		return nil
	}
	rest := strings.TrimSpace(text[len("export"):])
	word := identifierAt(rest, 0)
	switch {
	case word == "default":
		return []string{"default"}
	case strings.HasPrefix(rest, "*"):
		rest = strings.TrimSpace(rest[1:])
		if identifierAt(rest, 0) == "as" {
			if name := identifierAt(strings.TrimSpace(rest[2:]), 0); name != "" {
				return []string{name}
			}
			return nil
		}
		return []string{"*"}
	case word == "type" && strings.HasPrefix(strings.TrimSpace(rest[4:]), "{"):
		return exportListNames(strings.TrimSpace(rest[4:]))
	case strings.HasPrefix(rest, "{"):
		return exportListNames(rest)
	}
	for word == "declare" || word == "async" || word == "abstract" {
		rest = strings.TrimSpace(rest[len(word):])
		word = identifierAt(rest, 0)
	}
	switch word {
	case "const", "let", "var", "function", "class", "type", "interface", "enum", "namespace", "module", "import":
	default:
		return nil
	}
	rest = strings.TrimSpace(rest[len(word):])
	if word == "const" && identifierAt(rest, 0) == "enum" {
		rest = strings.TrimSpace(rest[len("enum"):])
	}
	if word == "function" {
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "*"))
	}
	if name := identifierAt(rest, 0); name != "" {
		return []string{name}
	}
	return nil
}

// variableBindings returns each plain name an exported `const`, `let` or
// `var` statement binds, with its declarator text, splitting at commas
// outside brackets, strings and templates (TJAA-V0-761). A destructuring
// declarator, or any other statement, binds nothing here.
func variableBindings(text string) map[string]string {
	if identifierAt(text, 0) != "export" {
		return nil
	}
	rest := strings.TrimSpace(text[len("export"):])
	if word := identifierAt(rest, 0); word == "declare" {
		rest = strings.TrimSpace(rest[len(word):])
	}
	word := identifierAt(rest, 0)
	if word != "const" && word != "let" && word != "var" {
		return nil
	}
	rest = strings.TrimSpace(rest[len(word):])
	if identifierAt(rest, 0) == "enum" {
		return nil
	}
	bound := map[string]string{}
	depth, quote, start := 0, byte(0), 0
	for index := 0; index <= len(rest); index++ {
		if index < len(rest) {
			character := rest[index]
			switch {
			case quote != 0:
				if character == '\\' {
					index++
				} else if character == quote {
					quote = 0
				}
				continue
			case character == '"' || character == '\'' || character == '`':
				quote = character
				continue
			case character == '(' || character == '[' || character == '{':
				depth++
				continue
			case character == ')' || character == ']' || character == '}':
				depth--
				continue
			case character != ',' || depth != 0:
				continue
			}
		}
		declarator := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest[start:index]), ";"))
		if name := identifierAt(declarator, 0); name != "" {
			bound[name] = word + " " + declarator
		}
		start = index + 1
	}
	return bound
}

func exportListNames(rest string) []string {
	end := strings.IndexByte(rest, '}')
	if end < 0 {
		return nil
	}
	names := []string{}
	for _, item := range strings.Split(rest[1:end], ",") {
		fields := strings.Fields(item)
		if len(fields) != 0 && fields[0] == "type" && len(fields) > 1 {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		names = append(names, fields[len(fields)-1])
	}
	sort.Strings(names)
	return names
}

func exportedStatements(statements []symbolStatement) map[string][]string {
	byName := map[string][]string{}
	for _, statement := range statements {
		for _, name := range statement.names {
			text := statement.text
			if declarator, ok := statement.bound[name]; ok {
				text = declarator
			}
			byName[name] = append(byName[name], text)
		}
	}
	for name := range byName {
		sort.Strings(byName[name])
	}
	return byName
}

func otherStatements(statements []symbolStatement) []string {
	other := []string{}
	for _, statement := range statements {
		if len(statement.names) == 0 {
			other = append(other, statement.text)
		}
	}
	sort.Strings(other)
	return other
}

func statementTexts(statements []symbolStatement) []string {
	texts := []string{}
	for _, statement := range statements {
		texts = append(texts, statement.text)
	}
	return texts
}

func allIn(names []string, set map[string]bool) bool {
	for _, name := range names {
		if !set[name] {
			return false
		}
	}
	return true
}
