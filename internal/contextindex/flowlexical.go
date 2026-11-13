package contextindex

import (
	"bytes"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin/dialect"
)

// FlowCodeLines shares the native literal/comment masks with the bounded flow
// producer. It preserves line/column positions; masked bytes are never facts.
// language is an impact dialect; its registered profile masks the text, and
// any other language gets the web mask (LGP-V0-025). Web interpolation and
// dynamic Ruby dispatch intentionally remain unresolved.
func FlowCodeLines(language, text string) []string {
	if profile, ok := dialect.LookupImpact(language); ok {
		return profile.CodeLines(text)
	}
	return flowWebCodeLines(text)
}

// flowWebCodeLines is the web mask split into lines: the code lines of the
// TypeScript plugin's impact profile (webdialect.Impact), which core registers
// because the mask is core's (LGP-V0-027).
func flowWebCodeLines(text string) []string {
	return strings.Split(flowWebMask(text), "\n")
}

// flowWebMask additionally treats slash expressions conservatively and blanks
// JSX text. The native web mask does not claim a JavaScript regex/JSX grammar.
func flowWebMask(text string) string {
	code := []byte(stripWebNonCode(text))
	markupDepth, expressionDepth := 0, 0
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c == '<' && i > 0 && webIdentifierByte(code[i-1]) && !flowExpressionKeywords[flowWordBefore(code, i)] {
			// A type-argument list right after a name and before a call
			// (useMemo<T>(, useState<Map<K, V>>() is not markup: only the list
			// is passed over, so the call and what follows stay code (V1-2415).
			// A keyword such as return is no callee: return<div>(x) y</div>
			// is markup.
			if end := typeArgumentsEnd(code, i); end > 0 {
				i = end
				continue
			}
			// Markup never opens straight after a name, so any other such '<'
			// (a return type `): Promise<T> {`, an annotation `: Map<K, V> =`,
			// a comparison `i<n`) stays code rather than blanking the rest of
			// the file up to a closing tag.
			continue
		}
		if c == '<' && i+1 < len(code) && (code[i+1] >= 'A' && code[i+1] <= 'Z' || code[i+1] >= 'a' && code[i+1] <= 'z' || code[i+1] == '/') {
			// An attribute expression can hold '>' (onChange={(e) => f(e)}),
			// so the tag ends at the first '>' outside braces. A ';', '(' or ')'
			// outside braces never occurs in a tag, so a '<' that reaches one
			// first is a comparison (i <n;) and stays code.
			end, braces, comparison := i+1, 0, false
			for end < len(code) && (code[end] != '>' || braces > 0) {
				switch code[end] {
				case '{':
					braces++
				case '}':
					if braces > 0 {
						braces--
					}
				case ';', '(', ')':
					comparison = braces == 0
				}
				if comparison {
					break
				}
				end++
			}
			if comparison {
				continue
			}
			if end == len(code) && braces > 0 {
				// Unbalanced braces (a brace inside a regex literal) fall back
				// to the first '>' rather than running the tag to the end.
				end = bytes.IndexByte(code[i+1:], '>')
				if end < 0 {
					break
				}
				end += i + 1
			}
			if end == len(code) {
				break
			}
			closing := code[i+1] == '/'
			self := end > i && code[end-1] == '/'
			if closing {
				if markupDepth > 0 {
					markupDepth--
				}
			} else if !self {
				markupDepth++
			}
			i = end
			continue
		}
		if markupDepth > 0 && expressionDepth == 0 {
			if c == '{' {
				expressionDepth = 1
				continue
			}
			code[i] = blankOrNewline(c)
			continue
		}
		if expressionDepth > 0 {
			if c == '{' {
				expressionDepth++
			}
			if c == '}' {
				expressionDepth--
			}
		}
		// A slash left after comment/string masking can start a regex. Losing an
		// arithmetic suffix is preferable to inventing executable regex contents.
		if c == '/' {
			for i < len(code) && code[i] != '\n' {
				code[i] = ' '
				i++
			}
			if i == len(code) {
				break
			}
		}
	}
	return string(code)
}

// flowExpressionKeywords are the words an expression, and so a JSX element,
// can follow; a `<` after one opens markup, never a type-argument list.
var flowExpressionKeywords = map[string]bool{
	"return": true, "yield": true, "await": true, "case": true, "default": true, "else": true,
	"do": true, "in": true, "of": true, "typeof": true, "instanceof": true, "void": true,
	"delete": true, "throw": true, "new": true,
}

// flowWordBefore returns the identifier that ends just before end.
func flowWordBefore(code []byte, end int) string {
	start := end
	for start > 0 && webIdentifierByte(code[start-1]) {
		start--
	}
	return string(code[start:end])
}

// typeArgumentsEnd returns the index of the '>' that closes the type-argument
// list opening at code[open] when only a call's '(' follows it, or 0. The list
// holds names, nested lists and the type punctuation of one line; a '(' ,
// ')', ';', '=' or line break inside it means it is not a type-argument list.
func typeArgumentsEnd(code []byte, open int) int {
	depth := 0
	for i := open; i < len(code); i++ {
		switch c := code[i]; {
		case c == '<':
			depth++
		case c == '>':
			depth--
			if depth == 0 {
				next := i + 1
				for next < len(code) && (code[next] == ' ' || code[next] == '\t') {
					next++
				}
				if next < len(code) && code[next] == '(' {
					return i
				}
				return 0
			}
		case webIdentifierByte(c) || strings.IndexByte(" \t,.[]|&{}:?'\"", c) >= 0:
		default:
			return 0
		}
	}
	return 0
}
