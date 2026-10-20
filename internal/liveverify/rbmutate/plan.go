package rbmutate

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/analyzerruby"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// Operator names: the bounded Ruby set (FPK-V0-068). Each mutant is one
// line-confined rewrite of the source text, planned in Go over the masked code
// view of the repository's Ruby lexer; no Ruby runs to produce a mutant.
const (
	OperatorRelationalBoundary = "relational-boundary"
	OperatorRelationalNegation = "relational-negation"
	OperatorArithmeticSwap     = "arithmetic-swap"
	OperatorConstantDecrement  = "constant-decrement"
	OperatorConstantIncrement  = "constant-increment"
	OperatorNegateCondition    = "negate-condition"
)

// binarySwaps is the closed operator table, keyed by the exact operator token.
var binarySwaps = map[string]struct{ operator, replacement string }{
	"<": {OperatorRelationalBoundary, "<="}, "<=": {OperatorRelationalBoundary, "<"},
	">": {OperatorRelationalBoundary, ">="}, ">=": {OperatorRelationalBoundary, ">"},
	"==": {OperatorRelationalNegation, "!="}, "!=": {OperatorRelationalNegation, "=="},
	"+": {OperatorArithmeticSwap, "-"}, "-": {OperatorArithmeticSwap, "+"},
	"*": {OperatorArithmeticSwap, "/"}, "/": {OperatorArithmeticSwap, "*"},
}

// compoundOperators are read whole, longest first, so `<=>` is never `<=`,
// `+=` is never `+`, and `**` is never `*`.
var compoundOperators = []string{
	"**=", "<=>", "===", "...", "<<=", ">>=", "&&=", "||=",
	"**", "==", "!=", "<=", ">=", "=~", "!~", "&&", "||", "<<", ">>",
	"+=", "-=", "*=", "/=", "%=", "|=", "&=", "^=", "->", "=>", "..", "::", "&.",
}

// conditionKeywords open a condition the planner may negate.
var conditionKeywords = map[string]bool{"if": true, "elsif": true, "unless": true, "while": true, "until": true}

// operandKeywords end an operand; every other keyword makes a following
// operator unary or meaningless, so it is never mutated.
var operandKeywords = map[string]bool{"true": true, "false": true, "nil": true, "self": true, "end": true}

var rubyKeywords = map[string]bool{
	"alias": true, "and": true, "begin": true, "break": true, "case": true, "class": true, "def": true,
	"defined?": true, "do": true, "else": true, "elsif": true, "ensure": true, "for": true, "if": true,
	"in": true, "module": true, "next": true, "not": true, "or": true, "redo": true, "rescue": true,
	"retry": true, "return": true, "super": true, "then": true, "undef": true, "unless": true,
	"until": true, "when": true, "while": true, "yield": true,
}

// continuationTokens ending a condition mean it continues on the next line.
var continuationTokens = map[string]bool{"and": true, "or": true, "not": true, "(": true, "[": true, "{": true, ",": true, "\\": true, "|": true}

type tokenKind int

const (
	kindWord tokenKind = iota
	kindNumber
	kindLiteral
	kindOperator
	kindPunct
)

type token struct {
	kind       tokenKind
	text       string
	start, end int
	spaceAfter bool
	decimal    bool // a plain decimal integer literal
}

// site is one candidate mutation: replace source[Start:End] with Replacement.
type site struct {
	Operator    string
	Line        int
	Start, End  int
	Original    string
	Replacement string
}

// planMutations lists every candidate mutation inside lines (all lines when
// lines is empty) in source order. ok is false when the source does not lex.
func planMutations(source []byte, lines []mutate.LineSpan) ([]site, bool) {
	masked, commentOffsets, ok := analyzerruby.MaskSource(source)
	if !ok || len(masked) != len(source) {
		return nil, false
	}
	comments := make(map[int]bool, len(commentOffsets))
	for _, offset := range commentOffsets {
		comments[offset] = true
	}
	var sites []site
	for number, bounds := range lineBounds(source) {
		if !inSpans(number+1, lines) {
			continue
		}
		tokens := lineTokens(source, masked, comments, bounds[0], bounds[1])
		sites = append(sites, planLine(source, tokens, number+1)...)
	}
	sort.SliceStable(sites, func(left, right int) bool { return sites[left].Start < sites[right].Start })
	return sites, true
}

func lineBounds(source []byte) [][2]int {
	var bounds [][2]int
	start := 0
	for index, current := range source {
		if current == '\n' {
			bounds = append(bounds, [2]int{start, index})
			start = index + 1
		}
	}
	if start < len(source) {
		bounds = append(bounds, [2]int{start, len(source)})
	}
	return bounds
}

func inSpans(line int, spans []mutate.LineSpan) bool {
	if len(spans) == 0 {
		return true
	}
	for _, span := range spans {
		if line >= span.Start && line <= span.End {
			return true
		}
	}
	return false
}

// lineTokens tokenizes one physical line of the masked view. A blanked run is
// one literal operand, spaces inside it included, unless the masker recorded
// it as a comment, which ends the line's code.
func lineTokens(source, masked []byte, comments map[int]bool, from, to int) []token {
	var tokens []token
	for at := from; at < to; {
		current := masked[at]
		switch {
		case current == ' ' && source[at] != ' ' && source[at] != '\t' && source[at] != '\r':
			if comments[at] {
				return markSpacing(masked, tokens, to)
			}
			end := at
			for end < to && masked[end] == ' ' && !comments[end] {
				end++
			}
			for end > at && (source[end-1] == ' ' || source[end-1] == '\t' || source[end-1] == '\r') {
				end--
			}
			tokens = append(tokens, token{kind: kindLiteral, start: at, end: end})
			at = end
		case current == ' ' || current == '\t' || current == '\r':
			at++
		case wordStart(current) || current == '@' || current == '$':
			end := at + 1
			for end < to && wordByte(masked[end]) {
				end++
			}
			if end < to && (masked[end] == '?' || masked[end] == '!') && (end+1 >= to || masked[end+1] != '=') {
				end++
			}
			tokens = append(tokens, token{kind: kindWord, text: string(masked[at:end]), start: at, end: end})
			at = end
		case current >= '0' && current <= '9':
			end, decimal := numberEnd(masked, at, to)
			tokens = append(tokens, token{kind: kindNumber, text: string(masked[at:end]), start: at, end: end, decimal: decimal})
			at = end
		case current == ':' && at+1 < to && (wordStart(masked[at+1]) || masked[at+1] == '"'):
			end := at + 1
			for end < to && wordByte(masked[end]) {
				end++
			}
			tokens = append(tokens, token{kind: kindLiteral, start: at, end: end})
			at = end
		default:
			text := operatorAt(masked, at, to)
			kind := kindOperator
			if strings.ContainsAny(text, "()[]{},;") || text == "\\" {
				kind = kindPunct
			}
			tokens = append(tokens, token{kind: kind, text: text, start: at, end: at + len(text)})
			at += len(text)
		}
	}
	return markSpacing(masked, tokens, to)
}

func markSpacing(masked []byte, tokens []token, to int) []token {
	for index := range tokens {
		end := tokens[index].end
		tokens[index].spaceAfter = end >= to || masked[end] == ' ' || masked[end] == '\t'
	}
	return tokens
}

func wordStart(current byte) bool {
	return current == '_' || current >= 'a' && current <= 'z' || current >= 'A' && current <= 'Z' || current >= 0x80
}

func wordByte(current byte) bool {
	return wordStart(current) || current >= '0' && current <= '9' || current == '@' || current == '$'
}

// numberEnd reads one numeric literal and reports whether it is a plain
// decimal integer: no radix prefix, fraction, exponent, or suffix.
func numberEnd(masked []byte, at, to int) (int, bool) {
	end := at
	for end < to && (masked[end] >= '0' && masked[end] <= '9' || masked[end] == '_') {
		end++
	}
	decimal := !(masked[at] == '0' && end-at > 1)
	if end+1 < to && masked[end] == '.' && masked[end+1] >= '0' && masked[end+1] <= '9' {
		decimal = false
		end++
		for end < to && (masked[end] >= '0' && masked[end] <= '9' || masked[end] == '_') {
			end++
		}
	}
	// An exponent, signed or not, belongs to the float: `1e-3` is one
	// literal, never `1e` minus 3. A radix prefix stops the digits first.
	if exponent := exponentEnd(masked, end, to); exponent > end {
		decimal, end = false, exponent
	}
	for end < to && wordByte(masked[end]) {
		decimal = false
		end++
	}
	return end, decimal
}

// exponentEnd is the end of a float exponent (`e`, an optional sign, digits)
// starting at at, or at itself when there is none.
func exponentEnd(masked []byte, at, to int) int {
	if at >= to || masked[at] != 'e' && masked[at] != 'E' {
		return at
	}
	next := at + 1
	if next < to && (masked[next] == '+' || masked[next] == '-') {
		next++
	}
	if next >= to || masked[next] < '0' || masked[next] > '9' {
		return at
	}
	for next < to && (masked[next] >= '0' && masked[next] <= '9' || masked[next] == '_') {
		next++
	}
	return next
}

func operatorAt(masked []byte, at, to int) string {
	rest := string(masked[at:to])
	for _, candidate := range compoundOperators {
		if strings.HasPrefix(rest, candidate) {
			return candidate
		}
	}
	return rest[:1]
}

// planLine lists one line's mutants: binary operator swaps, integer constant
// off-by-one, and condition negation.
func planLine(source []byte, tokens []token, line int) []site {
	var sites []site
	classLine := len(tokens) > 0 && tokens[0].kind == kindWord && tokens[0].text == "class"
	for index, current := range tokens {
		switch current.kind {
		case kindOperator:
			swap, known := binarySwaps[current.text]
			if !known || index == 0 || !operand(tokens[index-1]) || classLine && current.text == "<" {
				continue
			}
			if (current.text == "-" || current.text == "+" || current.text == "*") && ambiguousArgument(tokens, index) {
				continue
			}
			sites = append(sites, site{Operator: swap.operator, Line: line, Start: current.start, End: current.end, Original: current.text, Replacement: swap.replacement})
		case kindNumber:
			if !current.decimal || index > 0 && tokens[index-1].end == current.start && tokens[index-1].text == "." {
				continue
			}
			sites = append(sites, constantSites(current, line)...)
		case kindWord:
			if negated, ok := negateCondition(source, tokens, index, line); ok {
				sites = append(sites, negated)
			}
		}
	}
	return sites
}

// operand reports whether a token ends an operand, so an operator after it
// is binary.
func operand(previous token) bool {
	switch previous.kind {
	case kindNumber, kindLiteral:
		return true
	case kindWord:
		return !rubyKeywords[previous.text] || operandKeywords[previous.text]
	case kindPunct:
		return previous.text == ")" || previous.text == "]" || previous.text == "}"
	}
	return false
}

// ambiguousArgument is Ruby's own reading of `foo -1`: a method name, a space,
// then a sign glued to its operand is an argument, not a binary operator.
func ambiguousArgument(tokens []token, index int) bool {
	previous := tokens[index-1]
	return previous.kind == kindWord && previous.spaceAfter && !tokens[index].spaceAfter
}

func constantSites(current token, line int) []site {
	value, err := strconv.ParseUint(strings.ReplaceAll(current.text, "_", ""), 10, 62)
	if err != nil {
		return nil
	}
	var sites []site
	if value > 0 {
		sites = append(sites, site{Operator: OperatorConstantDecrement, Line: line, Start: current.start, End: current.end, Original: current.text, Replacement: strconv.FormatUint(value-1, 10)})
	}
	return append(sites, site{Operator: OperatorConstantIncrement, Line: line, Start: current.start, End: current.end, Original: current.text, Replacement: strconv.FormatUint(value+1, 10)})
}

// negateCondition wraps the condition after an if, elsif, unless, while, or
// until keyword in `!(...)`. The condition runs to `then`, `do`, `;`, or the
// end of the line's code; a condition that continues on the next line, whose
// brackets do not balance, or that a bare `end` follows on the same line
// (`def f; x if y end`) is never touched.
func negateCondition(source []byte, tokens []token, index, line int) (site, bool) {
	keyword := tokens[index]
	if !conditionKeywords[keyword.text] || index > 0 && tokens[index-1].text == "." {
		return site{}, false
	}
	if index+1 < len(tokens) && tokens[index+1].start == keyword.end && tokens[index+1].text == ":" {
		return site{}, false // a hash key such as `if:`
	}
	first, last, depth := index+1, index+1, 0
	for ; last < len(tokens); last++ {
		text := tokens[last].text
		if depth == 0 && (text == ";" || tokens[last].kind == kindWord && (text == "then" || text == "do")) {
			break
		}
		if depth == 0 && text == "end" && tokens[last].kind == kindWord && tokens[last-1].text != "." && tokens[last-1].text != "&." && tokens[last-1].text != "::" {
			return site{}, false
		}
		switch text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
		if depth < 0 {
			return site{}, false
		}
	}
	if depth != 0 || last == first {
		return site{}, false
	}
	final := tokens[last-1]
	if continuationTokens[final.text] || final.kind == kindOperator {
		return site{}, false
	}
	start, end := tokens[first].start, final.end
	original := string(source[start:end])
	return site{Operator: OperatorNegateCondition, Line: line, Start: start, End: end, Original: original, Replacement: "!(" + original + ")"}, true
}

// render applies one site to the source.
func render(source []byte, candidate site) []byte {
	rendered := make([]byte, 0, len(source)+len(candidate.Replacement))
	rendered = append(rendered, source[:candidate.Start]...)
	rendered = append(rendered, candidate.Replacement...)
	return append(rendered, source[candidate.End:]...)
}
