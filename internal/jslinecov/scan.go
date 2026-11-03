package jslinecov

import (
	"errors"
	"fmt"
)

// body is one function body: the lines of its braces.
type body struct{ open, close int }

// tokKind is the class of the last significant token, which decides what a
// following `/`, `{` or `<` means.
type tokKind int

const (
	tokNone    tokKind = iota
	tokIdent           // identifier or non-keyword word
	tokKeyword         // a keyword after which an expression starts
	tokValue           // number, string, template, regex
	tokClose           // ) ] }
	tokArrow           // =>
	tokPunct           // any other punctuator
)

// exprKeywords are the words after which `/` starts a regular expression and
// `<` starts JSX.
var exprKeywords = map[string]bool{
	"return": true, "typeof": true, "instanceof": true, "in": true, "of": true, "new": true, "delete": true,
	"void": true, "throw": true, "case": true, "do": true, "else": true, "yield": true, "await": true,
}

// blockKeywords open a parenthesised head whose `{` is a statement block, not
// a function body.
var blockKeywords = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "with": true}

// errJSX refuses markup: its text is not JavaScript, so quotes and braces in it
// cannot be read without a JSX grammar.
var errJSX = errors.New("JSX markup")

// bodies lists every function body of a JavaScript or TypeScript source: the
// braces after `=>`, and after a parameter list `)` (or a TypeScript return
// type following one) that no if, for, while, switch, catch or with heads. It
// skips comments, strings, template text and regular expressions, and refuses
// unbalanced input, a string running past its line, and JSX, so a body it
// reports is one the source has (LPCV-V0-966).
func bodies(src []byte) ([]body, error) {
	type frame struct {
		ch   byte // '{', '(', '[' or '`' for a template substitution
		line int
		fn   bool   // a '{' that opens a function body
		head string // for '(': the word before it
	}
	var (
		stack   []frame
		out     []body
		line    = 1
		prev    tokKind
		word    string // the last word token, for keyword checks
		closed  frame  // the last frame a ')' closed
		retType bool   // between `):` and the body or `=>`
		retAt   int    // the stack depth retType was set at
	)
	// inRet reports a return type open at this depth; a ternary's `(x) :`
	// inside brackets that have since closed is not one.
	inRet := func() bool { return retType && len(stack) == retAt }
	push := func(f frame) { stack = append(stack, f) }
	pop := func(want byte) (frame, error) {
		if len(stack) == 0 || stack[len(stack)-1].ch != want {
			return frame{}, fmt.Errorf("unbalanced %q at line %d", closer(want), line)
		}
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(stack) < retAt {
			retType = false
		}
		return f, nil
	}
	// template scans template text from i until its end or a substitution.
	template := func(i int) (int, error) {
		for ; i < len(src); i++ {
			switch src[i] {
			case '\\':
				if i+1 < len(src) && src[i+1] == '\n' {
					line++
				}
				i++
			case '\n':
				line++
			case '`':
				return i + 1, nil
			case '$':
				if i+1 < len(src) && src[i+1] == '{' {
					push(frame{ch: '`', line: line})
					return i + 2, nil
				}
			}
		}
		return i, fmt.Errorf("unterminated template at line %d", line)
	}
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
			continue
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i < len(src) && !(src[i] == '*' && i+1 < len(src) && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			if i >= len(src) {
				return nil, fmt.Errorf("unterminated comment at line %d", line)
			}
			i += 2
			continue
		case c == '#' && i == 0 && i+1 < len(src) && src[1] == '!':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case c == '\'' || c == '"':
			j := i + 1
			for ; j < len(src) && src[j] != c; j++ {
				if src[j] == '\n' {
					return nil, fmt.Errorf("string runs past line %d", line)
				}
				if src[j] == '\\' {
					j++
					if j < len(src) && src[j] == '\n' {
						line++
					}
				}
			}
			if j >= len(src) {
				return nil, fmt.Errorf("unterminated string at line %d", line)
			}
			i, prev = j+1, tokValue
			continue
		case c == '`':
			j, err := template(i + 1)
			if err != nil {
				return nil, err
			}
			i, prev = j, tokValue
			continue
		case c == '/' && regexAllowed(prev):
			j, err := skipRegex(src, i)
			if err != nil {
				return nil, fmt.Errorf("%w at line %d", err, line)
			}
			i, prev = j, tokValue
			continue
		case c == '<' && regexAllowed(prev) && i+1 < len(src) && (isWordStart(src[i+1]) || src[i+1] == '>'):
			return nil, errJSX
		case isWordStart(c):
			j := i
			for j < len(src) && isWordPart(src[j]) {
				j++
			}
			word = string(src[i:j])
			prev = tokIdent
			if exprKeywords[word] {
				prev = tokKeyword
			}
			i = j
			continue
		case c >= '0' && c <= '9':
			for i < len(src) && (isWordPart(src[i]) || src[i] == '.') {
				i++
			}
			prev = tokValue
			continue
		}
		// Punctuators.
		switch c {
		case '(':
			head := ""
			if prev == tokIdent || prev == tokKeyword {
				head = word
			}
			push(frame{ch: '(', line: line, head: head})
			prev = tokPunct
		case ')':
			f, err := pop('(')
			if err != nil {
				return nil, err
			}
			closed, prev = f, tokClose
			if nextSignificant(src, i+1) == ':' {
				retType, retAt = !blockKeywords[f.head], len(stack)
			}
		case '[':
			push(frame{ch: '[', line: line})
			prev = tokPunct
		case ']':
			if _, err := pop('['); err != nil {
				return nil, err
			}
			prev = tokClose
		case '{':
			f := frame{ch: '{', line: line}
			switch rt := inRet(); {
			case prev == tokArrow:
				f.fn = true
			case prev == tokClose && src[lastSignificant(src, i)] == ')' && !rt:
				f.fn = !blockKeywords[closed.head]
			case rt && typeBrace(src[lastSignificant(src, i)]):
				// an object type inside the return type
			case rt:
				f.fn = true
			}
			if f.fn {
				retType = false
			}
			push(f)
			prev = tokPunct
		case '}':
			if len(stack) > 0 && stack[len(stack)-1].ch == '`' {
				stack = stack[:len(stack)-1]
				j, err := template(i + 1)
				if err != nil {
					return nil, err
				}
				i, prev = j, tokValue
				continue
			}
			f, err := pop('{')
			if err != nil {
				return nil, err
			}
			if f.fn {
				out = append(out, body{f.line, line})
			}
			prev = tokClose
		case ';':
			retType = false
			prev = tokPunct
		case '=':
			if i+1 < len(src) && src[i+1] == '>' {
				i += 2
				retType, prev = false, tokArrow
				continue
			}
			if !(i+1 < len(src) && src[i+1] == '=') && inRet() {
				retType = false
			}
			prev = tokPunct
		default:
			prev = tokPunct
		}
		i++
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("unclosed %q from line %d", stack[len(stack)-1].ch, stack[len(stack)-1].line)
	}
	return out, nil
}

func closer(open byte) byte {
	switch open {
	case '(':
		return ')'
	case '[':
		return ']'
	}
	return '}'
}

// typeBrace reports whether a `{` after b, inside a return type, opens an
// object type rather than the body.
func typeBrace(b byte) bool {
	switch b {
	case ':', '|', '&', '<', ',', '(', '[', '?':
		return true
	}
	return false
}

func regexAllowed(prev tokKind) bool {
	return prev == tokNone || prev == tokKeyword || prev == tokPunct || prev == tokArrow
}

func isWordStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isWordPart(c byte) bool { return isWordStart(c) || c >= '0' && c <= '9' }

// skipRegex returns the index after the regular expression literal at i.
func skipRegex(src []byte, i int) (int, error) {
	class := false
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case '\n':
			return 0, errors.New("regular expression runs past its line")
		case '[':
			class = true
		case ']':
			class = false
		case '/':
			if !class {
				j++
				for j < len(src) && isWordPart(src[j]) {
					j++
				}
				return j, nil
			}
		}
	}
	return 0, errors.New("unterminated regular expression")
}

// nextSignificant is the first byte at or after i that is not space or a
// comment; 0 at the end.
func nextSignificant(src []byte, i int) byte {
	for i < len(src) {
		switch c := src[i]; {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i += 2
		default:
			return c
		}
	}
	return 0
}

// lastSignificant is the index of the last byte before i that is not space;
// comments between are not expected where it is asked.
func lastSignificant(src []byte, i int) int {
	for i--; i > 0; i-- {
		switch src[i] {
		case ' ', '\t', '\r', '\n':
			continue
		}
		return i
	}
	return 0
}
