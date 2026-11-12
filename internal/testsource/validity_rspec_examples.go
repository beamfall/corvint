package testsource

import (
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/testdiscovery"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The RSpec example hygiene and strength facts (TSA-V0-062..067). They reuse the TSA-V0-048
// reader, so they abstain exactly when it does.

// rspecMetadataSymbol is testdiscovery.RSpecMetadataSymbol, kept under its old name.
var rspecMetadataSymbol = testdiscovery.RSpecMetadataSymbol

// rspecMetadataKey is testdiscovery.RSpecMetadataKey, kept under its old name.
var rspecMetadataKey = testdiscovery.RSpecMetadataKey

// rspecSkipStatement is testdiscovery.RSpecSkipStatement, kept under its old name.
var rspecSkipStatement = testdiscovery.RSpecSkipStatement

// rspecConditional is testdiscovery.RSpecConditional, kept under its old name.
var rspecConditional = testdiscovery.RSpecConditional

// rspecBlockStart is testdiscovery.RSpecBlockStart, kept under its old name.
var rspecBlockStart = testdiscovery.RSpecBlockStart

// rspecExpectation is testdiscovery.RSpecExpectation, kept under its old name.
var rspecExpectation = testdiscovery.RSpecExpectation

// rspecDescribedClassStub is testdiscovery.RSpecDescribedClassStub, kept under its old name.
var rspecDescribedClassStub = testdiscovery.RSpecDescribedClassStub

// RSpecExampleSources reads one `_spec.rb` file's examples (TSA-V0-062). It returns each example's
// source facts keyed by its declaring line, and the reader's uncertainty reason; when the reason is
// not "" the map is empty. The findings are the ValidityFindings that name the example followed by
// TSA-V0-063..066 in rule order. Any other file, or a file over MaxFileBytes, is uncertain.
func RSpecExampleSources(name, text string) (map[int]testvalidity.ExampleSource, string) {
	return RSpecExampleSourcesWithSupport(name, text, testvalidity.RubySupport{Unbound: "rspec-support-not-given"})
}

// RSpecExampleSourcesWithSupport is RSpecExampleSources with the bound support files a helper
// call can resolve to (TSA-V0-088).
func RSpecExampleSourcesWithSupport(name, text string, support testvalidity.RubySupport) (map[int]testvalidity.ExampleSource, string) {
	out := map[int]testvalidity.ExampleSource{}
	if !strings.HasSuffix(name, "_spec.rb") {
		return out, "not-an-rspec-file"
	}
	base := ValidityFindings(name, text)
	if base.Uncertain {
		return out, base.Reason
	}
	r := newRSpecReader(name, text)
	if reason := r.Read(); reason != "" {
		return out, reason
	}
	r.Support = newRSpecSupport(support)
	r.Flags = testvalidity.BuiltinFlagLookups()
	if support.Flags != nil {
		r.Flags = *support.Flags
	}
	r.App = support.App
	r.Excluded = rspecExclusions(support.Files)
	byTest := map[string][]testvalidity.SourceFinding{}
	for _, f := range base.Findings {
		byTest[f.Test] = append(byTest[f.Test], f)
	}
	for _, e := range r.Examples {
		test := fmt.Sprintf("%s:%d", name, e.Line)
		findings := append([]testvalidity.SourceFinding{}, byTest[test]...)
		findings = append(findings, exampleFindings(r, test, e)...)
		count, anchors := expectations(e.Path, e.Body)
		out[e.Line] = testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: count, Findings: findings, ExpectationAnchors: anchors, TitlePath: r.TitlePath(e)}
	}
	// The flow findings follow each example's other findings (TSA-V0-120..130), the stub and flag
	// findings follow them (TSA-V0-430), and the never-written assertion targets come last
	// (TSA-V0-443).
	for _, e := range r.Examples {
		test := fmt.Sprintf("%s:%d", name, e.Line)
		source := out[e.Line]
		c := rspecReaderFlowContext(r, name, e)
		source.Findings = append(source.Findings, c.flowFindings(test)...)
		stubs, conditions := c.stubFindings(test)
		source.Findings = append(source.Findings, stubs...)
		source.Findings = append(source.Findings, c.neverWritten(test)...)
		if len(conditions) > 0 {
			source.Conditions = conditions
		}
		out[e.Line] = source
	}
	return out, ""
}

// rspecGroupTitle is testdiscovery.RSpecGroupTitle, kept under its old name.
var rspecGroupTitle = testdiscovery.RSpecGroupTitle

// exampleFindings returns the TSA-V0-063..066 findings of one example: at most one finding per
// rule, the example's own declaration first and then the innermost group.
func exampleFindings(r *rspecReader, test string, e rspecExample) []testvalidity.SourceFinding {
	at := func(line int) string { return fmt.Sprintf("%s:%d", e.Path, line) }
	finding := func(rule, severity, location, cause, message string) testvalidity.SourceFinding {
		return testvalidity.SourceFinding{Rule: rule, Severity: severity, Test: test, Title: e.Title, Location: location, Cause: cause, Message: message}
	}
	out := []testvalidity.SourceFinding{}
	// Each cause is the `path:line` of the declaration that skips, focuses or shares the example;
	// through an inclusion it can sit in another file (TSA-V0-090).
	skipped, focused, shared := "", "", ""
	head := e.Body[0].Text
	switch declarator(head) {
	case "xit", "xspecify", "xexample", "xscenario", "skip", "pending":
		skipped = at(e.Line)
	case "fit", "fspecify", "fexample", "fscenario", "focus":
		focused = at(e.Line)
	}
	s, f := metadata(head)
	skipped, focused = pickCause(skipped, s, at(e.Line)), pickCause(focused, f, at(e.Line))
	if skipped == "" {
		if line := skipStatement(e.Body); line != 0 {
			skipped = at(line)
		}
	}
	for g := e.Group; g > 0; g = r.Groups[g].Parent {
		group := r.Groups[g]
		word := declarator(group.Head)
		s, f := metadata(group.Head)
		cause := fmt.Sprintf("%s:%d", group.Path, group.Line)
		skipped = pickCause(skipped, s || word == "xdescribe" || word == "xcontext", cause)
		focused = pickCause(focused, f || word == "fdescribe" || word == "fcontext", cause)
		if shared == "" && sharedDefinitionWords[word] {
			shared = cause
		}
	}
	if skipped != "" {
		out = append(out, finding(testvalidity.RuleRSpecSkipped, testvalidity.SeverityInvalid, at(e.Line), skipped,
			"the example is skipped or pending, so its result proves nothing about the code; remove the skip, pending, x-prefix or skip metadata"))
	} else {
		out = append(out, skipConfigFindings(r, test, e)...)
	}
	if focused != "" {
		out = append(out, finding(testvalidity.RuleRSpecFocused, testvalidity.SeverityInvalid, at(e.Line), focused,
			"the example or a group around it is focused, so a run filters the suite to it; remove the f-prefix or focus metadata"))
	}
	if shared != "" {
		out = append(out, finding(testvalidity.RuleSharedExampleOnly, testvalidity.SeverityInvalid, at(e.Line), shared,
			"the example is declared in a shared example group, so its assertion runs only through an including group the cited file:line does not name; cite an example in the including spec, or the qualified reference SPEC:INCLUDE_LINE>SHARED:EXAMPLE_LINE (TSA-V0-089)"))
	}
	if location, cause, ok := describedClassStub(r, e); ok {
		out = append(out, finding(testvalidity.RuleDescribedClassStubbed, testvalidity.SeverityAdvisory, location, cause,
			"described_class is stubbed, so an expectation can observe the stub instead of the unit under test"))
	}
	out = append(out, denialFindings(r, test, e)...)
	return append(out, vacuousFindings(r, test, e)...)
}

func pick(current int, hit bool, line int) int {
	if current == 0 && hit {
		return line
	}
	return current
}

func pickCause(current string, hit bool, cause string) string {
	if current == "" && hit {
		return cause
	}
	return current
}

// sharedDefinitionWords is testdiscovery.SharedDefinitionWords, kept under its old name.
var sharedDefinitionWords = testdiscovery.SharedDefinitionWords

// declarator is testdiscovery.Declarator, kept under its old name.
func declarator(head string) string { return testdiscovery.Declarator(head) }

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
		text := body[k].Text
		if rubyEnd.MatchString(text) {
			depth--
			continue
		}
		if depth == 0 && rspecSkipStatement.MatchString(text) && !rspecConditional.MatchString(stripRubyStrings(text)) {
			return body[k].Line
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
		n := len(rspecExpectation.FindAllStringIndex(stripRubyStrings(l.Text), -1))
		if n == 0 {
			continue
		}
		count += n
		anchors = append(anchors, fmt.Sprintf("%s:%d", name, l.Line))
	}
	return count, anchors
}

// describedClassStub returns the first described_class stub in the example body, caused by the
// example, or else in a before, let! or subject! hook on its group chain, innermost group first,
// caused by the hook.
func describedClassStub(r *rspecReader, e rspecExample) (string, string, bool) {
	if line := stubLine(e.Body); line != 0 {
		return fmt.Sprintf("%s:%d", e.Path, line), fmt.Sprintf("%s:%d", e.Path, e.Line), true
	}
	for g := e.Group; g >= 0; g = r.Groups[g].Parent {
		for _, h := range r.Groups[g].Hooks {
			if line := stubLine(h.Body); line != 0 {
				return fmt.Sprintf("%s:%d", h.Path, line), fmt.Sprintf("%s:%d", h.Path, h.Line), true
			}
		}
	}
	return "", "", false
}

func stubLine(lines []rubyLine) int {
	for _, l := range lines {
		if rspecDescribedClassStub.MatchString(stripRubyStrings(l.Text)) {
			return l.Line
		}
	}
	return 0
}
