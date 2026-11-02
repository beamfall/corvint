// SPDX-License-Identifier: Apache-2.0
package typescript

import "strings"

// playwrightDeferredMembers are the members of the proven `test` binding whose function-literal
// argument Playwright registers without running it while tests are collected; "" is the plain
// `test(...)` call. A `describe` callback runs during collection, so it is absent (TJAA-V0-725).
var playwrightDeferredMembers = map[string]bool{"": true, "only": true, "skip": true, "fixme": true, "fail": true, "beforeEach": true, "afterEach": true, "beforeAll": true, "afterAll": true}

// playwrightUnguardedImports are @playwright/test exports that cannot reach the `test` object,
// so the monkeypatch guard does not follow them (TJAA-V0-725).
var playwrightUnguardedImports = map[string]bool{"expect": true, "defineConfig": true, "devices": true}

// playwrightGuardDeclarations are the words before a name that make `NAME(` a declaration or a
// construction rather than a direct call (TJAA-V0-725).
var playwrightGuardDeclarations = map[string]bool{"function": true, "class": true, "const": true, "let": true, "var": true, "new": true}

// playwrightTestBodies returns the brace ranges of the test and hook bodies of a lexed module
// whose callee is a `test` binding a named import from @playwright/test proves, and the offset and
// reason of the first break of the monkeypatch guard, or patchWhy "" (TJAA-V0-725).
func playwrightTestBodies(clean string, lexed playwrightLexed) (bodies [][2]int, patchAt int, patchWhy string) {
	opens := make(map[int]int, len(lexed.closes))
	for open, close := range lexed.closes {
		opens[close] = open
	}
	breakAt := func(at int, why string) {
		if patchWhy == "" || at < patchAt {
			patchAt, patchWhy = at, why
		}
	}
	loadWhy := "`@playwright/test` is loaded other than by a named import, so a test body the closure defers can run while tests are collected"
	proven := map[string]bool{}
	guarded := map[string][][2]int{}
	for _, quote := range []string{"'", `"`} {
		literal := quote + "@playwright/test" + quote
		for from := 0; ; {
			index := strings.Index(clean[from:], literal)
			if index < 0 {
				break
			}
			at := from + index
			from = at + len(literal)
			if lexed.code[at] || at > 0 && !lexed.code[at-1] {
				continue
			}
			previous := playwrightPreviousCode(clean, at)
			switch playwrightWordBefore(clean, previous) {
			case "import":
				continue
			case "from":
			default:
				breakAt(at, loadWhy)
				continue
			}
			brace := playwrightPreviousCode(clean, previous-len("from")+1)
			open, ok := opens[brace]
			if brace < 0 || clean[brace] != '}' || !ok {
				breakAt(at, loadWhy)
				continue
			}
			keyword := playwrightPreviousCode(clean, open)
			kind := playwrightWordBefore(clean, keyword)
			if kind == "type" && playwrightWordBefore(clean, playwrightPreviousCode(clean, keyword-len("type")+1)) == "import" {
				continue
			}
			if kind != "import" && kind != "export" {
				breakAt(at, loadWhy)
				continue
			}
			for _, specifier := range strings.Split(clean[open+1:brace], ",") {
				fields := strings.Fields(specifier)
				name, local := "", ""
				switch {
				case len(fields) == 0 || fields[0] == "type":
					continue
				case len(fields) == 1:
					name, local = fields[0], fields[0]
				case len(fields) == 3 && fields[1] == "as":
					name, local = fields[0], fields[2]
				default:
					breakAt(at, loadWhy)
					continue
				}
				if playwrightUnguardedImports[name] {
					continue
				}
				if kind == "export" {
					breakAt(at, loadWhy)
					continue
				}
				guarded[local] = append(guarded[local], [2]int{open, brace})
				proven[local] = proven[local] || name == "test"
			}
		}
	}
	for local, specifiers := range guarded {
		why := "`" + local + "` is rewritten, rebound or used other than as a direct call, so a test body the closure defers can run while tests are collected"
	occurrences:
		for _, offset := range playwrightTokenOffsets(clean, local) {
			if !lexed.code[offset] {
				continue
			}
			for _, specifier := range specifiers {
				if offset > specifier[0] && offset < specifier[1] {
					continue occurrences
				}
			}
			previous := playwrightPreviousCode(clean, offset)
			if previous >= 0 && clean[previous] == '.' {
				continue
			}
			members := []string{}
			next := playwrightNextCode(clean, offset+len(local))
			for next < len(clean) && clean[next] == '.' {
				start := playwrightNextCode(clean, next+1)
				member := ""
				if start < len(clean) {
					member = playwrightIdentifierAt(clean, start)
				}
				if member == "" {
					break
				}
				members = append(members, member)
				next = playwrightNextCode(clean, start+len(member))
			}
			if next >= len(clean) || clean[next] != '(' || !lexed.code[next] || playwrightGuardDeclarations[playwrightWordBefore(clean, previous)] {
				breakAt(offset, why)
				continue
			}
			close, ok := lexed.closes[next]
			if !ok {
				breakAt(offset, why)
				continue
			}
			if after := playwrightNextCode(clean, close+1); after < len(clean) && (clean[after] == '{' || clean[after] == ':') {
				breakAt(offset, why)
				continue
			}
			if proven[local] && len(members) <= 1 && playwrightDeferredMembers[strings.Join(members, "")] {
				if body, ok := playwrightFunctionLiteralBody(clean, lexed, opens, next, close); ok {
					bodies = append(bodies, body)
				}
			}
		}
	}
	return bodies, patchAt, patchWhy
}

// playwrightFunctionLiteralBody returns the block of the call's last argument when it is an arrow
// `=> {` or a `function (...) {` literal, optionally `async`, that starts the argument directly
// after the call's `(` or a top-level `,` (TJAA-V0-725).
func playwrightFunctionLiteralBody(clean string, lexed playwrightLexed, opens map[int]int, call, close int) ([2]int, bool) {
	last := playwrightPreviousCode(clean, close)
	if last > call && clean[last] == ',' {
		last = playwrightPreviousCode(clean, last)
	}
	open, ok := opens[last]
	if last <= call || clean[last] != '}' || !ok || lexed.parent[open] != call {
		return [2]int{}, false
	}
	before := playwrightPreviousCode(clean, open)
	start := -1
	switch {
	case before >= 1 && clean[before] == '>' && clean[before-1] == '=':
		parameters := playwrightPreviousCode(clean, before-1)
		if parameters >= 0 && clean[parameters] == ')' {
			if start, ok = opens[parameters]; !ok {
				return [2]int{}, false
			}
		} else if name := playwrightWordBefore(clean, parameters); name != "" {
			start = parameters - len(name) + 1
		}
	case before >= 0 && clean[before] == ')':
		parameters, ok := opens[before]
		if !ok {
			return [2]int{}, false
		}
		keyword := playwrightPreviousCode(clean, parameters)
		word := playwrightWordBefore(clean, keyword)
		if word != "" && word != "function" {
			keyword = playwrightPreviousCode(clean, keyword-len(word)+1)
			word = playwrightWordBefore(clean, keyword)
		}
		if word == "function" {
			start = keyword - len(word) + 1
		}
	}
	if start < 0 {
		return [2]int{}, false
	}
	previous := playwrightPreviousCode(clean, start)
	if playwrightWordBefore(clean, previous) == "async" {
		previous = playwrightPreviousCode(clean, previous-len("async")+1)
	}
	if previous != call && (previous < 0 || clean[previous] != ',' || lexed.parent[previous] != call) {
		return [2]int{}, false
	}
	return [2]int{open, last}, true
}
