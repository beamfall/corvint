// Package extmutate is the external-suite mutation discriminator behind
// `corvint-test e2e discriminate` (CCE-V0-170..176). It mutates the changed
// Ruby and TypeScript/JavaScript application lines of a base..head change with
// a documented, closed operator set, rebuilds the application through a
// caller-declared hook, runs a caller-declared external test command or a
// Corvint JS/Ruby provider, and records killed, survived or not-run per
// (hunk, test), pinned to the application revision, the test revision and the
// digest of every run's receipt. It never decides adequacy: a survivor is a
// survivor of this operator set and these tests only.
package extmutate

import (
	"path"
	"strings"
)

// OperatorSet names the closed operator vocabulary below (CCE-V0-171).
const OperatorSet = "corvint-extmutate-operators/0"

// Language is the operator family a changed file belongs to.
type Language string

const (
	Ruby       Language = "ruby"
	JavaScript Language = "ts-js"
)

// operator is one textual replacement. A site matches only in code (outside
// string literals and comments) and only where the surrounding characters
// keep it the same token.
type operator struct {
	Name        string
	From, To    string
	Languages   []Language
	word        bool // From must be a whole identifier word.
	spaced      bool // From must have a space on both sides.
	leading     bool // From must be the first token of the line.
	notFollowed string
	notPreceded string
}

// operators is ordered so a longer token is tried before its prefix at the
// same position (=== before ==).
var operators = []operator{
	{Name: "strict-eq-negate", From: "===", To: "!==", Languages: []Language{JavaScript}},
	{Name: "strict-ne-negate", From: "!==", To: "===", Languages: []Language{JavaScript}},
	{Name: "eq-negate", From: "==", To: "!=", Languages: []Language{Ruby, JavaScript}, notFollowed: "=~", notPreceded: "=!<>"},
	{Name: "ne-negate", From: "!=", To: "==", Languages: []Language{Ruby, JavaScript}, notFollowed: "="},
	{Name: "le-boundary", From: "<=", To: "<", Languages: []Language{Ruby, JavaScript}, spaced: true},
	{Name: "ge-boundary", From: ">=", To: ">", Languages: []Language{Ruby, JavaScript}, spaced: true},
	{Name: "lt-boundary", From: "<", To: "<=", Languages: []Language{Ruby, JavaScript}, spaced: true},
	{Name: "gt-boundary", From: ">", To: ">=", Languages: []Language{Ruby, JavaScript}, spaced: true},
	{Name: "and-or", From: "&&", To: "||", Languages: []Language{Ruby, JavaScript}, notFollowed: "="},
	{Name: "or-and", From: "||", To: "&&", Languages: []Language{Ruby, JavaScript}, notFollowed: "="},
	{Name: "true-false", From: "true", To: "false", Languages: []Language{Ruby, JavaScript}, word: true},
	{Name: "false-true", From: "false", To: "true", Languages: []Language{Ruby, JavaScript}, word: true},
	{Name: "if-unless", From: "if", To: "unless", Languages: []Language{Ruby}, word: true, leading: true},
	{Name: "unless-if", From: "unless", To: "if", Languages: []Language{Ruby}, word: true, leading: true},
}

// Operators returns the operator names of a language in planning order.
func Operators(language Language) []string {
	var names []string
	for _, op := range operators {
		for _, l := range op.Languages {
			if l == language {
				names = append(names, op.Name)
			}
		}
	}
	return names
}

var languageByExtension = map[string]Language{
	".rb": Ruby, ".rake": Ruby,
	".ts": JavaScript, ".tsx": JavaScript, ".mts": JavaScript, ".cts": JavaScript,
	".js": JavaScript, ".jsx": JavaScript, ".mjs": JavaScript, ".cjs": JavaScript,
}

// DefaultExcludedDirectories are the directory names whose files are not
// application code: test trees and vendored or generated trees (CCE-V0-170).
var DefaultExcludedDirectories = []string{"__tests__", "build", "cypress", "dist", "e2e", "features", "node_modules", "playwright", "spec", "test", "tests", "vendor"}

// Directory match modes (CCE-V0-237).
const (
	DirectoryMatchAny     = "any"
	DirectoryMatchLeading = "leading"
)

// Exclusion rules of a changed file that has no hunk (CCE-V0-236).
const (
	ExcludedLanguage    = "language"
	ExcludedTestName    = "test-name"
	ExcludedDeclaration = "declaration"
	ExcludedDeleted     = "deleted"
	ExcludedNoAdditions = "no-added-lines"
	// ExcludedDirectoryPrefix is followed by the matched directory name.
	ExcludedDirectoryPrefix = "directory:"
)

// Exclusion is the effective directory-name rule: Directories match any
// directory segment of a path, or only its first one (CCE-V0-237).
type Exclusion struct {
	DirectoryMatch string   `json:"directoryMatch"`
	Directories    []string `json:"directories"`
}

// DefaultExclusion is the CCE-V0-170 rule.
func DefaultExclusion() Exclusion {
	return Exclusion{DirectoryMatch: DirectoryMatchAny, Directories: append([]string{}, DefaultExcludedDirectories...)}
}

// ApplicationLanguage returns the operator family of a repository-relative
// path, or false for a non-application file: another language, a vendored or
// generated tree, or a test file by directory or name (CCE-V0-170).
func ApplicationLanguage(name string) (Language, bool) {
	language, rule := DefaultExclusion().Classify(name)
	return language, rule == ""
}

// Classify returns the operator family of a repository-relative path, or the
// rule that excludes it (CCE-V0-236).
func (e Exclusion) Classify(name string) (Language, string) {
	language, ok := languageByExtension[path.Ext(name)]
	if !ok {
		return "", ExcludedLanguage
	}
	dirs := strings.Split(path.Dir(name), "/")
	if e.DirectoryMatch == DirectoryMatchLeading {
		dirs = dirs[:1]
	}
	for _, dir := range dirs {
		for _, excluded := range e.Directories {
			if dir == excluded {
				return "", ExcludedDirectoryPrefix + dir
			}
		}
	}
	base := path.Base(name)
	if strings.HasSuffix(base, ".d.ts") {
		return "", ExcludedDeclaration
	}
	if strings.HasSuffix(base, "_spec.rb") || strings.HasSuffix(base, "_test.rb") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return "", ExcludedTestName
	}
	return language, ""
}

// Site is one planned mutant location: a 1-based line and byte column.
type Site struct {
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Operator    string `json:"operator"`
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
}

// PlanLine returns every operator site of one source line in column order.
// blockComment carries a JS block comment open across lines and is updated.
func PlanLine(language Language, lineNumber int, line string, blockComment *bool) []Site {
	code := codeMask(language, line, blockComment)
	var sites []Site
	for i := 0; i < len(line); i++ {
		if !code[i] {
			continue
		}
		for _, op := range operators {
			if !op.applies(language) || !op.matches(line, code, i) {
				continue
			}
			sites = append(sites, Site{Line: lineNumber, Column: i + 1, Operator: op.Name, Original: op.From, Replacement: op.To})
			i += len(op.From) - 1
			break
		}
	}
	return sites
}

func (op operator) applies(language Language) bool {
	for _, l := range op.Languages {
		if l == language {
			return true
		}
	}
	return false
}

func (op operator) matches(line string, code []bool, i int) bool {
	end := i + len(op.From)
	if end > len(line) || line[i:end] != op.From {
		return false
	}
	for j := i; j < end; j++ {
		if !code[j] {
			return false
		}
	}
	before, after := byte(' '), byte(' ')
	if i > 0 {
		before = line[i-1]
	}
	if end < len(line) {
		after = line[end]
	}
	// A keyword is a whole word: not part of an identifier, a method call
	// (`.true`), a symbol (`:true`), a predicate (`true?`) or a hash key (`true:`).
	if op.word && (i > 0 && (wordByte(before) || before == '.' || before == ':') || end < len(line) && (wordByte(after) || after == '?' || after == '!' || after == ':')) {
		return false
	}
	if op.spaced && (before != ' ' || after != ' ') {
		return false
	}
	if op.leading && strings.TrimSpace(line[:i]) != "" {
		return false
	}
	if op.notFollowed != "" && end < len(line) && strings.IndexByte(op.notFollowed, after) >= 0 {
		return false
	}
	if op.notPreceded != "" && i > 0 && strings.IndexByte(op.notPreceded, before) >= 0 {
		return false
	}
	return true
}

func wordByte(b byte) bool {
	return b == '_' || b == '$' || b == '@' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// codeMask marks the bytes of a line that are code: not inside a string,
// template or regexp-free quoted literal, and not inside a comment. It is a
// line-local lexer; a string spanning lines is treated as code on its later
// lines, so a mutant there may be uncompilable or equivalent, never hidden.
func codeMask(language Language, line string, blockComment *bool) []bool {
	code := make([]bool, len(line))
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case *blockComment:
			if c == '*' && i+1 < len(line) && line[i+1] == '/' {
				*blockComment = false
				i++
			}
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
		case language == Ruby && c == '#':
			return code
		case language == JavaScript && c == '/' && i+1 < len(line) && line[i+1] == '/':
			return code
		case language == JavaScript && c == '/' && i+1 < len(line) && line[i+1] == '*':
			*blockComment = true
			i++
		default:
			code[i] = true
		}
	}
	return code
}
