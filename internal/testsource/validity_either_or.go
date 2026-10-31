package testsource

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// EITHER-OR-OUTCOME reads (TSA-V0-160..164): one assertion that accepts any of several distinct
// literal outcomes, so a regression from one outcome to another still passes.

// eoFindings reports every either-or assertion inside a JS/TS test body (TSA-V0-160, TSA-V0-161).
// An assertion outside every test, or one carrying a valid justification annotation, is not
// reported (TSA-V0-163).
func (s *source) eoFindings() []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	for i := range s.tokens {
		values, form := s.eoAssertion(i)
		if len(values) < 2 {
			continue
		}
		test, ok := s.eoEnclosingTest(i)
		if !ok {
			continue
		}
		line := s.tokens[i].Line
		where := fmt.Sprintf("%s:%d", s.path, line)
		if reason, ok := eoAllowed(s.comments, s.eoCodeLines(), line); ok {
			s.exemptions = append(s.exemptions, testvalidity.RuleExemption{Rule: testvalidity.RuleEitherOrOutcome, Location: where, Reason: reason})
			continue
		}
		out = append(out, testvalidity.SourceFinding{
			Rule: testvalidity.RuleEitherOrOutcome, Severity: testvalidity.SeverityInvalid,
			Test: fmt.Sprintf("%s:%d", s.path, test.line), Title: test.title, Location: where, Cause: where,
			Message: eoMessage(form, values),
		})
	}
	return out
}

func eoMessage(form string, values []string) string {
	return fmt.Sprintf("%s accepts any of %s, so it passes whichever of them the product returns and cannot detect a regression from one to another; assert the one correct value, or annotate the line with `%s %s: <reason>` when the outcome is genuinely non-deterministic",
		form, strings.Join(values, ", "), testvalidity.ValidityAllowMarker, testvalidity.RuleEitherOrOutcome)
}

// eoAssertion reads the assertion starting at token i and returns the distinct literal outcomes
// it accepts with the form's name, or nil when it is not an either-or form (TSA-V0-161).
func (s *source) eoAssertion(i int) ([]string, string) {
	if c, ok := s.readExpect(i); ok {
		return s.eoExpect(c)
	}
	if !s.ident(i, "assert") || s.memberAccess(i) {
		return nil, ""
	}
	member, open := "", i+1
	if s.punct(i+1, ".") && s.ident(i+2, "") {
		member, open = s.tokens[i+2].Text, i+3
	}
	if !s.punct(open, "(") || s.match[open] < 0 {
		return nil, ""
	}
	args := s.splitArgs(open, s.match[open])
	switch {
	case (member == "" || member == "ok" || member == "isTrue") && len(args) >= 1:
		return s.eoTruthySubject(args[0])
	case member == "oneOf" && len(args) >= 2:
		return s.eoArray(args[1]), "an assert.oneOf set"
	case (member == "include" || member == "includes") && len(args) >= 2:
		return s.eoArray(args[0]), "an assert.include on a literal array"
	case member == "match" && len(args) >= 2 && args[1][1]-args[1][0] == 1:
		return eoRegex(s.tokens[args[1][0]]), "a regular expression alternation"
	}
	return nil, ""
}

// eoExpect reads one expect chain: toContain on a literal array, `[...].includes(x)` or `||` of
// equalities asserted true, a oneOf matcher, or toMatch with an alternation. A negated chain
// accepts everything but the set, so it is never either-or.
func (s *source) eoExpect(c expectChain) ([]string, string) {
	if c.matcher == "" || s.eoNegated(c) {
		return nil, ""
	}
	margs := s.splitArgs(c.margs[0]-1, c.margs[1])
	switch c.matcher {
	case "toContain", "toContainEqual":
		return s.eoArray(c.subject), "toContain on a literal array"
	case "oneOf", "toBeOneOf":
		if len(margs) == 1 {
			return s.eoArray(margs[0]), "a oneOf matcher"
		}
	case "toMatch":
		if len(margs) == 1 && margs[0][1]-margs[0][0] == 1 {
			return eoRegex(s.tokens[margs[0][0]]), "a regular expression alternation"
		}
	case "toBeTruthy":
		return s.eoTruthySubject(c.subject)
	case "toBe", "toEqual", "toStrictEqual":
		if len(margs) == 1 && margs[0][1]-margs[0][0] == 1 && s.ident(margs[0][0], "true") {
			return s.eoTruthySubject(c.subject)
		}
	}
	return nil, ""
}

// eoNegated reports a `.not` member between the expect call and the matcher.
func (s *source) eoNegated(c expectChain) bool {
	end := c.end
	if c.margs[0] > 0 {
		end = c.margs[0]
	}
	for j := c.subject[1] + 1; j < end; j++ {
		if s.ident(j, "not") && s.memberAccess(j) {
			return true
		}
	}
	return false
}

// eoTruthySubject reads a subject asserted true: `[a, b].includes(x)` or `x === a || x === b`.
func (s *source) eoTruthySubject(r [2]int) ([]string, string) {
	if s.punct(r[0], "[") && s.match[r[0]] > 0 {
		end := s.match[r[0]]
		if s.punct(end+1, ".") && s.ident(end+2, "includes") && s.punct(end+3, "(") && s.match[end+3] == r[1]-1 {
			return s.eoArray([2]int{r[0], end + 1}), "includes on a literal array"
		}
		return nil, ""
	}
	parts := s.eoSplitOperator(s.eoUnwrap(r), "||")
	if len(parts) < 2 {
		return nil, ""
	}
	operand, values, seen := "", []string{}, map[string]bool{}
	for _, part := range parts {
		part = s.eoUnwrap(part)
		sides := s.eoSplitOperator(part, "===")
		if len(sides) != 2 {
			sides = s.eoSplitOperator(part, "==")
		}
		if len(sides) != 2 {
			return nil, ""
		}
		value, other, ok := s.eoLiteralSide(sides[0], sides[1])
		if !ok {
			return nil, ""
		}
		rendered := s.render(other[0], other[1])
		if operand != "" && rendered != operand {
			return nil, ""
		}
		operand = rendered
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	return values, "`||` of equalities on one operand"
}

// eoUnwrap strips parentheses that enclose the whole range.
func (s *source) eoUnwrap(r [2]int) [2]int {
	for r[1]-r[0] > 2 && s.punct(r[0], "(") && s.match[r[0]] == r[1]-1 {
		r = [2]int{r[0] + 1, r[1] - 1}
	}
	return r
}

// eoLiteralSide returns the literal of a comparison and the other side's range, when exactly one
// side is a single literal.
func (s *source) eoLiteralSide(a, b [2]int) (string, [2]int, bool) {
	if v, ok := s.eoScalar(a); ok && b[1] > b[0] {
		if _, both := s.eoScalar(b); !both {
			return v, b, true
		}
	}
	if v, ok := s.eoScalar(b); ok && a[1] > a[0] {
		if _, both := s.eoScalar(a); !both {
			return v, a, true
		}
	}
	return "", [2]int{}, false
}

// eoScalar reads a range holding one number or exact string literal.
func (s *source) eoScalar(r [2]int) (string, bool) {
	if r[1]-r[0] != 1 {
		return "", false
	}
	t := s.tokens[r[0]]
	switch {
	case t.Kind == appmap.JSNumber:
		// Compare numbers by value, so `200` and `200.0` are one outcome.
		if n, err := strconv.ParseFloat(t.Text, 64); err == nil {
			return strconv.FormatFloat(n, 'g', -1, 64), true
		}
		return t.Text, true
	case (t.Kind == appmap.JSString || t.Kind == appmap.JSTemplate) && t.Literal:
		return fmt.Sprintf("%q", t.Text), true
	}
	return "", false
}

// eoArray reads a range that is exactly one array literal of scalar literals and returns its
// distinct values, or nil when any element is not a literal.
func (s *source) eoArray(r [2]int) []string {
	if !s.punct(r[0], "[") || s.match[r[0]] != r[1]-1 {
		return nil
	}
	values, seen := []string{}, map[string]bool{}
	for _, element := range s.splitArgs(r[0], r[1]-1) {
		v, ok := s.eoScalar(element)
		if !ok {
			return nil
		}
		if !seen[v] {
			seen[v] = true
			values = append(values, v)
		}
	}
	return values
}

// eoSplitOperator splits a range at every top-level occurrence of a punctuation operator written
// as consecutive glued single-character tokens (`||`, `===`, `==`). A longer operator that starts
// with op (`===` for `==`) is not a split point.
func (s *source) eoSplitOperator(r [2]int, op string) [][2]int {
	parts, start := [][2]int{}, r[0]
	for i := r[0]; i < r[1]; i++ {
		if s.match[i] > i {
			i = s.match[i]
			continue
		}
		run, n := s.eoOperatorRun(i, r[1])
		if n == 0 {
			continue
		}
		if run == op {
			parts = append(parts, [2]int{start, i})
			start = i + n
		}
		i += n - 1
	}
	return append(parts, [2]int{start, r[1]})
}

// eoOperatorRun reads the operator run of `=|&!<>` punctuation starting at token i.
func (s *source) eoOperatorRun(i, end int) (string, int) {
	n := 0
	var b strings.Builder
	for j := i; j < end && s.tokens[j].Kind == appmap.JSPunct && strings.Contains("=|&!<>", s.tokens[j].Text) && (j == i || s.tokens[j].Glued); j++ {
		b.WriteString(s.tokens[j].Text)
		n++
	}
	return b.String(), n
}

var (
	eoAlternative = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	eoDigits      = regexp.MustCompile(`^[0-9]+$`)
	eoDigitClass  = regexp.MustCompile(`^([0-9]*)\[([0-9]{2,})\]([0-9]*)$`)
)

// eoRegex reads a regular expression literal that accepts a closed set of outcomes: an anchored
// alternation of plain words (`/^(DENIED|FORBIDDEN)$/`), an alternation of digit strings
// (`/401|403/`), or a digit class inside a status code (`/^40[13]$/`) (TSA-V0-161). An unanchored
// alternation of words is a text search and is not read.
func eoRegex(t appmap.JSToken) []string {
	src := t.Source
	if t.Kind != appmap.JSString || t.Literal || !strings.HasPrefix(src, "/") {
		return nil
	}
	end := strings.LastIndex(src, "/")
	if end <= 0 {
		return nil
	}
	body := src[1:end]
	anchored := strings.HasPrefix(body, "^") && strings.HasSuffix(body, "$") && !strings.HasSuffix(body, `\$`)
	if anchored {
		body = body[1 : len(body)-1]
		// `^A|B$` anchors only the first and last alternatives, so it is not a closed set.
		anchored = !eoTopLevelAlternation(body)
	}
	if m := eoDigitClass.FindStringSubmatch(body); m != nil && (anchored || len(m[1])+1+len(m[3]) == 3) {
		values := []string{}
		for _, d := range strings.Split(m[2], "") {
			if v := m[1] + d + m[3]; !eoContains(values, v) {
				values = append(values, v)
			}
		}
		return values
	}
	for _, prefix := range []string{"(?:", "("} {
		if strings.HasPrefix(body, prefix) && strings.HasSuffix(body, ")") && !strings.ContainsAny(body[len(prefix):len(body)-1], "()") {
			body = body[len(prefix) : len(body)-1]
			break
		}
	}
	values, numeric := []string{}, true
	for _, alt := range strings.Split(body, "|") {
		if !eoAlternative.MatchString(alt) {
			return nil
		}
		numeric = numeric && eoDigits.MatchString(alt)
		if !eoContains(values, alt) {
			values = append(values, alt)
		}
	}
	if !anchored && !numeric {
		return nil
	}
	return values
}

// eoTopLevelAlternation reports a `|` outside every group of a regular expression body.
func eoTopLevelAlternation(body string) bool {
	depth := 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

func eoContains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}

// eoEnclosingTest returns the innermost test declaration whose body holds token i.
func (s *source) eoEnclosingTest(i int) (decl, bool) {
	found, ok := decl{}, false
	for _, d := range s.decls {
		if d.kind == "test" && within(i, d.body) && (!ok || d.at > found.at) {
			found, ok = d, true
		}
	}
	return found, ok
}

func (s *source) eoCodeLines() map[int]bool {
	lines := map[int]bool{}
	for _, t := range s.tokens {
		lines[t.Line] = true
	}
	return lines
}

// eoAllowed reports a valid `corvint-validity-allow EITHER-OR-OUTCOME: <reason>` annotation for
// line: a comment on the line itself, or one alone on the line above. A missing reason or another
// rule's name does not exempt the line (TSA-V0-163). The annotation's reason is returned so the
// exemption retains it (TSA-V0-870).
func eoAllowed(comments []appmap.JSComment, codeLines map[int]bool, line int) (string, bool) {
	for _, c := range comments {
		last := c.Line + strings.Count(c.Text, "\n")
		if c.Line != line && !(last == line-1 && !codeLines[c.Line]) {
			continue
		}
		if reason, ok := eoAnnotation(c.Text); ok {
			return reason, true
		}
	}
	return "", false
}

// eoAnnotation returns the reason of text carrying the either-or justification annotation, and
// whether it carries one with a reason.
func eoAnnotation(text string) (string, bool) {
	_, rest, ok := strings.Cut(text, testvalidity.ValidityAllowMarker)
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	rule, reason, ok := strings.Cut(strings.TrimSpace(rest), ":")
	reason = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(reason), "*/"))
	reason = strings.TrimSpace(strings.TrimSuffix(reason, "-->"))
	if !ok || strings.TrimSpace(rule) != testvalidity.RuleEitherOrOutcome || reason == "" || strings.HasPrefix(reason, "-->") {
		return "", false
	}
	return reason, true
}

// eoContractExpected matches one expected-value line of a behaviour contract: a Markdown,
// YAML or JSON key or label whose name starts with `expected` (TSA-V0-162).
var eoContractExpected = regexp.MustCompile(`(?i)^\s*(?:[-*|]\s*)?(?:\*\*)?["']?(expected[A-Za-z _-]*)["']?(?:\*\*)?\s*[:|]\s*(?:\*\*)?\s*(.+?)\s*$`)

var (
	eoContractOr    = regexp.MustCompile(`(?i)\s+or\s+`)
	eoContractSlash = regexp.MustCompile(`\s*/\s*`)
	eoContractEnum  = regexp.MustCompile(`^(?:[1-5][0-9]{2}|[A-Z][A-Z0-9_]*)$`)
)

// eoContractFindings reports every behaviour-contract expected value naming two or more distinct
// outcomes joined by `or` (single words) or `/` (status codes or upper-case enum names), unless
// the line or the line above carries the justification annotation (TSA-V0-162, TSA-V0-163). The
// finding is invalid against the contract line itself. Each exempted line is returned with the
// annotation's reason (TSA-V0-870).
func eoContractFindings(name, text string) ([]testvalidity.SourceFinding, []testvalidity.RuleExemption) {
	out := []testvalidity.SourceFinding{}
	var exempted []testvalidity.RuleExemption
	lines := strings.Split(text, "\n")
	jsonContract, jsonAllowed := strings.HasSuffix(name, ".json"), map[int]string{}
	if jsonContract {
		jsonAllowed = eoJSONAllowed(text)
	}
	for n, raw := range lines {
		m := eoContractExpected.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		values := eoContractValues(eoContractValueText(m[2]))
		if len(values) < 2 {
			continue
		}
		where := fmt.Sprintf("%s:%d", name, n+1)
		reason, allowed := eoAnnotation(raw)
		if !allowed && n > 0 && eoContractComment(lines[n-1]) {
			reason, allowed = eoAnnotation(lines[n-1])
		}
		if !allowed {
			reason, allowed = jsonAllowed[n+1]
		}
		if allowed {
			exempted = append(exempted, testvalidity.RuleExemption{Rule: testvalidity.RuleEitherOrOutcome, Location: where, Reason: reason})
			continue
		}
		message := eoMessage("the contract's expected value", values)
		if jsonContract {
			// JSON has no comment lines, so the message also names the sibling-member form
			// (TSA-V0-310).
			message += fmt.Sprintf("; JSON has no comments, so in a JSON contract annotate instead with a sibling member `\"%s\": {\"%s\": \"%s: <reason>\"}` in the same object",
				eoJSONAllowMember, strings.TrimSpace(m[1]), testvalidity.RuleEitherOrOutcome)
		}
		out = append(out, testvalidity.SourceFinding{
			Rule: testvalidity.RuleEitherOrOutcome, Severity: testvalidity.SeverityInvalid,
			Test: where, Title: strings.TrimSpace(m[1]), Location: where, Cause: where,
			Message: message,
		})
	}
	return out, exempted
}

// eoContractComment reports a contract line that is a comment: YAML `#`, `//`, or a Markdown
// `<!--` comment. Data on the line above, even naming the annotation, does not exempt a value.
func eoContractComment(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "<!--")
}

// eoContractValueText is an expected value without a trailing comment on its line: a YAML ` #`,
// a ` //` or a Markdown `<!--` comment. The comment, which may carry the justification annotation,
// is read separately from the value (TSA-V0-870).
func eoContractValueText(value string) string {
	for _, mark := range []string{"<!--", " #", "\t#", " //", "\t//"} {
		if i := strings.Index(value, mark); i >= 0 {
			value = value[:i]
		}
	}
	return value
}

// eoContractValues splits an expected value into its alternatives when every alternative is one
// outcome token, and returns the distinct ones.
func eoContractValues(value string) []string {
	value = strings.TrimRight(strings.TrimSpace(value), ",|*")
	value = strings.Trim(strings.TrimSpace(value), `"'`)
	parts, slash := eoContractOr.Split(value, -1), false
	if len(parts) < 2 {
		parts, slash = eoContractSlash.Split(value, -1), true
	}
	if len(parts) < 2 {
		return nil
	}
	values := []string{}
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), "`\"'*.")
		if !eoAlternative.MatchString(p) || slash && !eoContractEnum.MatchString(p) {
			return nil
		}
		if !eoContains(values, p) {
			values = append(values, p)
		}
	}
	return values
}

// eoIsContract reports a behaviour-contract file the either-or wording reader applies to.
func eoIsContract(name string) bool {
	for _, ext := range []string{".md", ".yml", ".yaml", ".json"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}
