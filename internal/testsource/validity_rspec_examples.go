package testsource

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The RSpec example hygiene and strength facts (TSA-V0-062..067). They reuse the TSA-V0-048
// reader, so they abstain exactly when it does.

var (
	rspecDeclarator = regexp.MustCompile(`^(?:RSpec\.)?(\w+)`)
	// rspecMetadataSymbol matches `:skip`, `:pending` or `:focus` metadata after the description.
	rspecMetadataSymbol = regexp.MustCompile(`[,(\s]:(skip|pending|focus)\b`)
	// rspecMetadataKey matches `skip: VALUE`, `pending: VALUE` or `focus: VALUE`; group 2 is VALUE.
	rspecMetadataKey = regexp.MustCompile(`[,(\s](skip|pending|focus):\s*(\w*)`)
	// rspecSkipStatement is a skip or pending call statement in an example body.
	rspecSkipStatement = regexp.MustCompile(`^(?:skip|pending)\b`)
	rspecConditional   = regexp.MustCompile(`\s(?:if|unless)\s`)
	rspecBlockStart    = regexp.MustCompile(`\{|\bdo\b`)
	// rspecExpectation counts one RSpec expectation: expect(...), expect { }, expect do,
	// is_expected, should, should_not and expect_any_instance_of.
	rspecExpectation = regexp.MustCompile(`\bexpect\s*(?:\(|\{|do\b)|\bis_expected\b|\bshould(?:_not)?\b|\bexpect_any_instance_of\b`)
	// rspecDescribedClassStub matches a message stub or spy on described_class.
	rspecDescribedClassStub = regexp.MustCompile(`\b(?:allow|expect)(?:_any_instance_of)?\s*\(\s*described_class\s*\)\s*\.\s*(?:to|not_to|to_not)\b\s*\(?\s*(?:receive\w*|have_received)\b`)
)

// RSpecExampleSources reads one `_spec.rb` file's examples (TSA-V0-062). It returns each example's
// source facts keyed by its declaring line, and the reader's uncertainty reason; when the reason is
// not "" the map is empty. The findings are the ValidityFindings that name the example followed by
// TSA-V0-063..066 in rule order. Any other file, or a file over MaxFileBytes, is uncertain.
func RSpecExampleSources(name, text string) (map[int]testvalidity.ExampleSource, string) {
	out := map[int]testvalidity.ExampleSource{}
	if !strings.HasSuffix(name, "_spec.rb") {
		return out, "not-an-rspec-file"
	}
	base := ValidityFindings(name, text)
	if base.Uncertain {
		return out, base.Reason
	}
	r := &rspecReader{lines: rubyLines(text), groups: []rspecGroup{{parent: -1, decls: map[string]string{}}}}
	if reason := r.read(); reason != "" {
		return out, reason
	}
	byTest := map[string][]testvalidity.SourceFinding{}
	for _, f := range base.Findings {
		byTest[f.Test] = append(byTest[f.Test], f)
	}
	for _, e := range r.examples {
		test := fmt.Sprintf("%s:%d", name, e.line)
		findings := append([]testvalidity.SourceFinding{}, byTest[test]...)
		findings = append(findings, r.exampleFindings(name, test, e)...)
		count, anchors := expectations(name, e.body)
		out[e.line] = testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: count, Findings: findings, ExpectationAnchors: anchors}
	}
	return out, ""
}

// exampleFindings returns the TSA-V0-063..066 findings of one example: at most one finding per
// rule, the example's own declaration first and then the innermost group.
func (r *rspecReader) exampleFindings(name, test string, e rspecExample) []testvalidity.SourceFinding {
	at := func(line int) string { return fmt.Sprintf("%s:%d", name, line) }
	finding := func(rule, severity, location, cause, message string) testvalidity.SourceFinding {
		return testvalidity.SourceFinding{Rule: rule, Severity: severity, Test: test, Title: e.title, Location: location, Cause: cause, Message: message}
	}
	out := []testvalidity.SourceFinding{}
	skipped, focused, shared := 0, 0, 0
	head := e.body[0].text
	switch declarator(head) {
	case "xit", "xspecify", "xexample", "xscenario", "skip", "pending":
		skipped = e.line
	case "fit", "fspecify", "fexample", "fscenario", "focus":
		focused = e.line
	}
	s, f := metadata(head)
	skipped, focused = pick(skipped, s, e.line), pick(focused, f, e.line)
	if skipped == 0 {
		skipped = skipStatement(e.body)
	}
	for g := e.group; g > 0; g = r.groups[g].parent {
		group := r.groups[g]
		word := declarator(group.head)
		s, f := metadata(group.head)
		skipped = pick(skipped, s || word == "xdescribe" || word == "xcontext", group.line)
		focused = pick(focused, f || word == "fdescribe" || word == "fcontext", group.line)
		if shared == 0 && (word == "shared_examples" || word == "shared_examples_for" || word == "shared_context") {
			shared = group.line
		}
	}
	if skipped != 0 {
		out = append(out, finding(testvalidity.RuleRSpecSkipped, testvalidity.SeverityInvalid, at(e.line), at(skipped),
			"the example is skipped or pending, so its result proves nothing about the code; remove the skip, pending, x-prefix or skip metadata"))
	}
	if focused != 0 {
		out = append(out, finding(testvalidity.RuleRSpecFocused, testvalidity.SeverityInvalid, at(e.line), at(focused),
			"the example or a group around it is focused, so a run filters the suite to it; remove the f-prefix or focus metadata"))
	}
	if shared != 0 {
		out = append(out, finding(testvalidity.RuleSharedExampleOnly, testvalidity.SeverityInvalid, at(e.line), at(shared),
			"the example is declared in a shared example group, so its assertion runs only through an including group the cited file:line does not name; cite an example in the including spec"))
	}
	if location, cause, ok := r.describedClassStub(name, e); ok {
		out = append(out, finding(testvalidity.RuleDescribedClassStubbed, testvalidity.SeverityAdvisory, location, cause,
			"described_class is stubbed, so an expectation can observe the stub instead of the unit under test"))
	}
	return out
}

func pick(current int, hit bool, line int) int {
	if current == 0 && hit {
		return line
	}
	return current
}

// declarator returns a group or example line's first word, after any `RSpec.` receiver.
func declarator(head string) string {
	if m := rspecDeclarator.FindStringSubmatch(head); m != nil {
		return m[1]
	}
	return ""
}

// metadata reports skip-or-pending and focus metadata on a declaring line outside string
// literals and before its block opens: a symbol, or a key whose value is not false or nil.
func metadata(head string) (skip, focus bool) {
	text := stripRubyStrings(head)
	if at := rspecBlockStart.FindStringIndex(text); at != nil {
		text = text[:at[0]]
	}
	set := func(key string) {
		if key == "focus" {
			focus = true
		} else {
			skip = true
		}
	}
	for _, m := range rspecMetadataSymbol.FindAllStringSubmatch(text, -1) {
		set(m[1])
	}
	for _, m := range rspecMetadataKey.FindAllStringSubmatch(text, -1) {
		if m[2] != "false" && m[2] != "nil" {
			set(m[1])
		}
	}
	return skip, focus
}

// skipStatement returns the line of the first unconditional skip or pending statement directly in
// an example body, outside any nested block, or 0.
func skipStatement(body []rubyLine) int {
	depth := 0
	for k := 1; k < len(body); k++ {
		text := body[k].text
		if rubyEnd.MatchString(text) {
			depth--
			continue
		}
		if depth == 0 && rspecSkipStatement.MatchString(text) && !rspecConditional.MatchString(stripRubyStrings(text)) {
			return body[k].line
		}
		if rubyOpens(text) {
			depth++
		}
	}
	return 0
}

// expectations counts the expectations in an example's lines outside string literals and returns
// their distinct `path:line` locations (TSA-V0-067).
func expectations(name string, body []rubyLine) (int, []string) {
	count, anchors := 0, []string{}
	for _, l := range body {
		n := len(rspecExpectation.FindAllStringIndex(stripRubyStrings(l.text), -1))
		if n == 0 {
			continue
		}
		count += n
		anchors = append(anchors, fmt.Sprintf("%s:%d", name, l.line))
	}
	return count, anchors
}

// describedClassStub returns the first described_class stub in the example body, caused by the
// example, or else in a before, let! or subject! hook on its group chain, innermost group first,
// caused by the hook.
func (r *rspecReader) describedClassStub(name string, e rspecExample) (string, string, bool) {
	at := func(line int) string { return fmt.Sprintf("%s:%d", name, line) }
	if line := stubLine(e.body); line != 0 {
		return at(line), at(e.line), true
	}
	for g := e.group; g >= 0; g = r.groups[g].parent {
		for _, h := range r.groups[g].hooks {
			if line := stubLine(h.body); line != 0 {
				return at(line), at(h.line), true
			}
		}
	}
	return "", "", false
}

func stubLine(lines []rubyLine) int {
	for _, l := range lines {
		if rspecDescribedClassStub.MatchString(stripRubyStrings(l.text)) {
			return l.line
		}
	}
	return 0
}
