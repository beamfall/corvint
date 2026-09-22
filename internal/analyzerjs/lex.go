package analyzerjs

import (
	"fmt"
	"sort"
	"strings"
)

var errImportBudget = fmt.Errorf("import budget")
var errLexicalInput = fmt.Errorf("ambiguous lexical input")
var errUnsupportedLexical = fmt.Errorf("unsupported lexical input")
var errModuleDynamic = fmt.Errorf("dynamic module input")
var errModuleSecret = fmt.Errorf("credential module input")

type slashGoal uint8

const (
	slashUnknown slashGoal = iota
	slashRegex
	slashDivision
)

type tokenKind uint8

const (
	tokenNone tokenKind = iota
	tokenControl
	tokenIdentifier
	tokenProperty
	tokenValue
	tokenOpenParen
	tokenCloseParen
	tokenOperator
)

type lexicalState struct {
	control    [MaxDepth]bool
	parenStart [MaxDepth]int
	parens     int
	previous   tokenKind
	slash      slashGoal
}

type lexer struct {
	lexicalState
	shadow, emitted                                         [MaxDepth]bool
	braceParens                                             [MaxDepth]int
	last                                                    byte
	depth                                                   int
	lastParenStart, lastParenEnd                            int
	lastRequire                                             bool
	start, function, parameters, functionShadow, bodyShadow bool
}

func importsOfLimit(s string, limit int) ([]string, error) {
	if len(s) > MaxSourceBytes {
		return nil, fmt.Errorf("source size")
	}
	if limit < 0 || limit > MaxFacts {
		return nil, errImportBudget
	}
	imports := make([]string, 0, min(limit, 32))
	add := func(module string) error {
		if len(imports) == limit {
			return errImportBudget
		}
		imports = append(imports, module)
		return nil
	}
	state := lexer{lexicalState: lexicalState{slash: slashRegex}, depth: 1, lastParenStart: -1, lastParenEnd: -1, start: true}
	for i := 0; i < len(s); {
		next, skipped, err := skipTrivia(s, i)
		if err != nil {
			return nil, err
		}
		if skipped {
			if strings.ContainsAny(s[i:next], "\n\r") {
				state.start = true
			}
			i = next
			continue
		}
		prev := state.last
		state.last = s[i]
		next, atom, err := skipAtom(s, i, &state.lexicalState, 0)
		if err != nil {
			return nil, err
		}
		if atom {
			i = next
			state.start = false
			continue
		}
		if isIdentStart(s[i]) {
			end := i + 1
			for end < len(s) && isIdent(s[end]) {
				end++
			}
			word := s[i:end]
			if state.parameters {
				if word == "require" {
					state.functionShadow = true
				}
				state.setWord(word)
				state.lastRequire, state.start = word == "require", false
				i = end
				continue
			}
			if word == "function" || word == "var" || word == "enum" || word == "namespace" {
				next, err := trivia(s, end)
				if err == nil && next < len(s) && s[next] == '*' {
					next, err = trivia(s, next+1)
				}
				if err != nil {
					return nil, err
				}
				// Hoisted bindings reach calls lexed before them.
				if next < len(s) && wordAt(s, next, "require") {
					return nil, errUnsupportedLexical
				}
				state.function = word == "function"
			}
			if word == "catch" {
				next, err := trivia(s, end)
				if err != nil {
					return nil, err
				}
				state.function = next < len(s) && s[next] == '('
			}
			if word == "const" || word == "let" || word == "var" || word == "class" {
				shadow, err := directRequireDeclaration(s, end)
				if err != nil {
					return nil, err
				}
				if shadow && state.emitted[state.depth-1] {
					return nil, errUnsupportedLexical
				}
				if shadow {
					state.shadow[state.depth-1] = true
				}
			}
			if (word == "import" || word == "export") && state.depth == 1 && state.start {
				after, module, found, err := moduleDecl(s, end, word == "export")
				if err != nil {
					return nil, err
				}
				if found {
					if err := add(module); err != nil {
						return nil, err
					}
				}
				if after <= i {
					after = end
				}
				i = after
				state.setValue()
				state.start = after > 0 && (s[after-1] == ';' || s[after-1] == '\n' || s[after-1] == '\r')
				continue
			}
			if word == "require" {
				after, module, found, err := bareRequire(s, i)
				if err != nil {
					return nil, err
				}
				if found && !state.shadow[state.depth-1] && state.previous != tokenProperty {
					if err := add(module); err != nil {
						return nil, err
					}
					state.emitted[state.depth-1] = true
				}
				if !found && state.previous != tokenProperty && strings.IndexByte(",{[:.(", prev) >= 0 {
					next, err := trivia(s, end)
					if err != nil {
						return nil, err
					}
					// A bare `require` after a list, pattern, or parameter
					// delimiter may be a later declarator or method parameter.
					key := next < len(s) && s[next] == ':' && state.parens == state.braceParens[state.depth-1]
					if next == len(s) || s[next] != '(' && s[next] != '.' && !key {
						return nil, errUnsupportedLexical
					}
				}
				if !found {
					after = end
				}
				state.setWord(word)
				state.lastRequire = !found
				state.start = false
				i = after
				continue
			}
			state.setWord(word)
			state.lastRequire = false
			state.start = false
			i = end
			continue
		}
		if s[i] >= '0' && s[i] <= '9' {
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			state.setValue()
			state.start = false
			continue
		}
		switch s[i] {
		case '(':
			if state.function {
				state.function, state.parameters, state.functionShadow = false, true, false
				state.slash, state.previous, state.lastRequire = slashRegex, tokenOpenParen, false
			} else if err := state.openParen(i); err != nil {
				return nil, err
			}
		case ')':
			if state.parameters {
				state.parameters, state.bodyShadow = false, state.functionShadow
				state.slash, state.previous, state.lastRequire = slashDivision, tokenCloseParen, false
			} else {
				start, err := state.closeParen()
				if err != nil {
					return nil, err
				}
				state.lastParenStart, state.lastParenEnd = start, i
			}
		case '{':
			if state.depth == MaxDepth {
				return nil, errLexicalInput
			}
			state.shadow[state.depth] = state.shadow[state.depth-1] || state.bodyShadow
			state.emitted[state.depth], state.braceParens[state.depth] = false, state.parens
			state.depth, state.bodyShadow = state.depth+1, false
			state.setOperator()
		case '}':
			if state.depth == 1 {
				return nil, errLexicalInput
			}
			state.emitted[state.depth-2] = state.emitted[state.depth-2] || state.emitted[state.depth-1]
			state.depth--
			state.setOperator()
			state.start = true
			i++
			continue
		case '=':
			if i+1 < len(s) && s[i+1] == '>' {
				bindsRequire, err := arrowBindsRequire(s, state)
				if err != nil {
					return nil, err
				}
				if bindsRequire {
					return nil, errUnsupportedLexical
				}
				i += 2
				state.setOperator()
				state.start = false
				continue
			}
			state.setOperator()
		case '+', '-':
			if i+1 < len(s) && s[i+1] == s[i] {
				return nil, errUnsupportedLexical
			}
			state.setOperator()
		case '.':
			if i+2 < len(s) && s[i+1] == '.' && s[i+2] == '.' {
				state.setOperator()
				state.lastRequire, state.start = false, false
				i += 3
				continue
			}
			state.slash, state.previous, state.lastRequire = slashUnknown, tokenProperty, false
		case ']':
			state.setValue()
		default:
			state.setOperator()
		}
		state.start = s[i] == ';'
		i++
	}
	if state.depth != 1 || state.parens != 0 || state.parameters {
		return nil, errLexicalInput
	}
	sort.Strings(imports)
	return imports, nil
}

func (state *lexicalState) setValue() {
	state.slash, state.previous = slashDivision, tokenValue
}

func (state *lexicalState) setOperator() {
	state.slash, state.previous = slashRegex, tokenOperator
}

func (state *lexicalState) setWord(word string) {
	member := state.previous == tokenProperty
	control := state.previous == tokenControl
	state.slash, state.previous = slashDivision, tokenIdentifier
	if member {
		return
	}
	if prefixWord(word) {
		state.slash = slashRegex
	}
	// `for await (` keeps its control header: the closing parenthesis must
	// restore expression-start state or genuine JSX after it lexes as operators.
	if controlWord(word) || word == "await" && control {
		state.previous = tokenControl
	}
}

func (state *lexicalState) openParen(i int) error {
	if state.parens == MaxDepth {
		return errLexicalInput
	}
	state.control[state.parens] = state.previous == tokenControl
	state.parenStart[state.parens] = i
	state.parens++
	state.slash, state.previous = slashRegex, tokenOpenParen
	return nil
}

func (state *lexicalState) closeParen() (int, error) {
	if state.parens == 0 {
		return 0, errLexicalInput
	}
	state.parens--
	start := state.parenStart[state.parens]
	state.previous = tokenCloseParen
	state.slash = slashDivision
	if state.control[state.parens] {
		state.slash = slashRegex
	}
	return start, nil
}

func bareRequire(s string, i int) (int, string, bool, error) {
	i, err := trivia(s, i+len("require"))
	if err != nil || i == len(s) || s[i] != '(' {
		return i, "", false, err
	}
	i, err = trivia(s, i+1)
	if err != nil || i == len(s) || s[i] != '\'' && s[i] != '"' {
		return i, "", false, err
	}
	after, module, err := quoted(s, i)
	if err != nil {
		return after, "", false, err
	}
	after, err = trivia(s, after)
	if err != nil || after == len(s) || s[after] != ')' {
		return after, "", false, err
	}
	return after + 1, module, true, nil
}

func directRequireDeclaration(s string, i int) (bool, error) {
	i, err := trivia(s, i)
	if err != nil || i == len(s) {
		return false, err
	}
	if s[i] == '{' || s[i] == '[' {
		next, binds, err := bindingTarget(s, i, len(s), 0)
		if err != nil {
			return false, err
		}
		if binds {
			return false, errUnsupportedLexical
		}
		shadow, err := declaratorTailBindsRequire(s, next)
		if err != nil {
			return false, err
		}
		// A later declarator that could bind `require` fails closed: `var` is
		// function-scoped, so recording a block-depth shadow would be unsound.
		if shadow {
			return false, errUnsupportedLexical
		}
		return false, nil
	}
	if !isIdentStart(s[i]) {
		return false, nil
	}
	end := i + 1
	for end < len(s) && isIdent(s[end]) {
		end++
	}
	return s[i:end] == "require", nil
}

func arrowBindsRequire(s string, state lexer) (bool, error) {
	if state.previous == tokenIdentifier {
		return state.lastRequire, nil
	}
	if state.previous != tokenCloseParen || state.lastParenStart < 0 || state.lastParenEnd < state.lastParenStart {
		return false, errUnsupportedLexical
	}
	return arrowParametersBindRequire(s, state.lastParenStart, state.lastParenEnd)
}

func arrowParametersBindRequire(s string, start, end int) (bool, error) {
	i, err := trivia(s, start+1)
	if err != nil {
		return false, err
	}
	if i == end {
		return false, nil
	}
	for {
		rest := false
		if s[i] == '.' {
			if i+3 > end || s[i:i+3] != "..." {
				return false, errUnsupportedLexical
			}
			rest = true
			if i, err = trivia(s, i+3); err != nil {
				return false, err
			}
			if i == end {
				return false, errUnsupportedLexical
			}
		}
		var binds bool
		i, binds, err = bindingTarget(s, i, end, 0)
		if err != nil {
			return false, err
		}
		if binds {
			return true, nil
		}
		if i, err = trivia(s, i); err != nil {
			return false, err
		}
		if i < end && s[i] == ':' {
			if i, err = boundedExpression(s, i+1, end); err != nil {
				return false, err
			}
		}
		if i < end && s[i] == '=' {
			if rest {
				return false, errUnsupportedLexical
			}
			if i, err = boundedExpression(s, i+1, end); err != nil {
				return false, err
			}
		}
		if i == end {
			return false, nil
		}
		if s[i] != ',' || rest {
			return false, errUnsupportedLexical
		}
		if i, err = trivia(s, i+1); err != nil {
			return false, err
		}
		if i == end {
			return false, nil
		}
	}
}

const maxPatternDepth = 16

// bindingTarget parses one closed binding target — an identifier, an object
// pattern, or an array pattern — and reports whether it could bind `require`
// (property keys count). Every unrecognized production fails closed.
func bindingTarget(s string, i, end, depth int) (int, bool, error) {
	if depth == maxPatternDepth || i == end {
		return i, false, errUnsupportedLexical
	}
	if isIdentStart(s[i]) {
		name := i
		for i++; i < end && isIdent(s[i]); i++ {
		}
		return i, s[name:i] == "require", nil
	}
	if s[i] == '{' {
		return objectPattern(s, i, end, depth)
	}
	if s[i] == '[' {
		return arrayPattern(s, i, end, depth)
	}
	return i, false, errUnsupportedLexical
}

func objectPattern(s string, i, end, depth int) (int, bool, error) {
	binds := false
	i, err := trivia(s, i+1)
	if err != nil {
		return i, false, err
	}
	for {
		if i == end {
			return i, false, errUnsupportedLexical
		}
		if s[i] == '}' {
			return i + 1, binds, nil
		}
		if s[i] == '.' {
			if i+3 > end || s[i:i+3] != "..." {
				return i, false, errUnsupportedLexical
			}
			if i, err = trivia(s, i+3); err != nil {
				return i, false, err
			}
			if i == end || !isIdentStart(s[i]) {
				return i, false, errUnsupportedLexical
			}
			name := i
			for i++; i < end && isIdent(s[i]); i++ {
			}
			binds = binds || s[name:i] == "require"
			if i, err = trivia(s, i); err != nil {
				return i, false, err
			}
			if i == end || s[i] != '}' {
				return i, false, errUnsupportedLexical
			}
			return i + 1, binds, nil
		}
		var b bool
		i, b, err = objectPatternProperty(s, i, end, depth)
		if err != nil {
			return i, false, err
		}
		binds = binds || b
		if i == end {
			return i, false, errUnsupportedLexical
		}
		if s[i] == '}' {
			return i + 1, binds, nil
		}
		if s[i] != ',' {
			return i, false, errUnsupportedLexical
		}
		if i, err = trivia(s, i+1); err != nil {
			return i, false, err
		}
	}
}

func objectPatternProperty(s string, i, end, depth int) (int, bool, error) {
	binds := false
	var err error
	switch {
	case isIdentStart(s[i]):
		name := i
		for i++; i < end && isIdent(s[i]); i++ {
		}
		key := s[name:i]
		if i, err = trivia(s, i); err != nil {
			return i, false, err
		}
		if i < end && s[i] == ':' {
			binds = key == "require"
			if i, err = trivia(s, i+1); err != nil {
				return i, false, err
			}
			var b bool
			i, b, err = bindingTarget(s, i, end, depth+1)
			if err != nil {
				return i, false, err
			}
			binds = binds || b
			if i, err = trivia(s, i); err != nil {
				return i, false, err
			}
		} else {
			binds = key == "require"
		}
	case s[i] == '\'' || s[i] == '"':
		if i, _, err = quoted(s, i); err != nil {
			return i, false, err
		}
		i, binds, err = objectPatternKeyedBinding(s, i, end, depth)
		if err != nil {
			return i, false, err
		}
	case s[i] >= '0' && s[i] <= '9':
		for i++; i < end && (s[i] >= '0' && s[i] <= '9' || s[i] == '.'); i++ {
		}
		i, binds, err = objectPatternKeyedBinding(s, i, end, depth)
		if err != nil {
			return i, false, err
		}
	default:
		return i, false, errUnsupportedLexical
	}
	if i < end && s[i] == '=' {
		if i, err = boundedExpression(s, i+1, end); err != nil {
			return i, false, err
		}
	}
	return i, binds, nil
}

func objectPatternKeyedBinding(s string, i, end, depth int) (int, bool, error) {
	i, err := trivia(s, i)
	if err != nil {
		return i, false, err
	}
	if i == end || s[i] != ':' {
		return i, false, errUnsupportedLexical
	}
	if i, err = trivia(s, i+1); err != nil {
		return i, false, err
	}
	i, binds, err := bindingTarget(s, i, end, depth+1)
	if err != nil {
		return i, false, err
	}
	if i, err = trivia(s, i); err != nil {
		return i, false, err
	}
	return i, binds, nil
}

func arrayPattern(s string, i, end, depth int) (int, bool, error) {
	binds := false
	i, err := trivia(s, i+1)
	if err != nil {
		return i, false, err
	}
	for {
		if i == end {
			return i, false, errUnsupportedLexical
		}
		if s[i] == ']' {
			return i + 1, binds, nil
		}
		if s[i] == ',' {
			if i, err = trivia(s, i+1); err != nil {
				return i, false, err
			}
			continue
		}
		if s[i] == '.' {
			if i+3 > end || s[i:i+3] != "..." {
				return i, false, errUnsupportedLexical
			}
			if i, err = trivia(s, i+3); err != nil {
				return i, false, err
			}
			var b bool
			i, b, err = bindingTarget(s, i, end, depth+1)
			if err != nil {
				return i, false, err
			}
			binds = binds || b
			if i, err = trivia(s, i); err != nil {
				return i, false, err
			}
			if i == end || s[i] != ']' {
				return i, false, errUnsupportedLexical
			}
			return i + 1, binds, nil
		}
		var b bool
		i, b, err = bindingTarget(s, i, end, depth+1)
		if err != nil {
			return i, false, err
		}
		binds = binds || b
		if i, err = trivia(s, i); err != nil {
			return i, false, err
		}
		if i < end && s[i] == '=' {
			if i, err = boundedExpression(s, i+1, end); err != nil {
				return i, false, err
			}
		}
		if i == end {
			return i, false, errUnsupportedLexical
		}
		if s[i] == ']' {
			return i + 1, binds, nil
		}
		if s[i] != ',' {
			return i, false, errUnsupportedLexical
		}
		if i, err = trivia(s, i+1); err != nil {
			return i, false, err
		}
	}
}

// ambiguousJSXAtom handles `<` at expression start inside a retrospective
// scanner. A successful JSX reading is accepted only when the non-JSX reading
// of the same span contains no top-level scanner delimiter — otherwise the two
// readings disagree about binding structure and the input fails closed.
func ambiguousJSXAtom(s string, i int, state *lexicalState) (int, bool, error) {
	next, jsx, err := skipJSX(s, i, 1)
	if err != nil {
		return i, false, err
	}
	if !jsx {
		return i, false, nil
	}
	if jsxSpanCrossesBoundary(s, i, next) {
		return i, false, errUnsupportedLexical
	}
	state.setValue()
	return next, true, nil
}

func jsxSpanCrossesBoundary(s string, i, end int) bool {
	state := lexicalState{slash: slashRegex}
	depth := 0
	for i < end {
		if depth == 0 && (s[i] == '\n' || s[i] == '\r' || s[i] == ';') {
			return true
		}
		next, skipped, err := skipTrivia(s, i)
		if err != nil {
			return true
		}
		if skipped {
			if depth == 0 && strings.ContainsAny(s[i:next], "\r\n\u2028\u2029") {
				return true
			}
			i = next
			continue
		}
		if s[i] == '\'' || s[i] == '"' || s[i] == '`' || s[i] == '/' {
			next, atom, err := skipAtom(s, i, &state, 1)
			if err != nil {
				return true
			}
			if atom {
				i = next
				continue
			}
		}
		switch {
		case s[i] == '{' || s[i] == '[' || s[i] == '(':
			depth++
			state.setOperator()
		case s[i] == '}' || s[i] == ']' || s[i] == ')':
			if depth == 0 {
				return true
			}
			depth--
			state.setValue()
		case s[i] == ',' || s[i] == '=':
			if depth == 0 {
				return true
			}
			state.setOperator()
		case isIdentStart(s[i]):
			name := i
			for i++; i < end && isIdent(s[i]); i++ {
			}
			state.setWord(s[name:i])
			continue
		case s[i] >= '0' && s[i] <= '9':
			for i < end && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			state.setValue()
			continue
		case s[i] == '.':
			if i+2 < end && s[i+1] == '.' && s[i+2] == '.' {
				state.setOperator()
				i += 3
				continue
			}
			state.slash, state.previous = slashUnknown, tokenProperty
		default:
			state.setOperator()
		}
		i++
	}
	return i != end || depth != 0
}

// boundedExpression skips one nonempty balanced expression (an annotation or a
// default value) and returns at the first top-level `,`, `=`, closer, or the
// span end. Expressions never bind names, so their identifiers are not
// inspected.
func boundedExpression(s string, i, end int) (int, error) {
	state := lexicalState{slash: slashRegex}
	depth, seen := 0, false
	for i < end {
		next, skipped, err := skipTrivia(s, i)
		if err != nil {
			return i, err
		}
		if skipped {
			i = next
			continue
		}
		if depth == 0 {
			switch s[i] {
			case ',', '=', '}', ']', ')':
				if !seen {
					return i, errUnsupportedLexical
				}
				return i, nil
			}
		}
		seen = true
		if s[i] == '<' && state.slash == slashRegex {
			next, jsx, err := ambiguousJSXAtom(s, i, &state)
			if err != nil {
				return i, err
			}
			if jsx {
				i = next
				continue
			}
			state.setOperator()
			i++
			continue
		}
		next, atom, err := skipAtom(s, i, &state, 1)
		if err != nil {
			return i, err
		}
		if atom {
			i = next
			continue
		}
		switch {
		case s[i] == '{' || s[i] == '[' || s[i] == '(':
			depth++
			state.setOperator()
		case s[i] == '}' || s[i] == ']' || s[i] == ')':
			depth--
			state.setValue()
		case isIdentStart(s[i]):
			name := i
			for i++; i < end && isIdent(s[i]); i++ {
			}
			state.setWord(s[name:i])
			continue
		case s[i] >= '0' && s[i] <= '9':
			for i < end && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			state.setValue()
			continue
		case s[i] == '.':
			if i+2 < end && s[i+1] == '.' && s[i+2] == '.' {
				state.setOperator()
				i += 3
				continue
			}
			state.slash, state.previous = slashUnknown, tokenProperty
		default:
			state.setOperator()
		}
		i++
	}
	if !seen {
		return i, errUnsupportedLexical
	}
	return i, nil
}

// declaratorTailBindsRequire scans the rest of a declaration statement whose
// first declarator was a destructuring pattern: later declarators can still
// bind `require`, so every one is inspected. The statement must end at a
// top-level `;`, a top-level closer, or end of input — a top-level newline
// cannot be bounded against ASI continuation and fails closed.
func declaratorTailBindsRequire(s string, i int) (bool, error) {
	state := lexicalState{slash: slashRegex}
	depth := 0
	shadow := false
	for i < len(s) {
		if depth == 0 {
			switch s[i] {
			case ';', ')', '}', ']':
				return shadow, nil
			case '\n', '\r':
				return false, errUnsupportedLexical
			case ',':
				var err error
				i, err = trivia(s, i+1)
				if err != nil {
					return false, err
				}
				if i == len(s) {
					return false, errUnsupportedLexical
				}
				if s[i] == '{' || s[i] == '[' {
					next, binds, err := bindingTarget(s, i, len(s), 0)
					if err != nil {
						return false, err
					}
					if binds {
						return false, errUnsupportedLexical
					}
					i = next
					continue
				}
				if !isIdentStart(s[i]) {
					return false, errUnsupportedLexical
				}
				name := i
				for i++; i < len(s) && isIdent(s[i]); i++ {
				}
				shadow = shadow || s[name:i] == "require"
				state.setWord(s[name:i])
				continue
			}
		}
		next, skipped, err := skipTrivia(s, i)
		if err != nil {
			return false, err
		}
		if skipped {
			i = next
			continue
		}
		if s[i] == '<' && state.slash == slashRegex {
			next, jsx, err := ambiguousJSXAtom(s, i, &state)
			if err != nil {
				return false, err
			}
			if jsx {
				i = next
				continue
			}
			state.setOperator()
			i++
			continue
		}
		next, atom, err := skipAtom(s, i, &state, 1)
		if err != nil {
			return false, err
		}
		if atom {
			i = next
			continue
		}
		switch {
		case s[i] == '{' || s[i] == '[' || s[i] == '(':
			depth++
			state.setOperator()
		case s[i] == '}' || s[i] == ']' || s[i] == ')':
			depth--
			state.setValue()
		case isIdentStart(s[i]):
			name := i
			for i++; i < len(s) && isIdent(s[i]); i++ {
			}
			state.setWord(s[name:i])
			continue
		case s[i] >= '0' && s[i] <= '9':
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			state.setValue()
			continue
		case s[i] == '.':
			if i+2 < len(s) && s[i+1] == '.' && s[i+2] == '.' {
				state.setOperator()
				i += 3
				continue
			}
			state.slash, state.previous = slashUnknown, tokenProperty
		default:
			state.setOperator()
		}
		i++
	}
	return shadow, nil
}

func skipCode(s string, i, end, braces int, stop byte, depth int) (int, error) {
	next, _, _, err := scanCode(s, i, end, braces, stop, depth, false)
	return next, err
}

func scanCode(s string, i, end, braces int, stop byte, depth int, m bool) (int, string, bool, error) {
	brackets := 0
	state := lexicalState{slash: slashRegex}
	atomDepth := 0
	if !m {
		atomDepth = depth + 1
	}
	for i < end {
		if m && braces == 0 && (s[i] == '\n' || s[i] == '\r' || s[i] == ';') {
			return i + 1, "", false, nil
		}
		next, skipped, err := skipTrivia(s, i)
		if err != nil {
			return i, "", false, err
		}
		if skipped {
			i = next
			continue
		}
		next, atom, err := skipAtom(s, i, &state, atomDepth)
		if err != nil {
			return i, "", false, err
		}
		if atom {
			i = next
			continue
		}
		switch s[i] {
		case '{':
			braces++
			state.setOperator()
			i++
			continue
		case '}':
			if !m && braces == 1 && state.parens == 0 && brackets == 0 && stop == '}' {
				return i + 1, "", false, nil
			}
			if braces == 0 {
				if m {
					return i, "", false, errLexicalInput
				}
				return i, "", false, errUnsupportedLexical
			}
			braces--
			state.setOperator()
			i++
			continue
		case '(':
			if err := state.openParen(i); err != nil {
				return i, "", false, err
			}
			i++
			continue
		case ')':
			if _, err := state.closeParen(); err != nil {
				return i, "", false, err
			}
			i++
			continue
		case '[':
			if !m {
				brackets++
			}
			state.setOperator()
			i++
			continue
		case ']':
			if !m && brackets == 0 {
				return i, "", false, errUnsupportedLexical
			}
			if !m {
				brackets--
			}
			state.setValue()
			i++
			continue
		case ',':
			if !m && stop == ',' && braces == 0 && state.parens == 0 && brackets == 0 {
				return i, "", false, nil
			}
		case ';':
			if m {
				return i + 1, "", false, nil
			}
		}
		if isIdentStart(s[i]) {
			start := i
			for i++; i < end && isIdent(s[i]); i++ {
			}
			if m && braces == 0 && s[start:i] == "from" && state.previous != tokenProperty {
				next, err := trivia(s, i)
				if err != nil || next == len(s) || s[next] != '\'' && s[next] != '"' {
					return next, "", false, err
				}
				after, value, err := quoted(s, next)
				return after, value, err == nil, err
			}
			state.setWord(s[start:i])
			continue
		}
		if s[i] >= '0' && s[i] <= '9' {
			for i < end && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			state.setValue()
			continue
		}
		if s[i] == '.' {
			if i+2 < end && s[i+1] == '.' && s[i+2] == '.' {
				state.setOperator()
				i += 3
				continue
			}
			state.slash, state.previous = slashUnknown, tokenProperty
		} else {
			state.setOperator()
		}
		i++
	}
	if braces != 0 || state.parens != 0 || !m && brackets != 0 {
		if m {
			return i, "", false, errLexicalInput
		}
		return i, "", false, errUnsupportedLexical
	}
	return i, "", false, nil
}

func skipAtom(s string, i int, state *lexicalState, depth int) (int, bool, error) {
	if s[i] >= 0x80 {
		return i, false, errLexicalInput
	}
	if s[i] == '\\' {
		return i, false, errUnsupportedLexical
	}
	switch s[i] {
	case '\'', '"':
		next, err := skipString(s, i, s[i])
		if err != nil {
			return i, false, err
		}
		state.setValue()
		return next, true, nil
	case '`':
		next, err := skipTemplate(s, i, depth)
		if err != nil {
			return i, false, err
		}
		state.setValue()
		return next, true, nil
	case '/':
		next, regex, err := skipRegex(s, i, state.slash)
		if err != nil {
			return i, false, err
		}
		if regex {
			state.setValue()
			return next, true, nil
		}
		state.setOperator()
		return i + 1, true, nil
	case '<':
		// JSX only opens at expression start; after a value `<` is a
		// comparison or type-argument list and stays an operator.
		if state.slash != slashRegex {
			break
		}
		next, jsx, err := skipJSX(s, i, depth)
		if err != nil {
			return i, false, err
		}
		if jsx {
			state.setValue()
			return next, true, nil
		}
	}
	return i, false, nil
}

func skipRegex(s string, i int, goal slashGoal) (int, bool, error) {
	if goal == slashUnknown {
		return i, false, errUnsupportedLexical
	}
	if goal == slashDivision {
		return i, false, nil
	}
	class := false
	for i++; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '\n' || s[i] == '\r' {
			return i, false, errLexicalInput
		}
		if s[i] == '[' {
			class = true
			continue
		}
		if s[i] == ']' {
			class = false
			continue
		}
		if s[i] == '/' && !class {
			for i++; i < len(s) && isIdentStart(s[i]); i++ {
			}
			return i, true, nil
		}
	}
	return i, false, errLexicalInput
}

func skipJSX(s string, i, depth int) (int, bool, error) {
	if depth == MaxDepth {
		return i, false, errLexicalInput
	}
	if i+1 == len(s) || s[i+1] != '>' && !isIdentStart(s[i+1]) {
		return i, false, nil
	}
	jsxDepth := 0
	for i < len(s) {
		if s[i] == '{' {
			next, err := skipEmbeddedExpression(s, i+1, depth+1)
			if err != nil {
				return i, false, err
			}
			i = next
			continue
		}
		if s[i] != '<' {
			i++
			continue
		}
		next, closing, selfClosing, err := skipJSXTag(s, i, depth+1)
		if err != nil {
			return i, false, err
		}
		if closing {
			jsxDepth--
		} else if !selfClosing {
			jsxDepth++
		}
		if jsxDepth < 0 {
			return i, false, errLexicalInput
		}
		i = next
		if jsxDepth == 0 {
			return i, true, nil
		}
	}
	return i, false, errLexicalInput
}

func skipJSXTag(s string, i, depth int) (int, bool, bool, error) {
	if depth == MaxDepth {
		return i, false, false, errLexicalInput
	}
	j := i + 1
	closing := j < len(s) && s[j] == '/'
	if closing {
		j++
	}
	if j == len(s) {
		return i, false, false, errLexicalInput
	}
	if s[j] == '>' {
		return j + 1, closing, false, nil
	}
	if !isIdentStart(s[j]) {
		return i, false, false, errLexicalInput
	}
	selfClosing := false
	for j < len(s) {
		switch s[j] {
		case '\\', '\'', '"', '<':
			return i, false, false, errLexicalInput
		case '{':
			if closing {
				return i, false, false, errLexicalInput
			}
			next, err := skipEmbeddedExpression(s, j+1, depth+1)
			if err != nil {
				return i, false, false, err
			}
			j = next
			continue
		case '>':
			return j + 1, closing, selfClosing, nil
		case '/':
			selfClosing = true
		default:
			if s[j] != ' ' && s[j] != '\t' && s[j] != '\n' && s[j] != '\r' {
				selfClosing = false
			}
		}
		j++
	}
	return i, false, false, errLexicalInput
}

func skipEmbeddedExpression(s string, i, depth int) (int, error) {
	if depth == MaxDepth {
		return i, errLexicalInput
	}
	return skipCode(s, i, len(s), 1, '}', depth)
}

func moduleDecl(s string, i int, export bool) (int, string, bool, error) {
	i, err := trivia(s, i)
	if err != nil || i == len(s) || s[i] == '(' {
		return i, "", false, err
	}
	if !export && (s[i] == '\'' || s[i] == '"') {
		next, module, err := quoted(s, i)
		return next, module, err == nil, err
	}
	if err := importBindsRequire(s, i); !export && err != nil {
		return i, "", false, err
	}
	if !export && wordAt(s, i, "type") {
		i, err = trivia(s, i+4)
		if err != nil {
			return i, "", false, err
		}
	}
	if !export && isIdentStart(s[i]) {
		end := i + 1
		for end < len(s) && isIdent(s[end]) {
			end++
		}
		next, err := trivia(s, end)
		if err != nil {
			return next, "", false, err
		}
		if next < len(s) && s[next] == '=' {
			next, err = trivia(s, next+1)
			if err != nil {
				return next, "", false, err
			}
			if wordAt(s, next, "require") {
				return bareRequire(s, next)
			}
		}
	}
	// The main lexer tracks declaration bindings, parameters, and bodies.
	for _, word := range strings.Fields("default const let var class function async") {
		if export && wordAt(s, i, word) {
			return i, "", false, nil
		}
	}
	return scanCode(s, i, len(s), 0, 0, 0, true)
}

// importBindsRequire fails closed when a hoisted import clause names `require`.
func importBindsRequire(s string, i int) error {
	for i < len(s) && (isIdent(s[i]) || strings.IndexByte(" \t\r\n/{},*", s[i]) >= 0) {
		next, skipped, err := skipTrivia(s, i)
		for !skipped && next < len(s) && isIdent(s[next]) {
			next++
		}
		if err == nil && s[i:next] == "require" {
			err = errUnsupportedLexical
		}
		if err != nil {
			return err
		}
		i = max(next, i+1)
	}
	return nil
}

func trivia(s string, i int) (int, error) {
	for {
		next, skipped, err := skipTrivia(s, i)
		if err != nil || !skipped {
			return i, err
		}
		i = next
	}
}

func skipTrivia(s string, i int) (int, bool, error) {
	if i == len(s) {
		return i, false, nil
	}
	if strings.IndexByte(" \t\r\n", s[i]) >= 0 {
		return i + 1, true, nil
	}
	if s[i] != '/' || i+1 == len(s) {
		return i, false, nil
	}
	if s[i+1] == '/' {
		if end := strings.IndexAny(s[i:], "\n\r\u2028\u2029"); end >= 0 {
			return i + end, true, nil
		}
		return len(s), true, nil
	}
	if s[i+1] != '*' {
		return i, false, nil
	}
	for i += 2; i+1 < len(s); i++ {
		if s[i] == '*' && s[i+1] == '/' {
			return i + 2, true, nil
		}
	}
	return i, true, errLexicalInput
}

func skipString(s string, i int, quote byte) (int, error) {
	for i++; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == quote {
			return i + 1, nil
		}
		if s[i] == '\n' || s[i] == '\r' {
			return i, errLexicalInput
		}
	}
	return i, errLexicalInput
}

func skipTemplate(s string, i, depth int) (int, error) {
	if depth == MaxDepth {
		return i, errLexicalInput
	}
	for i++; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '`' {
			return i + 1, nil
		}
		if s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			next, err := skipEmbeddedExpression(s, i+2, depth+1)
			if err != nil {
				return i, err
			}
			i = next - 1
		}
	}
	return i, errLexicalInput
}

func wordAt(s string, i int, word string) bool {
	if i > 0 && isIdent(s[i-1]) || len(s)-i < len(word) || s[i:i+len(word)] != word {
		return false
	}
	end := i + len(word)
	return end == len(s) || !isIdent(s[end])
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
func isIdent(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' }

func prefixWord(word string) bool {
	switch word {
	case "await", "break", "case", "catch", "continue", "debugger", "default", "delete", "do", "else", "extends", "finally", "for", "if", "in", "instanceof", "new", "of", "return", "switch", "throw", "try", "typeof", "void", "while", "with", "yield":
		return true
	}
	return false
}

func controlWord(word string) bool {
	switch word {
	case "catch", "for", "if", "switch", "while", "with":
		return true
	}
	return false
}

func quoted(s string, i int) (int, string, error) {
	quote, start := s[i], i+1
	for i = start; i < len(s); i++ {
		if s[i] == '\\' || s[i] < 0x21 || s[i] > 0x7e {
			return i, "", errLexicalInput
		}
		if s[i] != quote {
			continue
		}
		value := s[start:i]
		if !printable(value) || len(value) > 256 {
			return i, "", errLexicalInput
		}
		if strings.Contains(value, ":") && !strings.HasPrefix(value, "node:") && !strings.HasPrefix(value, "bun:") {
			if strings.Contains(value, "@") {
				return i, "", errModuleSecret
			}
			return i, "", errModuleDynamic
		}
		if strings.Contains(value, "//") {
			return i, "", errModuleDynamic
		}
		if strings.Contains(value, "@") && !strings.HasPrefix(value, "@") {
			return i, "", errModuleSecret
		}
		return i + 1, value, nil
	}
	return i, "", errLexicalInput
}
