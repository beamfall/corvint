package tcq

import (
	"regexp"
	"strings"
)

var pythonTestName = regexp.MustCompile(`^test_[A-Za-z0-9_]+$`)

// pythonFunction is one `def`/`async def` found in the target blob, with the
// exact spans TCQ-V0-047 requires: every one is validated against the token
// scan, and any span the tokens do not corroborate abstains the edge.
type pythonFunction struct {
	unit      testUnit
	nameStart int
	nameEnd   int
	docStart  int
	docEnd    int
	hasDoc    bool
	// topLevel marks a function whose direct lexical parent is the module or a
	// class. TCQ-V0-013 admits only those as units; a nested local function is
	// still an extractor candidate, so it is kept and filtered later.
	topLevel bool
	// name is retained only to key the extractor's selector; TCQ-V0-020 keeps it
	// out of every persisted artifact.
	name string
}

// pythonAnalysis is the whole-blob scan TCQ-V0-007 permits: it derives execution
// keys and collision counts and nothing else about sibling claims.
type pythonAnalysis struct {
	functions []pythonFunction
	// withheld records that grammar or offset validation failed, so no unit in
	// this blob may be published even though the anchor candidates remain valid.
	withheld bool
}

// analyzePython implements profile `python-ast/1` (TCQ-V0-013). Validation order
// is frozen: grammar support first, then offset/token validation, and only the
// first applicable reason is emitted (TCQ-V0-047).
//
// A grammar or offset failure abstains the affected edges but does NOT erase
// the anchor candidates already derived: TCQ-V0-011 requires a supported
// re-derived anchor profile to be retained, so only the units are withheld.
func analyzePython(path, blobOID string, data []byte) (pythonAnalysis, error) {
	lines, err := tokenizePython(data)
	if err != nil {
		return pythonAnalysis{}, err
	}
	block, index, err := buildPythonBlock(lines, 0, 0)
	if err != nil {
		return pythonAnalysis{}, err
	}
	if index != len(lines) {
		return pythonAnalysis{}, errPythonGrammar
	}
	walk := &pythonWalk{path: path, blobOID: blobOID, data: data, lines: lines, lineStarts: lineStarts(data)}
	walkErr := walk.block(block, nil, true)
	analysis := pythonAnalysis{functions: walk.functions}
	if walkErr != nil || walk.offsetMismatch {
		analysis.withheld = true
		for index := range analysis.functions {
			analysis.functions[index].unit = testUnit{}
			analysis.functions[index].topLevel = false
		}
	}
	if walkErr != nil {
		return analysis, walkErr
	}
	if walk.offsetMismatch {
		return analysis, errPythonOffset
	}
	return analysis, nil
}

type pythonWalk struct {
	path           string
	blobOID        string
	data           []byte
	lines          []pyLine
	lineStarts     []int
	functions      []pythonFunction
	offsetMismatch bool
}

// block walks one suite, threading the decorators of every enclosing class down
// to each method: TCQ-V0-015 makes a class-level skip decorator apply to the
// methods under it.
func (walk *pythonWalk) block(statements []pyStmt, classDecorators [][]pyToken, topLevel bool) error {
	var pending [][]pyToken
	for _, statement := range statements {
		if decorator := decoratorTokens(statement); decorator != nil {
			pending = append(pending, decorator)
			continue
		}
		if err := walk.statement(statement, pending, classDecorators, topLevel); err != nil {
			return err
		}
		pending = nil
	}
	return nil
}

func (walk *pythonWalk) statement(statement pyStmt, decorators, classDecorators [][]pyToken, topLevel bool) error {
	switch leadingKeyword(statement.tokens) {
	case "class":
		return walk.block(statement.body(), append(append([][]pyToken{}, classDecorators...), decorators...), topLevel)
	case "def":
		if err := walk.function(statement, decorators, classDecorators, topLevel); err != nil {
			return err
		}
		// A function's own suite may hold nested definitions. They are never
		// units (TCQ-V0-013) but remain extractor candidates.
		return walk.block(statement.body(), nil, false)
	}
	if isPost39Grammar(statement) {
		return errPythonGrammar
	}
	return walk.block(statement.body(), classDecorators, false)
}

// isPost39Grammar rejects the structured-pattern grammar Python 3.10 added.
// TCQ-V0-013 freezes `python-ast/1` to the 3.9 grammar, so source using it is
// `unsupported-python-grammar` rather than a best-effort scan.
func isPost39Grammar(statement pyStmt) bool {
	tokens := statement.tokens
	if len(tokens) == 0 || tokens[0].kind != pyName {
		return false
	}
	if tokens[0].text == "except" && len(tokens) > 1 && tokens[1].text == "*" {
		return true
	}
	return (tokens[0].text == "match" || tokens[0].text == "case") && statement.compound()
}

// leadingKeyword names the construct a statement opens, seeing through `async`.
func leadingKeyword(tokens []pyToken) string {
	if len(tokens) == 0 || tokens[0].kind != pyName {
		return ""
	}
	if tokens[0].text == "async" && len(tokens) > 1 {
		return tokens[1].text
	}
	return tokens[0].text
}

func decoratorTokens(statement pyStmt) []pyToken {
	if statement.compound() || len(statement.tokens) < 2 || statement.tokens[0].text != "@" {
		return nil
	}
	return statement.tokens[1:]
}

func (walk *pythonWalk) function(statement pyStmt, decorators, classDecorators [][]pyToken, topLevel bool) error {
	header := statement.tokens
	nameIndex := 1
	if header[0].text == "async" {
		nameIndex = 2
	}
	if len(header) <= nameIndex || header[nameIndex].kind != pyName {
		return errPythonGrammar
	}
	name := header[nameIndex].text
	body := statement.body()
	if len(body) == 0 {
		return errPythonGrammar
	}
	if !strings.HasPrefix(name, "test_") {
		return nil
	}
	docStart, docEnd, hasDoc := walk.docstringSpan(body[0], pythonTestName.MatchString(name))
	function := pythonFunction{
		nameStart: header[nameIndex].start, nameEnd: header[nameIndex].end,
		docStart: docStart, docEnd: docEnd, hasDoc: hasDoc,
		topLevel: topLevel, name: name,
	}
	if !pythonTestName.MatchString(name) {
		walk.functions = append(walk.functions, function)
		return nil
	}
	if !walk.validNameToken(header[0].start, header[nameIndex]) {
		walk.offsetMismatch = true
		walk.functions = append(walk.functions, function)
		return nil
	}
	function.unit = walk.buildUnit(statement, body, name, decorators, classDecorators)
	walk.functions = append(walk.functions, function)
	return nil
}

// validNameToken enforces TCQ-V0-047: the runtime name must resolve to exactly
// one NAME token on the declaration line, so a signature that repeats the name
// yields no span at all rather than an arbitrary one.
func (walk *pythonWalk) validNameToken(functionStart int, nameToken pyToken) bool {
	declarationLine := lineAt(walk.lineStarts, functionStart)
	matches := 0
	for _, token := range walk.allNameTokens(nameToken.text) {
		if lineAt(walk.lineStarts, token.start) == declarationLine {
			matches++
		}
	}
	return matches == 1
}

func (walk *pythonWalk) buildUnit(statement pyStmt, body []pyStmt, name string, decorators, classDecorators [][]pyToken) testUnit {
	bodyStart := body[0].tokens[0].start
	bodyEnd := statement.end()
	direct := body
	if hasLeadingDocstring(body[0]) {
		direct = body[1:]
	}
	return testUnit{
		path: walk.path, blobOID: walk.blobOID,
		bodyStart: int64(bodyStart), bodyEnd: int64(bodyEnd),
		bodySHA256:       sha256Hex(walk.data[bodyStart:bodyEnd]),
		executionKey:     executionKey(walk.path, name),
		extractorProfile: unitProfilePython,
		runtimeName:      name,
		associationKind:  kindPythonTestFunction,
		empty:            pythonEmptyBody(direct),
		skipped:          pythonSkipped(direct, decorators, classDecorators),
	}
}

// docstringSpan returns the span of a leading docstring. TCQ-V0-047 forbids
// publishing a parser-dependent span, so an implicitly concatenated docstring —
// whose constant covers several tokens — abstains instead of guessing one.
func (walk *pythonWalk) docstringSpan(first pyStmt, validate bool) (int, int, bool) {
	if !hasLeadingDocstring(first) {
		return -1, -1, false
	}
	if validate && len(first.tokens) != 1 {
		walk.offsetMismatch = true
	}
	return first.tokens[0].start, first.tokens[len(first.tokens)-1].end, true
}

func hasLeadingDocstring(statement pyStmt) bool {
	if statement.compound() || len(statement.tokens) == 0 {
		return false
	}
	for _, token := range statement.tokens {
		if token.kind != pyString {
			return false
		}
	}
	return true
}

// pythonEmptyBody implements TCQ-V0-014: after one leading docstring is removed,
// a body of only `pass`, standalone constants or ellipsis, and value-less
// `return` is empty. `return <expression>` and calls are non-empty — which says
// nothing about whether the test asserts anything.
func pythonEmptyBody(direct []pyStmt) bool {
	for _, statement := range direct {
		if !pythonNoop(statement) {
			return false
		}
	}
	return true
}

func pythonNoop(statement pyStmt) bool {
	if statement.compound() {
		return false
	}
	tokens := statement.tokens
	if len(tokens) == 1 && tokens[0].kind == pyName && (tokens[0].text == "pass" || tokens[0].text == "return") {
		return true
	}
	return pythonConstantExpression(tokens)
}

func pythonConstantExpression(tokens []pyToken) bool {
	if len(tokens) == 0 {
		return false
	}
	if allStrings(tokens) {
		return true
	}
	if len(tokens) != 1 {
		return false
	}
	token := tokens[0]
	switch {
	case token.kind == pyNumber:
		return true
	case token.kind == pyOp && token.text == "...":
		return true
	case token.kind == pyName && (token.text == "True" || token.text == "False" || token.text == "None"):
		return true
	}
	return false
}

func allStrings(tokens []pyToken) bool {
	for _, token := range tokens {
		if token.kind != pyString {
			return false
		}
	}
	return true
}
