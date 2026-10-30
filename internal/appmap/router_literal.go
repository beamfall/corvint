package appmap

import (
	"fmt"
	"regexp"
	"strings"
)

const maxRouteDepth = 128

var routeLiteralName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// lexRoutes is a conservative delimiter lexer for the two opt-in dialects. Unlike the JS
// lexer it retains JSX closing slashes and Ruby line boundaries; it never evaluates source.
func lexRoutes(text string, ruby bool) ([]token, bool) {
	out := []token{}
	line := 1
	depth := 0
	for i := 0; i < len(text); {
		c := text[i]
		if c == '\n' {
			if ruby {
				out = append(out, token{kind: tokPunct, text: ";", line: line})
			}
			line++
			i++
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' {
			i++
			continue
		}
		if ruby && c == '#' || !ruby && strings.HasPrefix(text[i:], "//") {
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		}
		if !ruby && strings.HasPrefix(text[i:], "/*") {
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				return out, false
			}
			end += i + 4
			line += strings.Count(text[i:end], "\n")
			i = end
			continue
		}
		if !ruby && c == '/' && (len(out) == 0 || !isPunct(out[len(out)-1], "<")) {
			if re, _ := regexStarts(out, 0); re {
				end := regexEnd(text, i)
				if end < 0 {
					return out, false
				}
				out = append(out, token{kind: tokTemplate, text: "opaque-regex", subst: true, line: line})
				i = end
				continue
			}
		}
		// A Ruby regexp literal in value position (after `,`, `(`, `[`, `{`, `:` or `=>`) is one
		// opaque value; a `/` anywhere else, or one not closed on its line, stays punctuation
		// (AMAP-V0-352).
		if ruby && c == '/' && len(out) > 0 && (next(out, len(out)-1, ",", "(", "[", "{", ":") ||
			len(out) > 1 && isPunct(out[len(out)-1], ">") && isPunct(out[len(out)-2], "=")) {
			if end := regexEnd(text, i); end > 0 {
				out = append(out, token{kind: tokTemplate, text: "opaque-regex", subst: true, line: line, code: text[i:end]})
				i = end
				continue
			}
		}
		t := token{line: line}
		switch {
		case c == '\'' || c == '"' || c == '`':
			start := i
			var closed bool
			if c == '`' {
				if ruby {
					return out, false
				}
				t.text, t.subst, t.inexact, t.unsure, i = readTemplate(text, i)
				closed = !t.unsure
				t.kind = tokTemplate
			} else {
				t.text, t.inexact, closed, i = readQuoted(text, i)
				t.kind = tokString
				t.code = text[start:i]
			}
			// Ruby escape/interpolation semantics differ from JS; refuse rather than decode approximately.
			if ruby && (strings.Contains(text[start:i], "\\") || c != '\'' && (strings.Contains(t.text, "#{") || strings.Contains(t.text, "#@") || strings.Contains(t.text, "#$"))) {
				t.inexact = true
			}
			if !closed {
				return out, false
			}
			line += strings.Count(text[start:i], "\n")
		case isIdentStart(c):
			start := i
			i++
			for i < len(text) && isIdentPart(text[i]) {
				i++
			}
			t.kind = tokIdent
			t.text = text[start:i]
		default:
			t.kind = tokPunct
			t.text = string(c)
			i++
			if c == '{' || c == '[' || c == '(' {
				depth++
				if depth > maxRouteDepth {
					return out, false
				}
			}
			if c == '}' || c == ']' || c == ')' {
				depth--
				if depth < 0 {
					return out, false
				}
			}
		}
		out = append(out, t)
	}
	return out, depth == 0
}

func routerUnknown(e blobEntry, line int, reason string) Unknown {
	return Unknown{Kind: "router", Ref: fmt.Sprintf("%s:%d", e.path, line), Path: e.path, Line: line, Reason: reason}
}

// Literal route grammar excludes optional/regex patterns; terminal splats are shared by dialects.
func literalRoutePath(p string) bool {
	if strings.ContainsAny(p, "?()[]{}\\#") {
		return false
	}
	for _, segment := range strings.Split(p, "/") {
		if strings.Contains(segment, ":") && (!strings.HasPrefix(segment, ":") || !routeLiteralName.MatchString(segment[1:])) {
			return false
		}
	}
	if i := strings.IndexByte(p, '*'); i >= 0 {
		if i > 0 && p[i-1] != '/' || strings.ContainsAny(p[i+1:], "/*:") {
			return false
		}
		if p[i+1:] != "" && !routeLiteralName.MatchString(p[i+1:]) {
			return false
		}

	}
	return true
}

func routeJoin(parent, child string) string {
	return strings.TrimRight(parent, "/") + "/" + strings.TrimLeft(child, "/")
}
