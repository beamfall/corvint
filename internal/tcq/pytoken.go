package tcq

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// errPythonGrammar is the TCQ-V0-013 refusal: source the frozen Python 3.9
// grammar does not accept. There is no fallback host parse.
var errPythonGrammar = errors.New("unsupported-python-grammar")

// errPythonOffset is the TCQ-V0-047 refusal: the source parsed, but a name,
// docstring, body, or function boundary did not land on the expected token
// boundary, so no span may be published for that edge.
var errPythonOffset = errors.New("python-offset-mismatch")

type pyKind int

const (
	pyName pyKind = iota
	pyNumber
	pyString
	pyOp
)

// pyToken carries original-source UTF-8 byte offsets, as TCQ-V0-047 requires of
// every published span.
type pyToken struct {
	kind  pyKind
	text  string
	start int
	end   int
}

// pyLine is one logical line: a physical-line run joined across bracket nesting,
// explicit backslash continuations, and multi-line strings.
type pyLine struct {
	indent int
	tokens []pyToken
}

func (line pyLine) start() int { return line.tokens[0].start }
func (line pyLine) end() int   { return line.tokens[len(line.tokens)-1].end }

// tokenizePython produces the logical lines of one UTF-8 Python blob. It is a
// bounded lexical scan, not a parser: everything TCQ needs from Python 3.9
// source (function boundaries, docstring spans, statement shape) is decidable
// from tokens plus indentation, and anything it cannot tokenize abstains rather
// than guessing.
func tokenizePython(data []byte) ([]pyLine, error) {
	if !utf8.Valid(data) {
		return nil, errPythonGrammar
	}
	scanner := &pyScanner{data: data, line: 1}
	if err := scanner.run(); err != nil {
		return nil, err
	}
	return scanner.lines, nil
}

type pyScanner struct {
	data    []byte
	pos     int
	line    int
	depth   int
	lines   []pyLine
	current []pyToken
	indent  int
	tokens  int
}

func (scanner *pyScanner) run() error {
	for scanner.pos < len(scanner.data) {
		if len(scanner.current) == 0 && scanner.depth == 0 {
			if scanner.skipBlankLine() {
				continue
			}
		}
		if err := scanner.step(); err != nil {
			return err
		}
	}
	if scanner.depth != 0 {
		return errPythonGrammar
	}
	scanner.flush()
	return nil
}

// skipBlankLine consumes one physical line that carries no token, so blank and
// comment-only lines never affect the indentation structure.
func (scanner *pyScanner) skipBlankLine() bool {
	indent, cursor := scanner.measureIndent()
	if cursor < len(scanner.data) && scanner.data[cursor] != '\n' && scanner.data[cursor] != '\r' && scanner.data[cursor] != '#' {
		scanner.indent = indent
		scanner.pos = cursor
		return false
	}
	scanner.pos = cursor
	scanner.skipToLineEnd()
	return true
}

func (scanner *pyScanner) measureIndent() (int, int) {
	width, cursor := 0, scanner.pos
	for cursor < len(scanner.data) {
		switch scanner.data[cursor] {
		case ' ':
			width++
		case '\t':
			width = width + 8 - width%8
		case '\f':
			width = 0
		default:
			return width, cursor
		}
		cursor++
	}
	return width, cursor
}

func (scanner *pyScanner) skipToLineEnd() {
	for scanner.pos < len(scanner.data) && scanner.data[scanner.pos] != '\n' {
		scanner.pos++
	}
	if scanner.pos < len(scanner.data) {
		scanner.pos++
		scanner.line++
	}
}

func (scanner *pyScanner) step() error {
	current := scanner.data[scanner.pos]
	switch {
	case current == ' ' || current == '\t' || current == '\f' || current == '\r':
		scanner.pos++
		return nil
	case current == '\n':
		return scanner.endPhysicalLine()
	case current == '#':
		scanner.skipComment()
		return nil
	case current == '\\':
		return scanner.continuation()
	case isPythonStringStart(scanner.data, scanner.pos):
		return scanner.readString()
	case isPythonIdentStart(current):
		return scanner.readNameOrPrefixedString()
	case current >= '0' && current <= '9':
		return scanner.readNumber()
	case current == '.' && scanner.pos+1 < len(scanner.data) && isDigit(scanner.data[scanner.pos+1]):
		return scanner.readNumber()
	default:
		return scanner.readOperator()
	}
}

func (scanner *pyScanner) endPhysicalLine() error {
	scanner.pos++
	scanner.line++
	if scanner.depth == 0 {
		scanner.flush()
	}
	return nil
}

func (scanner *pyScanner) skipComment() {
	for scanner.pos < len(scanner.data) && scanner.data[scanner.pos] != '\n' {
		scanner.pos++
	}
}

func (scanner *pyScanner) continuation() error {
	next := scanner.pos + 1
	if next < len(scanner.data) && scanner.data[next] == '\r' {
		next++
	}
	if next >= len(scanner.data) || scanner.data[next] != '\n' {
		return errPythonGrammar
	}
	scanner.pos = next + 1
	scanner.line++
	return nil
}

func (scanner *pyScanner) flush() {
	if len(scanner.current) == 0 {
		return
	}
	scanner.lines = append(scanner.lines, pyLine{indent: scanner.indent, tokens: scanner.current})
	scanner.current = nil
}

func (scanner *pyScanner) emit(kind pyKind, start, end int) error {
	scanner.tokens++
	if scanner.tokens > maxPythonNodes {
		return fail(CodeResourceExhausted)
	}
	scanner.current = append(scanner.current, pyToken{
		kind: kind, text: string(scanner.data[start:end]), start: start, end: end,
	})
	return nil
}

// readNameOrPrefixedString resolves the one ambiguity in Python's lexical
// grammar that matters here: an identifier-looking run may be a string prefix
// (`rb"..."`), in which case the whole literal is one STRING token.
func (scanner *pyScanner) readNameOrPrefixedString() error {
	start := scanner.pos
	cursor := scanner.pos
	for cursor < len(scanner.data) && isPythonIdentByte(scanner.data[cursor]) {
		cursor++
	}
	word := string(scanner.data[start:cursor])
	if cursor < len(scanner.data) && isQuote(scanner.data[cursor]) && isStringPrefix(word) {
		scanner.pos = cursor
		return scanner.readStringFrom(start)
	}
	scanner.pos = cursor
	return scanner.emit(pyName, start, cursor)
}

func (scanner *pyScanner) readString() error { return scanner.readStringFrom(scanner.pos) }

func (scanner *pyScanner) readStringFrom(start int) error {
	quote := scanner.data[scanner.pos]
	length := 1
	if scanner.pos+2 < len(scanner.data) && scanner.data[scanner.pos+1] == quote && scanner.data[scanner.pos+2] == quote {
		length = 3
	}
	cursor := scanner.pos + length
	closing := strings.Repeat(string(quote), length)
	for cursor < len(scanner.data) {
		if scanner.data[cursor] == '\\' {
			if cursor+1 >= len(scanner.data) {
				return errPythonGrammar
			}
			if scanner.data[cursor+1] == '\n' {
				scanner.line++
			}
			cursor += 2
			continue
		}
		if scanner.data[cursor] == '\n' {
			if length == 1 {
				return errPythonGrammar
			}
			scanner.line++
		}
		if scanner.data[cursor] == quote && strings.HasPrefix(string(scanner.data[cursor:]), closing) {
			cursor += length
			scanner.pos = cursor
			return scanner.emit(pyString, start, cursor)
		}
		cursor++
	}
	return errPythonGrammar
}

func (scanner *pyScanner) readNumber() error {
	start := scanner.pos
	cursor := scanner.pos
	for cursor < len(scanner.data) {
		current := scanner.data[cursor]
		if isPythonIdentByte(current) || current == '.' {
			cursor++
			continue
		}
		exponent := (current == '+' || current == '-') && cursor > start &&
			(scanner.data[cursor-1] == 'e' || scanner.data[cursor-1] == 'E')
		if exponent && !strings.HasPrefix(strings.ToLower(string(scanner.data[start:cursor])), "0x") {
			cursor++
			continue
		}
		break
	}
	scanner.pos = cursor
	return scanner.emit(pyNumber, start, cursor)
}

// multi-byte operators TCQ must not split: `...` is the Ellipsis constant and
// `:=` must never be read as the suite-opening colon.
var pythonMultiOps = []string{"...", ":=", "->", "**=", "//=", ">>=", "<<=", "==", "!=", "<=", ">=", "//", "**", ">>", "<<"}

func (scanner *pyScanner) readOperator() error {
	start := scanner.pos
	rest := scanner.data[scanner.pos:]
	for _, candidate := range pythonMultiOps {
		if strings.HasPrefix(string(rest), candidate) {
			scanner.pos += len(candidate)
			return scanner.emit(pyOp, start, scanner.pos)
		}
	}
	switch scanner.data[scanner.pos] {
	case '(', '[', '{':
		scanner.depth++
	case ')', ']', '}':
		scanner.depth--
		if scanner.depth < 0 {
			return errPythonGrammar
		}
	}
	scanner.pos++
	return scanner.emit(pyOp, start, scanner.pos)
}

func isDigit(current byte) bool { return current >= '0' && current <= '9' }

func isQuote(current byte) bool { return current == '\'' || current == '"' }

func isPythonIdentStart(current byte) bool {
	return current == '_' || (current >= 'a' && current <= 'z') || (current >= 'A' && current <= 'Z') || current >= 0x80
}

func isPythonIdentByte(current byte) bool {
	return isPythonIdentStart(current) || isDigit(current)
}

func isPythonStringStart(data []byte, pos int) bool { return isQuote(data[pos]) }

// isStringPrefix admits exactly the Python 3.9 literal prefixes, in any case.
func isStringPrefix(word string) bool {
	switch strings.ToLower(word) {
	case "r", "b", "u", "f", "rb", "br", "fr", "rf":
		return true
	}
	return false
}
