package testsource

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/testdiscovery"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The RSpec reader behind MEMOIZED-BLOCK-EXPECTATION (TSA-V0-048..050) and BLOCK-NOT-CALLED
// (TSA-V0-053..056). It is not a Ruby parser: it reads logical lines (rubyLines) for example-group
// nesting, the subject/let/let!/before declarations and the `def` helpers of each group and the
// block expectations of each example, and abstains on anything it cannot balance.

// maxRSpecDepth is testdiscovery.MaxRSpecDepth, kept under its old name.
const maxRSpecDepth = testdiscovery.MaxRSpecDepth

// rspecBlockExpr is testdiscovery.RSpecBlockExpr, kept under its old name.
var rspecBlockExpr = testdiscovery.RSpecBlockExpr

// rspecBlockOpen is testdiscovery.RSpecBlockOpen, kept under its old name.
var rspecBlockOpen = testdiscovery.RSpecBlockOpen

// rspecLoneName is testdiscovery.RSpecLoneName, kept under its old name.
var rspecLoneName = testdiscovery.RSpecLoneName

// rspecBlockClose is testdiscovery.RSpecBlockClose, kept under its old name.
var rspecBlockClose = testdiscovery.RSpecBlockClose

// rspecPercent is testdiscovery.RSpecPercent, kept under its old name.
var rspecPercent = testdiscovery.RSpecPercent

// rspecIdentifier is testdiscovery.RSpecIdentifier, kept under its old name.
var rspecIdentifier = testdiscovery.RSpecIdentifier

// rubyKeywords is testdiscovery.RubyKeywords, kept under its old name.
var rubyKeywords = testdiscovery.RubyKeywords

// rspecGroup is testdiscovery.RSpecGroup, kept under its old name.
type rspecGroup = testdiscovery.RSpecGroup

// rspecHook is testdiscovery.RSpecHook, kept under its old name.
type rspecHook = testdiscovery.RSpecHook

// rspecExample is testdiscovery.RSpecExample, kept under its old name.
type rspecExample = testdiscovery.RSpecExample

// rspecReader is testdiscovery.RSpecReader, kept under its old name.
type rspecReader = testdiscovery.RSpecReader

// rspecValidity reports every `expect { NAME }` block expectation whose NAME is the subject or a
// let that a memoizing hook of the example's own or an enclosing group already invoked
// (TSA-V0-048, TSA-V0-049), then every BLOCK-NOT-CALLED helper and the examples that call it
// (TSA-V0-053..055). It abstains, with no findings, when the file's blocks do not balance or nest
// too deeply (TSA-V0-050).
func rspecValidity(name, text string) ValidityReport {
	report := ValidityReport{Findings: []testvalidity.SourceFinding{}}
	r := newRSpecReader(name, text)
	if reason := r.Read(); reason != "" {
		report.Uncertain, report.Reason = true, reason
		return report
	}
	for _, e := range r.Examples {
		report.Findings = append(report.Findings, memoizedFindings(r, e)...)
	}
	report.Findings = append(report.Findings, blockNotCalled(r)...)
	return report
}

// newRSpecReader is testdiscovery.NewRSpecReader, kept under its old name.
func newRSpecReader(path, text string) *rspecReader { return testdiscovery.NewRSpecReader(path, text) }

// rspecIncludeLine is testdiscovery.RSpecIncludeLine, kept under its old name.
var rspecIncludeLine = testdiscovery.RSpecIncludeLine

// rspecInclude is testdiscovery.RSpecInclude, kept under its old name.
type rspecInclude = testdiscovery.RSpecInclude

// sharedName is testdiscovery.SharedName, kept under its old name.
func sharedName(args string) (string, bool) { return testdiscovery.SharedName(args) }

// memoizedFindings returns the MEMOIZED-BLOCK-EXPECTATION findings of one example (TSA-V0-048).
func memoizedFindings(r *rspecReader, e rspecExample) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	for _, target := range blockExpectations(e.Body) {
		canonical := rspecReaderCanonical(r, e.Group, target.name)
		if canonical == "" {
			continue
		}
		hook, kind, ok := memoizingHook(r, e.Group, canonical)
		if !ok {
			continue
		}
		test := fmt.Sprintf("%s:%d", e.Path, e.Line)
		cause := fmt.Sprintf("%s:%d", hook.Path, hook.Line)
		out = append(out, testvalidity.SourceFinding{
			Rule: testvalidity.RuleMemoizedBlockExpectation, Severity: testvalidity.SeverityInvalid,
			Test: test, Title: e.Title, Location: fmt.Sprintf("%s:%d", e.Path, target.line), Cause: cause,
			Message: fmt.Sprintf("expect { %s } re-reads a value the %s at %s already memoized, so the block runs nothing and a change, raise_error or have_received expectation on it is vacuous; call the action inside the block (for example expect { %s.perform }) or stop invoking %s in that hook", target.name, kind, cause, target.name, target.name),
		})
	}
	return out
}

// stripRubyStrings is testdiscovery.StripRubyStrings, kept under its old name.
func stripRubyStrings(text string) string { return testdiscovery.StripRubyStrings(text) }

// invokedNames is testdiscovery.InvokedNames, kept under its old name.
func invokedNames(body []rubyLine) map[string]bool { return testdiscovery.InvokedNames(body) }

type blockTarget struct {
	line int
	name string
}

// blockExpectations returns each `expect { NAME }` or multi-line `expect do NAME end` block
// expectation in an example's lines whose whole block is one bare name.
func blockExpectations(body []rubyLine) []blockTarget {
	out := []blockTarget{}
	for k, l := range body {
		for _, m := range rspecBlockExpr.FindAllStringSubmatch(stripRubyStrings(l.Text), -1) {
			out = append(out, blockTarget{line: l.Line, name: m[1]})
		}
		if rspecBlockOpen.MatchString(l.Text) && k+2 < len(body) && rspecBlockClose.MatchString(body[k+2].Text) {
			if m := rspecLoneName.FindStringSubmatch(body[k+1].Text); m != nil {
				out = append(out, blockTarget{line: l.Line, name: m[1]})
			}
		}
	}
	return out
}

// canonical resolves name from group g, innermost declaration first: "let:NAME" for a let or a
// named subject, and for `subject` when the innermost subject is named NAME; "subject" for an
// anonymous or undeclared `subject`; "" for a name no group in the chain declares, which is not a
// memoized helper. Distinct named subjects stay distinct helpers.
func rspecReaderCanonical(r *rspecReader, g int, name string) string {
	if rubyKeywords[name] {
		return ""
	}
	if d := declaring(r, g, name); d >= 0 {
		switch v := r.Groups[d].Decls[name]; v {
		case "let":
			return "let:" + name
		default:
			return v
		}
	}
	if name == "subject" {
		return "subject"
	}
	return ""
}

// declaring returns the innermost group on g's chain that declares name, or -1.
func declaring(r *rspecReader, g int, name string) int {
	for ; g >= 0; g = r.Groups[g].Parent {
		if r.Groups[g].Decls[name] != "" {
			return g
		}
	}
	return -1
}

// memoizingHook returns the first hook, innermost group first, then in source order, of group g
// or an enclosing group that invokes the canonical helper, and its kind. A hook in a sibling group
// is never on the chain.
func memoizingHook(r *rspecReader, g int, canonical string) (rspecHook, string, bool) {
	for start := g; g >= 0; g = r.Groups[g].Parent {
		for _, h := range r.Groups[g].Hooks {
			if h.Own != "" {
				if rspecReaderCanonical(r, start, h.Own) == canonical {
					return h, h.Kind, true
				}
				if d := declaring(r, start, h.Own); d >= 0 && d != g {
					continue // the helper is overridden below this hook's group; its body never runs
				}
			}
			for name := range h.Names {
				if rspecReaderCanonical(r, start, name) == canonical {
					return h, h.Kind, true
				}
			}
		}
	}
	return rspecHook{}, "", false
}
