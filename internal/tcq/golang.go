package tcq

import (
	"bytes"
	"errors"
	"regexp"
	"unicode/utf8"
)

// errGoParse is the TCQ-V0-016 refusal: comment/string lookalikes, wrong
// signatures, and lexical failure all abstain as `unparseable-test-unit`.
var errGoParse = errors.New("unparseable-test-unit")

var (
	goTestNamePattern = regexp.MustCompile(`^Test[A-Z][A-Za-z0-9_]*$`)
	goCasePattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,96}$`)
)

type goKind int

const (
	goName goKind = iota
	goString
	goRune
	goPunct
)

type goToken struct {
	kind  goKind
	text  string
	start int
	end   int
}

// goCase is one admitted table-case leaf: the span of the literal's contents,
// exclusive of its quotes, and the case value.
type goCase struct {
	start int
	end   int
	value string
}

// goFunction is one validated `TestX` and the case leaves admitted inside it.
type goFunction struct {
	unit      testUnit
	nameStart int
	nameEnd   int
	cases     []goCase
}

type goAnalysis struct {
	functions []goFunction
}

// tokenizeGo is the bounded token scan of profile `go-lexical/1` (TCQ-V0-016).
// It pairs comments, interpreted and raw strings, rune literals, parentheses,
// and braces so a `func TestX(` inside a comment or string can never be read as
// a declaration.
func tokenizeGo(data []byte) ([]goToken, error) {
	scanner := &goScanner{data: data}
	if err := scanner.run(); err != nil {
		return nil, err
	}
	if err := scanner.checkDelimiters(); err != nil {
		return nil, err
	}
	return scanner.tokens, nil
}

type goScanner struct {
	data   []byte
	pos    int
	tokens []goToken
}

func (scanner *goScanner) run() error {
	for scanner.pos < len(scanner.data) {
		if err := scanner.step(); err != nil {
			return err
		}
		if len(scanner.tokens) > maxGoTokens {
			return fail(CodeResourceExhausted)
		}
	}
	return nil
}

func (scanner *goScanner) step() error {
	current := scanner.data[scanner.pos]
	switch {
	case current == ' ' || current == '\t' || current == '\r' || current == '\n':
		scanner.pos++
		return nil
	case scanner.hasPrefix("//"):
		return scanner.skipLineComment()
	case scanner.hasPrefix("/*"):
		return scanner.skipBlockComment()
	case current == '"' || current == '\'' || current == '`':
		return scanner.readLiteral(current)
	case isGoIdentStart(current):
		return scanner.readName()
	default:
		scanner.tokens = append(scanner.tokens, goToken{
			kind: goPunct, text: string(current), start: scanner.pos, end: scanner.pos + 1,
		})
		scanner.pos++
		return nil
	}
}

func (scanner *goScanner) hasPrefix(prefix string) bool {
	return bytes.HasPrefix(scanner.data[scanner.pos:], []byte(prefix))
}

func (scanner *goScanner) skipLineComment() error {
	end := bytes.IndexByte(scanner.data[scanner.pos+2:], '\n')
	if end < 0 {
		scanner.pos = len(scanner.data)
		return nil
	}
	scanner.pos += 2 + end + 1
	return nil
}

func (scanner *goScanner) skipBlockComment() error {
	end := bytes.Index(scanner.data[scanner.pos+2:], []byte("*/"))
	if end < 0 {
		return errGoParse
	}
	scanner.pos += 2 + end + 2
	return nil
}

func (scanner *goScanner) readLiteral(quote byte) error {
	start := scanner.pos
	cursor := scanner.pos + 1
	closed := false
	for cursor < len(scanner.data) {
		if quote != '`' && scanner.data[cursor] == '\\' {
			cursor += 2
			continue
		}
		if scanner.data[cursor] == quote {
			cursor++
			closed = true
			break
		}
		cursor++
	}
	if !closed {
		return errGoParse
	}
	raw := scanner.data[start:cursor]
	if !utf8.Valid(raw) {
		return errGoParse
	}
	kind := goString
	if quote == '\'' {
		kind = goRune
	}
	scanner.tokens = append(scanner.tokens, goToken{kind: kind, text: string(raw), start: start, end: cursor})
	scanner.pos = cursor
	return nil
}

func (scanner *goScanner) readName() error {
	start := scanner.pos
	cursor := scanner.pos + 1
	for cursor < len(scanner.data) && isGoIdentByte(scanner.data[cursor]) {
		cursor++
	}
	scanner.tokens = append(scanner.tokens, goToken{
		kind: goName, text: string(scanner.data[start:cursor]), start: start, end: cursor,
	})
	scanner.pos = cursor
	return nil
}

var goClosers = map[string]string{")": "(", "]": "[", "}": "{"}

func (scanner *goScanner) checkDelimiters() error {
	var stack []string
	for _, token := range scanner.tokens {
		switch token.text {
		case "(", "[", "{":
			stack = append(stack, token.text)
		case ")", "]", "}":
			if len(stack) == 0 || stack[len(stack)-1] != goClosers[token.text] {
				return errGoParse
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) > 0 {
		return errGoParse
	}
	return nil
}

func isGoIdentStart(current byte) bool {
	return current == '_' || (current >= 'a' && current <= 'z') || (current >= 'A' && current <= 'Z')
}

func isGoIdentByte(current byte) bool {
	return isGoIdentStart(current) || (current >= '0' && current <= '9')
}

// goMatching returns the index of the token closing the delimiter at start.
func goMatching(tokens []goToken, start int, opening, closing string) int {
	depth := 0
	for index := start; index < len(tokens); index++ {
		switch tokens[index].text {
		case opening:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// goDeclarationLeading requires `func` to be the first lexical item on its
// physical source line, so a `func` appearing as a value or a type cannot be
// mistaken for a top-level declaration.
func goDeclarationLeading(data []byte, tokens []goToken, index int) bool {
	token := tokens[index]
	lineStart := bytes.LastIndexByte(data[:token.start], '\n') + 1
	if len(bytes.Trim(data[lineStart:token.start], " \t\r")) > 0 {
		return false
	}
	if index == 0 {
		return true
	}
	return !goValuePosition[tokens[index-1].text]
}

// goValuePosition names the tokens after which a `func` keyword opens a function
// literal or type rather than a declaration.
var goValuePosition = map[string]bool{
	"=": true, ":": true, ",": true, ".": true, "+": true, "-": true, "*": true,
	"/": true, "%": true, "&": true, "|": true, "^": true, "!": true,
	"<": true, ">": true, "(": true, "[": true, "{": true,
}
