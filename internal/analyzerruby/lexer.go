package analyzerruby

import "strings"

// The bounded lexical states for `ruby.source`. The scanner produces an
// offset-preserving masked view in which every comment, string, heredoc,
// percent literal, regex, and `__END__` data section is blanked to spaces. Only
// that masked view may be searched for the closed token set, so an
// `RSpec.describe` spelled inside a comment, a string, a heredoc body, or a
// data section cannot become an observation.
//
// The state machine is ported from the reference Python implementation's
// `_lex`; the offset-preserving mask is what makes the closed token set
// decidable without evaluating Ruby.

const maxHeredocs = 128

// heredoc is one pending heredoc body terminator.
type heredoc struct {
	name   string
	indent bool
}

// masker carries the lexical state across lines of one Ruby source file.
type masker struct {
	src []byte
	out []byte

	heredocs     []heredoc
	quote        byte // 0, '\'', '"', '`', or '%' for a delimited literal
	percentOpen  byte
	percentClose byte
	percentDepth int
	escaped      bool
	lastCode     byte
	dataSection  bool
	blockComment bool
}

// maskRuby returns the masked code view. An unterminated string, heredoc, or
// block comment fails closed.
func maskRuby(src []byte) ([]byte, string) {
	m := &masker{src: src, out: append([]byte(nil), src...)}
	for at := 0; at < len(src); {
		stop := at
		for stop < len(src) && src[stop] != '\n' {
			stop++
		}
		next := stop
		if next < len(src) {
			next++
		}
		if why := m.line(at, stop, next); why != "" {
			return nil, why
		}
		at = next
	}
	if m.quote != 0 || len(m.heredocs) != 0 || m.blockComment {
		return nil, "MALFORMED_INPUT"
	}
	return m.out, ""
}

// blank erases a byte range from the code view, preserving line structure.
func (m *masker) blank(from, to int) {
	for i := from; i < to; i++ {
		if m.out[i] != '\n' {
			m.out[i] = ' '
		}
	}
}

// line dispatches one raw line to the state that owns it. body is [from,stop),
// excluding the line terminator; next is the start of the following line.
func (m *masker) line(from, stop, next int) string {
	// A CRLF line keeps its `\r`; Ruby matches directives and terminators
	// without it.
	body := strings.TrimSuffix(string(m.src[from:stop]), "\r")
	switch {
	case m.dataSection:
		m.blank(from, next)
		return ""
	case m.blockComment:
		m.blank(from, next)
		if body == "=end" || strings.HasPrefix(body, "=end ") || strings.HasPrefix(body, "=end\t") {
			m.blockComment = false
		}
		return ""
	case len(m.heredocs) != 0:
		return m.heredocBody(from, stop, next, body)
	case m.quote == 0 && (body == "=begin" || strings.HasPrefix(body, "=begin ") || strings.HasPrefix(body, "=begin\t")):
		m.blockComment = true
		m.blank(from, next)
		return ""
	case m.quote == 0 && strings.TrimSpace(body) == "__END__":
		m.dataSection = true
		m.blank(from, next)
		return ""
	}
	return m.code(from, stop, next)
}

// heredocBody blanks a heredoc body line and pops the terminator when it lands.
func (m *masker) heredocBody(from, stop, next int, body string) string {
	m.blank(from, next)
	terminator := body
	if m.heredocs[0].indent {
		terminator = strings.TrimSpace(body)
	}
	if terminator == m.heredocs[0].name {
		m.heredocs = m.heredocs[1:]
	}
	return ""
}

// code scans one ordinary source line, opening and closing lexical states.
func (m *masker) code(from, stop, next int) string {
	line := m.src[from:stop]
	codeEnd := len(line)
	for codeEnd > 0 && isSpace(line[codeEnd-1]) {
		codeEnd--
	}
	// The preceding-token memory is per line, matching the reference lexer: a
	// `/` opening a fresh line follows a completed statement, so it starts a
	// regex rather than continuing the previous line's division.
	m.lastCode = 0
	var pending []heredoc
	for at := 0; at < len(line); {
		if m.quote != 0 {
			at = m.continueQuote(from, line, at)
			continue
		}
		advanced, why := m.openState(from, line, at, codeEnd, &pending)
		if why != "" {
			return why
		}
		at = advanced
	}
	if len(m.heredocs)+len(pending) > maxHeredocs {
		return "LIMIT_EXCEEDED"
	}
	m.heredocs = append(m.heredocs, pending...)
	return ""
}

// continueQuote consumes one byte inside an open string, percent literal, or
// regex and closes it on the matching delimiter.
func (m *masker) continueQuote(from int, line []byte, at int) int {
	current := line[at]
	m.out[from+at] = ' '
	if m.quote == '%' {
		switch {
		case m.escaped:
			m.escaped = false
		case current == '\\':
			m.escaped = true
		case current == m.percentOpen && m.percentOpen != m.percentClose:
			m.percentDepth++
		case current == m.percentClose:
			m.percentDepth--
			if m.percentDepth == 0 {
				m.quote = 0
				m.lastCode = current
			}
		}
		return at + 1
	}
	switch {
	case m.escaped:
		m.escaped = false
	case current == '\\':
		m.escaped = true
	case current == m.quote:
		m.quote = 0
		m.lastCode = current
	}
	return at + 1
}

// openState recognizes the start of a comment, string, percent literal, regex,
// or heredoc, or advances over one ordinary code byte.
func (m *masker) openState(from int, line []byte, at, codeEnd int, pending *[]heredoc) (int, string) {
	current := line[at]
	switch {
	case current == '#':
		m.blank(from+at, from+len(line))
		return len(line), ""
	case current == '\'' || current == '"' || current == '`':
		m.quote = current
		m.escaped = false
		m.out[from+at] = ' '
		m.lastCode = current
		return at + 1, ""
	case current == '%':
		// After a value, `%=` is modulo assignment rather than a literal.
		if open, width, ok := percentLiteral(line, at); ok && (open != '=' || m.regexStart(line, at, codeEnd)) {
			m.quote = '%'
			m.percentOpen = open
			m.percentClose = closingDelimiter(open)
			m.percentDepth = 1
			m.escaped = false
			m.blank(from+at, from+at+width)
			return at + width, ""
		}
	case current == '?':
		// Where a value may start, `?"` is a character literal; after a value
		// `?` is the ternary operator (the regex position rule again).
		if width := characterLiteral(line, at); width > 0 && m.regexStart(line, at, codeEnd) {
			m.blank(from+at, from+at+width)
			m.lastCode = '\''
			return at + width, ""
		}
	case current == '/':
		if m.regexStart(line, at, codeEnd) {
			m.quote = '%'
			m.percentOpen, m.percentClose = '/', '/'
			m.percentDepth = 1
			m.escaped = false
			m.out[from+at] = ' '
			return at + 1, ""
		}
	case current == '<' && at+1 < len(line) && line[at+1] == '<':
		if name, indent, width, ok := heredocStart(line, at); ok && heredocPosition(line, at) {
			if len(m.heredocs)+len(*pending)+1 > maxHeredocs {
				return 0, "LIMIT_EXCEEDED"
			}
			*pending = append(*pending, heredoc{name, indent})
			return at + width, ""
		}
	}
	if !isSpace(current) {
		m.lastCode = current
	}
	return at + 1, ""
}

// regexStart decides whether `/` opens a regex literal or is division. The
// decision is lexical: a regex may only start where a value may start.
func (m *masker) regexStart(line []byte, at, codeEnd int) bool {
	if m.lastCode == 0 || strings.IndexByte("=([{,:;!&|?~", m.lastCode) >= 0 {
		return true
	}
	lookbehind := strings.TrimRight(string(line[:at]), " \t")
	if endsWithAnyWord(lookbehind, regexKeywords) || endsWithAnyWord(lookbehind, regexCommands) {
		return true
	}
	if endsWithReceiverCall(lookbehind) {
		return true
	}
	if strings.HasSuffix(lookbehind, "=~") || strings.HasSuffix(lookbehind, "!~") || strings.HasSuffix(lookbehind, "=>") {
		return true
	}
	return at > 0 && isSpace(line[at-1]) && at == codeEnd-1
}

var regexKeywords = []string{"and", "if", "not", "or", "return", "then", "unless", "until", "when", "while", "yield"}
var regexCommands = []string{"abort", "assert_match", "fail", "p", "print", "puts", "raise", "refute_match", "warn"}
var regexReceivers = []string{"gsub", "match", "scan", "split", "sub"}

func endsWithAnyWord(text string, words []string) bool {
	for _, word := range words {
		if !strings.HasSuffix(text, word) {
			continue
		}
		if len(text) == len(word) || isSpace(text[len(text)-len(word)-1]) {
			return true
		}
	}
	return false
}

// endsWithReceiverCall matches `.gsub`, `.sub!`, `.match?` and friends, whose
// argument position admits a regex literal.
func endsWithReceiverCall(text string) bool {
	trimmed := strings.TrimRight(text, "!?")
	for _, name := range regexReceivers {
		if strings.HasSuffix(trimmed, "."+name) {
			return true
		}
	}
	return false
}

// characterLiteral returns the width of a `?c` or `?\\c` literal whose
// character is neither whitespace nor an identifier byte, or zero.
func characterLiteral(line []byte, at int) int {
	next := at + 1
	if next >= len(line) || isSpace(line[next]) || identifierByte(line[next]) {
		return 0
	}
	if line[next] != '\\' {
		return 2
	}
	if next+1 >= len(line) {
		return 0
	}
	return 3
}

// heredocPosition rejects a `<<` glued to a preceding value (`a<<b`, `)<<b`),
// which Ruby lexes as a shift; a glued keyword (`return<<A`) still opens one.
func heredocPosition(line []byte, at int) bool {
	if at == 0 {
		return true
	}
	if !identifierByte(line[at-1]) && strings.IndexByte(")]}\"'`", line[at-1]) < 0 {
		return true
	}
	return endsWithAnyWord(string(line[:at]), regexKeywords)
}

// percentLiteral matches `%`, an optional type letter, and a delimiter that is
// neither alphanumeric nor whitespace.
func percentLiteral(line []byte, at int) (byte, int, bool) {
	cursor := at + 1
	if cursor < len(line) && strings.IndexByte("qQwWiIxrs", line[cursor]) >= 0 {
		cursor++
	}
	if cursor >= len(line) {
		return 0, 0, false
	}
	delimiter := line[cursor]
	if alnum(delimiter) || isSpace(delimiter) {
		return 0, 0, false
	}
	return delimiter, cursor + 1 - at, true
}

func closingDelimiter(open byte) byte {
	switch open {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	case '<':
		return '>'
	}
	return open
}

// heredocStart matches `<<`, an optional `-`/`~` indent flag, an optionally
// quoted identifier, and returns the terminator plus the consumed width.
func heredocStart(line []byte, at int) (string, bool, int, bool) {
	cursor := at + 2
	indent := false
	if cursor < len(line) && (line[cursor] == '-' || line[cursor] == '~') {
		indent = true
		cursor++
	}
	var quote byte
	if cursor < len(line) && (line[cursor] == '\'' || line[cursor] == '"' || line[cursor] == '`') {
		quote = line[cursor]
		cursor++
	}
	start := cursor
	if cursor >= len(line) || !identifierStart(line[cursor]) {
		return "", false, 0, false
	}
	for cursor < len(line) && identifierByte(line[cursor]) {
		cursor++
	}
	name := string(line[start:cursor])
	if quote != 0 {
		if cursor >= len(line) || line[cursor] != quote {
			return "", false, 0, false
		}
		cursor++
	}
	return name, indent, cursor - at, true
}

func identifierStart(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '_'
}

func identifierByte(b byte) bool { return identifierStart(b) || b >= '0' && b <= '9' }

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}
