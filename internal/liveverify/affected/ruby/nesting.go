package ruby

import (
	"regexp"
	"sort"
	"strings"
)

var (
	rubyWord       = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*[?!]?`)
	namespaceName  = regexp.MustCompile(`^[\t ]+(?:::)?([A-Z][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*)`)
	endlessDef     = regexp.MustCompile(`^[\t ]+(?:self\.)?[^\s(=]+(?:\([^)\n]*\)[\t ]*|[\t ]+)=(?:[^=~>]|$)`)
	evaluatedOpen  = regexp.MustCompile(`\b(?:eval|class_eval|module_eval|instance_eval|class_exec|module_exec|instance_exec)\b`)
	openerKeywords = map[string]bool{"and": true, "or": true, "not": true, "then": true, "else": true, "do": true, "begin": true, "when": true, "in": true}
)

// maskHeredocBodies blanks each heredoc body and terminator line, keeping the
// line count, so constant-looking words in heredoc text add no edge
// (RUBY-AFFECTED-V0-043). It fails toward keeping text: a body whose
// terminator never appears, or whose opener line passes it to an evaluating
// call such as class_eval, stays as written.
func maskHeredocBodies(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, len(lines))
	copy(out, lines)
	var pending []string
	bodyStart := -1
	keep := false
	for index, line := range lines {
		if len(pending) != 0 {
			if !keep {
				out[index] = ""
			}
			if strings.TrimSpace(line) == pending[0] {
				pending = pending[1:]
			}
			continue
		}
		masked := maskQuotedLiteralsAndInlineComments(line)
		for _, match := range heredocStart.FindAllStringSubmatchIndex(line, -1) {
			if masked[match[0]] == '<' {
				pending = append(pending, line[match[2]:match[3]])
			}
		}
		bodyStart = index + 1
		keep = evaluatedOpen.MatchString(masked)
	}
	if len(pending) != 0 && bodyStart >= 0 {
		copy(out[bodyStart:], lines[bodyStart:])
	}
	return strings.Join(out, "\n")
}

// namespaceFrame is one `class` or `module` body: the bytes from its keyword to
// the end of its matching `end`.
type namespaceFrame struct {
	start, end int
	name       string
}

// lexicalScopes answers which namespaces Ruby's lexical scope can name at an
// offset. When the keyword scan cannot balance the file's blocks, it falls
// back to every namespace the file opens (RUBY-AFFECTED-V0-043).
type lexicalScopes struct {
	frames   []namespaceFrame
	fallback []string
}

func scanLexicalScopes(raw, masked string) lexicalScopes {
	frames, ok := namespaceFrames(raw, masked)
	if !ok {
		return lexicalScopes{fallback: openedNamespaces(masked)}
	}
	return lexicalScopes{frames: frames}
}

// at names the enclosing namespaces at offset, each alone and nested below
// every enclosing one, with their prefixes, as openedNamespaces does for the
// whole file.
func (scopes lexicalScopes) at(offset int) []string {
	if scopes.fallback != nil || scopes.frames == nil {
		return scopes.fallback
	}
	set := map[string]bool{}
	nested := ""
	for _, frame := range scopes.frames {
		if offset < frame.start || offset >= frame.end {
			continue
		}
		if nested == "" {
			nested = frame.name
		} else {
			nested += "::" + frame.name
		}
		for _, name := range []string{frame.name, nested} {
			for prefix := name; prefix != ""; {
				set[prefix] = true
				cut := strings.LastIndex(prefix, "::")
				if cut < 0 {
					break
				}
				prefix = prefix[:cut]
			}
		}
	}
	return sortedKeys(set)
}

// namespaceFrames matches block keywords to their `end` in quote-masked
// source and returns the named class and module bodies ordered by start. It
// reports false for a file it cannot balance, or that holds an `=begin`
// block or `__END__` data the mask does not cover.
func namespaceFrames(raw, masked string) ([]namespaceFrame, bool) {
	for _, line := range strings.Split(masked, "\n") {
		if strings.HasPrefix(line, "=begin") || strings.TrimSpace(line) == "__END__" {
			return nil, false
		}
	}
	type open struct{ frame int }
	var stack []open
	frames := []namespaceFrame{}
	loopDoLine := -1
	for _, loc := range rubyWord.FindAllStringIndex(masked, -1) {
		start, end := loc[0], loc[1]
		word := masked[start:end]
		if start > 0 && (masked[start-1] == '.' || masked[start-1] == ':') {
			continue
		}
		if end < len(masked) && masked[end] == ':' && (end+1 >= len(masked) || masked[end+1] != ':') {
			continue
		}
		line := strings.Count(masked[:start], "\n")
		switch word {
		case "class", "module":
			frame := -1
			if name := namespaceName.FindStringSubmatch(masked[end:]); name != nil {
				frames = append(frames, namespaceFrame{start: start, name: name[1]})
				frame = len(frames) - 1
			}
			stack = append(stack, open{frame})
		case "def":
			if !endlessDef.MatchString(lineRest(masked, end)) {
				stack = append(stack, open{-1})
			}
		case "begin", "case":
			stack = append(stack, open{-1})
		case "do":
			if loopDoLine == line {
				loopDoLine = -1
				continue
			}
			stack = append(stack, open{-1})
		case "for":
			stack = append(stack, open{-1})
			loopDoLine = line
		case "if", "unless", "while", "until":
			if !statementStart(raw, masked, start) {
				continue
			}
			stack = append(stack, open{-1})
			if word == "while" || word == "until" {
				loopDoLine = line
			}
		case "end":
			if len(stack) == 0 {
				return nil, false
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.frame >= 0 {
				frames[top.frame].end = end
			}
		}
	}
	if len(stack) != 0 {
		return nil, false
	}
	sort.SliceStable(frames, func(i, j int) bool { return frames[i].start < frames[j].start })
	return frames, true
}

func lineRest(text string, from int) string {
	if cut := strings.IndexByte(text[from:], '\n'); cut >= 0 {
		return text[from : from+cut]
	}
	return text[from:]
}

// statementStart reports whether the keyword at start begins an expression,
// and so needs an `end`, rather than modifying the expression before it.
func statementStart(raw, masked string, start int) bool {
	index := start - 1
	for index >= 0 && (masked[index] == ' ' || masked[index] == '\t') {
		if raw[index] != ' ' && raw[index] != '\t' {
			return false // a masked literal ends the expression before it
		}
		index--
	}
	if index < 0 || masked[index] == '\n' || masked[index] == ';' {
		return true
	}
	current := masked[index]
	switch {
	case current == '!' || current == '?':
		return index == 0 || !isWordByte(masked[index-1])
	case isWordByte(current):
		wordStart := index
		for wordStart > 0 && isWordByte(masked[wordStart-1]) {
			wordStart--
		}
		return openerKeywords[masked[wordStart:index+1]]
	case current == ')' || current == ']' || current == '}':
		return false
	}
	return true
}

func isWordByte(value byte) bool {
	return value == '_' || value >= '0' && value <= '9' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
