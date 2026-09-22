// Package pythonsyntax layers syntax validation over the closed Python 3.12
// subset grammar. It lives outside pythongrammar so an executable that only
// extracts facts — the separately buildable Python analyzer candidate — does
// not reach this route at all (ACP-009).

package pythonsyntax

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/pythongrammar"
)

// ParseValidated parses the closed Python 3.12 subset exactly as
// pythongrammar.ParsePython312Subset does, and additionally rejects sources the
// subset treats as necessarily invalid.
func ParseValidated(subject string, source []byte, collector pythongrammar.FactSink) pythongrammar.Failure {
	return pythongrammar.ParseChecked(subject, source, collector, invalidSyntax)
}

// SourceSyntaxValid validates the analyzer's closed Python 3.12 source grammar
// without applying its request, fact, or output capability limits.
func SourceSyntaxValid(path string, source []byte) bool {
	if !strings.HasSuffix(path, ".py") {
		return false
	}
	if !utf8.Valid(source) || len(source) == 0 || bytes.IndexByte(source, '\r') >= 0 || bytes.IndexByte(source, 0) >= 0 {
		return false
	}
	if pythongrammar.ContainsPython314ExceptList(source) {
		return false
	}
	return ParseValidated(path, source, discardFacts{}) == ""
}

type discardFacts struct{}

func (discardFacts) Add(_, _, _, _ string, _, _ int) pythongrammar.Failure { return "" }

// invalidSyntax rejects necessary-invalid shapes inside otherwise opaque spans.
// It deliberately avoids treating every expression outside the fact grammar as invalid.
func invalidSyntax(parser *pythongrammar.Parser) bool {
	if invalidOpaqueSpans(parser, parser.Tokens) {
		return true
	}
	depth := 0
	for index, token := range parser.Tokens {
		if token.Kind == pythongrammar.KindSymbol && token.End == token.Start+1 {
			switch parser.Source[token.Start] {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
		}
		if token.Kind == pythongrammar.KindNumber && invalidNumber(parser, index) {
			return true
		}
		if token.Kind == pythongrammar.KindString && invalidSimpleFString(parser, token) {
			return true
		}
		if token.Kind == pythongrammar.KindSymbol && token.End == token.Start+2 {
			if !walrusTarget(parser, index, depth) {
				return true
			}
			if depth == 0 && index+1 < len(parser.Tokens) && parser.Tokens[index+1].Kind == pythongrammar.KindNewline {
				return true
			}
			if invalidNamedExpressionValue(parser, parser.Tokens[index+1:]) {
				return true
			}
		}
	}
	for index := 0; index < len(parser.Tokens); {
		if parser.Tokens[index].Kind == pythongrammar.KindNewline || parser.Tokens[index].Kind == pythongrammar.KindIndent || parser.Tokens[index].Kind == pythongrammar.KindDedent {
			index++
			continue
		}
		end := parser.StatementEnd(index)
		statement := parser.Tokens[index:end]
		if len(statement) > 0 && statement[len(statement)-1].Kind == pythongrammar.KindNewline {
			statement = statement[:len(statement)-1]
		}
		if invalidStatement(parser, statement) {
			return true
		}
		index = end
		if index < len(parser.Tokens) && parser.Symbol(parser.Tokens[index], ";") {
			index++
		}
	}
	return false
}

func invalidStatement(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	if len(tokens) == 0 {
		return false
	}
	if parser.Symbol(tokens[0], "@") {
		return invalidDecorator(parser, tokens)
	}
	if parser.DefinitionStart(tokens) {
		colon := parser.HeaderColon(tokens, 0)
		return colon >= 0 && colon+1 < len(tokens) && invalidSimple(parser, tokens[colon+1:])
	}
	if parser.Name(tokens[0], "import") || parser.Name(tokens[0], "from") {
		return false
	}
	if parser.TypeAliasStart(tokens) {
		return invalidTypeAlias(parser, tokens)
	}
	if compoundStart(parser, tokens) {
		return invalidCompound(parser, tokens)
	}
	return invalidSimple(parser, tokens)
}

func invalidSimple(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	if len(tokens) == 0 {
		return false
	}
	switch parser.Text(tokens[0]) {
	case "pass", "break", "continue":
		return len(tokens) != 1
	case "global", "nonlocal":
		return invalidNameList(parser, tokens[1:])
	case "return", "raise", "assert", "del", "await", "yield":
		tail := tokens[1:]
		if parser.Name(tokens[0], "yield") && len(tail) > 0 && parser.Name(tail[0], "from") {
			tail = tail[1:]
			if len(tail) == 0 {
				return true
			}
		}
		if (parser.Name(tokens[0], "assert") || parser.Name(tokens[0], "del") || parser.Name(tokens[0], "await")) && len(tail) == 0 {
			return true
		}
		if parser.Name(tokens[0], "del") && len(tail) > 0 && (tail[0].Kind == pythongrammar.KindNumber || tail[0].Kind == pythongrammar.KindString || parser.Operand(tail[0]) && pythongrammar.PythonKeyword(parser.Text(tail[0]))) {
			return true
		}
		if len(tail) > 0 && invalidExpressionStart(parser, tail) {
			return true
		}
		if parser.Name(tokens[0], "raise") {
			marker := topLevelName(parser, tail, "from")
			if marker >= 0 && invalidExpressionStart(parser, tail[marker+1:]) {
				return true
			}
		}
	}
	if bareNamedExpression(parser, tokens) || topLevelWalrus(parser, tokens) || invalidExpressionStart(parser, tokens) || adjacentOperands(parser, tokens) || invalidNestedExpressions(parser, tokens) || danglingLambda(parser, tokens) {
		return true
	}
	for index := range tokens {
		if assignment(parser, tokens, index) && invalidExpressionStart(parser, tokens[index+1:]) {
			return true
		}
	}
	return false
}

func invalidNameList(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	if len(tokens) == 0 {
		return true
	}
	for index, token := range tokens {
		if index%2 == 0 {
			if token.Kind != pythongrammar.KindName || pythongrammar.SyntaxName(parser.Text(token)) != "" {
				return true
			}
			continue
		}
		if !parser.Symbol(token, ",") {
			return true
		}
	}
	return len(tokens)%2 == 0
}

func invalidDecorator(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	name, next, static, reason := parser.StaticCallee(tokens[1:])
	if reason != "" || !static || name == "" {
		return false
	}
	rest := tokens[1:]
	if next == len(rest) || !parser.Symbol(rest[next], "(") {
		return false
	}
	end := parser.Matching(rest, next)
	if end != len(rest)-1 {
		return false
	}
	parts, reason := parser.CommaSeparated(rest[next+1 : end])
	if reason != "" {
		return true
	}
	seenKeyword := false
	for _, part := range parts {
		if invalidExpressionStart(parser, part) || adjacentOperands(parser, part) || invalidNestedExpressions(parser, part) || parser.DanglingUnary(part) || danglingLambda(parser, part) {
			return true
		}
		for index := range part {
			if assignment(parser, part, index) && invalidExpressionStart(parser, part[index+1:]) {
				return true
			}
		}
		// '@decorator(x=1, 2)': once a keyword argument appears, every later
		// argument must itself be a keyword argument or a '*'/'**' unpack —
		// a plain positional argument may not follow one.
		if len(part) > 0 && parser.SymbolByte(part[0], '*') {
			continue
		}
		keyword := len(part) > 1 && part[0].Kind == pythongrammar.KindName && assignment(parser, part, 1)
		if seenKeyword && !keyword {
			return true
		}
		if keyword {
			seenKeyword = true
		}
	}
	return false
}

func invalidTypeAlias(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	for index := 2; index < len(tokens); index++ {
		if !assignment(parser, tokens, index) || !parser.TopLevel(tokens, index) {
			continue
		}
		right := tokens[index+1:]
		return invalidExpressionStart(parser, right) || adjacentOperands(parser, right) || invalidNestedExpressions(parser, right)
	}
	return true
}

func compoundStart(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	if len(tokens) == 0 || tokens[0].Kind != pythongrammar.KindName {
		return false
	}
	switch parser.Text(tokens[0]) {
	case "if", "elif", "else", "for", "while", "try", "except", "finally", "with":
		return true
	case "async":
		return len(tokens) > 1 && (parser.Name(tokens[1], "for") || parser.Name(tokens[1], "with"))
	case "match", "case":
		return parser.HasTopLevel(tokens[1:], ":")
	}
	return false
}

func invalidCompound(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	colon := parser.HeaderColon(tokens, 1)
	if colon < 0 {
		return true
	}
	word, start := parser.Text(tokens[0]), 1
	if word == "async" {
		word, start = parser.Text(tokens[1]), 2
	}
	header := tokens[start:colon]
	switch word {
	case "else", "try", "finally":
		if len(header) != 0 {
			return true
		}
	case "for":
		marker := topLevelName(parser, header, "in")
		if marker <= 0 || marker == len(header)-1 {
			return true
		}
		if adjacentOperands(parser, header[:marker]) || invalidTargetList(parser, header[:marker]) || invalidExpressionStart(parser, header[marker+1:]) || adjacentOperands(parser, header[marker+1:]) {
			return true
		}
	case "if", "elif", "while", "with", "match", "case":
		if invalidExpressionStart(parser, header) || adjacentOperands(parser, header) || invalidNestedExpressions(parser, header) || danglingExpression(parser, header) {
			return true
		}
		if word == "with" && danglingName(parser, header, "as") {
			return true
		}
	case "except":
		if adjacentOperands(parser, header) || invalidNestedExpressions(parser, header) || danglingName(parser, header, "as") {
			return true
		}
	}
	return colon+1 < len(tokens) && invalidSimple(parser, tokens[colon+1:])
}

// invalidTargetList reports whether tokens — a 'for' loop's target, before
// 'in' — contains a top-level binary or keyword operator. 'for x + y in
// values:' is not a valid target: a target list holds only names, attribute
// access ('.'), subscripts/tuples (nested brackets), and star targets ('*'),
// never an arithmetic or comparison expression.
func invalidTargetList(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	depth := 0
	for _, token := range tokens {
		if token.Kind == pythongrammar.KindSymbol && token.End == token.Start+1 {
			switch parser.Source[token.Start] {
			case '(', '[':
				depth++
				continue
			case ')', ']':
				depth--
				continue
			}
		}
		if depth != 0 {
			continue
		}
		if parser.KeywordOperator(token) {
			return true
		}
		if token.Kind == pythongrammar.KindSymbol && parser.BinaryConnector(token) && !parser.SymbolByte(token, '.') && !parser.SymbolByte(token, '*') {
			return true
		}
	}
	return false
}

func topLevelName(parser *pythongrammar.Parser, tokens []pythongrammar.Token, want string) int {
	for index, token := range tokens {
		if parser.TopLevel(tokens, index) && parser.Name(token, want) {
			return index
		}
	}
	return -1
}

func invalidExpressionStart(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	for len(tokens) > 0 && (tokens[0].Kind == pythongrammar.KindNewline || tokens[0].Kind == pythongrammar.KindIndent || tokens[0].Kind == pythongrammar.KindDedent) {
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return true
	}
	if tokens[0].Kind == pythongrammar.KindName {
		switch parser.Text(tokens[0]) {
		case "and", "or", "in", "is", "if", "else", "for", "async", "as", "from":
			return true
		}
		return false
	}
	if tokens[0].Kind != pythongrammar.KindSymbol {
		return false
	}
	value := parser.Source[tokens[0].Start]
	if value == '.' {
		if parser.EllipsisAt(tokens, 0) {
			return false
		}
		return len(tokens) < 2 || tokens[1].Kind != pythongrammar.KindNumber || tokens[0].End != tokens[1].Start
	}
	if value == ',' {
		return true
	}
	// A leading unary +/-/~/* is a valid expression start only if what
	// follows it is: 'x = +/ 2' pairs a unary '+' with an operator ('/')
	// that can never itself start an expression.
	if tokens[0].End == tokens[0].Start+1 && strings.IndexByte("+-~*", value) >= 0 {
		return invalidExpressionStart(parser, tokens[1:])
	}
	return strings.IndexByte("/%@&|^<>=!.", value) >= 0
}

func invalidNamedExpressionValue(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	for len(tokens) > 0 && (tokens[0].Kind == pythongrammar.KindNewline || tokens[0].Kind == pythongrammar.KindIndent || tokens[0].Kind == pythongrammar.KindDedent) {
		tokens = tokens[1:]
	}
	if invalidExpressionStart(parser, tokens) {
		return true
	}
	// Starred expressions are valid in displays and argument lists, but the
	// expression immediately following ':=' may not itself be starred.
	return parser.SymbolByte(tokens[0], '*')
}

func bareNamedExpression(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	return len(tokens) > 1 && tokens[0].Kind == pythongrammar.KindName && parser.Symbol(tokens[1], ":=")
}

func topLevelWalrus(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	for index := range tokens {
		if parser.Symbol(tokens[index], ":=") && parser.TopLevel(tokens, index) {
			return true
		}
	}
	return false
}

// walrusTarget reports whether the ':=' at index — the lexer's only two-byte
// symbol — follows a bare name, the sole target CPython accepts for a named
// expression.
func walrusTarget(parser *pythongrammar.Parser, index, depth int) bool {
	if index == 0 || parser.Tokens[index-1].Kind != pythongrammar.KindName || pythongrammar.PythonKeyword(parser.Text(parser.Tokens[index-1])) {
		return false
	}
	if index < 2 {
		return true
	}
	if parser.SymbolByte(parser.Tokens[index-2], '.') {
		return false
	}
	// Outside brackets a named expression is not a tuple element, so 'a, b := 1'
	// is invalid where the bracketed 'g(a, b := 1)' is not.
	return depth != 0 || !parser.SymbolByte(parser.Tokens[index-2], ',')
}

func adjacentOperands(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	previous := -1
	for index, token := range tokens {
		if token.Kind == pythongrammar.KindNewline || token.Kind == pythongrammar.KindIndent || token.Kind == pythongrammar.KindDedent {
			previous = -1
			continue
		}
		// 'x = 1 {2}': a '{' display can never be a trailer, so it may not
		// directly follow an operand — unlike '(' (call) and '[' (subscript).
		if previous >= 0 && parser.SymbolByte(token, '{') {
			return true
		}
		// 'type Alias = first if else': a ternary 'if' can never be directly
		// followed by 'else' — a test expression must sit between them.
		if index > 0 && parser.Name(token, "else") && parser.Name(tokens[index-1], "if") {
			return true
		}
		if !parser.Operand(token) {
			previous = -1
			continue
		}
		if previous >= 0 && !(tokens[previous].Kind == pythongrammar.KindString && token.Kind == pythongrammar.KindString) {
			return true
		}
		previous = index
	}
	return false
}

func invalidNestedExpressions(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if !parser.ExpressionBracket(token) {
			continue
		}
		end := parser.Matching(tokens, index)
		if end < 0 {
			return true
		}
		interior := tokens[index+1 : end]
		if len(interior) > 0 && (invalidExpressionStart(parser, interior) || invalidNestedExpressions(parser, interior)) {
			return true
		}
		if end+1 < len(tokens) && parser.Operand(tokens[end+1]) {
			return true
		}
		index = end
	}
	return false
}

// invalidOpaqueSpans validates the expression structure that the fact grammar
// deliberately leaves opaque. It stays conservative: parameter and import
// lists keep their owning grammar, while displays, subscripts, groupings and
// call arguments must carry valid elements and separators.
func invalidOpaqueSpans(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	for index := 0; index < len(tokens); index++ {
		if !parser.ExpressionBracket(tokens[index]) {
			continue
		}
		end := parser.Matching(tokens, index)
		if end < 0 {
			return true
		}
		interior := tokens[index+1 : end]
		if invalidOpaqueSpans(parser, interior) {
			return true
		}
		if !parameterOrImportSpan(parser, tokens, index) && invalidOpaqueInterior(parser, tokens, index, interior) {
			return true
		}
		index = end
	}
	return false
}

func parameterOrImportSpan(parser *pythongrammar.Parser, tokens []pythongrammar.Token, open int) bool {
	if open > 0 && (parser.Name(tokens[open-1], "import") || parser.Name(tokens[open-1], "from")) {
		return true
	}
	if open < 2 || tokens[open-1].Kind != pythongrammar.KindName {
		return false
	}
	return parser.Name(tokens[open-2], "def") || open > 2 && parser.Name(tokens[open-3], "async") && parser.Name(tokens[open-2], "def")
}

func invalidOpaqueInterior(parser *pythongrammar.Parser, enclosing []pythongrammar.Token, open int, interior []pythongrammar.Token) bool {
	parts, invalid := opaqueParts(parser, interior)
	if invalid {
		return true
	}
	for _, part := range parts {
		if invalidExpressionStart(parser, part) || invalidOpaqueSequence(parser, part) || mixedAdjacentLiterals(parser, part) {
			return true
		}
	}
	opener := parser.Source[enclosing[open].Start]
	if opener == '{' && invalidBraceParts(parser, parts) {
		return true
	}
	return opener == '(' && callSpan(parser, enclosing, open) && invalidCallParts(parser, parts)
}

func opaqueParts(parser *pythongrammar.Parser, tokens []pythongrammar.Token) ([][]pythongrammar.Token, bool) {
	if len(trimLayout(tokens)) == 0 {
		return nil, false
	}
	parts := make([][]pythongrammar.Token, 0, 4)
	start, lambda := 0, false
	for index := range tokens {
		if parser.TopLevel(tokens, index) && parser.Name(tokens[index], "lambda") {
			lambda = true
			continue
		}
		if lambda && parser.TopLevel(tokens, index) && parser.Symbol(tokens[index], ":") {
			lambda = false
			continue
		}
		if lambda {
			continue
		}
		if !parser.Symbol(tokens[index], ",") || !parser.TopLevel(tokens, index) {
			continue
		}
		part := trimLayout(tokens[start:index])
		if len(part) == 0 {
			return nil, true
		}
		parts = append(parts, part)
		start = index + 1
	}
	last := trimLayout(tokens[start:])
	if len(last) > 0 {
		parts = append(parts, last)
	}
	return parts, false
}

func invalidOpaqueSequence(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	expectOperand, lastString, symbolUnary, previousWord := true, false, false, ""
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if layoutToken(token) {
			continue
		}
		if parser.ExpressionBracket(token) {
			end := parser.Matching(tokens, index)
			if end < 0 || !expectOperand && parser.SymbolByte(token, '{') {
				return true
			}
			expectOperand, lastString, symbolUnary, previousWord = false, false, false, ""
			index = end
			continue
		}
		if expectOperand && parser.Name(token, "lambda") {
			colon := parser.SpanColon(tokens, index+1)
			if colon < 0 {
				return true
			}
			index = colon
			lastString, symbolUnary, previousWord = false, false, ""
			continue
		}
		if token.Kind == pythongrammar.KindName && parser.KeywordOperator(token) {
			word := parser.Text(token)
			if expectOperand && symbolUnary {
				return true
			}
			if word == "not" {
				if !expectOperand && !nextTopLevelName(parser, tokens, index+1, "in") {
					return true
				}
				previousWord = word
				continue
			}
			if expectOperand && !(word == "for" && previousWord == "async") {
				return true
			}
			expectOperand, lastString, symbolUnary, previousWord = true, false, false, word
			continue
		}
		if expectOperand && token.Kind == pythongrammar.KindName && (parser.Name(token, "await") || parser.Name(token, "yield") || previousWord == "yield" && parser.Name(token, "from")) {
			if symbolUnary {
				return true
			}
			previousWord = parser.Text(token)
			continue
		}
		if !expectOperand && token.Kind == pythongrammar.KindName && parser.Name(token, "as") {
			expectOperand, lastString, symbolUnary, previousWord = true, false, false, "as"
			continue
		}
		if parser.EllipsisAt(tokens, index) {
			if !expectOperand {
				return true
			}
			expectOperand, lastString, symbolUnary, previousWord = false, false, false, ""
			index += 2
			continue
		}
		if parser.Operand(token) || token.Kind == pythongrammar.KindName {
			if !expectOperand && !(lastString && token.Kind == pythongrammar.KindString) {
				return true
			}
			expectOperand, lastString, symbolUnary, previousWord = false, token.Kind == pythongrammar.KindString, false, ""
			continue
		}
		if parser.Symbol(token, ":") {
			expectOperand, lastString, symbolUnary, previousWord = true, false, false, ""
			continue
		}
		if assignment(parser, tokens, index) {
			if !opaqueKeywordAssignment(tokens[:index]) {
				return true
			}
			expectOperand, lastString, symbolUnary, previousWord = true, false, false, ""
			continue
		}
		if !parser.BinaryConnector(token) {
			return true
		}
		if expectOperand {
			if token.End != token.Start+1 || strings.IndexByte("+-~*", parser.Source[token.Start]) < 0 {
				return true
			}
			if parser.SymbolByte(token, '*') && index+1 < len(tokens) && parser.SymbolByte(tokens[index+1], '*') && token.End == tokens[index+1].Start {
				index++
			}
			symbolUnary = true
		} else {
			for index+1 < len(tokens) && parser.BinaryConnector(tokens[index+1]) && tokens[index].End == tokens[index+1].Start {
				index++
			}
		}
		expectOperand, lastString, previousWord = true, false, ""
	}
	if !expectOperand {
		return false
	}
	last := tokens[len(tokens)-1]
	return !parser.Symbol(last, ":") && (len(tokens) < 3 || !parser.EllipsisAt(tokens, len(tokens)-3))
}

func opaqueKeywordAssignment(tokens []pythongrammar.Token) bool {
	var significant []pythongrammar.Token
	for _, token := range tokens {
		if !layoutToken(token) {
			significant = append(significant, token)
		}
	}
	return len(significant) == 1 && significant[0].Kind == pythongrammar.KindName
}

func nextTopLevelName(parser *pythongrammar.Parser, tokens []pythongrammar.Token, start int, want string) bool {
	for index := start; index < len(tokens); index++ {
		if layoutToken(tokens[index]) {
			continue
		}
		return parser.TopLevel(tokens, index) && parser.Name(tokens[index], want)
	}
	return false
}

func trimLayout(tokens []pythongrammar.Token) []pythongrammar.Token {
	for len(tokens) > 0 && layoutToken(tokens[0]) {
		tokens = tokens[1:]
	}
	for len(tokens) > 0 && layoutToken(tokens[len(tokens)-1]) {
		tokens = tokens[:len(tokens)-1]
	}
	return tokens
}

func layoutToken(token pythongrammar.Token) bool {
	return token.Kind == pythongrammar.KindNewline || token.Kind == pythongrammar.KindIndent || token.Kind == pythongrammar.KindDedent
}

func callSpan(parser *pythongrammar.Parser, tokens []pythongrammar.Token, open int) bool {
	if open == 0 {
		return false
	}
	previous := tokens[open-1]
	if parser.Operand(previous) {
		return true
	}
	return parser.SymbolByte(previous, ')') || parser.SymbolByte(previous, ']') || parser.SymbolByte(previous, '}')
}

func invalidCallParts(parser *pythongrammar.Parser, parts [][]pythongrammar.Token) bool {
	seenKeyword := false
	for _, part := range parts {
		if parser.SymbolByte(part[0], '*') {
			continue
		}
		keyword := false
		for index := range part {
			if !assignment(parser, part, index) || !parser.TopLevel(part, index) {
				continue
			}
			keyword = index == 1 && part[0].Kind == pythongrammar.KindName
			if keyword && topLevelWalrus(parser, part[index+1:]) {
				return true
			}
			if !keyword && topLevelName(parser, part, "lambda") < 0 {
				return true
			}
			break
		}
		if seenKeyword && !keyword {
			return true
		}
		seenKeyword = seenKeyword || keyword
	}
	return false
}

func invalidBraceParts(parser *pythongrammar.Parser, parts [][]pythongrammar.Token) bool {
	class := 0
	for _, part := range parts {
		if topLevelName(parser, part, "for") >= 0 {
			return false
		}
		current := 2
		colon := topLevelColon(parser, part)
		if colon >= 0 && topLevelWalrus(parser, part) {
			return true
		}
		if colon >= 0 || len(part) > 1 && parser.SymbolByte(part[0], '*') && parser.SymbolByte(part[1], '*') {
			current = 1
		}
		if class != 0 && class != current {
			return true
		}
		class = current
	}
	return false
}

func topLevelColon(parser *pythongrammar.Parser, tokens []pythongrammar.Token) int {
	lambda := false
	for index := range tokens {
		if !parser.TopLevel(tokens, index) {
			continue
		}
		if parser.Name(tokens[index], "lambda") {
			lambda = true
			continue
		}
		if parser.Symbol(tokens[index], ":") {
			if lambda {
				lambda = false
				continue
			}
			return index
		}
	}
	return -1
}

func mixedAdjacentLiterals(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	previous := -1
	for index, token := range tokens {
		if layoutToken(token) {
			continue
		}
		if token.Kind != pythongrammar.KindString {
			previous = -1
			continue
		}
		if previous >= 0 && bytesLiteral(parser, tokens[previous]) != bytesLiteral(parser, token) {
			return true
		}
		previous = index
	}
	return false
}

func bytesLiteral(parser *pythongrammar.Parser, token pythongrammar.Token) bool {
	quote, found := pythongrammar.StringStart(parser.Source, token.Start)
	return found && strings.ContainsAny(string(parser.Source[token.Start:quote]), "bB")
}

func danglingExpression(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	for len(tokens) > 0 && tokens[len(tokens)-1].Kind == pythongrammar.KindNewline {
		tokens = tokens[:len(tokens)-1]
	}
	if len(tokens) == 0 {
		return false
	}
	last := tokens[len(tokens)-1]
	if parser.BinaryConnector(last) || parser.KeywordOperator(last) {
		return true
	}
	return false
}

func danglingLambda(parser *pythongrammar.Parser, tokens []pythongrammar.Token) bool {
	for index, token := range tokens {
		if !parser.Name(token, "lambda") {
			continue
		}
		colon := parser.SpanColon(tokens, index+1)
		if colon >= 0 && colon == len(tokens)-1 {
			return true
		}
	}
	return false
}

func danglingName(parser *pythongrammar.Parser, tokens []pythongrammar.Token, want string) bool {
	for index, token := range tokens {
		if parser.Name(token, want) && (index+1 == len(tokens) || parser.Symbol(tokens[index+1], ",")) {
			return true
		}
	}
	return false
}

func assignment(parser *pythongrammar.Parser, tokens []pythongrammar.Token, index int) bool {
	return parser.Symbol(tokens[index], "=") && !parser.Comparison(tokens, index)
}

func invalidNumber(parser *pythongrammar.Parser, index int) bool {
	value := strings.ToLower(parser.Text(parser.Tokens[index]))
	if strings.Contains(value, "__") || strings.HasSuffix(value, "_") || strings.Contains(value, "._") || strings.Contains(value, "_.") || strings.Contains(value, "e_") || strings.Contains(value, "_e") {
		return true
	}
	if strings.HasPrefix(value, "0b") {
		return invalidBasedDigits(value[2:], "01")
	}
	if strings.HasPrefix(value, "0o") {
		return invalidBasedDigits(value[2:], "01234567")
	}
	if strings.HasPrefix(value, "0x") {
		return invalidBasedDigits(value[2:], "0123456789abcdef")
	}
	if strings.Contains(value, "..") {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && !strings.ContainsRune("_.ej", character) {
			return true
		}
	}
	if strings.Count(value, ".") > 1 || strings.Count(value, "e") > 1 {
		return true
	}
	// '1e2.3': the decimal point of a float must precede its exponent, not
	// follow it — 'e' starts the exponent digits, so no '.' may come after.
	if eAt := strings.IndexByte(value, 'e'); eAt >= 0 {
		if dotAt := strings.LastIndexByte(value, '.'); dotAt > eAt {
			return true
		}
	}
	if at := strings.IndexByte(value, 'j'); at >= 0 && at != len(value)-1 {
		return true
	}
	plain := strings.TrimSuffix(value, "j")
	if !strings.ContainsAny(plain, ".e") && len(plain) > 1 && plain[0] == '0' {
		for _, digit := range plain[1:] {
			if digit >= '1' && digit <= '9' {
				return true
			}
		}
	}
	if strings.HasSuffix(plain, "e") {
		if index+2 >= len(parser.Tokens) || !parser.Symbol(parser.Tokens[index+1], "+") && !parser.Symbol(parser.Tokens[index+1], "-") || parser.Tokens[index].End != parser.Tokens[index+1].Start || parser.Tokens[index+1].End != parser.Tokens[index+2].Start || parser.Tokens[index+2].Kind != pythongrammar.KindNumber {
			return true
		}
	}
	return false
}

func invalidBasedDigits(value, digits string) bool {
	if strings.HasPrefix(value, "_") {
		value = value[1:]
	}
	if value == "" {
		return true
	}
	for _, digit := range value {
		if digit != '_' && !strings.ContainsRune(digits, digit) {
			return true
		}
	}
	return false
}

func invalidSimpleFString(parser *pythongrammar.Parser, token pythongrammar.Token) bool {
	quote, found := pythongrammar.StringStart(parser.Source, token.Start)
	if !found || !pythongrammar.FString(parser.Source[token.Start:quote]) {
		return false
	}
	// One pass, three modes: 0 is literal text, where a brace must be doubled;
	// 1 is a replacement field, where displays, calls, subscripts and string
	// literals nest; 2 is a format spec, which is literal text in which only
	// nested replacement fields nest. specs counts the enclosing specs, so a
	// nested field returns to its spec rather than to the literal text.
	mode, depth, specs, start := 0, 0, 0, 0
	for index := quote + 1; index < token.End; index++ {
		value := parser.Source[index]
		if mode == 1 {
			switch {
			case value == '\'' || value == '"':
				index = pythongrammar.SkipString(parser.Source[:token.End], index) - 1
			case strings.IndexByte("([{", value) >= 0:
				depth++
			case depth > 0:
				if strings.IndexByte(")]}", value) >= 0 {
					depth--
				}
			case value == '}', value == ':', value == '!' && index+1 < token.End && parser.Source[index+1] != '=':
				if value == '!' && (index+2 >= token.End || !strings.ContainsRune("sra", rune(parser.Source[index+1])) || parser.Source[index+2] != ':' && parser.Source[index+2] != '}') {
					return true
				}
				if invalidField(parser, parser.Source[start:index], value == ':') {
					return true
				}
				if mode, specs = 2, specs+1; value == '}' {
					if mode, specs = 0, specs-1; specs > 0 {
						mode = 2
					}
				}
			}
			continue
		}
		if value != '{' && value != '}' {
			continue
		}
		if mode == 2 {
			if value == '{' {
				mode, depth, start = 1, 0, index+1
				continue
			}
			if specs--; specs > 0 {
				continue
			}
			mode = 0
			continue
		}
		if index+1 < token.End && parser.Source[index+1] == value {
			index++
			continue
		}
		// Literal text carries doubled braces, so a lone '}' is invalid.
		if value == '}' {
			return true
		}
		mode, depth, start = 1, 0, index+1
	}
	return mode != 0
}

// invalidField reports whether a replacement field's expression is necessarily
// invalid, after dropping the trailing '=' of a self-documenting field. An empty
// expression is: CPython rejects f"{}" and f"{:>10}".
func invalidField(parser *pythongrammar.Parser, field []byte, formatStart bool) bool {
	if trimmed := bytes.TrimRight(field, " \t"); len(trimmed) > 0 && trimmed[len(trimmed)-1] == '=' && (len(trimmed) == 1 || strings.IndexByte("=!<>", trimmed[len(trimmed)-2]) < 0) {
		field = trimmed[:len(trimmed)-1]
	}
	if bytes.IndexByte(field, '\n') >= 0 {
		return false
	}
	tokens, reason := pythongrammar.LexPython312(field)
	fieldParser := &pythongrammar.Parser{Source: field, Tokens: tokens}
	if formatStart && topLevelName(fieldParser, tokens, "lambda") >= 0 {
		return true
	}
	if reason != "" || invalidExpressionStart(fieldParser, tokens) || adjacentOperands(fieldParser, tokens) || invalidNestedExpressions(fieldParser, tokens) || danglingExpression(fieldParser, tokens) {
		return true
	}
	for index := range tokens {
		if tokens[index].Kind == pythongrammar.KindNumber && invalidNumber(fieldParser, index) {
			return true
		}
	}
	return false
}
