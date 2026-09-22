package tcq

import "strings"

var goCaseKeys = map[string]bool{"name": true, "testName": true, "test_name": true}

// analyzeGo implements profile `go-lexical/1` (TCQ-V0-016..018). It admits only
// a path ending `_test.go` and only top-level, no-receiver
// `func TestX(<name> *testing.T) { ... }`.
func analyzeGo(path, blobOID string, data []byte) (goAnalysis, error) {
	if !strings.HasSuffix(path, "_test.go") {
		return goAnalysis{}, nil
	}
	tokens, err := tokenizeGo(data)
	if err != nil {
		return goAnalysis{}, err
	}
	scan := goUnitScan{path: path, blobOID: blobOID, data: data, tokens: tokens}
	return scan.run()
}

type goUnitScan struct {
	path       string
	blobOID    string
	data       []byte
	tokens     []goToken
	braceDepth int
}

func (scan *goUnitScan) run() (goAnalysis, error) {
	var functions []goFunction
	index := 0
	for index < len(scan.tokens) {
		if err := scan.trackBrace(scan.tokens[index]); err != nil {
			return goAnalysis{}, err
		}
		function, next, err := scan.declaration(index)
		if err != nil {
			return goAnalysis{}, err
		}
		if function == nil {
			index = next
			continue
		}
		functions = append(functions, *function)
		index = next
	}
	if scan.braceDepth != 0 {
		return goAnalysis{}, errGoParse
	}
	return goAnalysis{functions: functions}, nil
}

func (scan *goUnitScan) trackBrace(token goToken) error {
	switch token.text {
	case "{":
		scan.braceDepth++
	case "}":
		scan.braceDepth--
		if scan.braceDepth < 0 {
			return errGoParse
		}
	}
	return nil
}

// declaration attempts one `func TestX(t *testing.T)` at brace depth zero. It
// returns the next index to resume from; a nil function means "not a unit here".
func (scan *goUnitScan) declaration(index int) (*goFunction, int, error) {
	token := scan.tokens[index]
	if scan.braceDepth != 0 || token.text != "func" || index+3 >= len(scan.tokens) ||
		!goDeclarationLeading(scan.data, scan.tokens, index) {
		return nil, index + 1, nil
	}
	name := scan.tokens[index+1]
	if name.kind != goName || !goTestNamePattern.MatchString(name.text) || scan.tokens[index+2].text != "(" {
		return nil, index + 1, nil
	}
	closeParams := goMatching(scan.tokens, index+2, "(", ")")
	if closeParams < 0 {
		return nil, 0, errGoParse
	}
	parameter, ok := goTestingParameter(scan.tokens[index+3 : closeParams])
	if !ok {
		return nil, closeParams + 1, nil
	}
	if closeParams+1 >= len(scan.tokens) || scan.tokens[closeParams+1].text != "{" {
		return nil, 0, errGoParse
	}
	closeBody := goMatching(scan.tokens, closeParams+1, "{", "}")
	if closeBody < 0 {
		return nil, 0, errGoParse
	}
	function := scan.function(name, parameter, closeParams, closeBody)
	return &function, closeBody + 1, nil
}

// goTestingParameter admits exactly `<name> *testing.T` and nothing else: a
// receiver, extra parameters, or a `*testing.B` is a different signature and
// TCQ-V0-016 does not scan it.
func goTestingParameter(params []goToken) (string, bool) {
	if len(params) != 5 || params[0].kind != goName || params[1].text != "*" ||
		params[2].text != "testing" || params[3].text != "." || params[4].text != "T" {
		return "", false
	}
	return params[0].text, true
}

func (scan *goUnitScan) function(name goToken, parameter string, closeParams, closeBody int) goFunction {
	bodyTokens := effectiveGoTokens(scan.tokens[closeParams+2 : closeBody])
	bodyStart := scan.tokens[closeParams+1].end
	bodyEnd := scan.tokens[closeBody].start
	unit := testUnit{
		path: scan.path, blobOID: scan.blobOID,
		bodyStart: int64(bodyStart), bodyEnd: int64(bodyEnd),
		bodySHA256:       sha256Hex(scan.data[bodyStart:bodyEnd]),
		executionKey:     executionKey(scan.path, name.text),
		extractorProfile: unitProfileGo,
		runtimeName:      name.text,
		associationKind:  kindGoTestFunction,
		empty:            goEmptyBody(bodyTokens),
		skipped:          goSkipped(bodyTokens, parameter),
	}
	return goFunction{unit: unit, nameStart: name.start, nameEnd: name.end, cases: admittedCases(bodyTokens)}
}

// effectiveGoTokens drops semicolons so TCQ-V0-017's "only semicolons and one
// bare return" reduces to a token-count test.
func effectiveGoTokens(tokens []goToken) []goToken {
	kept := make([]goToken, 0, len(tokens))
	for _, token := range tokens {
		if token.text != ";" {
			kept = append(kept, token)
		}
	}
	return kept
}

// goEmptyBody implements TCQ-V0-017.
func goEmptyBody(body []goToken) bool {
	return len(body) == 0 || (len(body) == 1 && body[0].text == "return")
}

// goSkipped implements TCQ-V0-017: only a FIRST effective `<parameter>.Skip(`,
// `.Skipf(`, or `.SkipNow(` counts. A later or dynamic skip does not.
func goSkipped(body []goToken, parameter string) bool {
	if len(body) < 5 || body[0].text != parameter || body[1].text != "." || body[3].text != "(" {
		return false
	}
	switch body[2].text {
	case "Skip", "Skipf", "SkipNow":
		return true
	}
	return false
}

// admittedCases implements TCQ-V0-018: a case is admitted only when it is a
// simple unescaped ASCII string literal keyed by `name`/`testName`/`test_name`,
// or the first argument of an identifier's `.Run(` call (decision 0029), inside
// one validated parent test body. Dynamic, escaped, and concatenated cases are
// not admitted.
func admittedCases(body []goToken) []goCase {
	var cases []goCase
	for index, token := range body {
		if !goCaseKeyed(body, index) && !goRunArgument(body, index) {
			continue
		}
		if token.kind != goString || !strings.HasPrefix(token.text, `"`) || strings.Contains(token.text, `\`) {
			continue
		}
		value := token.text[1 : len(token.text)-1]
		if !goCasePattern.MatchString(value) {
			continue
		}
		cases = append(cases, goCase{start: token.start + 1, end: token.end - 1, value: value})
	}
	return cases
}

func goCaseKeyed(body []goToken, index int) bool {
	if index < 2 || body[index-1].text != ":" {
		return false
	}
	return goCaseKeys[body[index-2].text]
}

// goRunArgument matches `ident.Run(` spelled without interior space, the same
// shape the OCM extractor admits.
func goRunArgument(body []goToken, index int) bool {
	if index < 4 || body[index-1].text != "(" || body[index-2].text != "Run" || body[index-3].text != "." {
		return false
	}
	receiver := body[index-4]
	if receiver.kind != goName {
		return false
	}
	return receiver.end == body[index-3].start && body[index-3].end == body[index-2].start && body[index-2].end == body[index-1].start
}

// caseUnit derives the leaf unit for one admitted case. TCQ-V0-018 gives it the
// parent's body span and digest — the case has no body of its own — and
// TCQ-V0-009 makes its runtime name exactly `TestX/<case>`.
func caseUnit(parent testUnit, value string) testUnit {
	leaf := parent
	leaf.runtimeName = parent.runtimeName + "/" + value
	leaf.executionKey = executionKey(parent.path, leaf.runtimeName)
	leaf.associationKind = kindGoTableCase
	return leaf
}
