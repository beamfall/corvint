package pymutate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/pythongrammar"
)

// Operator names: the same closed set as the Go runner, each a single
// line-confined token rewrite of the source text. No Python runs to produce
// a mutant; Python only checks afterwards that the mutant still parses.
const (
	operatorNegateCondition = "negate-condition"
	operatorSwapBinary      = "swap-binary"
	operatorReplaceLiteral  = "replace-literal"
	operatorDeleteStatement = "delete-statement"
)

// binarySwaps is the closed operator table: arithmetic, comparison, and the
// boolean connectives, each swapped for the operator that most changes the
// value.
var binarySwaps = map[string]string{
	"+": "-", "-": "+", "*": "/", "/": "*",
	"==": "!=", "!=": "==", "<": ">=", ">=": "<", ">": "<=", "<=": ">",
	"and": "or", "or": "and",
}

// blockConditions are the statement keywords whose condition is negated.
var blockConditions = map[string]bool{"if": true, "elif": true, "while": true}

// operandKeywords are the keywords that end an operand, so an operator after
// one is binary; every other keyword before an operator makes it unary.
var operandKeywords = map[string]bool{"True": true, "False": true, "None": true}

// site is one candidate mutation: replace source[Start:End] with Replacement.
type site struct {
	Operator    string
	Line        int
	Start, End  int
	Replacement string
}

// mutant is one rendered mutation of the changed file.
type mutant struct {
	Operator string
	Line     int
	Source   []byte
}

// statement is one simple statement as a token range [first, last).
type statement struct {
	first, last int
	logical     int  // index of the logical line it sits on
	multiple    bool // more than one statement shares the logical line
}

// planner walks one tokenization of the source.
type planner struct {
	source []byte
	tokens []pythongrammar.Token
	lines  []int // byte offset where each line starts
	sites  []site
}

// planMutations lists every candidate mutation in source order. The source
// must tokenize; a file the lexer rejects yields an error, as a Go file that
// does not parse does.
func planMutations(source []byte) ([]site, error) {
	tokens, reason := pythongrammar.LexPython312(source)
	if reason != "" {
		return nil, fmt.Errorf("pymutate: %s", reason)
	}
	plan := &planner{source: source, tokens: mergeCompoundOperators(source, tokens), lines: lineStarts(source)}
	for _, current := range plan.statements() {
		plan.planStatement(current)
	}
	sort.SliceStable(plan.sites, func(left, right int) bool { return plan.sites[left].Start < plan.sites[right].Start })
	return plan.sites, nil
}

// compoundOperators are the multi-character operators the lexer emits one
// character at a time; the planner reads them whole so `+=` is never `+`
// and `!=` is never `!` followed by `=`.
var compoundOperators = map[string]bool{
	"==": true, "!=": true, "<=": true, ">=": true, "->": true, ":=": true,
	"**": true, "//": true, "<<": true, ">>": true,
	"+=": true, "-=": true, "*=": true, "/=": true, "%=": true, "&=": true, "|=": true, "^=": true, "@=": true,
	"**=": true, "//=": true, "<<=": true, ">>=": true, "...": true,
}

func mergeCompoundOperators(source []byte, tokens []pythongrammar.Token) []pythongrammar.Token {
	merged := make([]pythongrammar.Token, 0, len(tokens))
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		for count := compoundSpan(tokens, index); count > 1; count-- {
			end := tokens[index+count-1].End
			if compoundOperators[string(source[token.Start:end])] {
				token.End = end
				index += count - 1
				break
			}
		}
		merged = append(merged, token)
	}
	return merged
}

// compoundSpan counts how many byte-adjacent symbol tokens start at index,
// up to the longest compound operator.
func compoundSpan(tokens []pythongrammar.Token, index int) int {
	count := 0
	for index+count < len(tokens) && count < 3 {
		token := tokens[index+count]
		if token.Kind != pythongrammar.KindSymbol {
			break
		}
		if count > 0 && tokens[index+count-1].End != token.Start {
			break
		}
		count++
	}
	return count
}

func lineStarts(source []byte) []int {
	starts := []int{0}
	for offset, char := range source {
		if char == '\n' {
			starts = append(starts, offset+1)
		}
	}
	return starts
}

func (plan *planner) lineOf(offset int) int {
	return sort.Search(len(plan.lines), func(index int) bool { return plan.lines[index] > offset })
}

func (plan *planner) text(index int) string {
	token := plan.tokens[index]
	return string(plan.source[token.Start:token.End])
}

func (plan *planner) isSymbol(index int, want string) bool {
	return plan.tokens[index].Kind == pythongrammar.KindSymbol && plan.text(index) == want
}

func (plan *planner) isName(index int, want string) bool {
	return plan.tokens[index].Kind == pythongrammar.KindName && plan.text(index) == want
}

// statements splits the token stream at logical line ends and top-level
// semicolons; indentation tokens are not part of any statement.
func (plan *planner) statements() []statement {
	found := make([]statement, 0, 32)
	start, depth, logical := -1, 0, 0
	for index, token := range plan.tokens {
		switch token.Kind {
		case pythongrammar.KindIndent, pythongrammar.KindDedent:
			continue
		case pythongrammar.KindNewline:
			found = closeStatement(found, start, index, logical)
			start, logical = -1, logical+1
			continue
		}
		if start < 0 {
			start = index
		}
		depth = plan.adjustDepth(index, depth)
		if depth == 0 && plan.isSymbol(index, ";") {
			found = closeStatement(found, start, index, logical)
			start = -1
		}
	}
	found = closeStatement(found, start, len(plan.tokens), logical)
	return markShared(found)
}

func closeStatement(found []statement, start, end, logical int) []statement {
	if start < 0 || start >= end {
		return found
	}
	return append(found, statement{first: start, last: end, logical: logical})
}

func markShared(found []statement) []statement {
	perLine := map[int]int{}
	for _, current := range found {
		perLine[current.logical]++
	}
	for index := range found {
		found[index].multiple = perLine[found[index].logical] > 1
	}
	return found
}

func (plan *planner) adjustDepth(index, depth int) int {
	if plan.tokens[index].Kind != pythongrammar.KindSymbol {
		return depth
	}
	switch plan.text(index) {
	case "(", "[", "{":
		return depth + 1
	case ")", "]", "}":
		return depth - 1
	}
	return depth
}

func (plan *planner) record(operator string, start, end int, replacement string) {
	plan.sites = append(plan.sites, site{Operator: operator, Line: plan.lineOf(start), Start: start, End: end, Replacement: replacement})
}

func (plan *planner) planStatement(current statement) {
	plan.planCondition(current)
	plan.planBinary(current)
	plan.planReturnLiterals(current)
	plan.planDeletion(current)
}

// planCondition negates the condition of an if, elif, or while: everything
// between the keyword and the block colon is wrapped in `not (...)`.
func (plan *planner) planCondition(current statement) {
	if plan.tokens[current.first].Kind != pythongrammar.KindName || !blockConditions[plan.text(current.first)] {
		return
	}
	colon := plan.blockColon(current)
	if colon < 0 {
		return
	}
	keyword := plan.tokens[current.first]
	condition := strings.TrimSpace(string(plan.source[keyword.End:plan.tokens[colon].Start]))
	if condition == "" {
		return
	}
	plan.record(operatorNegateCondition, keyword.End, plan.tokens[colon].Start, " not ("+condition+")")
}

// blockColon finds the colon that opens the block of a compound statement.
func (plan *planner) blockColon(current statement) int {
	depth := 0
	for index := current.first + 1; index < current.last; index++ {
		depth = plan.adjustDepth(index, depth)
		if depth == 0 && plan.isSymbol(index, ":") {
			return index
		}
	}
	return -1
}

// planBinary swaps every binary operator in the table whose left operand is
// present, so a sign, a star argument, or a positional-only marker is never
// touched.
func (plan *planner) planBinary(current statement) {
	for index := current.first + 1; index < current.last; index++ {
		swapped, mutable := binarySwaps[plan.text(index)]
		if !mutable || !plan.operatorToken(index) || !plan.operandBefore(index) {
			continue
		}
		token := plan.tokens[index]
		plan.record(operatorSwapBinary, token.Start, token.End, swapped)
	}
}

func (plan *planner) operatorToken(index int) bool {
	kind := plan.tokens[index].Kind
	if kind == pythongrammar.KindSymbol {
		return true
	}
	return kind == pythongrammar.KindName && (plan.isName(index, "and") || plan.isName(index, "or"))
}

func (plan *planner) operandBefore(index int) bool {
	previous := plan.tokens[index-1]
	switch previous.Kind {
	case pythongrammar.KindNumber, pythongrammar.KindString:
		return true
	case pythongrammar.KindName:
		text := plan.text(index - 1)
		return !pythongrammar.PythonKeyword(text) || operandKeywords[text]
	case pythongrammar.KindSymbol:
		text := plan.text(index - 1)
		return text == ")" || text == "]" || text == "}"
	}
	return false
}

// planReturnLiterals replaces a number or plain string literal in a return
// statement, as the Go runner replaces literals in return results.
func (plan *planner) planReturnLiterals(current statement) {
	if !plan.isName(current.first, "return") {
		return
	}
	for index := current.first + 1; index < current.last; index++ {
		replacement, mutable := literalReplacement(plan.tokens[index].Kind, plan.text(index))
		if !mutable {
			continue
		}
		token := plan.tokens[index]
		plan.record(operatorReplaceLiteral, token.Start, token.End, replacement)
	}
}

func literalReplacement(kind byte, text string) (string, bool) {
	switch kind {
	case pythongrammar.KindNumber:
		if text == "0" {
			return "1", true
		}
		return "0", true
	case pythongrammar.KindString:
		if text[0] != '"' && text[0] != '\'' {
			return "", false
		}
		if text == `""` || text == `''` {
			return `"mutant"`, true
		}
		return `""`, true
	}
	return "", false
}

// planDeletion deletes one physical line holding one simple statement: an
// expression statement (a call), an augmented assignment, or an assignment
// to an attribute or subscript. A keyword statement is never deleted, so no
// block opener, definition, return, or import goes, and a plain name binding
// is never deleted: that almost always leaves an unbound name, a failure the
// test would report but the mutation did not cause.
func (plan *planner) planDeletion(current statement) {
	if current.multiple || plan.tokens[current.first].Kind != pythongrammar.KindName {
		return
	}
	if pythongrammar.PythonKeyword(plan.text(current.first)) {
		return
	}
	first, last := plan.tokens[current.first], plan.tokens[current.last-1]
	line := plan.lineOf(first.Start)
	if plan.lineOf(last.End-1) != line {
		return
	}
	if !plan.deletableShape(current) {
		return
	}
	start := plan.lines[line-1]
	end := len(plan.source)
	if line < len(plan.lines) {
		end = plan.lines[line]
	}
	plan.record(operatorDeleteStatement, start, end, "")
}

// deletableShape admits an expression statement or an assignment whose
// target is an attribute or a subscript, never a bare name or a tuple.
func (plan *planner) deletableShape(current statement) bool {
	assign := plan.assignmentAt(current)
	if assign < 0 {
		return true
	}
	if strings.HasSuffix(plan.text(assign), "=") && plan.text(assign) != "=" {
		return true // augmented assignment keeps the name bound
	}
	targetHasAccess, targetHasComma := false, false
	for index := current.first; index < assign; index++ {
		targetHasAccess = targetHasAccess || plan.isSymbol(index, ".") || plan.isSymbol(index, "[")
		targetHasComma = targetHasComma || plan.isSymbol(index, ",")
	}
	return targetHasAccess && !targetHasComma
}

// assignmentAt finds the top-level assignment operator of a statement, or -1.
func (plan *planner) assignmentAt(current statement) int {
	depth := 0
	for index := current.first; index < current.last; index++ {
		depth = plan.adjustDepth(index, depth)
		if depth != 0 || plan.tokens[index].Kind != pythongrammar.KindSymbol {
			continue
		}
		if assignmentOperator(plan.text(index)) {
			return index
		}
	}
	return -1
}

func assignmentOperator(text string) bool {
	if !strings.HasSuffix(text, "=") {
		return false
	}
	return text != "==" && text != "!=" && text != "<=" && text != ">="
}

// generateMutants renders one mutant per planned site inside the spans, in
// source order. Whether a mutant parses is Python's call, made by the judge
// inside the sandbox, so no cap is applied here.
func generateMutants(source []byte, spans []mutate.LineSpan) ([]mutant, error) {
	plan, err := planMutations(source)
	if err != nil {
		return nil, err
	}
	mutants := make([]mutant, 0, len(plan))
	for _, candidate := range plan {
		if !insideSpans(candidate.Line, spans) {
			continue
		}
		mutants = append(mutants, mutant{Operator: candidate.Operator, Line: candidate.Line, Source: render(source, candidate)})
	}
	return mutants, nil
}

func render(source []byte, candidate site) []byte {
	rendered := make([]byte, 0, len(source)+len(candidate.Replacement))
	rendered = append(rendered, source[:candidate.Start]...)
	rendered = append(rendered, candidate.Replacement...)
	return append(rendered, source[candidate.End:]...)
}

// insideSpans reports whether line lies in one of the spans; no spans admits
// every line.
func insideSpans(line int, spans []mutate.LineSpan) bool {
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
