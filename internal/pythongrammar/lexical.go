package pythongrammar

import (
	"bytes"
	"strings"
)

// These lexical helpers are shared by the parser and by callers that scan
// Python source without parsing it.

// ContainsPython314ExceptList reports whether the source uses the 3.14 bare
// `except` list form, which this 3.12 subset does not accept.
func ContainsPython314ExceptList(content []byte) bool {
	for index := 0; index < len(content); {
		if content[index] == '#' {
			for index < len(content) && content[index] != '\n' {
				index++
			}
			continue
		}
		if quote, found := StringStart(content, index); found {
			index = SkipString(content, quote)
			continue
		}
		if !pythonWordAt(content, index, "except") {
			index++
			continue
		}
		index += len("except")
		depth := 0
		for index < len(content) {
			if content[index] == '#' {
				for index < len(content) && content[index] != '\n' {
					index++
				}
				if depth == 0 {
					break
				}
				continue
			}
			if content[index] == '\n' {
				if depth > 0 || index > 0 && content[index-1] == '\\' {
					index++
					continue
				}
				break
			}
			if quote, found := StringStart(content, index); found {
				index = SkipString(content, quote)
				continue
			}
			switch content[index] {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				if depth > 0 {
					depth--
				}
			case ',':
				if depth == 0 {
					return true
				}
			case ':':
				if depth != 0 {
					break
				}
				for index < len(content) && content[index] != '\n' {
					index++
				}
				continue
			}
			index++
		}
	}
	return false
}

func pythonWordAt(content []byte, index int, word string) bool {
	end := index + len(word)
	if end > len(content) || !bytes.Equal(content[index:end], []byte(word)) {
		return false
	}
	return (index == 0 || !pythonWordByte(content[index-1])) && (end == len(content) || !pythonWordByte(content[end]))
}

// StringStart reports whether a string literal opens at index, returning the
// index of its opening quote.
func StringStart(content []byte, index int) (int, bool) {
	start := index
	for index < len(content) && index-start < 3 && strings.ContainsRune("rRbBuUfFtT", rune(content[index])) {
		index++
	}
	if index < len(content) && (content[index] == '\'' || content[index] == '"') {
		return index, true
	}
	if start < len(content) && (content[start] == '\'' || content[start] == '"') {
		return start, true
	}
	return 0, false
}

// SkipString returns the index just past the string literal opening at quoteAt.
func SkipString(content []byte, quoteAt int) int {
	quote := content[quoteAt]
	triple := quoteAt+2 < len(content) && content[quoteAt+1] == quote && content[quoteAt+2] == quote
	if triple {
		quoteAt += 3
	} else {
		quoteAt++
	}
	for quoteAt < len(content) {
		if content[quoteAt] == '\\' {
			quoteAt += 2
			continue
		}
		if triple && quoteAt+2 < len(content) && content[quoteAt] == quote && content[quoteAt+1] == quote && content[quoteAt+2] == quote {
			return quoteAt + 3
		}
		if !triple && content[quoteAt] == quote {
			return quoteAt + 1
		}
		quoteAt++
	}
	return quoteAt
}

func SyntaxName(value string) failure {
	if PythonKeyword(value) {
		return failureMalformed
	}
	if !pythonName(value) {
		return failureDynamic
	}
	return ""
}

func staticModule(value string) failure {
	value = strings.TrimLeft(value, ".")
	if value == "" {
		return ""
	}
	for _, part := range strings.Split(value, ".") {
		if reason := SyntaxName(part); reason != "" {
			return reason
		}
	}
	return ""
}

var pythonKeywords = map[string]struct{}{"False": {}, "None": {}, "True": {}, "and": {}, "as": {}, "assert": {}, "async": {}, "await": {}, "break": {}, "class": {}, "continue": {}, "def": {}, "del": {}, "elif": {}, "else": {}, "except": {}, "finally": {}, "for": {}, "from": {}, "global": {}, "if": {}, "import": {}, "in": {}, "is": {}, "lambda": {}, "nonlocal": {}, "not": {}, "or": {}, "pass": {}, "raise": {}, "return": {}, "try": {}, "while": {}, "with": {}, "yield": {}}

func pythonWordByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func pythonName(value string) bool {
	if len(value) == 0 || len(value) > 128 || PythonKeyword(value) {
		return false
	}
	for index := range value {
		c := value[index]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || index > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func PythonKeyword(value string) bool {
	_, found := pythonKeywords[value]
	return found
}

// FString reports whether the given literal prefix marks a formatted string.
func FString(prefix []byte) bool {
	for _, value := range prefix {
		if value == 'f' || value == 'F' {
			return true
		}
	}
	return false
}

// numberFollowsDot reports whether the dot at `index` opens a float literal
// rather than a delimiter. CPython's tokenizer makes exactly this test: a dot
// begins a NUMBER only when a decimal digit follows it immediately.
func numberFollowsDot(source []byte, index int) bool {
	return index+1 < len(source) && source[index+1] >= '0' && source[index+1] <= '9'
}
