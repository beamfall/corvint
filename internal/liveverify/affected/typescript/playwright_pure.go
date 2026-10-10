package typescript

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// playwrightImpureKeywords start expressions or statements that can run code, write state or
// suspend evaluation.
var playwrightImpureKeywords = map[string]bool{
	"async": true, "await": true, "break": true, "case": true, "catch": true, "class": true, "const": true,
	"continue": true, "debugger": true, "default": true, "delete": true, "do": true, "else": true,
	"export": true, "extends": true, "finally": true, "for": true, "function": true, "if": true,
	"import": true, "let": true, "new": true, "return": true, "super": true, "switch": true,
	"throw": true, "try": true, "var": true, "while": true, "with": true, "yield": true,
}

// playwrightOperatorKeywords are operators spelled as words; an operand follows each.
var playwrightOperatorKeywords = map[string]bool{"in": true, "instanceof": true, "typeof": true, "void": true}

// playwrightPunctuators are the admitted operators and separators, longest match first.
var playwrightPunctuators = []string{
	"!==", "===", ">>>", "**", "==", "!=", "<=", ">=", "&&", "||", "??", "?.", "<<", ">>",
	"+", "-", "*", "/", "%", "<", ">", "!", "~", "&", "|", "^", "?", ":", ".", ",", "(", ")", "[", "]", "{", "}",
}

// playwrightCompoundOperators form a compound assignment when '=' follows them.
var playwrightCompoundOperators = []string{">>>", "**", "&&", "||", "??", "<<", ">>", "+", "-", "*", "/", "%", "&", "|", "^"}

// playwrightPureExpression reports whether evaluating raw cannot run code or write state, so a
// non-identity `use` value cannot alter a devices descriptor or another project's identity
// (TJAA-V0-018). It admits literals, identifiers, member reads (including optional chaining and
// process.env.X), template literals whose substitutions are themselves admitted, array literals,
// object literals of `key: value` or shorthand properties, and unary, binary, logical and
// conditional combinations of those. It refuses every call (including tagged templates and
// optional calls), assignment, update, delete, new, await, yield, import, function, arrow and
// class expressions, spread, method or accessor definitions, the comma operator, regular
// expression literals and anything it does not recognize.
func playwrightPureExpression(raw string) bool {
	stack := []byte{}
	operandEnd := false
	tokens := 0
	for index := skipPlaywrightSpace(raw, 0); index < len(raw); index = skipPlaywrightSpace(raw, index) {
		tokens++
		character := raw[index]
		switch {
		case character == '\'' || character == '"' || character == '`':
			if operandEnd {
				return false // a template after an operand is a tagged template: a call
			}
			end, ok := playwrightQuotedEnd(raw, index)
			if character == '`' {
				end, ok = playwrightTemplateEnd(raw, index, true)
			}
			if !ok {
				return false
			}
			index, operandEnd = end, true
			continue
		case character >= '0' && character <= '9' || character == '.' && index+1 < len(raw) && raw[index+1] >= '0' && raw[index+1] <= '9':
			if operandEnd {
				return false
			}
			end := index + 1
			for end < len(raw) && (raw[end] == '.' || raw[end] == '_' || raw[end] >= '0' && raw[end] <= '9' || raw[end] >= 'a' && raw[end] <= 'z' || raw[end] >= 'A' && raw[end] <= 'Z') {
				end++
			}
			index, operandEnd = end, true
			continue
		}
		if word, end := playwrightIdentifier(raw, index); end > index {
			switch {
			case playwrightImpureKeywords[word]:
				return false
			case playwrightOperatorKeywords[word]:
				operandEnd = false
			case operandEnd:
				return false // adjacent operands only parse as an accessor or other definition
			default:
				operandEnd = true
			}
			index = end
			continue
		}
		punctuator := ""
		for _, candidate := range playwrightPunctuators {
			if strings.HasPrefix(raw[index:], candidate) {
				punctuator = candidate
				break
			}
		}
		next := raw[index+len(punctuator):]
		switch {
		case punctuator == "":
			return false // =, =>, ;, #, @, \ and other unrecognized text
		case strings.HasPrefix(next, "=") && slices.Contains(playwrightCompoundOperators, punctuator):
			return false // compound assignment
		case punctuator == "+" && strings.HasPrefix(next, "+"), punctuator == "-" && strings.HasPrefix(next, "-"):
			return false // update
		case punctuator == "." && strings.HasPrefix(next, ".."):
			return false // spread
		case punctuator == "/" && !operandEnd:
			return false // regular expression literal
		}
		index += len(punctuator)
		switch punctuator {
		case "(", "{":
			if operandEnd {
				return false // call, or a block after an operand
			}
			stack = append(stack, punctuator[0])
			operandEnd = false
		case "[":
			stack = append(stack, '[')
			operandEnd = false
		case ")", "]", "}":
			if len(stack) == 0 || !playwrightPair(stack[len(stack)-1], punctuator[0]) {
				return false
			}
			stack = stack[:len(stack)-1]
			operandEnd = true
		case ",":
			if len(stack) == 0 || stack[len(stack)-1] == '(' {
				return false // comma operator
			}
			operandEnd = false
		case ".", "?.":
			index = skipPlaywrightSpace(raw, index)
			if punctuator == "?." && strings.HasPrefix(raw[index:], "[") {
				operandEnd = false
				continue
			}
			word, end := playwrightIdentifier(raw, index)
			if word == "" {
				return false // optional call, private name or malformed member
			}
			index, operandEnd = end, true
		default:
			operandEnd = false
		}
	}
	return tokens != 0 && len(stack) == 0
}

func skipPlaywrightSpace(raw string, index int) int {
	for index < len(raw) && strings.IndexByte(" \t\r\n", raw[index]) >= 0 {
		index++
	}
	return index
}

// playwrightIdentifier returns the identifier or keyword at index and its end, or end == index.
func playwrightIdentifier(raw string, index int) (string, int) {
	end := index
	for end < len(raw) {
		character, size := utf8.DecodeRuneInString(raw[end:])
		if character != '_' && character != '$' && !unicode.IsLetter(character) && (end == index || !unicode.IsDigit(character)) {
			break
		}
		end += size
	}
	return raw[index:end], end
}

// playwrightQuotedEnd returns the index after the single- or double-quoted string at index.
func playwrightQuotedEnd(raw string, index int) (int, bool) {
	quote := raw[index]
	for cursor := index + 1; cursor < len(raw); cursor++ {
		switch raw[cursor] {
		case '\\':
			cursor++
		case '\n', '\r':
			return 0, false
		case quote:
			return cursor + 1, true
		}
	}
	return 0, false
}

// playwrightTemplateEnd returns the index after the template literal at index. With validate,
// every substitution must itself be a pure expression; nested templates are only skipped while
// a substitution's end is found, so each level is validated once.
func playwrightTemplateEnd(raw string, index int, validate bool) (int, bool) {
	for cursor := index + 1; cursor < len(raw); cursor++ {
		switch {
		case raw[cursor] == '\\':
			cursor++
		case raw[cursor] == '`':
			return cursor + 1, true
		case raw[cursor] == '$' && cursor+1 < len(raw) && raw[cursor+1] == '{':
			end, ok := playwrightSubstitutionEnd(raw, cursor+2)
			if !ok || validate && !playwrightPureExpression(raw[cursor+2:end]) {
				return 0, false
			}
			cursor = end
		}
	}
	return 0, false
}

// playwrightSubstitutionEnd returns the index of the brace closing a template substitution that
// starts at index, skipping nested strings, templates and braces.
func playwrightSubstitutionEnd(raw string, index int) (int, bool) {
	depth := 0
	for cursor := index; cursor < len(raw); cursor++ {
		switch raw[cursor] {
		case '\'', '"', '`':
			end, ok := playwrightQuotedEnd(raw, cursor)
			if raw[cursor] == '`' {
				end, ok = playwrightTemplateEnd(raw, cursor, false)
			}
			if !ok {
				return 0, false
			}
			cursor = end - 1
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return cursor, true
			}
			depth--
		}
	}
	return 0, false
}
