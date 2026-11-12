package shell

import (
	"slices"
	"strings"
)

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
// holds the brace body of a name's first definition (TSP-V0-004). cases and
// arithmetic count the `case` statements and `((` arithmetic commands, which
// the lexer folds away without a command of their own (a case header and its
// patterns are dropped, an arithmetic command reads as nested subshells), so
// the name-listing gate can refuse them (AFP-V0-9961).
type script struct {
	commands    []command
	data        []string
	functions   map[string]int
	definitions map[string]int
	bodies      map[string]body
	blocks      int
	naming      string
	// incomplete reports a here-document that no delimiter line ended: one
	// that reached the end of the text, one cut by the `)` closing its
	// command substitution, or one whose operator line never ended. Shells
	// disagree on such a body (bash ends an empty-delimited body inside `$(`
	// at the `)`; dash refuses the file), so a file holding one is held as a
	// dynamic source (AFP-V0-9971).
	incomplete bool
	cases      int
	arithmetic int
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
			l.out.cases++
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
			// An operator after a compound command's closing `}`, `)` or
			// `esac` ends no simple command, but it still joins the
			// compound to the next one (`{ :; } || tmp=...`), so it is the
			// next command's lead. Only an operator that makes the next
			// command conditional or a subshell replaces the lead, so the
			// lead never loses a refusal it had.
			switch trail {
			case "&&", "||", "|", "&":
				lead = trail
			}
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
		// Inside [[ ]], `|`, `&`, `<` and `>` belong to the conditional
		// expression (a `=~` regex, a `||` or `&&` between tests), not to the
		// command list, whether the test opens the command or follows a
		// control word such as `if`, `while` or `!` in it; a `[[` that is an
		// argument opens no test (openTest, AFP-V0-3145).
		if openTest(words) && strings.IndexByte("()|&<>", c) >= 0 {
			begin()
			l.pos++
			continue
		}
		switch {
		case c == '\n':
			endAt("\n")
			l.pos++
			for _, body := range pending {
				if l.heredoc(body, inFunction(), substituted && closer == ')') {
					// The substitution closed inside this body: the
					// later bodies of the line were never read.
					l.out.incomplete = true
					break
				}
			}
			pending = nil
		case c == ' ' || c == '\t':
			// Only space and tab are blanks. A carriage return is word text
			// in bash, dash, zsh and POSIX sh, so `a\r#` is one word and its
			// `#` starts no comment that could hide what follows
			// (AFP-V0-9970).
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
			// Past the whole operator: a second `&` or `|` read again
			// would end an empty command and turn the next command's
			// `&&` or `||` lead into `&` or `|`.
			l.pos += len(op)
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
			if l.peek(1) == '(' {
				l.out.arithmetic++
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
				if len(pending) != 0 {
					// `$(cat <<EOF)`: the body follows the closing line,
					// where shells disagree on how to read it.
					l.out.incomplete = true
				}
				return
			}
		default:
			begin()
			l.pos++
		}
	}
	endCommand()
	if len(pending) != 0 {
		// An operator on the last line, or one inside a substitution
		// that closed on its own line, has no body read.
		l.out.incomplete = true
	}
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
	// A quoted empty delimiter (`<<''`, `<<""`) is a here-document whose
	// body ends at the first empty line, as in bash and dash; its body is
	// data, never commands (AFP-V0-9971). Only a missing delimiter word is
	// no here-document.
	return body, raw != ""
}

// heredoc consumes one heredoc body, from pos to its delimiter line, as data.
// An expanding body's command substitutions are lexed as commands.
// substitution marks a body inside a `$(...)` or `<(...)`: there a line that
// starts with the delimiter and whose rest holds the unbalanced `)` ends the
// body, the rest being shell text that closes the substitution, as bash reads
// `EOF)` or, for an empty delimiter, any line holding the `)`,
// and heredoc reports that it stopped there, leaving pos at that rest. A body
// that no delimiter line ends marks the script incomplete (AFP-V0-9971).
func (l *lexer) heredoc(body heredoc, inFunction, substitution bool) bool {
	var lines []string
	closed, cut := false, false
	for l.pos < len(l.src) {
		start := l.pos
		end := strings.IndexByte(l.src[l.pos:], '\n')
		line := l.src[l.pos:]
		if end >= 0 {
			line = l.src[l.pos : l.pos+end]
			l.pos += end + 1
		} else {
			l.pos = len(l.src)
		}
		// The delimiter line must match byte for byte: a shell does not
		// trim a trailing carriage return, so `EOF\r` does not end a body
		// delimited by `EOF` (AFP-V0-9970).
		check := line
		if body.dash {
			check = strings.TrimLeft(check, "\t")
		}
		if check == body.delimiter {
			closed = true
			break
		}
		if substitution && strings.HasPrefix(check, body.delimiter) {
			// bash ends the body at a line that starts with the delimiter
			// and whose rest closes the substitution (`EOF)`, `EOF )`, or
			// for an empty delimiter any line holding the `)`), and reads
			// that rest as shell text.
			rest := len(line) - len(check) + len(body.delimiter)
			if unbalancedParen(line[rest:]) >= 0 {
				l.pos = start + rest
				cut = true
				break
			}
		}
		lines = append(lines, line)
	}
	if !closed {
		l.out.incomplete = true
	}
	text := strings.Join(lines, "\n")
	l.out.data = append(l.out.data, text)
	if body.expands {
		l.substitutions(text, inFunction)
	}
	return cut
}

// unbalancedParen returns the offset of the first `)` in line that no
// earlier `(` of the line opens, or -1.
func unbalancedParen(line string) int {
	depth := 0
	for at := 0; at < len(line); at++ {
		switch line[at] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return at
			}
			depth--
		}
	}
	return -1
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

// openTest reports words that open a conditional expression the lexer is
// still inside: their first word that is not a testLeaders control word is
// `[[`, in command position, and no `]]` follows it (AFP-V0-3145). A `[[`
// that is an argument (`echo [[ marker`) opens nothing, so the `&&`, `||`,
// `|` or `;` after it still ends the command.
func openTest(words []string) bool {
	at := 0
	for at < len(words) && testLeaders[words[at]] {
		at++
	}
	return at < len(words) && words[at] == "[[" && !slices.Contains(words[at+1:], "]]")
}

// testLeaders are the reserved words after which a command, and so a `[[`,
// may start within the words the lexer has gathered for one command: a
// separator (`;`, `&&`, `||`, `|`, a newline), a `{`, a `(`, a case arm's `)`
// or a command substitution already starts a new word list (AFP-V0-3145).
var testLeaders = map[string]bool{
	"if": true, "while": true, "until": true, "!": true, "time": true,
	"then": true, "do": true, "else": true, "elif": true,
}
