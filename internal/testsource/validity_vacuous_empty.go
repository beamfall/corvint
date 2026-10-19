package testsource

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// VACUOUS-EMPTY-EQUALITY reads (TSA-V0-172, TSA-V0-173): a count or equality assertion whose two
// operands are both derived from collections passes when nothing was seeded, since both sides are
// then zero. The static reading flags it unless the same test asserts one of the collections
// non-empty; it does not model fixture seeding, so a seeded fixture without a precondition is still
// reported, as an advisory.

var (
	veCountMatchers    = map[string]bool{"toHaveCount": true, "toHaveLength": true}
	veEqualityMatchers = map[string]bool{"toBe": true, "toEqual": true, "toStrictEqual": true}
	veNonEmptyLocator  = map[string]bool{"toBeVisible": true, "toHaveText": true, "toContainText": true, "toBeAttached": true}
)

// veFindings reports every count or equality assertion between two collection-derived operands in
// a test that asserts neither collection non-empty.
func (s *source) veFindings() []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	for _, d := range s.decls {
		if d.kind != "test" {
			continue
		}
		chains := s.veChains(d.body)
		for _, c := range chains {
			roots, ok := s.veOperands(c)
			if !ok || s.vePrecondition(chains, roots) {
				continue
			}
			line := s.tokens[c.subject[0]-1].Line
			where := fmt.Sprintf("%s:%d", s.path, line)
			out = append(out, testvalidity.SourceFinding{
				Rule: testvalidity.RuleVacuousEmptyEquality, Severity: testvalidity.SeverityAdvisory,
				Test: fmt.Sprintf("%s:%d", s.path, d.line), Title: d.title, Location: where, Cause: where,
				Message: fmt.Sprintf("%s compares %s with %s, and both are collection-derived, so the assertion also passes when neither collection was seeded (0 equals 0); assert a non-empty precondition first, such as `expect(%s.length).toBeGreaterThan(0)`, or compare with the exact seeded count",
					c.matcher, roots[0], roots[1], roots[1]),
			})
		}
	}
	return out
}

// veChains reads every expect chain inside a test body.
func (s *source) veChains(body [2]int) []expectChain {
	chains := []expectChain{}
	for j := body[0]; j < body[1]; j++ {
		if c, ok := s.readExpect(j); ok && c.matcher != "" {
			chains = append(chains, c)
		}
	}
	return chains
}

// veOperands returns the two collection roots of a count or equality assertion whose operands
// are both collection-derived; ok is false for any other assertion, including a negated one or one
// with a literal operand.
func (s *source) veOperands(c expectChain) ([2]string, bool) {
	if s.eoNegated(c) || c.margs[1] <= c.margs[0] || len(s.splitArgs(c.margs[0]-1, c.margs[1])) != 1 {
		return [2]string{}, false
	}
	if _, literal := s.veNumber(c.margs); literal {
		return [2]string{}, false
	}
	left, leftCount := s.veCollection(c.subject)
	right, rightCount := s.veCollection(c.margs)
	switch {
	case veCountMatchers[c.matcher]:
		// The subject of a count matcher is itself the collection.
		if !rightCount || s.veLiteral(c.subject) {
			return [2]string{}, false
		}
		return [2]string{s.render(c.subject[0], c.subject[1]), right}, true
	case veEqualityMatchers[c.matcher]:
		if !leftCount || !rightCount || s.veLiteral(c.subject) {
			return [2]string{}, false
		}
		return [2]string{left, right}, true
	}
	return [2]string{}, false
}

// veCollection strips a trailing `.length`, `.size` or `.count()` from an operand and reports
// whether one was there; await is ignored.
func (s *source) veCollection(r [2]int) (string, bool) {
	start, end := r[0], r[1]
	if s.ident(start, "await") {
		start++
	}
	switch {
	case end-start > 2 && s.punct(end-2, ".") && (s.ident(end-1, "length") || s.ident(end-1, "size")):
		return s.render(start, end-2), true
	case end-start > 4 && s.punct(end-4, ".") && s.ident(end-3, "count") && s.punct(end-2, "(") && s.punct(end-1, ")"):
		return s.render(start, end-4), true
	}
	return s.render(start, end), false
}

// veLiteral reports an operand that is one literal token.
func (s *source) veLiteral(r [2]int) bool {
	if r[1]-r[0] != 1 {
		return false
	}
	k := s.tokens[r[0]].Kind
	return k == appmap.JSNumber || (k == appmap.JSString || k == appmap.JSTemplate) && s.tokens[r[0]].Literal
}

// veNumber reads an operand that is one numeric literal.
func (s *source) veNumber(r [2]int) (float64, bool) {
	if r[1]-r[0] != 1 || s.tokens[r[0]].Kind != appmap.JSNumber {
		return 0, false
	}
	n, err := strconv.ParseFloat(s.tokens[r[0]].Text, 64)
	return n, err == nil
}

// vePrecondition reports an assertion in the same test that one of the roots is non-empty: a
// positive count or length, a lower bound of at least one, a negated empty assertion, or a
// visibility or text assertion on a member of the collection (TSA-V0-173).
func (s *source) vePrecondition(chains []expectChain, roots [2]string) bool {
	for _, c := range chains {
		subject, counted := s.veCollection(c.subject)
		whole := s.render(c.subject[0], c.subject[1])
		names := func(root string) bool {
			return subject == root || whole == root
		}
		if !names(roots[0]) && !names(roots[1]) {
			for _, root := range roots {
				if strings.HasPrefix(whole, root+".") && veNonEmptyLocator[c.matcher] && !s.eoNegated(c) {
					return true
				}
			}
			continue
		}
		n, number := s.veNumber(c.margs)
		negated := s.eoNegated(c)
		switch {
		case !negated && c.matcher == "toBeGreaterThan" && number && n >= 0:
			return true
		case !negated && c.matcher == "toBeGreaterThanOrEqual" && number && n >= 1:
			return true
		case !negated && (veCountMatchers[c.matcher] || veEqualityMatchers[c.matcher] && counted) && number && n >= 1:
			return true
		case !negated && c.matcher == "toBeTruthy" && counted:
			return true
		case negated && (veCountMatchers[c.matcher] || veEqualityMatchers[c.matcher] && counted) && number && n == 0:
			return true
		case negated && (c.matcher == "toBeEmpty" || c.matcher == "toEqual" && s.render(c.margs[0], c.margs[1]) == "[]"):
			return true
		case !negated && veNonEmptyLocator[c.matcher] && !counted:
			return true
		}
	}
	return false
}
