// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"fmt"
	"strings"
)

// jestCallNames are the Jest globals that register a describe block or a
// test. Their modifiers (.only, .skip, .concurrent, .failing, .todo) keep
// the title a plain argument; .each builds it per row, so it is computed.
var jestCallNames = map[string]bool{
	"describe": true, "fdescribe": true, "xdescribe": true,
	"test": true, "xtest": true, "it": true, "fit": true, "xit": true,
}

var jestModifiers = map[string]bool{"only": true, "skip": true, "concurrent": true, "failing": true, "todo": true}

// jestDescribes are the call names that open a describe block.
var jestDescribes = map[string]bool{"describe": true, "fdescribe": true, "xdescribe": true}

// jestTitlePaths reads a Jest test file's source and returns the title path
// (describe titles, then the test's title) of every test it registers, or
// why it cannot prove them (JST-V1-003, review P3-a): a describe or test
// call whose title is not one static string literal (an identifier, an
// import, a concatenation, a template with a substitution, an escape, or
// .each), a call that sits anywhere but at the top level or directly in a
// describe callback (inside a helper function, a loop, a condition or
// another call, whose caller can add ancestors the file does not show), or
// source the scanner cannot follow (an unterminated string, comment,
// template or regular expression, or unbalanced brackets). JSX text and
// other syntax the scanner does not model fail closed the same way.
func jestTitlePaths(source []byte) ([][]string, string) {
	s := jestScanner{src: string(source)}
	if reason := s.scan(); reason != "" {
		return nil, reason
	}
	return s.paths, ""
}

// jestFrame is one open bracket: '(', '[', '{', or 't' for a template
// substitution `${`. A '(' frame that a describe call opened carries its
// title.
type jestFrame struct {
	kind     byte
	describe bool
	title    string
}

type jestScanner struct {
	src   string
	at    int
	stack []jestFrame
	paths [][]string
	// prev is the last significant token: an identifier or keyword, or a
	// punctuation character as a one-byte string ("" at the start).
	prev string
}

func (s *jestScanner) line() int { return strings.Count(s.src[:min(s.at, len(s.src))], "\n") + 1 }

func (s *jestScanner) fail(format string, args ...any) string {
	return fmt.Sprintf("line %d: ", s.line()) + fmt.Sprintf(format, args...)
}

func identStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func identPart(c byte) bool { return identStart(c) || c >= '0' && c <= '9' }

// regexKeywords are the keywords after which `/` starts a regular
// expression rather than a division.
var regexKeywords = map[string]bool{
	"return": true, "typeof": true, "case": true, "do": true, "else": true, "in": true, "of": true, "new": true,
	"delete": true, "void": true, "throw": true, "instanceof": true, "yield": true, "await": true,
}

// regexAllowed reports whether a `/` after prev starts a regular expression.
func (s *jestScanner) regexAllowed() bool {
	switch {
	case s.prev == "":
		return true
	case identStart(s.prev[0]) || s.prev[0] >= '0' && s.prev[0] <= '9':
		return regexKeywords[s.prev]
	}
	return !strings.Contains(")]}", s.prev)
}

// skipSpace skips white space and comments; it returns a reason for an
// unterminated block comment.
func (s *jestScanner) skipSpace() string {
	for s.at < len(s.src) {
		switch c := s.src[s.at]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			s.at++
		case strings.HasPrefix(s.src[s.at:], "//"):
			end := strings.IndexByte(s.src[s.at:], '\n')
			if end < 0 {
				s.at = len(s.src)
			} else {
				s.at += end
			}
		case strings.HasPrefix(s.src[s.at:], "/*"):
			end := strings.Index(s.src[s.at+2:], "*/")
			if end < 0 {
				return s.fail("an unterminated block comment")
			}
			s.at += end + 4
		default:
			return ""
		}
	}
	return ""
}

// quoted skips a '...' or "..." string starting at s.at and returns its
// text, and whether it holds an escape.
func (s *jestScanner) quoted() (string, bool, string) {
	quote := s.src[s.at]
	start := s.at + 1
	escaped := false
	for i := start; i < len(s.src); i++ {
		switch s.src[i] {
		case '\\':
			escaped = true
			i++
		case '\n':
			s.at = i
			return "", false, s.fail("an unterminated string")
		case quote:
			s.at = i + 1
			return s.src[start:i], escaped, ""
		}
	}
	s.at = len(s.src)
	return "", false, s.fail("an unterminated string")
}

// template skips template text from s.at (just after '`' or the '}' that
// ends a substitution) to its closing '`', or to a `${`, which opens a 't'
// frame. It returns the text, whether it holds an escape, and whether it
// stopped at a substitution.
func (s *jestScanner) template() (text string, escaped, opened bool, reason string) {
	start := s.at
	for i := start; i < len(s.src); i++ {
		switch {
		case s.src[i] == '\\':
			escaped = true
			i++
		case s.src[i] == '`':
			s.at = i + 1
			s.prev = "0"
			return s.src[start:i], escaped, false, ""
		case s.src[i] == '$' && i+1 < len(s.src) && s.src[i+1] == '{':
			s.at = i + 2
			s.stack = append(s.stack, jestFrame{kind: 't'})
			s.prev = "{"
			return "", escaped, true, ""
		}
	}
	s.at = len(s.src)
	return "", false, false, s.fail("an unterminated template literal")
}

// regex skips a regular expression literal from its opening '/'.
func (s *jestScanner) regex() string {
	class := false
	for i := s.at + 1; i < len(s.src); i++ {
		switch c := s.src[i]; {
		case c == '\\':
			i++
		case c == '\n':
			s.at = i
			return s.fail("an unterminated regular expression")
		case c == '[':
			class = true
		case c == ']':
			class = false
		case c == '/' && !class:
			s.at = i + 1
			for s.at < len(s.src) && identPart(s.src[s.at]) {
				s.at++
			}
			return ""
		}
	}
	s.at = len(s.src)
	return s.fail("an unterminated regular expression")
}

// placed reports whether a call at the current depth is at the top level or
// directly in a describe callback: every open frame is a describe call's
// '(' or a '{' whose parent is one.
func (s *jestScanner) placed() bool {
	for index, frame := range s.stack {
		switch {
		case frame.kind == '(' && frame.describe:
		case frame.kind == '{' && index > 0 && s.stack[index-1].kind == '(' && s.stack[index-1].describe:
		default:
			return false
		}
	}
	return true
}

// ancestors are the titles of the describe calls open around s.at.
func (s *jestScanner) ancestors() []string {
	var titles []string
	for _, frame := range s.stack {
		if frame.kind == '(' && frame.describe {
			titles = append(titles, frame.title)
		}
	}
	return titles
}

// call reads a describe or test call whose name ends at s.at: its
// modifiers, its '(' and its static title, and opens its frame. A name not
// followed by a call is not a registration and is ignored.
func (s *jestScanner) call(name string) string {
	start := s.at
	for {
		if reason := s.skipSpace(); reason != "" {
			return reason
		}
		if s.at >= len(s.src) || s.src[s.at] != '.' {
			break
		}
		s.at++
		if reason := s.skipSpace(); reason != "" {
			return reason
		}
		end := s.at
		for end < len(s.src) && identPart(s.src[end]) {
			end++
		}
		modifier := s.src[s.at:end]
		s.at = end
		switch {
		case modifier == "each":
			return s.fail("%s.each builds its titles from a table, so they are not static", name)
		case !jestModifiers[modifier]:
			// Some other member (`it.name`): not a registration.
			s.at, s.prev = start, name
			return ""
		}
	}
	if s.at >= len(s.src) || s.src[s.at] != '(' {
		s.at, s.prev = start, name
		return ""
	}
	if !s.placed() {
		return s.fail("%s( is not at the top level or directly in a describe callback, so its ancestors are not known from the file", name)
	}
	s.at++
	if reason := s.skipSpace(); reason != "" {
		return reason
	}
	title, escaped, reason := "", false, ""
	switch {
	case s.at < len(s.src) && (s.src[s.at] == '\'' || s.src[s.at] == '"'):
		title, escaped, reason = s.quoted()
	case s.at < len(s.src) && s.src[s.at] == '`':
		s.at++
		opened := false
		title, escaped, opened, reason = s.template()
		if opened {
			return s.fail("the %s title is a template with a substitution, so it is not static", name)
		}
	default:
		return s.fail("the %s title is not a string literal, so it is not static", name)
	}
	if reason != "" {
		return reason
	}
	if escaped {
		return s.fail("the %s title %q holds an escape, so it is not read as static", name, title)
	}
	if reason := s.skipSpace(); reason != "" {
		return reason
	}
	if s.at >= len(s.src) || s.src[s.at] != ',' && s.src[s.at] != ')' {
		return s.fail("the %s title is not one string literal, so it is not static", name)
	}
	if jestDescribes[name] {
		s.stack = append(s.stack, jestFrame{kind: '(', describe: true, title: title})
	} else {
		s.stack = append(s.stack, jestFrame{kind: '('})
		s.paths = append(s.paths, append(s.ancestors(), title))
	}
	s.prev = "("
	return ""
}

func (s *jestScanner) scan() string {
	for {
		if reason := s.skipSpace(); reason != "" {
			return reason
		}
		if s.at >= len(s.src) {
			break
		}
		c := s.src[s.at]
		switch {
		case identStart(c):
			end := s.at
			for end < len(s.src) && identPart(s.src[end]) {
				end++
			}
			name := s.src[s.at:end]
			s.at = end
			if jestCallNames[name] && s.prev != "." && s.prev != "function" && s.prev != "?." {
				if reason := s.call(name); reason != "" {
					return reason
				}
				continue
			}
			s.prev = name
		case c >= '0' && c <= '9':
			for s.at < len(s.src) && (identPart(s.src[s.at]) || s.src[s.at] == '.') {
				s.at++
			}
			s.prev = "0"
		case c == '\'' || c == '"':
			if _, _, reason := s.quoted(); reason != "" {
				return reason
			}
			s.prev = "0"
		case c == '`':
			s.at++
			if _, _, _, reason := s.template(); reason != "" {
				return reason
			}
		case c == '/':
			if s.regexAllowed() {
				if reason := s.regex(); reason != "" {
					return reason
				}
				s.prev = "0"
			} else {
				s.at++
				s.prev = "/"
			}
		case c == '(' || c == '[' || c == '{':
			s.stack = append(s.stack, jestFrame{kind: c})
			s.at++
			s.prev = string(c)
		case c == ')' || c == ']' || c == '}':
			open := map[byte]byte{')': '(', ']': '[', '}': '{'}[c]
			if len(s.stack) == 0 {
				return s.fail("an unbalanced %q", c)
			}
			top := s.stack[len(s.stack)-1]
			s.stack = s.stack[:len(s.stack)-1]
			switch {
			case c == '}' && top.kind == 't':
				// The substitution ends; the template's text resumes.
				s.at++
				if _, _, _, reason := s.template(); reason != "" {
					return reason
				}
				continue
			case top.kind != open:
				return s.fail("an unbalanced %q", c)
			}
			s.at++
			s.prev = string(c)
		case c == '?' && strings.HasPrefix(s.src[s.at:], "?."):
			s.at += 2
			s.prev = "?."
		default:
			s.at++
			s.prev = string(c)
		}
	}
	if len(s.stack) != 0 {
		return s.fail("%d brackets are not closed", len(s.stack))
	}
	return ""
}
