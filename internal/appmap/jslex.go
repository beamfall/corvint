package appmap

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// The application map reads JavaScript and TypeScript lexically: a token stream that tells code
// from comment and literal, never an evaluator. Anything the compiler would need to execute to
// know is reported UNKNOWN (AMAP-V0-002, AMAP-V0-005).

type tokKind int

const (
	tokIdent tokKind = iota + 1
	tokString
	tokTemplate // a template literal; parts holds its text and substitution boundaries
	tokNumber
	tokPunct
)

type token struct {
	kind tokKind
	text string // identifier, punctuation, number, or a string literal's decoded body
	line int
	// subst is true for a template literal with at least one ${} substitution; text then holds
	// the literal with every substitution replaced by substMark.
	subst bool
	// inexact marks a string or template whose escapes could not be decoded exactly (legacy
	// octal, a lone surrogate); its text is then not the program's value and never a literal.
	inexact bool
}

// literal reports whether t is a string or substitution-free template whose value is exact.
func literal(t token) bool {
	return !t.inexact && (t.kind == tokString || (t.kind == tokTemplate && !t.subst))
}

// substMark stands for one ${} substitution inside a template literal's text.
const substMark = "\x00"

// lexJS tokenizes text and returns the tokens and a copy of text with every comment and literal
// body blanked to spaces (newlines kept), for line-shaped scans of code only.
var (
	controlKeywords = map[string]bool{"if": true, "while": true, "for": true, "with": true}
	regexKeywords   = map[string]bool{"return": true, "typeof": true, "case": true, "in": true, "of": true,
		"else": true, "do": true, "void": true, "delete": true, "throw": true, "new": true,
		"instanceof": true, "yield": true, "await": true}
)

func lexJS(text string) ([]token, string) {
	toks := []token{}
	code := []byte(text)
	line := 1
	blank := func(from, to int) {
		for k := from; k < to && k < len(code); k++ {
			if code[k] != '\n' {
				code[k] = ' '
			}
		}
	}
	// parens records, for each open '(', whether it opens a control-statement condition; after
	// its ')' a '/' starts a regular expression (if (x) /re/.test(s)), not a division.
	parens, closedControl := []bool{}, false
	prevAllowsRegex := func() bool {
		if len(toks) == 0 {
			return true
		}
		p := toks[len(toks)-1]
		switch p.kind {
		case tokIdent:
			return regexKeywords[p.text]
		case tokPunct:
			if p.text == ")" {
				return closedControl
			}
			return p.text != "]" && p.text != "}"
		}
		return false
	}
	i := 0
	for i < len(text) {
		c := text[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < len(text) && text[i+1] == '/':
			j := strings.IndexByte(text[i:], '\n')
			if j < 0 {
				j = len(text) - i
			}
			blank(i, i+j)
			i += j
		case c == '/' && i+1 < len(text) && text[i+1] == '*':
			j := strings.Index(text[i+2:], "*/")
			end := len(text)
			if j >= 0 {
				end = i + 2 + j + 2
			}
			line += strings.Count(text[i:end], "\n")
			blank(i, end)
			i = end
		case c == '\'' || c == '"':
			body, inexact, end := readQuoted(text, i)
			toks = append(toks, token{kind: tokString, text: body, line: line, inexact: inexact})
			line += strings.Count(text[i:end], "\n") // line continuations
			blank(i+1, end-1)
			i = end
		case c == '`':
			body, subst, inexact, end := readTemplate(text, i)
			toks = append(toks, token{kind: tokTemplate, text: body, line: line, subst: subst, inexact: inexact})
			line += strings.Count(text[i:end], "\n")
			blank(i+1, end-1)
			i = end
		case c == '/' && prevAllowsRegex():
			// A regular-expression literal closes on its own line; otherwise the slash is division.
			j := regexEnd(text, i)
			if j < 0 {
				toks = append(toks, token{kind: tokPunct, text: "/", line: line})
				i++
				continue
			}
			blank(i+1, j-1)
			// A regex is a value but never a literal string: a pattern is not the text it matches.
			toks = append(toks, token{kind: tokString, text: "", line: line, inexact: true})
			i = j
		case isIdentStart(c):
			j := i + 1
			for j < len(text) && isIdentPart(text[j]) {
				j++
			}
			toks = append(toks, token{kind: tokIdent, text: text[i:j], line: line})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(text) && (isIdentPart(text[j]) || text[j] == '.') {
				j++
			}
			toks = append(toks, token{kind: tokNumber, text: text[i:j], line: line})
			i = j
		default:
			switch c {
			case '(':
				prev := len(toks) > 0 && toks[len(toks)-1].kind == tokIdent && controlKeywords[toks[len(toks)-1].text]
				parens = append(parens, prev)
			case ')':
				closedControl = false
				if n := len(parens); n > 0 {
					closedControl, parens = parens[n-1], parens[:n-1]
				}
			}
			toks = append(toks, token{kind: tokPunct, text: string(c), line: line})
			i++
		}
	}
	return toks, string(code)
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentPart(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

// readQuoted reads a '...' or "..." literal starting at i; it ends at its closing quote or at the
// line break, so an unterminated literal cannot swallow the rest of the file.
func readQuoted(text string, i int) (string, bool, int) {
	q := text[i]
	var b strings.Builder
	inexact := false
	j := i + 1
	for j < len(text) && text[j] != q && text[j] != '\n' {
		if text[j] == '\\' && j+1 < len(text) {
			var ok bool
			j, ok = unescape(&b, text, j+1)
			inexact = inexact || !ok
			continue
		}
		b.WriteByte(text[j])
		j++
	}
	if j < len(text) && text[j] == q {
		j++
	}
	return b.String(), inexact, j
}

// unescape decodes the escape sequence whose character after the backslash is at text[j], writing
// its value to b and returning the index after it. It reports false for a sequence whose value it
// cannot know exactly: a legacy octal escape or a lone UTF-16 surrogate.
func unescape(b *strings.Builder, text string, j int) (int, bool) {
	c := text[j]
	switch c {
	case 'n':
		b.WriteByte('\n')
	case 'r':
		b.WriteByte('\r')
	case 't':
		b.WriteByte('\t')
	case 'b':
		b.WriteByte('\b')
	case 'f':
		b.WriteByte('\f')
	case 'v':
		b.WriteByte('\v')
	case '\n': // line continuation
	case '\r':
		if j+1 < len(text) && text[j+1] == '\n' {
			return j + 2, true
		}
	case '0':
		if j+1 < len(text) && text[j+1] >= '0' && text[j+1] <= '9' {
			return j + 1, false
		}
		b.WriteByte(0)
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return j + 1, false
	case 'x':
		if r, ok := hexRune(text, j+1, j+3); ok {
			b.WriteRune(r)
			return j + 3, true
		}
		return j + 1, false
	case 'u':
		r, end, ok := unicodeEscape(text, j+1)
		if ok && r >= 0xD800 && r <= 0xDBFF && end+1 < len(text) && text[end] == '\\' && text[end+1] == 'u' {
			if lo, end2, ok2 := unicodeEscape(text, end+2); ok2 && lo >= 0xDC00 && lo <= 0xDFFF {
				b.WriteRune((r-0xD800)<<10 + (lo - 0xDC00) + 0x10000)
				return end2, true
			}
		}
		if !ok || (r >= 0xD800 && r <= 0xDFFF) {
			return max(end, j+1), false
		}
		b.WriteRune(r)
		return end, true
	default:
		_, size := utf8.DecodeRuneInString(text[j:])
		b.WriteString(text[j : j+size])
		return j + size, true
	}
	return j + 1, true
}

// unicodeEscape reads the HHHH or {H+} after a \u at text[j].
func unicodeEscape(text string, j int) (rune, int, bool) {
	if j < len(text) && text[j] == '{' {
		end := strings.IndexByte(text[j:], '}')
		if end < 2 || end > 7 {
			return 0, j, false
		}
		r, ok := hexRune(text, j+1, j+end)
		return r, j + end + 1, ok && r <= utf8.MaxRune
	}
	r, ok := hexRune(text, j, j+4)
	if !ok {
		return 0, j, false // never consume past a short or non-hex escape
	}
	return r, j + 4, true
}

func hexRune(text string, from, to int) (rune, bool) {
	if to > len(text) {
		return 0, false
	}
	v, err := strconv.ParseUint(text[from:to], 16, 32)
	return rune(v), err == nil
}

// readTemplate reads a template literal starting at i, replacing each ${...} with substMark.
func readTemplate(text string, i int) (string, bool, bool, int) {
	var b strings.Builder
	subst, inexact := false, false
	j := i + 1
	for j < len(text) && text[j] != '`' {
		switch {
		case text[j] == '\\' && j+1 < len(text):
			var ok bool
			j, ok = unescape(&b, text, j+1)
			inexact = inexact || !ok
		case text[j] == '$' && j+1 < len(text) && text[j+1] == '{':
			subst = true
			b.WriteString(substMark)
			depth := 0
			for j < len(text) {
				if text[j] == '{' {
					depth++
				} else if text[j] == '}' {
					depth--
					if depth == 0 {
						j++
						break
					}
				}
				j++
			}
		default:
			b.WriteByte(text[j])
			j++
		}
	}
	if j < len(text) {
		j++
	}
	return b.String(), subst, inexact, j
}

func regexEnd(text string, i int) int {
	inClass := false
	for j := i + 1; j < len(text); j++ {
		switch text[j] {
		case '\n':
			return -1
		case '\\':
			j++
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if !inClass {
				k := j + 1
				for k < len(text) && isIdentPart(text[k]) {
					k++
				}
				return k
			}
		}
	}
	return -1
}

// jsValue is a literal value read from an object or array literal; kind "other" is anything the
// compiler would have to evaluate.
type jsValue struct {
	kind  string // string, bool, number, object, array, member (`X.Y`, str "X.Y"), other
	str   string
	tok   int // a member's token index: which binding of `X` it reads (AMAP-V0-022)
	obj   []jsPair
	arr   []jsValue
	line  int
	subst bool
}

type jsPair struct {
	key   string
	value jsValue
}

// get returns the value of key; as in JavaScript, the last of repeated keys wins.
func (v jsValue) get(key string) (jsValue, bool) {
	for i := len(v.obj) - 1; i >= 0; i-- {
		if v.obj[i].key == key {
			return v.obj[i].value, true
		}
	}
	return jsValue{}, false
}

// stringList reads an array of plain string literals; ok is false for anything else.
func (v jsValue) stringList() ([]string, bool) {
	if v.kind != "array" {
		return nil, false
	}
	out := []string{}
	for _, e := range v.arr {
		if e.kind != "string" {
			return nil, false
		}
		out = append(out, e.str)
	}
	return out, true
}

// parseValue reads one value starting at toks[i] and returns the index after it. A value it cannot
// read literally is skipped to the next ',' or closing bracket at its own depth.
func parseValue(toks []token, i int) (jsValue, int) {
	if i >= len(toks) {
		return jsValue{kind: "other"}, i
	}
	t := toks[i]
	switch {
	case literal(t):
		if next(toks, i+1, ",", "}", "]", ")") {
			return jsValue{kind: "string", str: t.text, line: t.line}, i + 1
		}
	case t.kind == tokNumber:
		if next(toks, i+1, ",", "}", "]", ")") {
			return jsValue{kind: "number", str: t.text, line: t.line}, i + 1
		}
	case t.kind == tokIdent && (t.text == "true" || t.text == "false"):
		if next(toks, i+1, ",", "}", "]", ")") {
			return jsValue{kind: "bool", str: t.text, line: t.line}, i + 1
		}
	case t.kind == tokIdent && next(toks, i+1, ".") && i+2 < len(toks) && toks[i+2].kind == tokIdent:
		if next(toks, i+3, ",", "}", "]", ")") {
			return jsValue{kind: "member", str: t.text + "." + toks[i+2].text, line: t.line, tok: i}, i + 3
		}
	case t.kind == tokPunct && t.text == "{":
		return parseObject(toks, i)
	case t.kind == tokPunct && t.text == "[":
		return parseArray(toks, i)
	}
	return jsValue{kind: "other", line: t.line, subst: t.kind == tokTemplate}, skipValue(toks, i)
}

func next(toks []token, i int, punct ...string) bool {
	if i >= len(toks) || toks[i].kind != tokPunct {
		return false
	}
	for _, p := range punct {
		if toks[i].text == p {
			return true
		}
	}
	return false
}

// skipValue advances past one expression to the ',' or closing bracket that ends it.
func skipValue(toks []token, i int) int {
	depth := 0
	for ; i < len(toks); i++ {
		t := toks[i]
		if t.kind != tokPunct {
			continue
		}
		switch t.text {
		case "{", "[", "(":
			depth++
		case "}", "]", ")":
			if depth == 0 {
				return i
			}
			depth--
		case ",":
			if depth == 0 {
				return i
			}
		}
	}
	return i
}

func parseObject(toks []token, i int) (jsValue, int) {
	v := jsValue{kind: "object", line: toks[i].line}
	i++
	for i < len(toks) {
		if next(toks, i, "}") {
			return v, i + 1
		}
		t := toks[i]
		if (t.kind != tokIdent && (t.kind != tokString || t.inexact)) || !next(toks, i+1, ":") {
			// spread, computed key, method or shorthand: the object is not a plain literal
			v.obj = append(v.obj, jsPair{key: "", value: jsValue{kind: "other", line: t.line}})
			i = skipValue(toks, i)
		} else {
			var val jsValue
			val, i = parseValue(toks, i+2)
			if _, dup := v.get(t.text); dup {
				// A repeated key replaces the earlier value; mark the object as not plain so a
				// reader that walks members cannot act on the superseded one (AMAP-V0-002).
				v.obj = append(v.obj, jsPair{key: "", value: jsValue{kind: "other", line: t.line}})
			}
			v.obj = append(v.obj, jsPair{key: t.text, value: val})
		}
		if next(toks, i, ",") {
			i++
			continue
		}
		if next(toks, i, "}") {
			return v, i + 1
		}
		return jsValue{kind: "other", line: v.line}, skipValue(toks, i)
	}
	return jsValue{kind: "other", line: v.line}, i
}

func parseArray(toks []token, i int) (jsValue, int) {
	v := jsValue{kind: "array", line: toks[i].line}
	i++
	for i < len(toks) {
		if next(toks, i, "]") {
			return v, i + 1
		}
		var val jsValue
		val, i = parseValue(toks, i)
		v.arr = append(v.arr, val)
		if next(toks, i, ",") {
			i++
			continue
		}
		if next(toks, i, "]") {
			return v, i + 1
		}
		return jsValue{kind: "other", line: v.line}, skipValue(toks, i)
	}
	return jsValue{kind: "other", line: v.line}, i
}

// closeParen returns the index of the ')' matching the '(' at toks[i].
func closeParen(toks []token, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		if toks[j].kind != tokPunct {
			continue
		}
		switch toks[j].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(toks) - 1
}
