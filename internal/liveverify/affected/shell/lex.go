package shell

import "strings"

// command is one simple command: its raw words, quoting kept, with a
// redirection operator kept as the redirect marker before its target.
// inFunction reports a command inside a function body; substituted, one
// inside a command or process substitution. block identifies the innermost
// function body, brace group, subshell or compound-command arm (an if, elif or
// else branch, a loop body, a case arm) holding the command: 0 is the file's
// top level and -1 a substitution's own commands, so two commands with one
// block id >= 0 run in that order in one straight-line list. lead and trail
// are the operators before and after it ("" at a list's start or end, "\n",
// ";", ";;", "&&", "||", "|", "&", "(" or ")").
type command struct {
	words       []string
	inFunction  bool
	substituted bool
	block       int
	lead, trail string
}

// script is what one lexing pass over a file yields: its commands in source
// order, its heredoc bodies, which are data rather than commands, and the
// functions it defines, each mapped to the number of commands before its
// first definition. definitions counts each name's definitions, and bodies
// holds the brace body of a name's first definition (TSP-V0-004).
type script struct {
	commands    []command
	data        []string
	functions   map[string]int
	definitions map[string]int
	bodies      map[string]body
	blocks      int
	naming      string
}

// body is a function's brace body: its block id and the block id holding
// the definition.
type body struct {
	block, parent int
}

// define records a function's first definition.
func (s *script) define(name string) {
	if _, seen := s.functions[name]; !seen {
		s.functions[name] = len(s.commands)
	}
	s.definitions[name]++
	s.naming = name
}

// redirect marks a redirection operator; the word after it is its target.
// writeRedirect marks one that only truncates and writes its target (`>`,
// `>|`), whose target no dirty path can reach (NSA-V0-016); every other
// operator, appending and `<>` included, keeps the read marker.
const (
	redirect      = "\x00redirect"
	writeRedirect = "\x00write"
)

// plainWords drops redirections and their targets.
func plainWords(words []string) []string {
	plain := make([]string, 0, len(words))
	for index := 0; index < len(words); index++ {
		if words[index] == redirect || words[index] == writeRedirect {
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
	out := script{functions: map[string]int{}, definitions: map[string]int{}, bodies: map[string]body{}}
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
	// expands reports an unquoted delimiter, whose body's command
	// substitutions run.
	expands bool
}

// frame is one open construct: a function body or brace group, a subshell
// (paren) or a compound command (keyword), with its block id.
type frame struct {
	function, paren, keyword bool
	id                       int
}

// parse reads commands until EOF or, when closer is ')', the parenthesis that
// closes the substitution it was called for, leaving pos past it.
func (l *lexer) parse(closer byte, inherited, substituted bool) {
	var (
		words       []string
		start       = -1
		pending     []heredoc
		stack       []frame
		pendingFunc bool
		// cases counts open case statements; pattern is set while a case
		// pattern list, whose words and `|` are not commands, is being read.
		cases   int
		pattern bool
	)
	block := func() int {
		if substituted {
			return -1
		}
		if len(stack) == 0 {
			return 0
		}
		return stack[len(stack)-1].id
	}
	push := func(entry frame) {
		l.out.blocks++
		entry.id = l.out.blocks
		if name := l.out.naming; entry.function && name != "" {
			if _, seen := l.out.bodies[name]; !seen {
				l.out.bodies[name] = body{block: entry.id, parent: block()}
			}
		}
		l.out.naming = ""
		stack = append(stack, entry)
	}
	// arm starts a compound command's next arm, a new block.
	arm := func() {
		if len(stack) > 0 && stack[len(stack)-1].keyword {
			l.out.blocks++
			stack[len(stack)-1].id = l.out.blocks
			return
		}
		push(frame{keyword: true})
	}
	popKeyword := func() {
		if len(stack) > 0 && stack[len(stack)-1].keyword {
			stack = stack[:len(stack)-1]
		}
	}
	var lead, trail string
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
		word := strings.ReplaceAll(l.src[start:l.pos], "\\\n", "")
		start = -1
		switch {
		case cases > 0 && len(words) == 0 && word == "esac":
			cases--
			pattern = false
			popKeyword()
		case pattern:
		case len(words) == 2 && words[0] == "case" && word == "in":
			words = nil
			cases++
			pattern = true
			push(frame{keyword: true})
		case len(words) == 0 && word == "{":
			push(frame{function: pendingFunc})
			pendingFunc = false
		case len(words) == 0 && word == "}":
			pop()
		case word == "{" && len(words) == 2 && words[0] == "function":
			l.out.define(words[1])
			words = nil
			push(frame{function: true})
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
				l.out.define(words[1])
			}
			pendingFunc = true
		} else {
			switch words[0] {
			case "then", "else", "elif", "do":
				arm()
			}
			l.out.commands = append(l.out.commands, command{words: words, inFunction: inFunction(), substituted: substituted, block: block(), lead: lead, trail: trail})
			lead = trail
			switch words[0] {
			case "if", "while", "until", "for", "select":
				push(frame{keyword: true})
			case "fi", "done":
				popKeyword()
			}
		}
		words = nil
	}
	// endAt ends a command at the operator op.
	endAt := func(op string) {
		trail = op
		endCommand()
		trail = ""
	}
	begin := func() {
		if start < 0 {
			start = l.pos
		}
	}
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if pattern && (c == '|' || c == '(' || c == ')') {
			endWord()
			if c == ')' {
				words = nil
				pattern = false
				arm()
			}
			l.pos++
			continue
		}
		// Inside [[ ]] parentheses, `|`, `&`, `<` and `>` belong to the
		// conditional expression, e.g. a `=~` regex, not to the command list.
		if len(words) > 0 && words[0] == "[[" && words[len(words)-1] != "]]" && strings.IndexByte("()|&<>", c) >= 0 {
			begin()
			l.pos++
			continue
		}
		switch {
		case c == '\n':
			endAt("\n")
			l.pos++
			for _, body := range pending {
				l.heredoc(body, inFunction())
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
			// A line continuation joins the words around it.
			if l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
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
		case c == ';' && cases > 0 && (l.peek(1) == ';' || l.peek(1) == '&'):
			endAt(";;")
			arm()
			l.pos += 2
			if l.peek(0) == '&' {
				l.pos++
			}
			pattern = true
		case c == ';' || c == '&' || c == '|':
			op := string(c)
			if (c == '&' || c == '|') && l.peek(1) == c {
				op += op
			}
			endAt(op)
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
			operator := l.pos
			for l.pos < len(l.src) && strings.IndexByte("<>&|", l.src[l.pos]) >= 0 {
				l.pos++
			}
			if op := l.src[operator:l.pos]; op == ">" || op == ">|" {
				words = append(words, writeRedirect)
			} else {
				words = append(words, redirect)
			}
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
			endAt("(")
			push(frame{paren: true})
			l.pos++
		case c == ')':
			endAt(")")
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
	l.out.define(words[len(words)-1])
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
	raw := l.src[begin:l.pos]
	body.delimiter = unquote(raw)
	body.expands = !strings.ContainsAny(raw, `'"\\`)
	return body, body.delimiter != ""
}

// heredoc consumes one heredoc body, from pos to its delimiter line, as data.
// An expanding body's command substitutions are lexed as commands.
func (l *lexer) heredoc(body heredoc, inFunction bool) {
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
	text := strings.Join(lines, "\n")
	l.out.data = append(l.out.data, text)
	if body.expands {
		l.substitutions(text, inFunction)
	}
}

// substitutions lexes the command substitutions of double-quote-like text.
func (l *lexer) substitutions(text string, inFunction bool) {
	sub := &lexer{src: text, out: l.out}
	for sub.pos < len(sub.src) {
		switch sub.src[sub.pos] {
		case '\\':
			sub.pos += 2
		case '`':
			sub.backtick(inFunction)
		case '$':
			sub.dollar(inFunction)
		default:
			sub.pos++
		}
	}
}
