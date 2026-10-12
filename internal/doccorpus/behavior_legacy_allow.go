package doccorpus

// The closed-world allowlist (DCP-V1-062, DCP-V1-063) decides whether a legacy case can reach
// parity-candidate. Every token on the case's static execution path (the file's top-level and
// suite structure, the bodies of the hooks that enclose the case, and the case body) must belong
// to one recognised statement form; the first token that does not marks the case unknown. The
// residue check stays as a diagnostic, but an unlisted form is refused here whatever it spells.

var (
	// legacyJasmineMatcher lists the Jasmine/Jest matchers admitted after expect(...). Matchers that
	// call their operand (toThrow...) are not listed.
	legacyJasmineMatcher = words("toBe toEqual toStrictEqual toContain toBeTruthy toBeFalsy toBeTrue toBeFalse toBeDefined toBeUndefined toBeNull toBeNaN toMatch toBeGreaterThan toBeGreaterThanOrEqual toBeLessThan toBeLessThanOrEqual toBeCloseTo toHaveLength toHaveSize toHaveClass toBePositiveInfinity toBeNegativeInfinity toHaveBeenCalled toHaveBeenCalledWith toHaveBeenCalledTimes toHaveBeenCalledOnceWith toBeInstanceOf toHaveProperty toMatchObject")
	// legacyChaiChain lists chai's language chains and flags, which may appear anywhere in a chain.
	legacyChaiChain = words("to be been is that which and has have with at of same but does still also not deep nested own ordered any all")
	// legacyChaiProperty lists the chai property assertions that may end a chain without a call.
	legacyChaiProperty = words("ok true false null undefined NaN exist empty finite extensible sealed frozen arguments")
	// legacyChaiMethod lists the chai assertion methods admitted with literal arguments. Methods that
	// call a function (throw, satisfy, change, respondTo) are not listed.
	legacyChaiMethod = words("equal equals eq eql eqls include includes contain contains a an above gt greaterThan below lt lessThan least gte greaterThanOrEqual most lte lessThanOrEqual within instanceof instanceOf property ownProperty haveOwnProperty lengthOf length match matches string keys key members oneOf closeTo approximately")
	// legacyCypressQuery lists the Cypress queries admitted before a should/contains assertion.
	legacyCypressQuery = words("get find first last eq children parent parents closest siblings next prev filter not location url title hash its root focused")
	legacyLiteralWord  = words("true false null")
)

// allowStructure checks the runner structure of tokens[lo:hi]: only runner suite, case and hook
// call statements with a static title and an inline callback, recursing into suite bodies, which
// all run when the file loads. It returns the first refused token, or -1.
func (p *legacyProducer) allowStructure(f *legacyFile, lo, hi int) int {
	t := f.tokens
	for k := lo; k < hi; {
		if legacyPunct(t, k, ";") {
			k++
			continue
		}
		c, ok := p.apiCall(f, k)
		if !ok {
			return k
		}
		args := legacyArgs(f, c.open)
		switch {
		case p.runner.suites[c.name]:
			if len(args) != 2 {
				return c.open
			}
			if bad := legacyAllowTitle(f, args[0]); bad >= 0 {
				return bad
			}
			body, bad := legacyPlainCallback(f, args[1])
			if bad >= 0 {
				return bad
			}
			if bad := p.allowStructure(f, body.lo, body.hi); bad >= 0 {
				return bad
			}
		case p.runner.cases[c.name]:
			if len(args) < 1 || len(args) > 2 {
				return c.open
			}
			if bad := legacyAllowTitle(f, args[0]); bad >= 0 {
				return bad
			}
			// The case callback runs only for that case; its shape is checked per case.
			if len(args) == 2 {
				if _, ok := legacyCallback(f, args[1]); !ok {
					return args[1].lo
				}
			}
		default: // hook
			if len(args) < 1 || len(args) > 2 {
				return c.open
			}
			if len(args) == 2 {
				if bad := legacyAllowTitle(f, args[0]); bad >= 0 {
					return bad
				}
			}
			if _, bad := legacyPlainCallback(f, args[len(args)-1]); bad >= 0 {
				return bad
			}
		}
		k = c.close + 1
		if k < hi && !legacyPunct(t, k, ";") && !t[k].nl {
			return k
		}
	}
	return -1
}

func legacyAllowTitle(f *legacyFile, r legacyRange) int {
	if r.hi-r.lo != 1 || f.tokens[r.lo].kind != 's' {
		return r.lo
	}
	return -1
}

// legacyPlainCallback returns the body of an anonymous, parameterless inline function spanning
// exactly r: () => {...}, () => expression or function () {...}, optionally async. A name or a
// parameter could shadow the names the allowlist trusts, so either is refused.
func legacyPlainCallback(f *legacyFile, r legacyRange) (legacyRange, int) {
	t, k := f.tokens, r.lo
	if k < r.hi && t[k].kind == 'i' && t[k].text == "async" {
		k++
	}
	arrow := !(k < r.hi && t[k].kind == 'i' && t[k].text == "function")
	if !arrow {
		k++
	}
	if !legacyPunct(t, k, "(") || k >= r.hi {
		return legacyRange{}, min(k, r.hi-1)
	}
	if f.match[k] != k+1 {
		return legacyRange{}, k + 1
	}
	k += 2
	if arrow {
		if !legacyPunct(t, k, "=>") || k+1 >= r.hi {
			return legacyRange{}, min(k, r.hi-1)
		}
		k++
		if legacyPunct(t, k, "{") && f.match[k] == r.hi-1 {
			return legacyRange{k + 1, r.hi - 1}, -1
		}
		return legacyRange{k, r.hi}, -1
	}
	if !legacyPunct(t, k, "{") || f.match[k] != r.hi-1 {
		return legacyRange{}, min(k, r.hi-1)
	}
	return legacyRange{k + 1, r.hi - 1}, -1
}

// allowBody checks that every statement of a hook (hook) or case body is one admitted form and
// returns the first refused token, or -1. keys receives, for each admitted case statement, the
// token the extraction must have consumed (the expect, should, and or contains name, or the
// precondition root), so an admitted form the extraction missed is still refused.
func (p *legacyProducer) allowBody(f *legacyFile, r legacyRange, hook bool, keys *[]int) int {
	t := f.tokens
	for k := r.lo; k < r.hi; {
		if legacyPunct(t, k, ";") {
			k++
			continue
		}
		if t[k].kind == 'i' && t[k].text == "await" && k+1 < r.hi {
			if t[k+1].nl {
				// Outside an async function a line break makes await a separate statement.
				return k
			}
			k++
		}
		end, bad := legacyAllowPrecondition(f, k, r.hi)
		switch {
		case bad < 0:
			*keys = append(*keys, k)
		case hook:
			return bad
		default:
			if end, bad = p.allowAssertion(f, k, r.hi, keys); bad >= 0 {
				return bad
			}
		}
		if end < r.hi && !legacyPunct(t, end, ";") && !t[end].nl {
			return end
		}
		k = end
	}
	return -1
}

// legacyAllowPrecondition admits root.command(literals) for a known precondition callee
// (cy.visit, cy.task, cy.intercept, page.goto, browser.url, ...), optionally followed by
// .as('alias'). It returns the statement end, or the first refused token.
func legacyAllowPrecondition(f *legacyFile, k, hi int) (int, int) {
	t := f.tokens
	if t[k].kind != 'i' || !legacyResidueRoot[t[k].text] {
		return 0, k
	}
	if !legacyPunct(t, k+1, ".") || k+2 >= hi || t[k+2].kind != 'i' {
		return 0, k + 1
	}
	if legacyPreconditionOf[t[k].text+"."+t[k+2].text] == "" {
		return 0, k + 2
	}
	end, bad := legacyAllowCall(f, k+3, hi)
	if bad >= 0 {
		return 0, bad
	}
	if legacyPunct(t, end, ".") && end+1 < hi && t[end+1].kind == 'i' && t[end+1].text == "as" {
		if !legacyPunct(t, end+2, "(") || f.match[end+2] != end+4 || t[end+3].kind != 's' {
			return 0, end + 2
		}
		end += 5
	}
	return end, -1
}

// legacyAllowCall admits '(' literal, ... ')' at k and returns the index after ')'.
func legacyAllowCall(f *legacyFile, k, hi int) (int, int) {
	if !legacyPunct(f.tokens, k, "(") || k >= hi {
		return 0, min(k, hi-1)
	}
	for _, arg := range legacyArgs(f, k) {
		if bad := legacyAllowLiteral(f, arg); bad >= 0 {
			return 0, bad
		}
	}
	return f.match[k] + 1, -1
}

// legacyAllowLiteral admits a string, number, true, false, null, a negative number, or an object
// or array literal of those with plain keys. Nothing in it can call code.
func legacyAllowLiteral(f *legacyFile, r legacyRange) int {
	t := f.tokens
	switch {
	case r.hi-r.lo == 1 && (t[r.lo].kind == 's' || t[r.lo].kind == 'n' || t[r.lo].kind == 'i' && legacyLiteralWord[t[r.lo].text]):
		return -1
	case r.hi-r.lo == 2 && legacyPunct(t, r.lo, "-") && t[r.lo+1].kind == 'n':
		return -1
	case r.hi-r.lo >= 2 && legacyPunct(t, r.lo, "[") && f.match[r.lo] == r.hi-1:
		for _, element := range legacyArgs(f, r.lo) {
			if element.hi == element.lo {
				return element.lo
			}
			if bad := legacyAllowLiteral(f, element); bad >= 0 {
				return bad
			}
		}
		return -1
	case r.hi-r.lo >= 2 && legacyPunct(t, r.lo, "{") && f.match[r.lo] == r.hi-1:
		for _, entry := range legacyArgs(f, r.lo) {
			key := t[entry.lo]
			if entry.hi-entry.lo < 3 || key.kind != 'i' && key.kind != 's' && key.kind != 'n' || !legacyPunct(t, entry.lo+1, ":") {
				return entry.lo
			}
			if bad := legacyAllowLiteral(f, legacyRange{entry.lo + 2, entry.hi}); bad >= 0 {
				return bad
			}
		}
		return -1
	}
	return r.lo
}

// legacyAllowOperand admits one literal or a plain member path (name or this, then .name...).
func legacyAllowOperand(f *legacyFile, r legacyRange) int {
	t := f.tokens
	if r.hi == r.lo {
		return r.lo
	}
	if legacyAllowLiteral(f, r) < 0 {
		return -1
	}
	if t[r.lo].kind != 'i' || legacyKeyword[t[r.lo].text] && t[r.lo].text != "this" {
		return r.lo
	}
	for k := r.lo + 1; k < r.hi; k += 2 {
		if !legacyPunct(t, k, ".") {
			return k
		}
		if k+1 >= r.hi || t[k+1].kind != 'i' {
			return min(k+1, r.hi-1)
		}
	}
	return -1
}

// allowAssertion admits expect(operand)[.withContext(literal)].matcher-chain(literals) and a
// cy-rooted query chain that ends in contains or in should/and assertions with a static chainer.
func (p *legacyProducer) allowAssertion(f *legacyFile, k, hi int, keys *[]int) (int, int) {
	t := f.tokens
	if t[k].kind != 'i' {
		return 0, k
	}
	switch t[k].text {
	case "expect":
		return legacyAllowExpect(f, k, hi, keys)
	case "cy":
		return legacyAllowCypress(f, k, hi, keys)
	}
	return 0, k
}

func legacyAllowExpect(f *legacyFile, k, hi int, keys *[]int) (int, int) {
	t := f.tokens
	if !legacyPunct(t, k+1, "(") {
		return 0, k + 1
	}
	close := f.match[k+1]
	if bad := legacyAllowOperand(f, legacyRange{k + 2, close}); bad >= 0 {
		return 0, bad
	}
	j := close + 1
	if legacyPunct(t, j, ".") && j+1 < hi && t[j+1].kind == 'i' && t[j+1].text == "withContext" {
		end, bad := legacyAllowCall(f, j+2, hi)
		if bad >= 0 {
			return 0, bad
		}
		j = end
	}
	last := ""
	for legacyPunct(t, j, ".") && j+1 < hi && t[j+1].kind == 'i' {
		word := t[j+1].text
		if !legacyJasmineMatcher[word] && !legacyChaiChain[word] && !legacyChaiProperty[word] && !legacyChaiMethod[word] {
			return 0, j + 1
		}
		last, j = word, j+2
		if legacyPunct(t, j, "(") {
			if !legacyJasmineMatcher[word] && !legacyChaiMethod[word] {
				return 0, j
			}
			end, bad := legacyAllowCall(f, j, hi)
			if bad >= 0 {
				return 0, bad
			}
			*keys = append(*keys, k)
			return end, -1
		}
		if legacyJasmineMatcher[word] {
			return 0, j - 1
		}
	}
	if !legacyChaiProperty[last] {
		// No matcher runs: a bare expect(...) or a chain that ends before its assertion.
		return 0, min(j, hi-1)
	}
	*keys = append(*keys, k)
	return j, -1
}

func legacyAllowCypress(f *legacyFile, k, hi int, keys *[]int) (int, int) {
	t := f.tokens
	j, asserted, should := k+1, false, false
	for legacyPunct(t, j, ".") && j+1 < hi && t[j+1].kind == 'i' {
		word, name := t[j+1].text, j+1
		switch {
		case word == "should" || word == "and" && should:
			if !legacyPunct(t, j+2, "(") {
				return 0, min(j+2, hi-1)
			}
			if args := legacyArgs(f, j+2); len(args) == 0 || args[0].hi-args[0].lo != 1 || t[args[0].lo].kind != 's' {
				return 0, j + 3
			}
			should = true
		case should:
			return 0, name
		case word == "contains":
			if !legacyPunct(t, j+2, "(") {
				return 0, min(j+2, hi-1)
			}
			if args := legacyArgs(f, j+2); len(args) == 0 || len(args) > 3 {
				return 0, j + 2
			}
		case legacyCypressQuery[word]:
		default:
			return 0, name
		}
		end, bad := legacyAllowCall(f, j+2, hi)
		if bad >= 0 {
			return 0, bad
		}
		if word == "should" || word == "and" || word == "contains" {
			*keys = append(*keys, name)
			asserted = true
		}
		j = end
	}
	if !asserted {
		// A query chain with no extracted assertion still fails the case when it finds nothing.
		return 0, min(j, hi-1)
	}
	return j, -1
}

// allowCase returns the first token on the case's execution path that the allowlist refuses,
// with the region it belongs to, or -1. keys is filled for the case body (DCP-V1-062).
func (p *legacyProducer) allowCase(f *legacyFile, s legacyScope, args []legacyRange, keys *[]int) (int, string) {
	switch {
	case f.allowAt >= 0:
		return f.allowAt, "file structure"
	case s.allowAt > 0:
		return s.allowAt - 1, "an enclosing hook"
	case len(args) < 2:
		return -1, ""
	}
	body, bad := legacyPlainCallback(f, args[1])
	if bad >= 0 {
		return bad, "case callback"
	}
	if bad := p.allowBody(f, body, false, keys); bad >= 0 {
		return bad, "case body"
	}
	return -1, ""
}

// allowHook returns 1 + the first refused token of a hook call's callback body, or 0.
func (p *legacyProducer) allowHook(f *legacyFile, c legacyCall) int {
	args := legacyArgs(f, c.open)
	if len(args) == 0 {
		return c.open + 1
	}
	body, bad := legacyPlainCallback(f, args[len(args)-1])
	if bad < 0 {
		var keys []int
		bad = p.allowBody(f, body, true, &keys)
	}
	return bad + 1
}
