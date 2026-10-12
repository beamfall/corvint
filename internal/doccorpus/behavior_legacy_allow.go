package doccorpus

import "strings"

// The closed-world allowlist (DCP-V1-062, DCP-V1-063) decides whether a legacy case can reach
// parity-candidate. Every token on the case's static execution path (the file's top-level and
// suite structure, the bodies of the hooks that enclose the case, and the case body) must belong
// to one recognised statement form; the first token that does not marks the case unknown. The
// residue check stays as a diagnostic, but an unlisted form is refused here whatever it spells.

var (
	// legacyJasmineMatcher lists the Jasmine/Jest matchers admitted after expect(...). Matchers that
	// call their operand (toThrow...) are not listed.
	legacyJasmineMatcher = words("toBe toEqual toStrictEqual toContain toBeTruthy toBeFalsy toBeTrue toBeFalse toBeDefined toBeUndefined toBeNull toBeNaN toMatch toBeGreaterThan toBeGreaterThanOrEqual toBeLessThan toBeLessThanOrEqual toBeCloseTo toHaveLength toHaveSize toHaveClass toBePositiveInfinity toBeNegativeInfinity toHaveBeenCalled toHaveBeenCalledWith toHaveBeenCalledTimes toHaveBeenCalledOnceWith toBeInstanceOf toHaveProperty toMatchObject")
	// legacyChainer is the closed set of whole chainer strings admitted in a Cypress should/and
	// call and, after an optional leading `to`, in a chai expect(...) chain. Each is a positive
	// assertion whose meaning is stated in DCP-V1-062, mapped to the number of expected values it
	// takes (-1: one or two, for have.attr). A chainer is never composed from words: a negated,
	// composed, zero-length or unlisted chainer is refused as a whole.
	legacyChainer = map[string]int{
		"be.visible": 0, "exist": 0, "be.checked": 0, "be.disabled": 0, "be.enabled": 0,
		"contain": 1, "contain.text": 1, "have.text": 1, "have.value": 1, "have.class": 1,
		"include.text": 1, "have.length": 1, "have.attr": -1,
	}
	// legacyCypressQuery lists the Cypress queries admitted before a should/contains assertion.
	legacyCypressQuery = words("get find first last eq children parent parents closest siblings next prev filter not location url title hash its root focused")
	legacyLiteralWord  = words("true false null")
	// legacyCypressTransform lists the chainer words after which Cypress yields a new subject
	// (have.attr, have.prop, have.property, ...). A should/and continuing such a chainer would assert
	// on that new subject while the criterion names the original locator, so it is refused.
	legacyCypressTransform = words("have.attr have.length")
)

// legacyAmbiguousValue reports whether a single string argument, which the criterion value
// records unquoted, could read the same as the canonical text of other literal arguments (a
// number, a negative number, true, false or null, an array or object, a quoted list, or the
// no-value marker), so that toBe(1) and toBe('1') would share one tuple.
func legacyAmbiguousValue(f *legacyFile, args []legacyRange) bool {
	if len(args) != 1 || args[0].hi-args[0].lo != 1 || f.tokens[args[0].lo].kind != 's' {
		return false
	}
	text := f.tokens[args[0].lo].text
	if text == "" || strings.ContainsRune("0123456789.-+[{('\"`", rune(text[0])) {
		return true
	}
	for _, word := range []string{"true", "false", "null"} {
		if strings.HasPrefix(text, word) {
			return true
		}
	}
	return false
}

// legacyAllowAssertion admits a whole chainer from legacyChainer with its expected values: the
// listed count, have.length only with a literal integer of at least one (a zero length passes on
// an empty subject and suppresses the existence check), and no ambiguous single string value.
func legacyAllowAssertion(f *legacyFile, chainer string, values []legacyRange) bool {
	n, ok := legacyChainer[chainer]
	switch {
	case !ok, n >= 0 && len(values) != n, n < 0 && (len(values) < 1 || len(values) > 2):
		return false
	case chainer == "have.length":
		return legacyPositiveCount(f, values)
	}
	return !legacyAmbiguousValue(f, values)
}

// legacyPositiveCount reports whether values is one decimal integer literal of at least one.
func legacyPositiveCount(f *legacyFile, values []legacyRange) bool {
	if len(values) != 1 || values[0].hi-values[0].lo != 1 || f.tokens[values[0].lo].kind != 'n' {
		return false
	}
	text := f.tokens[values[0].lo].text
	return text[0] >= '1' && text[0] <= '9' && strings.Trim(text, "0123456789") == ""
}

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

// legacyPlainCallback returns the body of an anonymous, parameterless, synchronous inline
// function with a block body spanning exactly r: () => {...} or function () {...}. A name or a
// parameter could shadow the names the allowlist trusts; an async function or an expression body
// can return a promise the runner awaits, so its completion order is not the source order. Each
// is refused.
func legacyPlainCallback(f *legacyFile, r legacyRange) (legacyRange, int) {
	t, k := f.tokens, r.lo
	if k < r.hi && t[k].kind == 'i' && t[k].text == "async" {
		return legacyRange{}, k
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
		return legacyRange{}, k
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
	// A Jasmine/Jest matcher is admitted only as the single call after expect(...), never negated.
	if legacyPunct(t, j, ".") && j+2 < hi && t[j+1].kind == 'i' && legacyJasmineMatcher[t[j+1].text] {
		word := t[j+1].text
		end, bad := legacyAllowCall(f, j+2, hi)
		if bad >= 0 {
			return 0, bad
		}
		args := legacyArgs(f, j+2)
		if (word == "toHaveLength" || word == "toHaveSize") && !legacyPositiveCount(f, args) || legacyAmbiguousValue(f, args) {
			return 0, j + 3
		}
		*keys = append(*keys, k)
		return end, -1
	}
	// A chai chain, less an optional leading `to`, must equal one whole legacyChainer entry: a
	// property entry is not called and a value entry is called with its values.
	words, start := []string{}, j
	for legacyPunct(t, j, ".") && j+1 < hi && t[j+1].kind == 'i' && !legacyPunct(t, j, "(") {
		words, j = append(words, t[j+1].text), j+2
		if legacyPunct(t, j, "(") {
			break
		}
	}
	if len(words) > 0 && words[0] == "to" {
		words = words[1:]
	}
	chainer, values, end := strings.Join(words, "."), []legacyRange{}, j
	if legacyPunct(t, j, "(") {
		var bad int
		if end, bad = legacyAllowCall(f, j, hi); bad >= 0 {
			return 0, bad
		}
		values = legacyArgs(f, j)
		if legacyChainer[chainer] == 0 {
			return 0, j
		}
	} else if n, ok := legacyChainer[chainer]; ok && n != 0 {
		return 0, min(j, hi-1)
	}
	if !legacyAllowAssertion(f, chainer, values) {
		// No admitted assertion runs: a bare expect(...), a composed, negated or unlisted chain.
		return 0, min(start+1, hi-1)
	}
	*keys = append(*keys, k)
	return end, -1
}

func legacyAllowCypress(f *legacyFile, k, hi int, keys *[]int) (int, int) {
	t := f.tokens
	// contained records that a contains ran; pending is a query after it that no should/and has
	// followed yet, whose implicit existence check no criterion would record.
	j, asserted, should, transformed, contained, pending := k+1, false, false, false, false, -1
	for legacyPunct(t, j, ".") && j+1 < hi && t[j+1].kind == 'i' {
		word, name := t[j+1].text, j+1
		switch {
		case word == "should" || word == "and" && should:
			if transformed {
				// The previous chainer yielded a new subject the criterion locator does not name.
				return 0, name
			}
			if !legacyPunct(t, j+2, "(") {
				return 0, min(j+2, hi-1)
			}
			args := legacyArgs(f, j+2)
			// The whole chainer must be one admitted positive assertion: a negated or zero-length
			// one would disable the implicit existence check that the query and contains criteria
			// before it claim, and a composed one may not run the assertion it names.
			if len(args) == 0 || args[0].hi-args[0].lo != 1 || t[args[0].lo].kind != 's' || !legacyAllowAssertion(f, t[args[0].lo].text, args[1:]) {
				return 0, j + 3
			}
			transformed = legacyCypressTransform[t[args[0].lo].text]
			should, pending = true, -1
		case should:
			return 0, name
		case word == "contains":
			if !legacyPunct(t, j+2, "(") {
				return 0, min(j+2, hi-1)
			}
			// Only contains(content) and contains(selector, content) are admitted: the criterion
			// records no options argument.
			args := legacyArgs(f, j+2)
			if len(args) == 0 || len(args) > 2 || len(args) == 2 && legacyPunct(t, args[1].lo, "{") {
				return 0, j + 2
			}
			if legacyAmbiguousValue(f, args[len(args)-1:]) {
				return 0, args[len(args)-1].lo
			}
			if pending >= 0 {
				return 0, name
			}
			contained = true
		case legacyCypressQuery[word]:
			if contained {
				if pending >= 0 {
					return 0, name
				}
				pending = name
			}
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
	if pending >= 0 {
		return 0, pending
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
