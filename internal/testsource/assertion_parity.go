package testsource

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The assertion-parity reading (TSA-V0-670..677): for a new test linked to a legacy test by
// --legacy, extract each side's expectations into normalised tuples (subject, matcher, matcher
// class, polarity, expected value), pair the legacy tuples with the new ones by expected value and
// polarity, and report unmatched legacy expectations, new expectations without an origin and
// weakened matchers. Every finding is advisory. Parity is reported only when both sides were read
// completely; a side with an expectation form, an assertion helper or a conditional the reading
// cannot follow abstains with a reason.

// Assertion-parity abstention reasons (TSA-V0-676).
const (
	assertReasonFileUnsupported   = "assertion-file-unsupported"
	assertReasonSourceUncertain   = "assertion-source-uncertain"
	assertReasonExampleNotFound   = "assertion-example-not-found"
	assertReasonFormUnsupported   = "assertion-form-unsupported"
	assertReasonHelperUnresolved  = "assertion-helper-unresolved"
	assertReasonConditional       = "assertion-conditional"
	assertReasonNoneFound         = "assertion-none-found"
	assertReasonExampleInLoop     = "assertion-example-in-loop"
	assertReasonTupleBoundReached = "assertion-bound-exceeded"
)

// maxAssertionTuples bounds the tuples one side reads (TSA-V0-676).
const maxAssertionTuples = 256

// Matcher classes, strongest first (TSA-V0-672). An unknown matcher has class "other" and
// strength 0, which never yields a weakened finding.
const (
	assertClassExact    = "exact"
	assertClassContains = "contains"
	assertClassPresence = "presence"
	assertClassOther    = "other"
)

var assertClassStrength = map[string]int{assertClassExact: 3, assertClassContains: 2, assertClassPresence: 1, assertClassOther: 0}

// AssertionTuple is one normalised expectation (TSA-V0-671).
type AssertionTuple struct {
	Location  string `json:"location"`
	Framework string `json:"framework"`
	Subject   string `json:"subject"`
	Matcher   string `json:"matcher"`
	Class     string `json:"class"`
	Negated   bool   `json:"negated"`
	// Expected is the normalised expected value, or for a matcher without one the subject's last
	// string literal (a selector, test id or accessible name) prefixed "subject:".
	Expected string `json:"expected"`
}

// AssertionSide is one side's tuples, or why they were not read (TSA-V0-671, TSA-V0-676).
type AssertionSide struct {
	Side     string           `json:"side"`
	Test     string           `json:"test"`
	Analysed bool             `json:"analysed"`
	Reason   string           `json:"reason,omitempty"`
	Location string           `json:"location,omitempty"`
	Tuples   []AssertionTuple `json:"tuples"`
}

// AssertionMatch pairs one legacy tuple with the new tuple that preserves it, by location.
type AssertionMatch struct {
	Legacy   string `json:"legacy"`
	New      string `json:"new"`
	Weakened bool   `json:"weakened"`
}

// AssertionParity is one link's comparison (TSA-V0-673).
type AssertionParity struct {
	New      string                       `json:"new"`
	Legacy   string                       `json:"legacy"`
	Status   string                       `json:"status"`
	Sides    []AssertionSide              `json:"sides"`
	Matches  []AssertionMatch             `json:"matches"`
	Findings []testvalidity.SourceFinding `json:"findings"`
}

// assertExtract is one side's raw extraction.
type assertExtract struct {
	test, reason, at string
	tuples           []AssertionTuple
}

func (x *assertExtract) abstain(reason, at string) {
	if x.reason == "" {
		x.reason, x.at = reason, at
	}
}

func (x *assertExtract) add(t AssertionTuple) {
	if len(x.tuples) >= maxAssertionTuples {
		x.abstain(assertReasonTupleBoundReached, t.Location)
		return
	}
	x.tuples = append(x.tuples, t)
}

// assertionParity reads both sides of one link at the revision (TSA-V0-670). A file that is not
// committed refuses tests-source-unavailable.
func (r Repo) assertionParity(commit string, link SeedLink) (AssertionParity, error) {
	sides := [2]assertExtract{}
	for k, ref := range []string{link.Legacy, link.New} {
		name, line, _ := parseSeedRef(ref)
		data, found, err := r.readOne(commit, name, MaxFileBytes)
		if err != nil {
			return AssertionParity{}, err
		}
		if !found {
			return AssertionParity{}, refuse("tests-source-unavailable", "%s is not a committed file at the revision", name)
		}
		sides[k] = extractAssertions(name, string(data), line)
	}
	return compareAssertions(link, sides[0], sides[1]), nil
}

// extractAssertions routes one side to the RSpec or JS/TS reading (TSA-V0-671).
func extractAssertions(name, text string, line int) assertExtract {
	switch {
	case strings.HasSuffix(name, "_spec.rb"):
		return rspecAssertions(name, text, line)
	case IsTestFile(name) || isCypressSpec(name):
		return jsAssertions(name, text, line)
	}
	x := assertExtract{test: fmt.Sprintf("%s:%d", name, line)}
	x.abstain(assertReasonFileUnsupported, x.test)
	return x
}

// isCypressSpec reports a Cypress spec file, a JS/TS base name ending `.cy` before its extension,
// which the assertion-parity reading also accepts (TSA-V0-671).
func isCypressSpec(name string) bool {
	extension := path.Ext(name)
	return webExtensions[extension] && strings.HasSuffix(strings.TrimSuffix(path.Base(name), extension), ".cy")
}

// compareAssertions pairs the legacy tuples with the new ones (TSA-V0-673..676).
func compareAssertions(link SeedLink, legacy, current assertExtract) AssertionParity {
	out := AssertionParity{New: link.New, Legacy: link.Legacy, Matches: []AssertionMatch{}, Findings: []testvalidity.SourceFinding{}}
	pairs := []struct {
		name string
		x    assertExtract
	}{{"legacy", legacy}, {"new", current}}
	for _, p := range pairs {
		side := AssertionSide{Side: p.name, Test: p.x.test, Analysed: p.x.reason == "", Reason: p.x.reason, Location: p.x.at, Tuples: []AssertionTuple{}}
		if p.x.tuples != nil {
			side.Tuples = p.x.tuples
		}
		out.Sides = append(out.Sides, side)
	}
	if legacy.reason != "" || current.reason != "" {
		out.Status = testvalidity.AssertionParityStatusNotAnalysed
		for k, p := range pairs {
			if p.x.reason == "" {
				continue
			}
			other := pairs[1-k]
			out.Findings = append(out.Findings, testvalidity.SourceFinding{
				Rule: testvalidity.RuleAssertionParityNotAnalysed, Severity: testvalidity.SeverityAdvisory,
				Test: link.New, Location: p.x.at, Cause: other.x.test,
				Message: fmt.Sprintf("the %s side's expectations were not all read (%s at %s), so assertion parity with %s is not reported", p.name, p.x.reason, p.x.at, other.x.test),
			})
		}
		return out
	}
	used := make([]bool, len(current.tuples))
	diverges := false
	for _, l := range legacy.tuples {
		// A tuple with the same expected value is preferred; otherwise a value that one side
		// expects as text and the other locates by (getByText("Saved") against have_content
		// "Saved") pairs them; otherwise a regular expression with a static body and a string
		// value one of which contains the other (TSA-V0-846).
		best, narrowed := -1, false
		for pass := 0; pass < 3 && best < 0; pass++ {
			for k, c := range current.tuples {
				same := c.Expected == l.Expected
				shrinks := false
				switch pass {
				case 1:
					same = c.Expected == "subject:"+l.Expected || l.Expected == "subject:"+c.Expected
				case 2:
					same, shrinks = regexContainment(l.Expected, c.Expected)
					same = same && !l.Negated
				}
				if used[k] || c.Negated != l.Negated || !same || l.Expected == "" || l.Expected == "subject:" {
					continue
				}
				if best < 0 || assertClassStrength[c.Class] > assertClassStrength[current.tuples[best].Class] {
					best, narrowed = k, shrinks
				}
			}
		}
		if best < 0 {
			diverges = true
			out.Findings = append(out.Findings, testvalidity.SourceFinding{
				Rule: testvalidity.RuleAssertionParityUnmatchedLegacy, Severity: testvalidity.SeverityAdvisory,
				Test: link.New, Location: current.test, Cause: l.Location,
				Message: fmt.Sprintf("the legacy expectation %s %s at %s has no new expectation with the same expected value and polarity", l.Matcher, l.Expected, l.Location),
			})
			continue
		}
		used[best] = true
		c := current.tuples[best]
		// An unanchored static-body match is a containment on either side (TSA-V0-846,
		// TSA-V0-852), so the same regex expected by both is not weakened.
		strength, legacyStrength := regexStrength(c), regexStrength(l)
		weakened := strength > 0 && legacyStrength > strength || narrowed
		out.Matches = append(out.Matches, AssertionMatch{Legacy: l.Location, New: c.Location, Weakened: weakened})
		if weakened {
			diverges = true
			out.Findings = append(out.Findings, testvalidity.SourceFinding{
				Rule: testvalidity.RuleAssertionParityWeakened, Severity: testvalidity.SeverityAdvisory,
				Test: link.New, Location: c.Location, Cause: l.Location,
				Message: fmt.Sprintf("the new %s (%s) at %s is weaker than the legacy %s (%s) at %s for %s", c.Matcher, c.Class, c.Location, l.Matcher, l.Class, l.Location, l.Expected),
			})
		}
	}
	for k, c := range current.tuples {
		if used[k] {
			continue
		}
		diverges = true
		out.Findings = append(out.Findings, testvalidity.SourceFinding{
			Rule: testvalidity.RuleAssertionParityNewWithoutOrigin, Severity: testvalidity.SeverityAdvisory,
			Test: link.New, Location: c.Location, Cause: legacy.test,
			Message: fmt.Sprintf("the new expectation %s %s at %s matches no legacy expectation of %s", c.Matcher, c.Expected, c.Location, legacy.test),
		})
	}
	out.Status = testvalidity.AssertionParityStatusParity
	if diverges {
		out.Status = testvalidity.AssertionParityStatusDiverges
	}
	return out
}

// regexStrength is a tuple's class strength, capped at a containment's when it expects a regular
// expression with a static body, which matches any string containing that text.
func regexStrength(t AssertionTuple) int {
	strength := assertClassStrength[t.Class]
	if _, regex := staticRegexBody(t.Expected); regex && strength > assertClassStrength[assertClassContains] {
		strength = assertClassStrength[assertClassContains]
	}
	return strength
}

// regexContainment pairs a legacy and a new expected value when exactly one is a regular
// expression with a static body (staticRegexBody) and one of the two texts contains the other, as
// `cy.url().should("include", "/p/new")` and `toHaveURL(/\/p\/new/)` do. weaker reports that the
// new side's text is the shorter one, so it accepts more than the legacy did (TSA-V0-846).
func regexContainment(legacy, current string) (paired, weaker bool) {
	lBody, lRegex := staticRegexBody(legacy)
	cBody, cRegex := staticRegexBody(current)
	switch {
	case lRegex == cRegex:
		return false, false
	case lRegex:
		legacy = lBody
	default:
		if strings.HasPrefix(legacy, "subject:") {
			return false, false
		}
		current = cBody
	}
	if strings.HasPrefix(current, "subject:") || legacy == "" || current == "" {
		return false, false
	}
	if !strings.Contains(legacy, current) && !strings.Contains(current, legacy) {
		return false, false
	}
	return true, !strings.Contains(current, legacy)
}

// staticRegexBody returns the text a normalised regular expression ("re:BODY") matches when its
// body has no metacharacter but a leading `^` or trailing `$` and escapes only punctuation, so
// it matches exactly the strings containing that text.
func staticRegexBody(expected string) (string, bool) {
	body, ok := strings.CutPrefix(expected, "re:")
	if !ok {
		return "", false
	}
	body = strings.TrimPrefix(body, "^")
	if n := len(body); n > 0 && body[n-1] == '$' && (n < 2 || body[n-2] != '\\') {
		body = body[:n-1]
	}
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case c == '\\':
			if i+1 >= len(body) || body[i+1] == '_' || body[i+1] < 0x80 && (body[i+1] >= '0' && body[i+1] <= '9' || body[i+1]|0x20 >= 'a' && body[i+1]|0x20 <= 'z') {
				return "", false
			}
			i++
			b.WriteByte(body[i])
		case strings.IndexByte(".^$*+?()[]{}|", c) >= 0:
			return "", false
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), b.Len() > 0
}

// assertMatcherClasses maps a lower-cased matcher name, framework-neutral, to its class
// (TSA-V0-672). Arity and options refine some entries in classify.
var assertMatcherClasses = map[string]string{
	// exact
	"eq": assertClassExact, "eql": assertClassExact, "equal": assertClassExact, "equals": assertClassExact,
	"be": assertClassExact, "tobe": assertClassExact, "toequal": assertClassExact, "tostrictequal": assertClassExact,
	"deep.equal": assertClassExact, "match_array": assertClassExact, "contain_exactly": assertClassExact,
	"tohavelength": assertClassExact, "have.length": assertClassExact, "lengthof": assertClassExact, "tohavecount": assertClassExact,
	"tohavetext": assertClassExact, "have.text": assertClassExact, "tohavevalue": assertClassExact, "have.value": assertClassExact,
	"tohaveurl": assertClassExact, "have_current_path": assertClassExact, "be_nil": assertClassExact, "tobenull": assertClassExact,
	"tobeundefined": assertClassExact, "null": assertClassExact, "undefined": assertClassExact, "be.null": assertClassExact,
	"be.undefined": assertClassExact, "true": assertClassExact, "false": assertClassExact, "be.true": assertClassExact,
	"be.false": assertClassExact, "tobetrue": assertClassExact, "tobefalse": assertClassExact, "tohavebeencalledwith": assertClassExact,
	"tohavebeencalledtimes": assertClassExact, "have_field": assertClassExact, "tohavetitle": assertClassExact, "have_title": assertClassExact,
	// contains
	"include": assertClassContains, "tocontain": assertClassContains, "contain": assertClassContains, "contains": assertClassContains,
	"tocontainequal": assertClassContains, "have_content": assertClassContains, "have_text": assertClassContains,
	"tocontaintext": assertClassContains, "contain.text": assertClassContains, "include.text": assertClassContains,
	"match": assertClassContains, "tomatch": assertClassContains, "start_with": assertClassContains, "end_with": assertClassContains,
	"tomatchobject": assertClassContains, "have_key": assertClassContains, "tohaveproperty": assertClassContains,
	"have.property": assertClassContains, "raise_error": assertClassContains, "tothrow": assertClassContains,
	"change": assertClassContains, "have.attr": assertClassContains, "tohaveattribute": assertClassContains,
	// presence
	"be_truthy": assertClassPresence, "tobetruthy": assertClassPresence, "be_present": assertClassPresence,
	"ok": assertClassPresence, "be.ok": assertClassPresence, "exist": assertClassPresence, "be.visible": assertClassPresence,
	"be_visible": assertClassPresence, "tobevisible": assertClassPresence, "tobedefined": assertClassPresence,
	"tobeattached": assertClassPresence, "have_css": assertClassPresence, "have_selector": assertClassPresence,
	"have_link": assertClassPresence, "have_button": assertClassPresence, "have_xpath": assertClassPresence,
	"be.exist": assertClassPresence, "tohavebeencalled": assertClassPresence, "have_received": assertClassPresence,
	"be_falsy": assertClassPresence, "tobefalsy": assertClassPresence, "be.called": assertClassPresence,
}

// classify gives a matcher's class from its name, its expected argument count and options:
// containment with an exact option is exact, a selector matcher with a text option is containment,
// a throw, raise or change with an argument or modifier is exact, and a negated null check is
// presence (TSA-V0-672).
func classify(matcher string, negated bool, args int, exact, text, modified bool) string {
	class, ok := assertMatcherClasses[strings.ToLower(matcher)]
	if !ok {
		return assertClassOther
	}
	m := strings.ToLower(matcher)
	switch {
	case exact && (class == assertClassContains || class == assertClassPresence):
		return assertClassExact
	case text && class == assertClassPresence:
		return assertClassContains
	case (m == "raise_error" || m == "tothrow" || m == "change") && (args > 0 && m != "change" || modified):
		return assertClassExact
	case (m == "tohaveattribute" || m == "have.attr") && args < 2:
		return assertClassPresence
	case negated && (m == "be_nil" || m == "tobenull" || m == "tobeundefined" || m == "null" || m == "undefined" || m == "be.null" || m == "be.undefined"):
		return assertClassPresence
	}
	return class
}

// The value key for matchers whose meaning is a fixed value.
var assertFixedValues = map[string]string{
	"be_nil": "null", "tobenull": "null", "null": "null", "be.null": "null", "tobeundefined": "null", "undefined": "null",
	"be.undefined": "null", "true": "true", "be.true": "true", "tobetrue": "true", "false": "false", "be.false": "false",
	"tobefalse": "false",
}

// normaliseLiteral renders an expected value: a string's content with whitespace collapsed, a
// Ruby symbol's name, a number, true/false/null, a regular expression as re:SOURCE, or any other
// expression with spacing, underscores, `@`, `this.` and case removed so that `user.full_name`
// and `this.user.fullName` agree (TSA-V0-671).
func normaliseLiteral(v string) string {
	v = strings.TrimSpace(v)
	switch {
	case v == "":
		return ""
	case len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'' || v[0] == '`' && v[len(v)-1] == '`' && !strings.Contains(v, "${")):
		inner := v[1 : len(v)-1]
		if v[0] == '"' {
			if unquoted, err := strconv.Unquote(v); err == nil {
				inner = unquoted
			}
		}
		return strings.Join(strings.Fields(inner), " ")
	case v == "nil" || v == "null" || v == "undefined":
		return "null"
	case v == "true" || v == "false":
		return v
	case seedNumber.MatchString(v):
		return v
	case len(v) > 1 && v[0] == ':' && rubyWord.MatchString(v[1:]):
		return v[1:]
	case len(v) > 2 && v[0] == '/' && strings.LastIndexByte(v, '/') > 0:
		return "re:" + v[1:strings.LastIndexByte(v, '/')]
	case strings.HasPrefix(v, "%r{") && strings.HasSuffix(v, "}"):
		return "re:" + v[3:len(v)-1]
	}
	v = strings.ReplaceAll(strings.ReplaceAll(v, "this.", ""), "@", "")
	var b strings.Builder
	for _, c := range v {
		if c == ' ' || c == '\t' || c == '_' {
			continue
		}
		b.WriteRune(c)
	}
	return strings.ToLower(b.String())
}

var assertStringLiteral = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`)

// subjectKey is the last string literal in a subject (a selector, test id or accessible name),
// else the normalised subject (TSA-V0-671).
func subjectKey(subject string) string {
	if all := assertStringLiteral.FindAllString(subject, -1); len(all) > 0 {
		return "subject:" + normaliseLiteral(all[len(all)-1])
	}
	return "subject:" + normaliseLiteral(subject)
}

// ---- RSpec and Capybara ----

var (
	rspecAssertTo       = regexp.MustCompile(`^\.\s*(to|not_to|to_not)\b\s*`)
	rspecMatcherHead    = regexp.MustCompile(`^([a-z_]\w*[?!]?)`)
	rspecHelperCall     = regexp.MustCompile(`^(?:expect_\w+|assert_\w+|assert\b|refute\w*|verify\w*|check_\w+|it_behaves_like|it_should_behave_like|include_examples)\b`)
	rspecControlOpen    = regexp.MustCompile(`^(?:if|unless|case|while|until|begin|for)\b`)
	rspecMidBlock       = regexp.MustCompile(`^(?:elsif|else|when|in|rescue|ensure)\b`)
	rspecLoopOpen       = regexp.MustCompile(`(?:\.(?:each\w*|times|map|select|reject|upto|downto|step)|\bloop)\b.*(?:\bdo|\{)(?:\s*\|[^|]*\|)?$`)
	rspecPlainOpen      = regexp.MustCompile(`(?:\bdo|\{)(?:\s*\|[^|]*\|)?$`)
	rspecModifierIf     = regexp.MustCompile(`\s(?:if|unless|while|until)\s|\s(?:and|or)\s|&&|\|\|`)
	rspecOneLiner       = regexp.MustCompile(`^(?:it|specify|example|scenario|its)\b[^{]*\{\s*(.*)\}$`)
	rspecShouldSyntax   = regexp.MustCompile(`(?:^|[\s.])should(?:_not)?\b`)
	assertBlockOpen     = regexp.MustCompile(`^expect\s*(?:\{|do\b)`)
	assertBlockClose    = regexp.MustCompile(`^(?:\}|end)\s*\.\s*(?:to|not_to|to_not)\b`)
	rspecCapybaraNames  = map[string]bool{"have_content": true, "have_text": true, "have_css": true, "have_selector": true, "have_link": true, "have_button": true, "have_field": true, "have_current_path": true, "have_xpath": true, "have_title": true, "be_visible": true}
	rspecSelectorNames  = map[string]bool{"have_css": true, "have_selector": true, "have_xpath": true}
	rspecModifierNames  = map[string]bool{"by": true, "from": true, "to": true, "with_message": true, "with": true, "once": true, "twice": true, "exactly": true, "times": true, "at_least": true, "at_most": true, "by_at_least": true, "by_at_most": true}
	rspecCompoundSuffix = regexp.MustCompile(`^\s*(?:\.\s*(?:and|or)\b|&|\|)`)
)

// rspecAssertions reads the expectations of the RSpec example declared on line (TSA-V0-671).
func rspecAssertions(name, text string, line int) assertExtract {
	at := func(n int) string { return fmt.Sprintf("%s:%d", name, n) }
	x := assertExtract{test: at(line)}
	r := &rspecReader{lines: rubyLines(text), groups: []rspecGroup{{parent: -1, decls: map[string]string{}}}}
	if r.read() != "" {
		x.abstain(assertReasonSourceUncertain, x.test)
		return x
	}
	found := -1
	for k, e := range r.examples {
		if e.line == line {
			found = k
		}
	}
	if found < 0 {
		x.abstain(assertReasonExampleNotFound, x.test)
		return x
	}
	e := r.examples[found]
	if e.loop {
		x.abstain(assertReasonExampleInLoop, x.test)
		return x
	}
	// frames holds, per open block inside the example, whether it is a conditional or loop: an
	// expectation inside one may not run, or run several times, so the side abstains.
	frames, block := []bool{}, false
	inControl := func() bool {
		for _, control := range frames {
			if control {
				return true
			}
		}
		return false
	}
	for k, l := range e.body {
		t := strings.TrimSpace(l.text)
		if k == 0 {
			m := rspecOneLiner.FindStringSubmatch(t)
			if m == nil {
				continue // the declaring line
			}
			t = strings.TrimSpace(m[1])
		}
		if k == len(e.body)-1 && k > 0 && (t == "end" || t == "}") {
			continue
		}
		masked := strings.TrimSpace(maskRubyStrings(t))
		switch {
		case masked == "end" || masked == "}" || strings.HasPrefix(masked, "end ") || strings.HasPrefix(masked, "end.") && !block:
			if len(frames) > 0 {
				frames = frames[:len(frames)-1]
			}
			continue
		case rspecMidBlock.MatchString(masked):
			continue
		case rspecControlOpen.MatchString(masked):
			frames = append(frames, true)
			continue
		case rspecHelperCall.MatchString(masked):
			x.abstain(assertReasonHelperUnresolved, at(l.line))
			return x
		case assertBlockOpen.MatchString(masked) && !strings.Contains(masked, ".to") && !strings.Contains(masked, ".not_to"):
			if inControl() {
				x.abstain(assertReasonConditional, at(l.line))
				return x
			}
			block = true
			continue
		case block && assertBlockClose.MatchString(masked):
			block = false
			if !assertRSpecMatcher(&x, at(l.line), "{block}", t[strings.Index(t, "."):]) {
				return x
			}
			continue
		case !rspecAssertLine.MatchString(masked):
			if rspecLoopOpen.MatchString(masked) {
				frames = append(frames, true)
			} else if rspecPlainOpen.MatchString(masked) {
				frames = append(frames, false)
			}
			continue
		case inControl() || rspecModifierIf.MatchString(masked):
			x.abstain(assertReasonConditional, at(l.line))
			return x
		case rspecShouldSyntax.MatchString(masked):
			x.abstain(assertReasonFormUnsupported, at(l.line))
			return x
		}
		subject, rest, ok := rspecSubject(t)
		if !ok {
			x.abstain(assertReasonFormUnsupported, at(l.line))
			return x
		}
		if !assertRSpecMatcher(&x, at(l.line), subject, rest) {
			return x
		}
	}
	if block {
		x.abstain(assertReasonFormUnsupported, x.test)
	}
	if x.reason == "" && len(x.tuples) == 0 {
		x.abstain(assertReasonNoneFound, x.test)
	}
	return x
}

// rspecSubject splits `expect(SUBJECT)REST`, `expect { BLOCK }REST` and `is_expected REST`.
func rspecSubject(t string) (string, string, bool) {
	if rest, ok := strings.CutPrefix(t, "is_expected"); ok {
		return "subject", rest, true
	}
	rest, ok := strings.CutPrefix(t, "expect")
	if !ok {
		return "", "", false
	}
	rest = strings.TrimLeft(rest, " ")
	if rest == "" {
		return "", "", false
	}
	switch rest[0] {
	case '(':
		end := rubyClose(rest, 0)
		if end < 0 {
			return "", "", false
		}
		return strings.TrimSpace(rest[1:end]), rest[end+1:], true
	case '{':
		end := rubyClose(rest, 0)
		if end < 0 {
			return "", "", false
		}
		return "{block}", rest[end+1:], true
	}
	if strings.HasPrefix(rest, "do ") {
		if i := strings.LastIndex(rest, " end."); i >= 0 {
			return "{block}", rest[i+4:], true
		}
	}
	return "", "", false
}

// rubyClose returns the index of the bracket closing the one at open, skipping string literals,
// or -1.
func rubyClose(s string, open int) int {
	pairs := map[byte]byte{'(': ')', '{': '}', '[': ']'}
	stack := []byte{}
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(s) && s[j] != c {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			i = j
		case pairs[c] != 0:
			stack = append(stack, pairs[c])
		case c == ')' || c == '}' || c == ']':
			if len(stack) == 0 || stack[len(stack)-1] != c {
				return -1
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i
			}
		}
	}
	return -1
}

// assertRSpecMatcher reads `.to MATCHER(ARGS)[.modifier(...)]*` after the subject and adds the tuple; a
// compound matcher or an unreadable form abstains and returns false.
func assertRSpecMatcher(x *assertExtract, at, subject, rest string) bool {
	m := rspecAssertTo.FindStringSubmatch(rest)
	if m == nil {
		x.abstain(assertReasonFormUnsupported, at)
		return false
	}
	negated := m[1] != "to"
	rest = rest[len(m[0]):]
	head := rspecMatcherHead.FindString(rest)
	if head == "" {
		x.abstain(assertReasonFormUnsupported, at)
		return false
	}
	rest = rest[len(head):]
	args := ""
	if strings.HasPrefix(rest, "(") {
		end := rubyClose(rest, 0)
		if end < 0 {
			x.abstain(assertReasonFormUnsupported, at)
			return false
		}
		args, rest = rest[1:end], rest[end+1:]
	} else if trimmed := strings.TrimSpace(rest); trimmed != "" && !strings.HasPrefix(trimmed, ".") {
		args, rest = trimmed, ""
	}
	modifiers := []string{}
	for {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			break
		}
		if rspecCompoundSuffix.MatchString(rest) || !strings.HasPrefix(rest, ".") {
			x.abstain(assertReasonFormUnsupported, at)
			return false
		}
		mod := rspecMatcherHead.FindString(strings.TrimSpace(rest[1:]))
		if !rspecModifierNames[mod] {
			x.abstain(assertReasonFormUnsupported, at)
			return false
		}
		rest = strings.TrimSpace(rest[1:])[len(mod):]
		value := ""
		if strings.HasPrefix(rest, "(") {
			end := rubyClose(rest, 0)
			if end < 0 {
				x.abstain(assertReasonFormUnsupported, at)
				return false
			}
			value, rest = rest[1:end], rest[end+1:]
		}
		modifiers = append(modifiers, mod+":"+normaliseLiteral(value))
	}
	parts := splitRubyArgs(args)
	positional, options := []string{}, map[string]string{}
	for _, p := range parts {
		if kv := rubyKeyValue.FindStringSubmatch(p); kv != nil && !strings.HasPrefix(p, ":") {
			options[strings.TrimSuffix(kv[1], ":")] = strings.TrimSpace(kv[2])
			continue
		}
		positional = append(positional, p)
	}
	exact := options["exact_text"] != "" && options["exact_text"] != "false" || options["exact"] == "true"
	text := options["text"] != ""
	expected := ""
	lower := strings.ToLower(head)
	switch {
	case assertFixedValues[lower] != "":
		expected = assertFixedValues[lower]
	case text:
		expected = normaliseLiteral(options["text"])
	case options["exact_text"] != "" && options["exact_text"] != "true" && options["exact_text"] != "false":
		expected = normaliseLiteral(options["exact_text"])
	case lower == "have_field" && options["with"] != "":
		expected = normaliseLiteral(options["with"])
	case len(positional) > 0 && (rspecSelectorNames[lower] || lower == "change"):
		expected = "subject:" + normaliseLiteral(positional[len(positional)-1])
	case len(positional) > 0:
		expected = normaliseLiteral(positional[0])
	default:
		expected = subjectKey(subject)
	}
	if lower == "have_field" && options["with"] == "" {
		exact = false
	}
	if len(modifiers) > 0 && (lower == "change" || lower == "raise_error") {
		expected += " " + strings.Join(modifiers, " ")
	}
	class := classify(head, negated, len(positional), exact, text, len(modifiers) > 0)
	if lower == "have_field" && options["with"] == "" {
		class = assertClassPresence
	}
	framework := "rspec"
	if rspecCapybaraNames[lower] {
		framework = "capybara"
	}
	x.add(AssertionTuple{Location: at, Framework: framework, Subject: subject, Matcher: head, Class: class, Negated: negated, Expected: expected})
	return true
}

// splitRubyArgs splits an argument list at top-level commas, outside strings and brackets.
func splitRubyArgs(args string) []string {
	out, depth, start := []string{}, 0, 0
	for i := 0; i < len(args); i++ {
		switch c := args[i]; {
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(args) && args[j] != c {
				if args[j] == '\\' {
					j++
				}
				j++
			}
			i = j
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			out = append(out, strings.TrimSpace(args[start:i]))
			start = i + 1
		}
	}
	if tail := strings.TrimSpace(args[min(start, len(args)):]); tail != "" {
		out = append(out, tail)
	}
	return out
}

// ---- JS/TS: Jest, Jasmine, Mocha/Chai, Cypress and Playwright ----

// chaiChainWords are Chai's language chains, which carry no meaning of their own.
var chaiChainWords = map[string]bool{"to": true, "be": true, "been": true, "is": true, "that": true, "which": true, "and": true,
	"has": true, "have": true, "with": true, "at": true, "of": true, "same": true, "but": true, "does": true, "still": true,
	"deep": true, "also": true, "resolves": true, "rejects": true}

var playwrightMatchers = map[string]bool{"toHaveText": true, "toContainText": true, "toBeVisible": true, "toHaveURL": true,
	"toHaveCount": true, "toHaveValue": true, "toHaveAttribute": true, "toBeAttached": true, "toHaveTitle": true,
	"toBeHidden": true, "toBeEnabled": true, "toBeDisabled": true, "toBeChecked": true, "toHaveClass": true}

var jsHelperPrefix = regexp.MustCompile(`^(?:expect|assert|verify|should|check)[A-Z_]\w*$`)

// jsAssertions reads the expectations of the JS/TS example declared on line (TSA-V0-671).
func jsAssertions(name, text string, line int) assertExtract {
	x := assertExtract{test: fmt.Sprintf("%s:%d", name, line)}
	s := parseSource(name, text)
	if s.uncertain {
		x.abstain(assertReasonSourceUncertain, x.test)
		return x
	}
	r := &jsReader{s: s, name: name}
	r.read()
	found, same := -1, 0
	for k, d := range r.decls {
		if d.kind == "test" && d.line == line {
			found, same = k, same+1
		}
	}
	if same != 1 || !r.decls[found].hasBody {
		x.abstain(assertReasonExampleNotFound, x.test)
		return x
	}
	if r.decls[found].eachKind != "" {
		x.abstain(assertReasonExampleInLoop, x.test)
		return x
	}
	body := r.decls[found].body
	at := func(k int) string { return r.at(s.tokens[k].Line) }
	for k := body[0]; k < body[1]; k++ {
		if s.ident(k, "") && !s.memberAccess(k) {
			switch s.tokens[k].Text {
			case "if", "for", "while", "switch", "catch", "do":
				x.abstain(assertReasonConditional, at(k))
				return x
			}
		}
		if s.ident(k, "") && s.memberAccess(k) && (s.tokens[k].Text == "forEach" || s.tokens[k].Text == "map") || s.conditionalOperator(k) && s.operandAsserts(k, body) {
			x.abstain(assertReasonConditional, at(k))
			return x
		}
	}
	for k := body[0]; k < body[1]; k++ {
		t := s.tokens[k]
		if t.Kind != appmap.JSIdent || s.assertionCount(k) {
			continue
		}
		switch {
		case t.Text == "should" && s.memberAccess(k):
			end, ok := s.cypressShould(&x, k, at(k))
			if !ok {
				return x
			}
			k = end
		case t.Text == "expect" && !s.memberAccess(k):
			c, ok := s.readExpect(k)
			if !ok {
				x.abstain(assertReasonFormUnsupported, at(k))
				return x
			}
			if !s.jsExpectTuple(&x, c, at(k)) {
				return x
			}
			k = c.end
		case !s.memberAccess(k) && s.punct(k+1, "(") && jsHelperPrefix.MatchString(t.Text):
			x.abstain(assertReasonHelperUnresolved, at(k))
			return x
		case s.jsAssertion(k):
			x.abstain(assertReasonFormUnsupported, at(k))
			return x
		}
	}
	if x.reason == "" && len(x.tuples) == 0 {
		x.abstain(assertReasonNoneFound, x.test)
	}
	return x
}

// conditionalOperator reports a `?` (of `?:` or `??`, not `?.`) or the first `|` of `||` or `&` of
// `&&` at k: an operator whose right operand runs only on some paths (TSA-V0-844).
func (s *source) conditionalOperator(k int) bool {
	switch {
	case s.punct(k, "?"):
		return !s.punct(k+1, ".")
	case s.punct(k, "|"), s.punct(k, "&"):
		return s.punct(k+1, s.tokens[k].Text) && s.tokens[k+1].Glued && !s.punct(k-1, s.tokens[k].Text)
	}
	return false
}

// operandAsserts reports whether the conditionally evaluated operand right of the operator at k
// holds an expectation. The operand runs, through any brackets or braces, to a comma, semicolon
// or closing bracket of its own level, or to a line break where no operator continues the
// expression. A `?:`, `??`, `||` or `&&` that only computes a value, as in `const a = b ?? c` or
// `expect(a ?? b)`, leaves every expectation unconditional (TSA-V0-837, TSA-V0-844).
func (s *source) operandAsserts(k int, body [2]int) bool {
	continues := func(t appmap.JSToken, open string) bool {
		return t.Kind == appmap.JSPunct && strings.Contains(open, t.Text)
	}
	depth := 0
	for i := k + 1; i < body[1]; i++ {
		t := s.tokens[i]
		if depth == 0 {
			if t.Kind == appmap.JSPunct && (t.Text == "," || t.Text == ";" || t.Text == ")" || t.Text == "]" || t.Text == "}") {
				return false
			}
			if i > k+1 && t.Line > s.tokens[i-1].Line && !continues(s.tokens[i-1], ".?:|&+-*/%=<>,([{!~^") && !continues(t, ".?:|&+-*/%=<>([") {
				return false
			}
		}
		switch {
		case s.ident(i, "expect") && !s.memberAccess(i), s.ident(i, "should") && s.memberAccess(i):
			return true
		case continues(t, "([{"):
			depth++
		case continues(t, ")]}"):
			depth--
		}
	}
	return false
}

// jsArgs returns the rendered top-level arguments of the call whose `(` is at open.
func (s *source) jsArgs(open int) []string {
	out, start := []string{}, open+1
	end := s.match[open]
	for i := open + 1; i < end; i++ {
		if (s.punct(i, "(") || s.punct(i, "[") || s.punct(i, "{")) && s.match[i] > i {
			i = s.match[i]
			continue
		}
		if s.punct(i, ",") {
			out = append(out, s.render(start, i))
			start = i + 1
		}
	}
	if start < end {
		out = append(out, s.render(start, end))
	}
	return out
}

// jsExpectTuple reads the members after `expect(SUBJECT)`: `.not`, Chai's language chains, one
// matcher call or terminal property. More than one matcher (a Chai `.and` chain) abstains.
func (s *source) jsExpectTuple(x *assertExtract, c expectChain, at string) bool {
	subject := s.render(c.subject[0], c.subject[1])
	negated, matcher, chai := false, "", false
	args := []string{}
	j := c.subject[1] + 1
	for s.punct(j, ".") && s.ident(j+1, "") && j <= c.end {
		member := s.tokens[j+1].Text
		j += 2
		call := s.punct(j, "(") && s.match[j] > 0
		switch {
		case member == "not":
			negated = true
		case chaiChainWords[member] && !call:
			chai = true
		case matcher != "":
			x.abstain(assertReasonFormUnsupported, at)
			return false
		default:
			matcher = member
			if call {
				args = s.jsArgs(j)
			}
		}
		if call {
			j = s.match[j] + 1
		}
	}
	if matcher == "" {
		x.abstain(assertReasonFormUnsupported, at)
		return false
	}
	name := matcher
	if chai && (matcher == "true" || matcher == "false" || matcher == "null" || matcher == "undefined" || matcher == "visible" || matcher == "exist" || matcher == "ok" || matcher == "called") {
		name = matcher
		if matcher == "visible" {
			name = "be.visible"
		}
	}
	framework := "jest"
	switch {
	case chai:
		framework = "chai"
	case playwrightMatchers[matcher]:
		framework = "playwright"
	case strings.HasPrefix(matcher, "toBeTrue") || strings.HasPrefix(matcher, "toBeFalse"):
		framework = "jasmine"
	}
	x.add(jsTuple(at, framework, subject, name, negated, args))
	return true
}

// jsTuple builds one JS/TS tuple from the matcher name and its rendered arguments.
func jsTuple(at, framework, subject, matcher string, negated bool, args []string) AssertionTuple {
	lower := strings.ToLower(matcher)
	exact := false
	expected := ""
	switch {
	case assertFixedValues[lower] != "":
		expected = assertFixedValues[lower]
	case (lower == "tohaveattribute" || lower == "have.attr") && len(args) >= 2:
		expected = normaliseLiteral(args[1])
	case len(args) > 0 && args[0] != "":
		expected = normaliseLiteral(args[0])
	default:
		expected = subjectKey(subject)
	}
	for _, a := range args[min(1, len(args)):] {
		if strings.Contains(strings.ReplaceAll(a, " ", ""), "exact:true") {
			exact = true
		}
	}
	return AssertionTuple{Location: at, Framework: framework, Subject: subject, Matcher: matcher,
		Class: classify(matcher, negated, len(args), exact, false, false), Negated: negated, Expected: expected}
}

// cypressShould reads `SUBJECT.should("CHAINER"[, VALUE])` and any `.and(...)` that follows,
// returning the last token read; a callback form abstains.
func (s *source) cypressShould(x *assertExtract, k int, at string) (int, bool) {
	start := k - 1 // the `.`
	for {
		j := start - 1
		if s.punct(j, ")") && s.match[j] >= 0 {
			j = s.match[j] - 1
		}
		if !s.ident(j, "") {
			break
		}
		start = j
		if !s.punct(j-1, ".") {
			break
		}
		start = j - 1
	}
	subject := s.render(start, k-1)
	end := k
	for member := k; ; {
		open := member + 1
		if !s.punct(open, "(") || s.match[open] < 0 {
			x.abstain(assertReasonFormUnsupported, at)
			return 0, false
		}
		args := s.jsArgs(open)
		if len(args) == 0 || len(args[0]) < 2 || args[0][0] != '"' {
			x.abstain(assertReasonFormUnsupported, at)
			return 0, false
		}
		chainer, _ := strconv.Unquote(args[0])
		negated := strings.HasPrefix(chainer, "not.")
		chainer = strings.TrimPrefix(chainer, "not.")
		x.add(jsTuple(at, "cypress", subject, chainer, negated, args[1:]))
		end = s.match[open]
		if !s.punct(end+1, ".") || !s.ident(end+2, "and") {
			break
		}
		member = end + 2
	}
	return end, true
}
