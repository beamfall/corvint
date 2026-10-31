package contextindex

import "strings"

// FlowCodeLines shares the native literal/comment masks with the bounded flow
// producer. It preserves line/column positions; masked bytes are never facts.
// Web interpolation and dynamic Ruby dispatch intentionally remain unresolved.
func FlowCodeLines(language, text string) []string {
	if language != "ruby" {
		return strings.Split(flowWebMask(text), "\n")
	}
	scanner := rubyScanner{}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = scanner.scanLine(line)
	}
	return lines
}

// flowWebMask additionally treats slash expressions conservatively and blanks
// JSX text. The native web mask does not claim a JavaScript regex/JSX grammar.
func flowWebMask(text string) string {
	code := []byte(stripWebNonCode(text))
	markupDepth, expressionDepth := 0, 0
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c == '<' && i > 0 && webIdentifierByte(code[i-1]) {
			// A type-argument list right after a name and before a call
			// (useMemo<T>(, useState<Map<K, V>>() is not markup: only the list
			// is passed over, so the call and what follows stay code (V1-2415).
			if end := typeArgumentsEnd(code, i); end > 0 {
				i = end
				continue
			}
		}
		if c == '<' && i+1 < len(code) && (code[i+1] >= 'A' && code[i+1] <= 'Z' || code[i+1] >= 'a' && code[i+1] <= 'z' || code[i+1] == '/') {
			end := i + 1
			for end < len(code) && code[end] != '>' {
				end++
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
