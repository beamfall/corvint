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

// heredocOpener matches a candidate heredoc start: an optional `-` or `~`
// flag, then an identifier, optionally quoted with matching quotes.
var heredocOpener = regexp.MustCompile(`<<([-~]?)(?:"([A-Za-z_][A-Za-z0-9_]*)"|'([A-Za-z_][A-Za-z0-9_]*)'|` + "`" + `([A-Za-z_][A-Za-z0-9_]*)` + "`" + `|([A-Za-z_][A-Za-z0-9_]*))`)

// regexKeyword finds a slash-delimited span holding a block keyword, which a
// regex literal such as /end/ could hold; division can match too, which only
// costs the whole-file fallback.
var regexKeyword = regexp.MustCompile(`/[^/\n]*\b(?:begin|case|class|def|do|end|for|if|module|unless|until|while)\b[^/\n]*/`)

type heredoc struct {
	terminator string
	literal    bool // single-quoted: no interpolation
}

// heredocOpeners returns the heredocs line opens. A start is accepted only in
// an expression position, not directly after an identifier or closing bracket
// (`items<<Widget` is a shift), and only with a quoted or upper-case
// identifier, so `items <<Widget` is not one either. An ambiguous start opens
// nothing and its body stays unmasked (RUBY-AFFECTED-V0-043).
func heredocOpeners(line string) []heredoc {
	masked := maskQuotedLiteralsAndInlineComments(line)
	var found []heredoc
	for _, match := range heredocOpener.FindAllStringSubmatchIndex(line, -1) {
		if masked[match[0]] != '<' {
			continue
		}
		if match[0] > 0 {
			if previous := line[match[0]-1]; isWordByte(previous) || previous == ')' || previous == ']' || previous == '}' {
				continue
			}
		}
		switch {
		case match[4] >= 0:
			found = append(found, heredoc{terminator: line[match[4]:match[5]]})
		case match[6] >= 0:
			found = append(found, heredoc{terminator: line[match[6]:match[7]], literal: true})
		case match[8] >= 0:
			found = append(found, heredoc{terminator: line[match[8]:match[9]]})
		case strings.ToUpper(line[match[10]:match[11]]) == line[match[10]:match[11]]:
			found = append(found, heredoc{terminator: line[match[10]:match[11]]})
		}
	}
	return found
}

// keepInterpolations blanks a heredoc body line except the code inside each
// `#{...}`, which runs. It reports false when an interpolation does not close
// on its line.
func keepInterpolations(line string) (string, bool) {
	out := []byte(strings.Repeat(" ", len(line)))
	for index := 0; index+1 < len(line); index++ {
		if line[index] != '#' || line[index+1] != '{' {
			continue
		}
		depth := 0
		closed := false
		for at := index + 1; at < len(line); at++ {
			switch line[at] {
			case '{':
				depth++
			case '}':
				depth--
			}
			if depth == 0 {
				copy(out[index+2:at], line[index+2:at])
				index = at
				closed = true
				break
			}
		}
		if !closed {
			return line, false
		}
	}
	return string(out), true
}

// maskHeredocBodies blanks each heredoc body and terminator line, keeping the
// line count, so constant-looking words in heredoc text add no edge
// (RUBY-AFFECTED-V0-043). Interpolated code in a heredoc that is not
// single-quoted stays. It fails toward keeping text: a body whose terminator
// never appears, whose opener line passes it to an evaluating call such as
// class_eval, or whose interpolation spans lines stays as written.
func maskHeredocBodies(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, len(lines))
	copy(out, lines)
	var pending []heredoc
	bodyStart := -1
	keep := false
	for index, line := range lines {
		if len(pending) != 0 {
			if strings.TrimSpace(line) == pending[0].terminator {
				if !keep {
					out[index] = ""
				}
				pending = pending[1:]
				continue
			}
			if keep {
				continue
			}
			if pending[0].literal {
				out[index] = ""
				continue
			}
			kept, ok := keepInterpolations(line)
			if !ok {
				keep = true
				copy(out[index:], lines[index:])
				continue
			}
			out[index] = kept
			continue
		}
		pending = heredocOpeners(line)
		bodyStart = index + 1
		keep = evaluatedOpen.MatchString(maskQuotedLiteralsAndInlineComments(line))
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
// block, `__END__` data, or a slash-delimited span holding a block keyword (a
// possible regex literal such as /end/), none of which the mask covers.
func namespaceFrames(raw, masked string) ([]namespaceFrame, bool) {
	for _, line := range strings.Split(masked, "\n") {
		if strings.HasPrefix(line, "=begin") || strings.TrimSpace(line) == "__END__" || regexKeyword.MatchString(line) {
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
