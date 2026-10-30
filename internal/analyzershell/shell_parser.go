package analyzershell

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// factCollector is shared by every input in one request. Limits are enforced
// before retaining a fact, so a hostile first input cannot grow an unbounded
// intermediate slice before the response is rejected.
type factCollector struct {
	r          Request
	facts      []Fact
	seen       map[Fact]struct{}
	outputSize int
}

func newFactCollector(r Request) *factCollector {
	return &factCollector{r: r, facts: make([]Fact, 0, 16), seen: make(map[Fact]struct{}, 16), outputSize: emptySuccessSize(r) + 1}
}

func (c *factCollector) add(in Input, kind, predicate, value string) string {
	f := Fact{Kind: kind, InputHandle: in.Handle, RelatedHandle: "-", Subject: in.Path, Predicate: predicate, Value: value, InstanceID: in.Path}
	if !factFieldsOK(f, false) {
		return "LIMIT_EXCEEDED"
	}
	f.EvidenceSHA256 = evidence(c.r, in, f)
	if !factFieldsOK(f, true) {
		return "LIMIT_EXCEEDED"
	}
	if _, ok := c.seen[f]; ok {
		return "DUPLICATE_VALUE"
	}
	if overBound(len(c.facts)+1, MaxFacts) {
		return "LIMIT_EXCEEDED"
	}
	next := c.outputSize + factJSONSize(f)
	if len(c.facts) != 0 {
		next++
	}
	if overBound(next, MaxOutputBytes) {
		return "OUTPUT_LIMIT"
	}
	c.facts = append(c.facts, f)
	c.seen[f] = struct{}{}
	c.outputSize = next
	return ""
}

func factFieldsOK(f Fact, includeEvidence bool) bool {
	if !factFieldOK(f.Kind) || !factFieldOK(f.InputHandle) || !factFieldOK(f.RelatedHandle) || !factFieldOK(f.Subject) || !factFieldOK(f.Predicate) || !factFieldOK(f.Value) || !factFieldOK(f.InstanceID) {
		return false
	}
	return !includeEvidence || factFieldOK(f.EvidenceSHA256)
}

func factFieldOK(field string) bool {
	if len(field) == 0 || overBound(len(field), MaxFactFieldBytes) {
		return false
	}
	for _, value := range []byte(field) {
		if value < ' ' || value > '~' {
			return false
		}
	}
	return true
}

type shellTokenKind uint8

const (
	shellWordToken shellTokenKind = iota
	shellNewline
	shellSemicolon
	shellAmpersand
	shellAndIf
	shellOrIf
	shellPipe
	shellPipeError
	shellRedirectIn
	shellRedirectOut
	shellRedirectAppend
	shellRedirectReadWrite
	shellRedirectClobber
	shellHereDoc
	shellHereDocTabs
	shellLeftBrace
	shellRightBrace
)

type shellWord struct {
	text          string
	raw           []byte
	variables     []string
	entry         bool
	expanded      bool
	dynamic       bool
	quoted        bool
	escaped       bool
	equal         int
	equalPlain    bool
	prefixPlain   bool
	containsSpace bool
}

func newShellWord() shellWord {
	return shellWord{equal: -1, prefixPlain: true, raw: make([]byte, 0, 32)}
}

func (w *shellWord) literal(value byte, quoted, escaped bool) {
	if escaped {
		w.escaped = true
	}
	if value == '=' && !escaped && w.equal < 0 {
		w.equal = len(w.raw)
		w.equalPlain = w.prefixPlain && !quoted
	}
	if quoted {
		w.quoted = true
		if w.equal < 0 {
			w.prefixPlain = false
		}
	}
	if escaped && w.equal < 0 {
		w.prefixPlain = false
	}
	if value == ' ' || value == '\t' {
		w.containsSpace = true
	}
	w.raw = append(w.raw, value)
}

func (w *shellWord) expansion(name string, entry bool) {
	if w.equal < 0 {
		w.prefixPlain = false
	}
	w.expanded = true
	if entry {
		w.entry = true
		return
	}
	w.variables = append(w.variables, name)
	w.raw = append(w.raw, 0)
}

type shellToken struct {
	kind shellTokenKind
	word shellWord
	line int
}

type hereDoc struct {
	delimiter       string
	stripTabs       bool
	quotedOrEscaped bool
}

// shellLexer is deliberately small, but it is a token stream rather than a
// word splitter: every quote and escape is resolved before operators can be
// structural, and every unsupported expansion remains terminal.
type shellLexer struct {
	source     string
	position   int
	line       int
	tokens     []shellToken
	pending    *hereDoc
	heredocs   []hereDoc
	tokenCount int
}

func lexShell(body []byte) ([]shellToken, string) {
	l := shellLexer{source: string(body), line: 1, tokens: make([]shellToken, 0, 32)}
	for l.position < len(l.source) {
		if overBound(l.tokenCount, 65_536) {
			return nil, "LIMIT_EXCEEDED"
		}
		c := l.source[l.position]
		if c == ' ' || c == '\t' {
			l.position++
			continue
		}
		if c == '\n' {
			if l.pending != nil {
				return nil, "MALFORMED_INPUT"
			}
			l.add(shellNewline, shellWord{})
			l.position++
			l.line++
			if why := l.consumeHereDocs(); why != "" {
				return nil, why
			}
			continue
		}
		if l.pending != nil && (c == '#' || strings.ContainsRune(";|&<>{}()", rune(c))) {
			return nil, "MALFORMED_INPUT"
		}
		if c == '#' {
			for l.position < len(l.source) && l.source[l.position] != '\n' {
				l.position++
			}
			continue
		}
		if why, structural := l.operator(); structural {
			if why != "" {
				return nil, why
			}
			continue
		}
		if l.pending != nil && c == '$' {
			return nil, "DYNAMIC_INPUT"
		}
		word, why := l.readWord()
		if why != "" {
			return nil, why
		}
		word.finish()
		if word.text == "" && !word.expanded {
			return nil, "MALFORMED_INPUT"
		}
		l.add(shellWordToken, word)
		if l.pending != nil {
			if word.expanded || word.dynamic || word.text == "" || len(word.text) > 128 || strings.ContainsAny(word.text, "`\\$;|&<>(){}") {
				return nil, "DYNAMIC_INPUT"
			}
			spec := *l.pending
			spec.delimiter = word.text
			spec.quotedOrEscaped = word.quoted || word.escaped
			if !spec.stripTabs && !spec.quotedOrEscaped {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			l.heredocs = append(l.heredocs, spec)
			l.pending = nil
		}
	}
	if l.pending != nil || len(l.heredocs) != 0 {
		return nil, "MALFORMED_INPUT"
	}
	return l.tokens, ""
}

func (l *shellLexer) add(kind shellTokenKind, word shellWord) {
	l.tokens = append(l.tokens, shellToken{kind: kind, word: word, line: l.line})
	l.tokenCount++
}

func (l *shellLexer) operator() (string, bool) {
	c := l.source[l.position]
	next := byte(0)
	if l.position+1 < len(l.source) {
		next = l.source[l.position+1]
	}
	switch c {
	case ';':
		if next == ';' || next == '&' {
			return "UNSUPPORTED_SCHEMA", true
		}
		l.add(shellSemicolon, shellWord{})
		l.position++
		return "", true
	case '&':
		if next == '&' {
			l.add(shellAndIf, shellWord{})
			l.position += 2
		} else {
			l.add(shellAmpersand, shellWord{})
			l.position++
		}
		return "", true
	case '|':
		if next == '|' {
			l.add(shellOrIf, shellWord{})
			l.position += 2
		} else if next == '&' {
			l.add(shellPipeError, shellWord{})
			l.position += 2
		} else {
			l.add(shellPipe, shellWord{})
			l.position++
		}
		return "", true
	case '<':
		if next == '<' {
			l.position += 2
			kind := shellHereDoc
			strip := false
			if l.position < len(l.source) && l.source[l.position] == '-' {
				kind, strip = shellHereDocTabs, true
				l.position++
			}
			l.add(kind, shellWord{})
			l.pending = &hereDoc{stripTabs: strip}
			return "", true
		}
		if next == '&' {
			return "UNSUPPORTED_SCHEMA", true
		}
		if next == '>' {
			l.add(shellRedirectReadWrite, shellWord{})
			l.position += 2
		} else {
			l.add(shellRedirectIn, shellWord{})
			l.position++
		}
		return "", true
	case '>':
		if next == '&' {
			return "UNSUPPORTED_SCHEMA", true
		}
		if next == '>' {
			l.add(shellRedirectAppend, shellWord{})
			l.position += 2
		} else if next == '|' {
			l.add(shellRedirectClobber, shellWord{})
			l.position += 2
		} else {
			l.add(shellRedirectOut, shellWord{})
			l.position++
		}
		return "", true
	case '{':
		l.add(shellLeftBrace, shellWord{})
		l.position++
		return "", true
	case '}':
		l.add(shellRightBrace, shellWord{})
		l.position++
		return "", true
	case '(', ')', '`':
		return "UNSUPPORTED_SCHEMA", true
	}
	return "", false
}

func (l *shellLexer) readWord() (shellWord, string) {
	w := newShellWord()
	quote := byte(0)
	for l.position < len(l.source) {
		c := l.source[l.position]
		if quote == 0 {
			if len(w.raw) == 0 && (strings.HasPrefix(l.source[l.position:], "[[") || strings.HasPrefix(l.source[l.position:], "]]")) {
				end := l.position + 2
				if end < len(l.source) && !shellWordBoundary(l.source[end]) {
					return shellWord{}, "DYNAMIC_INPUT"
				}
				w.literal(c, false, false)
				w.literal(c, false, false)
				l.position = end
				continue
			}
			if c == ' ' || c == '\t' || c == '\n' || strings.ContainsRune(";|&<>{}()", rune(c)) {
				break
			}
			if c == '#' && len(w.raw) == 0 {
				break
			}
			switch c {
			case '\'', '"':
				quote = c
				w.quoted = true
				if w.equal < 0 {
					w.prefixPlain = false
				}
				l.position++
			case '\\':
				if l.position+1 >= len(l.source) || l.source[l.position+1] == '\n' {
					return shellWord{}, "MALFORMED_INPUT"
				}
				l.position++
				w.literal(l.source[l.position], false, true)
				l.position++
			case '$':
				if why := l.expansion(&w); why != "" {
					return shellWord{}, why
				}
			default:
				if c == '[' || c == ']' {
					next := byte(0)
					if l.position+1 < len(l.source) {
						next = l.source[l.position+1]
					}
					if len(w.raw) == 0 && (next == ' ' || next == '\t' || next == '\n' || strings.ContainsRune(";|&<>{}()", rune(next))) {
						w.literal(c, false, false)
						l.position++
						continue
					}
				}
				if strings.ContainsRune("*?[]~", rune(c)) {
					return shellWord{}, "DYNAMIC_INPUT"
				}
				w.literal(c, false, false)
				l.position++
			}
			continue
		}
		if c == '\n' {
			return shellWord{}, "MALFORMED_INPUT"
		}
		if c == quote {
			quote = 0
			l.position++
			continue
		}
		if quote == '\'' {
			w.literal(c, true, false)
			l.position++
			continue
		}
		if c == '`' {
			return shellWord{}, "DYNAMIC_INPUT"
		}
		if c == '\\' {
			if l.position+1 >= len(l.source) {
				return shellWord{}, "MALFORMED_INPUT"
			}
			next := l.source[l.position+1]
			if next == '\n' {
				l.position += 2
				l.line++
				continue
			}
			if strings.ContainsRune("$`\"\\", rune(next)) {
				w.literal(next, true, true)
				l.position += 2
				continue
			}
			// POSIX preserves a backslash in double quotes unless it precedes
			// $, `, \", \\, or a newline. Keeping it literal prevents a
			// static path such as lib\\q.sh from being forged to libq.sh.
			w.literal('\\', true, false)
			l.position++
			continue
		}
		if c == '$' {
			if why := l.expansion(&w); why != "" {
				return shellWord{}, why
			}
			continue
		}
		w.literal(c, true, false)
		l.position++
	}
	if quote != 0 {
		return shellWord{}, "MALFORMED_INPUT"
	}
	w.finish()
	return w, ""
}

func shellWordBoundary(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || strings.ContainsRune(";|&<>{}()", rune(c))
}

func (w *shellWord) finish() {
	if w.text == "" && len(w.raw) != 0 {
		w.text = string(w.raw)
		w.raw = nil
	}
}

func (l *shellLexer) expansion(w *shellWord) string {
	if l.position+1 >= len(l.source) {
		return "DYNAMIC_INPUT"
	}
	l.position++
	c := l.source[l.position]
	if c == '(' || c == '`' {
		return "DYNAMIC_INPUT"
	}
	if c == '{' {
		l.position++
		start := l.position
		for l.position < len(l.source) && isShellNamePart(l.source[l.position]) {
			l.position++
		}
		if start == l.position || l.position >= len(l.source) || l.source[l.position] != '}' || !isShellNameStart(l.source[start]) {
			return "DYNAMIC_INPUT"
		}
		w.expansion(l.source[start:l.position], false)
		l.position++
		return ""
	}
	if c == '0' {
		w.expansion("", true)
		l.position++
		return ""
	}
	if !isShellNameStart(c) {
		return "DYNAMIC_INPUT"
	}
	start := l.position
	for l.position < len(l.source) && isShellNamePart(l.source[l.position]) {
		l.position++
	}
	w.expansion(l.source[start:l.position], false)
	return ""
}

func isShellNameStart(c byte) bool { return c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }
func isShellNamePart(c byte) bool  { return isShellNameStart(c) || c >= '0' && c <= '9' }

func nameOK(value string) bool {
	if value == "" || len(value) > 128 || !isShellNameStart(value[0]) {
		return false
	}
	for i := 1; i < len(value); i++ {
		if !isShellNamePart(value[i]) {
			return false
		}
	}
	return true
}

func commandOK(value string) bool {
	if value == "" || len(value) > 128 || !((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) {
		return false
	}
	for _, c := range []byte(value[1:]) {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '+' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

func signalOK(value string) bool {
	switch value {
	case "EXIT", "ERR", "HUP", "INT", "QUIT", "TERM":
		return true
	}
	return false
}

func (l *shellLexer) consumeHereDocs() string {
	for _, spec := range l.heredocs {
		matched := false
		for l.position < len(l.source) {
			end := strings.IndexByte(l.source[l.position:], '\n')
			if end < 0 {
				return "MALFORMED_INPUT"
			}
			end += l.position
			line := l.source[l.position:end]
			comparison := line
			if spec.stripTabs {
				comparison = strings.TrimLeft(comparison, "\t")
			}
			l.position = end + 1
			l.line++
			if comparison == spec.delimiter {
				matched = true
				break
			}
		}
		if !matched {
			return "MALFORMED_INPUT"
		}
	}
	l.heredocs = l.heredocs[:0]
	return ""
}

type shellParser struct {
	tokens    []shellToken
	position  int
	in        Input
	r         Request
	collector *factCollector
	bash      bool
}

func parseInto(in Input, body []byte, r Request, collector *factCollector) string {
	if in.Family != "shell.posix-bash" || !(strings.HasSuffix(in.Path, ".sh") || strings.HasSuffix(in.Path, ".bash")) {
		return "UNSUPPORTED_SCHEMA"
	}
	if len(body) == 0 || overBound(len(body), MaxInputBytes) || !utf8.Valid(body) || bytes.ContainsAny(body, "\x00\r\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f") {
		return "MALFORMED_INPUT"
	}
	tokens, why := lexShell(body)
	if why != "" {
		return why
	}
	p := shellParser{tokens: tokens, in: in, r: r, collector: collector, bash: len(r.Target.Features) == 1 && r.Target.Features[0] == "bash-5.2"}
	_, why = p.sequence(nil)
	return why
}

// parse is retained for narrow package tests; production uses the request-wide
// collector above so limits remain prospective across all inputs.
func parse(in Input, body []byte, r Request) ([]Fact, string) {
	c := newFactCollector(r)
	why := parseInto(in, body, r, c)
	return c.facts, why
}

func (p *shellParser) sequence(stops map[string]bool) (bool, string) {
	parsed := false
	requiresCommand := false
	for {
		for p.at(shellNewline) {
			if requiresCommand {
				return parsed, "MALFORMED_INPUT"
			}
			p.position++
		}
		if p.atStop(stops) {
			if requiresCommand {
				return parsed, "MALFORMED_INPUT"
			}
			return parsed, ""
		}
		if p.position >= len(p.tokens) {
			if requiresCommand {
				return parsed, "MALFORMED_INPUT"
			}
			return parsed, ""
		}
		if p.atListOperator() || p.at(shellRightBrace) {
			return parsed, "MALFORMED_INPUT"
		}
		var why string
		if p.at(shellLeftBrace) {
			p.position++
			var group bool
			if group, why = p.sequence(map[string]bool{"}": true}); why == "" {
				if !group || !p.groupTerminated() || !p.at(shellRightBrace) {
					why = "MALFORMED_INPUT"
				} else {
					p.position++
				}
			}
		} else if p.atWord("if") {
			why = p.ifClause()
		} else if p.atReservedControl() {
			why = "UNSUPPORTED_SCHEMA"
		} else {
			why = p.simpleCommand()
		}
		if why != "" {
			return parsed, why
		}
		parsed = true
		requiresCommand = false
		if p.position >= len(p.tokens) || p.atStop(stops) {
			return parsed, ""
		}
		if p.at(shellNewline) {
			continue
		}
		if !p.atListOperator() {
			return parsed, "MALFORMED_INPUT"
		}
		op := p.tokens[p.position].kind
		p.position++
		requiresCommand = op == shellAndIf || op == shellOrIf || op == shellPipe || op == shellPipeError
		if op == shellPipeError {
			return parsed, "UNSUPPORTED_SCHEMA"
		}
	}
}

func (p *shellParser) groupTerminated() bool {
	if p.position == 0 {
		return false
	}
	switch p.tokens[p.position-1].kind {
	case shellNewline, shellSemicolon, shellAmpersand:
		return true
	}
	return false
}

func (p *shellParser) ifClause() string {
	p.position++
	condition, why := p.sequence(map[string]bool{"then": true})
	if why != "" || !condition || !p.atWord("then") {
		return firstShellError(why, "MALFORMED_INPUT")
	}
	p.position++
	thenList, why := p.sequence(map[string]bool{"elif": true, "else": true, "fi": true})
	if why != "" || !thenList {
		return firstShellError(why, "MALFORMED_INPUT")
	}
	if p.atWord("elif") {
		return p.elifClause()
	}
	if p.atWord("else") {
		p.position++
		elseList, elseWhy := p.sequence(map[string]bool{"fi": true})
		if elseWhy != "" || !elseList {
			return firstShellError(elseWhy, "MALFORMED_INPUT")
		}
	}
	if !p.atWord("fi") {
		return "MALFORMED_INPUT"
	}
	p.position++
	return ""
}

func (p *shellParser) elifClause() string {
	p.position++
	condition, why := p.sequence(map[string]bool{"then": true})
	if why != "" || !condition || !p.atWord("then") {
		return firstShellError(why, "MALFORMED_INPUT")
	}
	p.position++
	thenList, why := p.sequence(map[string]bool{"elif": true, "else": true, "fi": true})
	if why != "" || !thenList {
		return firstShellError(why, "MALFORMED_INPUT")
	}
	if p.atWord("elif") {
		return p.elifClause()
	}
	if p.atWord("else") {
		p.position++
		elseList, elseWhy := p.sequence(map[string]bool{"fi": true})
		if elseWhy != "" || !elseList {
			return firstShellError(elseWhy, "MALFORMED_INPUT")
		}
	}
	if !p.atWord("fi") {
		return "MALFORMED_INPUT"
	}
	p.position++
	return ""
}

func firstShellError(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func (p *shellParser) simpleCommand() string {
	words := make([]shellWord, 0, 8)
	for p.position < len(p.tokens) && !p.at(shellNewline) && !p.atListOperator() && !p.at(shellRightBrace) && !p.atStop(nil) {
		t := p.tokens[p.position]
		switch t.kind {
		case shellWordToken:
			words = append(words, t.word)
			p.position++
		case shellRedirectIn, shellRedirectOut, shellRedirectAppend, shellRedirectReadWrite, shellRedirectClobber, shellHereDoc, shellHereDocTabs:
			p.position++
			if p.position >= len(p.tokens) || !p.at(shellWordToken) {
				return "MALFORMED_INPUT"
			}
			target := p.tokens[p.position].word
			p.position++
			if target.expanded || target.dynamic || target.text == "" {
				return "DYNAMIC_INPUT"
			}
		default:
			return "UNSUPPORTED_SCHEMA"
		}
	}
	if len(words) == 0 {
		return "MALFORMED_INPUT"
	}
	commandIndex := 0
	for commandIndex < len(words) {
		if _, ok := shellAssignment(words[commandIndex]); !ok {
			break
		}
		commandIndex++
	}
	if commandIndex < len(words) && shellReservedControl(words[commandIndex]) {
		return "UNSUPPORTED_SCHEMA"
	}
	for _, word := range words {
		if word.dynamic {
			return "DYNAMIC_INPUT"
		}
		if word.entry {
			if why := p.collector.add(p.in, "shell.script.entry", "declares-entry", "main"); why != "" {
				return why
			}
		}
		for _, variable := range word.variables {
			if !nameOK(variable) {
				return "DYNAMIC_INPUT"
			}
			if why := p.collector.add(p.in, "shell.env.read", "reads", variable); why != "" {
				return why
			}
		}
	}
	first := 0
	for first < len(words) {
		name, ok := shellAssignment(words[first])
		if !ok {
			break
		}
		if why := p.collector.add(p.in, "shell.env.write", "writes", name); why != "" {
			return why
		}
		first++
	}
	if first == len(words) {
		return ""
	}
	command := words[first]
	if command.expanded || command.dynamic || command.text == "" {
		return "DYNAMIC_INPUT"
	}
	args := words[first+1:]
	switch command.text {
	case ".", "source":
		if len(args) != 1 || args[0].expanded || args[0].dynamic || !pathOK(args[0].text) {
			return "DYNAMIC_INPUT"
		}
		if command.text == "source" && !p.bash {
			return "UNSUPPORTED_SCHEMA"
		}
		return p.collector.add(p.in, "shell.import.static", "imports", args[0].text)
	case "trap":
		if len(args) != 2 || !args[0].quoted || args[0].expanded || args[0].dynamic || !signalOK(args[1].text) {
			return "UNSUPPORTED_SCHEMA"
		}
		return p.collector.add(p.in, "shell.trap.static", "traps", args[1].text)
	case "eval", "exec", "command", "builtin":
		return "DYNAMIC_INPUT"
	case "local", "declare", "typeset":
		if !p.bash {
			return "UNSUPPORTED_SCHEMA"
		}
		fallthrough
	case "export", "readonly":
		for _, arg := range args {
			name := arg.text
			if assignment, ok := shellAssignment(arg); ok {
				name = assignment
			}
			if !nameOK(name) {
				return "UNSUPPORTED_SCHEMA"
			}
			if why := p.collector.add(p.in, "shell.env.write", "writes", name); why != "" {
				return why
			}
		}
		return ""
	case "unset":
		for _, arg := range args {
			if !plainShellIdentifier(arg) {
				return "UNSUPPORTED_SCHEMA"
			}
			if why := p.collector.add(p.in, "shell.env.write", "writes", arg.text); why != "" {
				return why
			}
		}
		return ""
	case "[", "test", "true", "false", "return", "break", "continue", "set", "shift", "getopts":
		return ""
	default:
		if !commandOK(command.text) {
			return "DYNAMIC_INPUT"
		}
		return p.collector.add(p.in, "shell.command.static", "invokes", command.text)
	}
}

func shellAssignment(word shellWord) (string, bool) {
	if word.equal <= 0 || !word.equalPlain || !word.prefixPlain {
		return "", false
	}
	name := word.text[:word.equal]
	return name, nameOK(name)
}

func plainShellIdentifier(word shellWord) bool {
	return nameOK(word.text) && word.equal < 0 && !word.quoted && !word.escaped && !word.expanded && !word.dynamic && word.prefixPlain
}

func (p *shellParser) at(kind shellTokenKind) bool {
	return p.position < len(p.tokens) && p.tokens[p.position].kind == kind
}

func (p *shellParser) atWord(value string) bool {
	return p.position < len(p.tokens) && p.tokens[p.position].kind == shellWordToken && p.tokens[p.position].word.controlWord(value)
}

func (p *shellParser) atReservedControl() bool {
	return p.position < len(p.tokens) && p.tokens[p.position].kind == shellWordToken && shellReservedControl(p.tokens[p.position].word)
}

func (p *shellParser) atStop(stops map[string]bool) bool {
	if len(stops) == 0 || p.position >= len(p.tokens) {
		return false
	}
	if p.tokens[p.position].kind == shellRightBrace {
		return stops["}"]
	}
	if p.tokens[p.position].kind != shellWordToken {
		return false
	}
	for value := range stops {
		if p.tokens[p.position].word.controlWord(value) {
			return true
		}
	}
	return false
}

func (w shellWord) controlWord(value string) bool {
	return w.text == value && !w.quoted && !w.expanded && !w.dynamic && w.prefixPlain
}

func shellReservedControl(word shellWord) bool {
	if !word.controlWord(word.text) {
		return false
	}
	switch word.text {
	case "!", "case", "coproc", "do", "done", "elif", "else", "esac", "fi", "for", "function", "if", "in", "select", "then", "time", "until", "while", "[[", "]]":
		return true
	}
	return false
}

func (p *shellParser) atListOperator() bool {
	if p.position >= len(p.tokens) {
		return false
	}
	switch p.tokens[p.position].kind {
	case shellSemicolon, shellAmpersand, shellAndIf, shellOrIf, shellPipe, shellPipeError:
		return true
	}
	return false
}

// shellWords is a compatibility helper for the focused lexer tests. The
// production parser above consumes tokens directly and never calls it.
func shellWords(line string) ([]string, string, string) {
	tokens, why := lexShell([]byte(line + "\n"))
	if why != "" {
		return nil, "", why
	}
	words := make([]string, 0, len(tokens))
	for _, token := range tokens {
		switch token.kind {
		case shellWordToken:
			words = append(words, token.word.text)
		case shellNewline:
			return words, "", ""
		case shellSemicolon:
			words = append(words, ";")
		case shellAmpersand:
			words = append(words, "&")
		case shellAndIf:
			words = append(words, "&&")
		case shellOrIf:
			words = append(words, "||")
		case shellPipe:
			words = append(words, "|")
		default:
			return nil, "", fmt.Sprintf("UNSUPPORTED_SCHEMA")
		}
	}
	return words, "", ""
}
