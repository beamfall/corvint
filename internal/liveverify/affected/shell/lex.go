package shell

import "strings"

// command is one simple command: its raw words, quoting kept, with a
// redirection operator kept as the redirect marker before its target.
// inFunction reports a command inside a function body; substituted, one
// inside a command or process substitution.
type command struct {
	words       []string
	inFunction  bool
	substituted bool
}

// script is what one lexing pass over a file yields: its commands in source
// order, its heredoc bodies, which are data rather than commands, and the
// names of the functions it defines.
type script struct {
	commands  []command
	data      []string
	functions map[string]bool
}

// redirect marks a redirection operator; the word after it is its target.
const redirect = "\x00redirect"

// plainWords drops redirections and their targets.
func plainWords(words []string) []string {
	plain := make([]string, 0, len(words))
	for index := 0; index < len(words); index++ {
		if words[index] == redirect {
			index++
			continue
		}
		plain = append(plain, words[index])
	}
	return plain
}

// parse lexes shell text into commands. It is a deliberately small lexer:
// quoting, comments, line continuations, heredocs, command and process
// substitutions, subshells, brace groups and function bodies are recognised,
// and anything else is kept as word text. A misreading can add or drop an
// edge in either direction only for constructs it does not model.
func parse(text string) script {
	out := script{functions: map[string]bool{}}
	lexer := &lexer{src: text, out: &out}
	lexer.parse(0, false, false)
	return out
}

type lexer struct {
	src string
	pos int
	out *script
}

type heredoc struct {
	delimiter string
	dash      bool
}

type frame struct{ function, paren bool }

// parse reads commands until EOF or, when closer is ')', the parenthesis that
// closes the substitution it was called for, leaving pos past it.
func (l *lexer) parse(closer byte, inherited, substituted bool) {
	var (
		words       []string
		start       = -1
		pending     []heredoc
		stack       []frame
		pendingFunc bool
	)
	inFunction := func() bool {
		if inherited {
			return true
		}
		for _, entry := range stack {
			if entry.function {
				return true
			}
		}
		return false
	}
	pop := func() {
		if len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
	}
	endWord := func() {
		if start < 0 {
			return
		}
		word := l.src[start:l.pos]
		start = -1
		switch {
		case len(words) == 0 && word == "{":
			stack = append(stack, frame{function: pendingFunc})
			pendingFunc = false
		case len(words) == 0 && word == "}":
			pop()
		case word == "{" && len(words) == 2 && words[0] == "function":
			l.out.functions[words[1]] = true
			words = nil
			stack = append(stack, frame{function: true})
			pendingFunc = false
		default:
			words = append(words, word)
		}
	}
	endCommand := func() {
		endWord()
		if len(words) == 0 {
			return
		}
		if words[0] == "function" {
			if len(words) > 1 {
				l.out.functions[words[1]] = true
			}
			pendingFunc = true
		} else {
			l.out.commands = append(l.out.commands, command{words: words, inFunction: inFunction(), substituted: substituted})
		}
		words = nil
	}
	begin := func() {
		if start < 0 {
			start = l.pos
		}
	}
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			endCommand()
			l.pos++
			for _, body := range pending {
				l.heredoc(body)
			}
			pending = nil
		case c == ' ' || c == '\t' || c == '\r':
			endWord()
			l.pos++
		case c == '#' && start < 0:
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '\\':
			if l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
				endWord()
				l.pos += 2
				continue
			}
			begin()
			l.pos = min(l.pos+2, len(l.src))
		case c == '\'':
			begin()
			l.singleQuoted()
		case c == '"':
			begin()
			l.doubleQuoted(inFunction())
		case c == '`':
			begin()
			l.backtick(inFunction())
		case c == '$':
			begin()
			l.dollar(inFunction())
		case c == ';' || c == '&' || c == '|':
			endCommand()
			l.pos++
		case (c == '<' || c == '>') && l.peek(1) == '(':
			begin()
			l.pos += 2
			l.parse(')', inFunction(), true)
		case c == '<' && strings.HasPrefix(l.src[l.pos:], "<<<"):
			endWord()
			l.pos += 3
			words = append(words, redirect)
		case c == '<' && l.peek(1) == '<':
			endWord()
			l.pos += 2
			if body, ok := l.heredocOperator(); ok {
				pending = append(pending, body)
			}
		case c == '<' || c == '>':
			if start >= 0 && strings.Trim(l.src[start:l.pos], "0123456789") == "" {
				start = -1
			}
			endWord()
			for l.pos < len(l.src) && strings.IndexByte("<>&|", l.src[l.pos]) >= 0 {
				l.pos++
			}
			words = append(words, redirect)
		case c == '(':
			if start >= 0 && strings.HasSuffix(l.src[start:l.pos], "=") {
				l.balanced('(', ')')
				continue
			}
			endWord()
			if l.functionParens(words) {
				words = nil
				pendingFunc = true
				continue
			}
			endCommand()
			stack = append(stack, frame{paren: true})
			l.pos++
		case c == ')':
			endCommand()
			l.pos++
			if len(stack) > 0 && stack[len(stack)-1].paren {
				pop()
				continue
			}
			if closer == ')' {
				return
			}
		default:
			begin()
			l.pos++
		}
	}
	endCommand()
}

func (l *lexer) peek(offset int) byte {
	if l.pos+offset < len(l.src) {
		return l.src[l.pos+offset]
	}
	return 0
}

// functionParens consumes the `()` of a function definition `name ()` or
// `function name ()` and reports whether it did.
func (l *lexer) functionParens(words []string) bool {
	if !(len(words) == 1 && name(words[0])) && !(len(words) == 2 && words[0] == "function") {
		return false
	}
	next := l.pos + 1
	for next < len(l.src) && (l.src[next] == ' ' || l.src[next] == '\t') {
		next++
	}
	if next >= len(l.src) || l.src[next] != ')' {
		return false
	}
	l.out.functions[words[len(words)-1]] = true
	l.pos = next + 1
	return true
}

func name(word string) bool {
	if word == "" {
		return false
	}
	for _, c := range word {
		if !(c == '_' || c == '-' || c == ':' || c == '.' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func (l *lexer) singleQuoted() {
	end := strings.IndexByte(l.src[l.pos+1:], '\'')
	if end < 0 {
		l.pos = len(l.src)
		return
	}
	l.pos += end + 2
}

func (l *lexer) doubleQuoted(inFunction bool) {
	l.pos++
	for l.pos < len(l.src) {
		switch l.src[l.pos] {
		case '\\':
			l.pos = min(l.pos+2, len(l.src))
		case '"':
			l.pos++
			return
		case '$':
			l.dollar(inFunction)
		case '`':
			l.backtick(inFunction)
		default:
			l.pos++
		}
	}
}

// backtick lexes a backquoted command substitution as commands of its own.
func (l *lexer) backtick(inFunction bool) {
	end := l.pos + 1
	for end < len(l.src) && l.src[end] != '`' {
		if l.src[end] == '\\' {
			end++
		}
		end++
	}
	end = min(end, len(l.src))
	inner := strings.ReplaceAll(l.src[l.pos+1:end], "\\`", "`")
	sub := &lexer{src: inner, out: l.out}
	sub.parse(0, inFunction, true)
	l.pos = min(end+1, len(l.src))
}

// dollar lexes one `$` expansion: a command substitution is lexed as commands
// of its own; arithmetic, parameter and ANSI-C forms are skipped whole.
func (l *lexer) dollar(inFunction bool) {
	switch {
	case strings.HasPrefix(l.src[l.pos:], "$(("):
		l.pos++
		l.balanced('(', ')')
	case l.peek(1) == '(':
		l.pos += 2
		l.parse(')', inFunction, true)
	case l.peek(1) == '{':
		l.pos++
		l.balanced('{', '}')
	case l.peek(1) == '\'':
		l.pos++
		for l.pos++; l.pos < len(l.src) && l.src[l.pos] != '\''; l.pos++ {
			if l.src[l.pos] == '\\' {
				l.pos++
			}
		}
		l.pos = min(l.pos+1, len(l.src))
	default:
		l.pos++
	}
}

// balanced skips from an opening delimiter at pos past its match, skipping
// quoted text.
func (l *lexer) balanced(open, close byte) {
	depth := 0
	for l.pos < len(l.src) {
		switch c := l.src[l.pos]; {
		case c == '\\':
			l.pos++
		case c == '\'':
			l.singleQuoted()
			continue
		case c == open:
			depth++
		case c == close:
			depth--
			if depth == 0 {
				l.pos++
				return
			}
		}
		l.pos++
	}
}

// heredocOperator reads the delimiter after `<<`.
func (l *lexer) heredocOperator() (heredoc, bool) {
	var body heredoc
	if l.peek(0) == '-' {
		body.dash = true
		l.pos++
	}
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.pos++
	}
	begin := l.pos
	for l.pos < len(l.src) && strings.IndexByte(" \t\n;&|<>()", l.src[l.pos]) < 0 {
		if l.src[l.pos] == '\'' || l.src[l.pos] == '"' {
			quote := l.src[l.pos]
			end := strings.IndexByte(l.src[l.pos+1:], quote)
			if end < 0 {
				l.pos = len(l.src)
				break
			}
			l.pos += end + 2
			continue
		}
		l.pos++
	}
	body.delimiter = unquote(l.src[begin:l.pos])
	return body, body.delimiter != ""
}

// heredoc consumes one heredoc body, from pos to its delimiter line, as data.
func (l *lexer) heredoc(body heredoc) {
	var lines []string
	for l.pos < len(l.src) {
		end := strings.IndexByte(l.src[l.pos:], '\n')
		line := l.src[l.pos:]
		if end >= 0 {
			line = l.src[l.pos : l.pos+end]
			l.pos += end + 1
		} else {
			l.pos = len(l.src)
		}
		check := strings.TrimSuffix(line, "\r")
		if body.dash {
			check = strings.TrimLeft(check, "\t")
		}
		if check == body.delimiter {
			break
		}
		lines = append(lines, line)
	}
	l.out.data = append(l.out.data, strings.Join(lines, "\n"))
}
