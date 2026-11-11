package testsource

import (
	"regexp"
	"sort"
	"strings"
)

// Shared Ruby lexical reading for the RSpec, seed-parity and teardown readers. These helpers
// outlived the retired actor-authorization check (V1-4822), which first carried them.

// rubyLine is one logical Ruby statement: comments removed, continuation lines joined, and the
// 1-based line it starts on.
type rubyLine struct {
	line int
	text string
	// path is the file the line was read from; the RSpec reader sets it so that a shared group
	// read through an inclusion keeps its own file's locations (TSA-V0-090).
	path string
}

// rubyLines splits Ruby source into logical statements.
func rubyLines(text string) []rubyLine {
	out := []rubyLine{}
	pending, start := "", 0
	heredocs := []rubyHeredoc{}
	for n, raw := range strings.Split(text, "\n") {
		if len(heredocs) > 0 {
			// A heredoc body is string text, not code (TSA-V0-396).
			if heredocs[0].closes(raw) {
				heredocs = heredocs[1:]
			}
			continue
		}
		stripped := strings.TrimSpace(stripRubyComment(raw))
		heredocs = rubyHeredocs(stripped)
		if pending == "" {
			start = n + 1
		}
		if pending != "" {
			pending += " "
		}
		pending += stripped
		if strings.HasSuffix(pending, ",") || strings.HasSuffix(pending, "(") || strings.HasSuffix(pending, "[") || strings.HasSuffix(pending, "\\") {
			pending = strings.TrimSuffix(pending, "\\")
			continue
		}
		if pending != "" {
			out = append(out, rubyLine{line: start, text: pending})
		}
		pending = ""
	}
	if pending != "" {
		out = append(out, rubyLine{line: start, text: pending})
	}
	return out
}

// rubyHeredoc is one heredoc a line opens: its terminator, and whether the terminator may be
// indented (`<<~ID`, `<<-ID`) or must start the line (`<<ID`).
type rubyHeredoc struct {
	id       string
	indented bool
}

// rubyHeredocStart is `<<~ID`, `<<-ID`, a quoted `<<'ID'` or `<<"ID"`, or a bare upper-case
// `<<ID`; a shovel (`list << item`) has a space and a lower-case operand.
var rubyHeredocStart = regexp.MustCompile(`<<([~-]?)(?:(['"])([A-Za-z_]\w*)['"]|([A-Za-z_]\w*))`)

func rubyHeredocs(text string) []rubyHeredoc {
	out := []rubyHeredoc{}
	for _, m := range rubyHeredocStart.FindAllStringSubmatch(text, -1) {
		id := m[3] + m[4]
		if m[1] == "" && m[2] == "" && strings.ToUpper(id) != id {
			continue
		}
		out = append(out, rubyHeredoc{id: id, indented: m[1] != ""})
	}
	return out
}

func (h rubyHeredoc) closes(raw string) bool {
	raw = strings.TrimRight(raw, " \t\r")
	if h.indented {
		raw = strings.TrimSpace(raw)
	}
	return raw == h.id
}

// stripRubyComment drops a `#` comment outside a quoted string.
func stripRubyComment(line string) string {
	quote := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0 && c == '\\':
			i++
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		case quote == 0 && c == '#':
			return line[:i]
		}
	}
	return line
}

var (
	rubyOpener = regexp.MustCompile(`^(def|class|module|if|unless|case|while|until|begin|for)\b`)
	rubyDo     = regexp.MustCompile(`\bdo(\s*\|[^|]*\|)?$`)
	rubyEnd    = regexp.MustCompile(`^end\b`)
	// rubyOneLine is a statement that opens and closes on one line, such as
	// `def read_data; build_report_io; end`; it opens no block (TSA-V0-084).
	rubyOneLine = regexp.MustCompile(`;\s*end$`)
	// rubyEndlessDef is an endless method, `def name = expr` or `def name(args) = expr`; it opens
	// no block. A setter (`def name=(v)`) and a comparison (`==`) are not one.
	rubyEndlessDef = regexp.MustCompile(`^def\s+[A-Za-z_]\w*[?!]?(?:\s+|\([^()]*\)\s*)=(?:[^=~]|$)`)
	// rubyValueOpener is if, unless, case, begin, while or until in value position, where it
	// opens a block its own `end` closes: after an assignment (`=`, `||=`, `+=`), a hash `=>`, a
	// label key (`key: if`), an append (`<< if`), an opening bracket, a comma, `&&` or `||`, a
	// line-start `else` (`else if`, which nests a block), and case or begin after `return`. A
	// modifier (`foo if bar`, `return if bar`) follows an expression or `return` and opens
	// nothing, and a keyword argument (`if: x`) is not the keyword (TSA-V0-396, V1-2078).
	rubyValueOpener = regexp.MustCompile(`(?:(?:=>?|[(\[,]|&&|\|\||\w:\s|<<\s|^else\s)\s*(if|unless|case|begin|while|until)|\breturn\s+(case|begin))(?:\s|[(;]|$)`)
	// rubyLineEnd is an `end` word; after a value opener it closes the block on the same line
	// unless it is a method or symbol (`.end`, `:end`, `end?`, `end:`), see rubyEndKeyword.
	rubyLineEnd = regexp.MustCompile(`\bend\b`)
)

func rubyOpens(text string) bool {
	if (rubyOpener.MatchString(text) || rubyDo.MatchString(text)) && !rubyOneLine.MatchString(text) && !rubyEndlessDef.MatchString(text) {
		return true
	}
	return rubyValueOpens(text)
}

// rubyValueOpens reports whether a line opens a block with a value-position keyword that the line
// does not also close (`x = if a then 1 else 2 end`); string literals are ignored (TSA-V0-396).
func rubyValueOpens(text string) bool {
	return rubyValueKeyword(text) != ""
}

// rubyValueKeyword returns the value-position keyword whose block a line opens and leaves open,
// or "" (TSA-V0-396).
func rubyValueKeyword(text string) string {
	text = stripRubyStrings(text)
	m := rubyValueOpener.FindStringSubmatchIndex(text)
	if m == nil || rubyEndKeyword(text[m[1]-1:]) {
		return ""
	}
	if m[2] >= 0 {
		return text[m[2]:m[3]]
	}
	return text[m[4]:m[5]]
}

// rubyEndKeyword reports whether text holds an `end` keyword, not a method call (`.end`, `end?`),
// a symbol (`:end`), a hash key (`end:`) or a variable (`@end`).
func rubyEndKeyword(text string) bool {
	for _, at := range rubyLineEnd.FindAllStringIndex(text, -1) {
		if at[0] > 0 && strings.ContainsRune(".:@$", rune(text[at[0]-1])) {
			continue
		}
		if at[1] < len(text) && strings.ContainsRune("?!:", rune(text[at[1]])) && !strings.HasPrefix(text[at[1]:], "::") {
			continue
		}
		return true
	}
	return false
}

// rubyBlockEnd returns the index of the `end` closing the block opened at lines[i], or len(lines).
func rubyBlockEnd(lines []rubyLine, i int) int {
	depth := 0
	for j := i; j < len(lines); j++ {
		if rubyOpens(lines[j].text) {
			depth++
		} else if rubyEnd.MatchString(lines[j].text) {
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(lines)
}

// sortedKeys returns a map's keys in deterministic order for the environment and teardown readers.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
