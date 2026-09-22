package tcq

// lineStarts indexes the byte offset of every physical line.
func lineStarts(data []byte) []int {
	starts := []int{0}
	for index, current := range data {
		if current == '\n' {
			starts = append(starts, index+1)
		}
	}
	return starts
}

// lineAt returns the one-based physical line containing pos.
func lineAt(starts []int, pos int) int {
	low, high := 0, len(starts)-1
	for low < high {
		middle := (low + high + 1) / 2
		if starts[middle] <= pos {
			low = middle
		} else {
			high = middle - 1
		}
	}
	return low + 1
}

// allNameTokens returns every NAME token in the blob carrying exactly text.
func (walk *pythonWalk) allNameTokens(text string) []pyToken {
	var matches []pyToken
	for _, line := range walk.lines {
		for _, token := range line.tokens {
			if token.kind == pyName && token.text == text {
				matches = append(matches, token)
			}
		}
	}
	return matches
}

// dottedName reads the `a.b.c` prefix of a token run and returns it with the
// index just past it. Anything else — a subscript, a call result, an alias — has
// no dotted name, which is exactly why TCQ-V0-015 resolves no aliases.
func dottedName(tokens []pyToken) (string, int) {
	if len(tokens) == 0 || tokens[0].kind != pyName {
		return "", 0
	}
	name := tokens[0].text
	index := 1
	for index+1 < len(tokens) && tokens[index].text == "." && tokens[index+1].kind == pyName {
		name += "." + tokens[index+1].text
		index += 2
	}
	return name, index
}

// callArguments returns the argument tokens when the run is exactly
// `<dotted name>(...)` with nothing after the closing parenthesis, plus whether
// the run was a call at all.
func callArguments(tokens []pyToken) ([]pyToken, bool) {
	_, index := dottedName(tokens)
	if index == 0 || index >= len(tokens) || tokens[index].text != "(" {
		return nil, false
	}
	closing := matchingParen(tokens, index)
	if closing != len(tokens)-1 {
		return nil, false
	}
	return tokens[index+1 : closing], true
}

// literalBool reports whether the first argument is exactly the given boolean
// literal. TCQ-V0-015 resolves no dynamic condition, so only a bare literal counts.
func literalBool(arguments []pyToken, expected string) bool {
	parts := splitTopLevel(arguments, ",")
	if len(parts) == 0 || len(parts[0]) != 1 {
		return false
	}
	return parts[0][0].kind == pyName && parts[0][0].text == expected
}

// pythonSkipDecorator implements the exact syntactic decorator forms of
// TCQ-V0-015. Aliases and dynamic conditions are deliberately not resolved.
func pythonSkipDecorator(tokens []pyToken) bool {
	name, index := dottedName(tokens)
	if index == 0 {
		return false
	}
	isCall := index < len(tokens) && tokens[index].text == "(" && matchingParen(tokens, index) == len(tokens)-1
	switch name {
	case "unittest.skip":
		return isCall
	case "pytest.mark.skip":
		return true
	}
	if !isCall {
		return false
	}
	arguments := tokens[index+1 : len(tokens)-1]
	switch name {
	case "unittest.skipIf", "pytest.mark.skipif":
		return literalBool(arguments, "True")
	case "unittest.skipUnless":
		return literalBool(arguments, "False")
	}
	return false
}

// pythonSkipped implements TCQ-V0-015: a decorator on the function or its
// enclosing class, or an unconditional skip as the first non-no-op statement.
func pythonSkipped(direct []pyStmt, decorators, classDecorators [][]pyToken) bool {
	for _, decorator := range append(append([][]pyToken{}, decorators...), classDecorators...) {
		if pythonSkipDecorator(decorator) {
			return true
		}
	}
	first, ok := firstEffectiveStatement(direct)
	if !ok {
		return false
	}
	return pythonSkipStatement(first)
}

func firstEffectiveStatement(direct []pyStmt) (pyStmt, bool) {
	for _, statement := range direct {
		if !pythonNoop(statement) {
			return statement, true
		}
	}
	return pyStmt{}, false
}

func pythonSkipStatement(statement pyStmt) bool {
	if statement.compound() {
		return false
	}
	tokens := statement.tokens
	if len(tokens) > 0 && tokens[0].kind == pyName && tokens[0].text == "raise" {
		name, index := dottedName(tokens[1:])
		return index > 0 && name == "unittest.SkipTest" && isCallRun(tokens[1:])
	}
	name, index := dottedName(tokens)
	if index == 0 || !isCallRun(tokens) {
		return false
	}
	return name == "self.skipTest" || name == "pytest.skip"
}

func isCallRun(tokens []pyToken) bool {
	_, ok := callArguments(tokens)
	return ok
}
