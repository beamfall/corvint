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
// non-empty, or its setup seeds data: a fixture the file declares or a hook in scope makes a
// seeding call (TSA-V0-305, TSA-V0-306). A test requesting a fixture declared in another file is
// withheld as a cross-file abstention, since that fixture may seed (TSA-V0-307).

var (
	veCountMatchers    = map[string]bool{"toHaveCount": true, "toHaveLength": true}
	veEqualityMatchers = map[string]bool{"toBe": true, "toEqual": true, "toStrictEqual": true}
	veNonEmptyLocator  = map[string]bool{"toBeVisible": true, "toHaveText": true, "toContainText": true, "toBeAttached": true}
)

// veFindings reports every count or equality assertion between two collection-derived operands in
// a test that asserts neither collection non-empty.
func veFindings(s *source) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	fixtures := chFixtures(s)
	for _, d := range s.Decls {
		if d.Kind != "test" {
			continue
		}
		params, _, _ := chFunction(s, d.Body)
		if xfSeeded(s, d, fixtures, chBindings(s, params)) {
			continue
		}
		undeclared := xfUndeclaredFixture(s, params, fixtures)
		chains := veChains(s, d.Body)
		for _, c := range chains {
			roots, ok := veOperands(s, c)
			if !ok || vePrecondition(s, chains, roots) {
				continue
			}
			line := s.Tokens[c.Subject[0]-1].Line
			where := fmt.Sprintf("%s:%d", s.Path, line)
			test := fmt.Sprintf("%s:%d", s.Path, d.Line)
			message := fmt.Sprintf("%s compares %s with %s, and both are collection-derived, so the assertion also passes when neither collection was seeded (0 equals 0); assert a non-empty precondition first, such as `expect(%s.length).toBeGreaterThan(0)`, or compare with the exact seeded count",
				c.Matcher, roots[0], roots[1], roots[1])
			if undeclared != "" {
				im, found := xfFixtureImport(s)
				module, cause := xfWhere(s, im, found, where)
				xfAbstain(s, testvalidity.RuleVacuousEmptyEquality, testvalidity.AbstainFixtureCrossFile, test, where, cause,
					message+fmt.Sprintf("; withheld: the test requests the fixture `%s`, which this file does not declare, so it is declared in %s and may seed the collection; this reading does not follow imports", undeclared, module))
				continue
			}
			out = append(out, testvalidity.SourceFinding{
				Rule: testvalidity.RuleVacuousEmptyEquality, Severity: testvalidity.SeverityAdvisory,
				Test: test, Title: d.Title, Location: where, Cause: where, Message: message,
			})
		}
	}
	return out
}

// veChains reads every expect chain inside a test body.
func veChains(s *source, body [2]int) []expectChain {
	chains := []expectChain{}
	for j := body[0]; j < body[1]; j++ {
		if c, ok := s.ReadExpect(j); ok && c.Matcher != "" {
			chains = append(chains, c)
		}
	}
	return chains
}

// veOperands returns the two collection roots of a count or equality assertion whose operands
// are both collection-derived; ok is false for any other assertion, including a negated one or one
// with a literal operand.
func veOperands(s *source, c expectChain) ([2]string, bool) {
	if eoNegated(s, c) || c.Margs[1] <= c.Margs[0] || len(s.SplitArgs(c.Margs[0]-1, c.Margs[1])) != 1 {
		return [2]string{}, false
	}
	if _, literal := veNumber(s, c.Margs); literal {
		return [2]string{}, false
	}
	left, leftCount := veCollection(s, c.Subject)
	right, rightCount := veCollection(s, c.Margs)
	switch {
	case veCountMatchers[c.Matcher]:
		// The subject of a count matcher is itself the collection.
		if !rightCount || veLiteral(s, c.Subject) {
			return [2]string{}, false
		}
		return [2]string{s.Render(c.Subject[0], c.Subject[1]), right}, true
	case veEqualityMatchers[c.Matcher]:
		if !leftCount || !rightCount || veLiteral(s, c.Subject) {
			return [2]string{}, false
		}
		return [2]string{left, right}, true
	}
	return [2]string{}, false
}

// veCollection strips a trailing `.length`, `.size` or `.count()` from an operand and reports
// whether one was there; await is ignored.
func veCollection(s *source, r [2]int) (string, bool) {
	start, end := r[0], r[1]
	if s.Ident(start, "await") {
		start++
	}
	switch {
	case end-start > 2 && s.Punct(end-2, ".") && (s.Ident(end-1, "length") || s.Ident(end-1, "size")):
		return s.Render(start, end-2), true
	case end-start > 4 && s.Punct(end-4, ".") && s.Ident(end-3, "count") && s.Punct(end-2, "(") && s.Punct(end-1, ")"):
		return s.Render(start, end-4), true
	}
	return s.Render(start, end), false
}

// veEmptyArg reports a matcher argument that asserts nothing is there: `[]` or an empty string,
// as in `toHaveText([])`, which passes on an empty collection.
func veEmptyArg(s *source, c expectChain) bool {
	if c.Margs[1] <= c.Margs[0] {
		return false
	}
	if s.Render(c.Margs[0], c.Margs[1]) == "[]" {
		return true
	}
	t := s.Tokens[c.Margs[0]]
	return c.Margs[1]-c.Margs[0] == 1 && (t.Kind == appmap.JSString || t.Kind == appmap.JSTemplate) && t.Literal && t.Text == ""
}

// veLiteral reports an operand that is one literal token.
func veLiteral(s *source, r [2]int) bool {
	if r[1]-r[0] != 1 {
		return false
	}
	k := s.Tokens[r[0]].Kind
	return k == appmap.JSNumber || (k == appmap.JSString || k == appmap.JSTemplate) && s.Tokens[r[0]].Literal
}

// veNumber reads an operand that is one numeric literal.
func veNumber(s *source, r [2]int) (float64, bool) {
	if r[1]-r[0] != 1 || s.Tokens[r[0]].Kind != appmap.JSNumber {
		return 0, false
	}
	n, err := strconv.ParseFloat(s.Tokens[r[0]].Text, 64)
	return n, err == nil
}

// vePrecondition reports an assertion in the same test that one of the roots is non-empty: a
// positive count or length, a lower bound of at least one, a negated empty assertion, or a
// visibility or text assertion on a member of the collection (TSA-V0-173).
func vePrecondition(s *source, chains []expectChain, roots [2]string) bool {
	for _, c := range chains {
		subject, counted := veCollection(s, c.Subject)
		whole := s.Render(c.Subject[0], c.Subject[1])
		names := func(root string) bool {
			return subject == root || whole == root
		}
		if !names(roots[0]) && !names(roots[1]) {
			for _, root := range roots {
				if strings.HasPrefix(whole, root+".") && veNonEmptyLocator[c.Matcher] && !eoNegated(s, c) && !veEmptyArg(s, c) {
					return true
				}
			}
			continue
		}
		n, number := veNumber(s, c.Margs)
		negated := eoNegated(s, c)
		switch {
		case !negated && c.Matcher == "toBeGreaterThan" && number && n >= 0:
			return true
		case !negated && c.Matcher == "toBeGreaterThanOrEqual" && number && n >= 1:
			return true
		case !negated && (veCountMatchers[c.Matcher] || veEqualityMatchers[c.Matcher] && counted) && number && n >= 1:
			return true
		case !negated && c.Matcher == "toBeTruthy" && counted:
			return true
		case negated && (veCountMatchers[c.Matcher] || veEqualityMatchers[c.Matcher] && counted) && number && n == 0:
			return true
		case negated && (c.Matcher == "toBeEmpty" || c.Matcher == "toEqual" && s.Render(c.Margs[0], c.Margs[1]) == "[]"):
			return true
		case !negated && veNonEmptyLocator[c.Matcher] && !counted && !veEmptyArg(s, c):
			return true
		}
	}
	return false
}
