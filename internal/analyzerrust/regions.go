// SPDX-License-Identifier: Apache-2.0

package analyzerrust

import "sort"

// ExecutableBodies extracts function-body interiors from the Rust dialect's
// bounded lexer, preserving byte columns in the exact supplied blob. It is a
// conservative lexical profile, not a compiler: macros, attributes and closure
// bodies need expansion/type information and refuse the entire extraction.
// Empty files and files containing declarations only have a complete empty set.
// No analyzer request, response or fact wire is changed (RSP-V0-005).
func ExecutableBodies(source []byte) ([]Body, error) {
	tokens, reason := tokenize(source)
	if reason != "" {
		return nil, Unsupported("rust-regions:" + reason)
	}
	lineStarts := []int{0}
	for i, b := range source {
		if b == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	position := func(offset int) (int32, int32) {
		line := sort.Search(len(lineStarts), func(i int) bool { return lineStarts[i] > offset }) - 1
		return int32(line + 1), int32(offset - lineStarts[line] + 1)
	}
	// Pair delimiters once. tokenize has already checked their nesting.
	ends := make(map[int]int)
	var stack []int
	for i, t := range tokens {
		if t.kind != tokenPunct {
			continue
		}
		switch t.text {
		case "#", "!", "|":
			// A ! could be a macro, a # could hide cfg/attribute expansion,
			// and a | could open an unmodeled closure. False positives widen.
			return nil, Unsupported("rust-regions:expansion-or-closure-unsupported")
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			ends[open] = i
		}
	}
	var bodies []Body
	type enclosingBody struct{ open, index int }
	var enclosing []enclosingBody
	for i, t := range tokens {
		for len(enclosing) > 0 && ends[enclosing[len(enclosing)-1].open] <= i {
			enclosing = enclosing[:len(enclosing)-1]
		}
		if t.kind != tokenIdent || t.text != "fn" {
			continue
		}
		// fn(...) as a type has no named body. A named fn must have a
		// parameter list; bodyless trait/extern declarations end at ';'.
		if i+1 == len(tokens) || tokens[i+1].text == "(" {
			continue
		}
		if tokens[i+1].kind != tokenIdent && tokens[i+1].kind != tokenRawIdent {
			return nil, Unsupported("rust-regions:function-name-unsupported")
		}
		open, parameters, angles := -1, false, 0
		for j := i + 2; j < len(tokens); j++ {
			x := tokens[j]
			if x.kind != tokenPunct {
				continue
			}
			switch x.text {
			case "<":
				angles++
			case ">":
				if angles > 0 && (j == 0 || tokens[j-1].text != "-") {
					angles--
				}
			case "(", "[":
				parameters = parameters || x.text == "("
				j = ends[j]
			case ";":
				j = len(tokens)
			case "{":
				if !parameters || angles != 0 {
					return nil, Unsupported("rust-regions:signature-unsupported")
				}
				open = j
				j = len(tokens)
			}
		}
		if open < 0 {
			continue
		}
		// Const generics can contain braces before the function body. Refuse
		// them rather than mistaking a signature expression for the body.
		for j := i + 2; j < open; j++ {
			if tokens[j].text == "const" {
				return nil, Unsupported("rust-regions:const-signature-unsupported")
			}
		}
		lo, hi := tokens[open].start+1, tokens[ends[open]].start
		if lo == hi { // An empty body has no executable interior.
			continue
		}
		sl, sc := position(lo)
		el, ec := position(hi)
		body := Body{ID: "fn@" + decimal(t.start), Range: Range{Start: sl, StartColumn: sc, End: el, EndColumn: ec}}
		if len(enclosing) > 0 {
			// IDs come from declaration offsets, not names (methods and nested
			// functions can repeat a name in one file).
			body.Parent = bodies[enclosing[len(enclosing)-1].index].ID
		}
		bodies = append(bodies, body)
		enclosing = append(enclosing, enclosingBody{open, len(bodies) - 1})
	}
	return bodies, nil
}
