package tcq

// pyStmt is one Python statement: its own tokens plus, for a compound
// statement, the suite it owns. Indentation alone builds this tree — TCQ needs
// function boundaries and statement shape, not a grammar.
type pyStmt struct {
	tokens []pyToken
	suite  []pyStmt
	inline []pyStmt
}

// body returns the statements of a compound statement's suite, whether written
// on the header line (`def f(): pass`) or as an indented block.
func (stmt pyStmt) body() []pyStmt {
	if len(stmt.inline) > 0 {
		return stmt.inline
	}
	return stmt.suite
}

func (stmt pyStmt) compound() bool { return len(stmt.suite) > 0 || len(stmt.inline) > 0 }

// end is the last source offset the statement and everything it owns reaches.
// It is the FunctionDef end offset the frozen grammar reports (TCQ-V0-013).
func (stmt pyStmt) end() int {
	last := stmt.tokens[len(stmt.tokens)-1].end
	if body := stmt.body(); len(body) > 0 {
		if deeper := body[len(body)-1].end(); deeper > last {
			last = deeper
		}
	}
	return last
}

// buildPythonBlock assembles the statements at one indentation level. A deeper
// line opens the previous statement's suite; a shallower line closes this level.
// A dedent to a width no enclosing level uses is not Python 3.9 source.
func buildPythonBlock(lines []pyLine, index, indent int) ([]pyStmt, int, error) {
	var block []pyStmt
	for index < len(lines) {
		line := lines[index]
		if line.indent < indent {
			return block, index, nil
		}
		if line.indent > indent {
			if len(block) == 0 {
				return nil, 0, errPythonGrammar
			}
			suite, next, err := buildPythonBlock(lines, index, line.indent)
			if err != nil {
				return nil, 0, err
			}
			block[len(block)-1].suite = suite
			index = next
			continue
		}
		statements, err := splitSimpleStatements(line.tokens)
		if err != nil {
			return nil, 0, err
		}
		block = append(block, statements...)
		index++
	}
	return block, index, nil
}

// splitSimpleStatements separates `a; b` and detaches a same-line suite from its
// compound header, so every element of the result is one statement.
func splitSimpleStatements(tokens []pyToken) ([]pyStmt, error) {
	header, rest := splitSuiteHeader(tokens)
	if header != nil {
		inline, err := splitSimpleStatements(rest)
		if err != nil {
			return nil, err
		}
		return []pyStmt{{tokens: header, inline: inline}}, nil
	}
	var statements []pyStmt
	for _, part := range splitTopLevel(tokens, ";") {
		if len(part) == 0 {
			continue
		}
		statements = append(statements, pyStmt{tokens: part})
	}
	if len(statements) == 0 {
		return nil, errPythonGrammar
	}
	return statements, nil
}

var pythonCompoundKeywords = map[string]bool{
	"def": true, "class": true, "if": true, "elif": true, "else": true, "for": true,
	"while": true, "with": true, "try": true, "except": true, "finally": true, "async": true,
}

// splitSuiteHeader returns (header, same-line body) when the line opens a suite
// on its own line, and (nil, nil) otherwise. The colon that opens a suite is the
// first one at bracket depth zero; `:=`, annotations, slices, and dict literals
// are separate tokens or nested, so none of them can be mistaken for it.
func splitSuiteHeader(tokens []pyToken) ([]pyToken, []pyToken) {
	if len(tokens) == 0 || tokens[0].kind != pyName || !pythonCompoundKeywords[tokens[0].text] {
		return nil, nil
	}
	depth := 0
	for index, token := range tokens {
		switch token.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case ":":
			if depth == 0 && index+1 < len(tokens) {
				return tokens[:index+1], tokens[index+1:]
			}
			if depth == 0 {
				return nil, nil
			}
		}
	}
	return nil, nil
}

// splitTopLevel cuts a token run on one separator at bracket depth zero.
func splitTopLevel(tokens []pyToken, separator string) [][]pyToken {
	var parts [][]pyToken
	depth, start := 0, 0
	for index, token := range tokens {
		switch token.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case separator:
			if depth == 0 {
				parts = append(parts, tokens[start:index])
				start = index + 1
			}
		}
	}
	return append(parts, tokens[start:])
}

// matchingParen returns the index of the `)` closing the `(` at open.
func matchingParen(tokens []pyToken, open int) int {
	depth := 0
	for index := open; index < len(tokens); index++ {
		switch tokens[index].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}
