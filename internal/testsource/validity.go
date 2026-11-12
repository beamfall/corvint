package testsource

import (
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testdiscovery"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// validitySourceTooLarge is testdiscovery.ValiditySourceTooLarge, kept under its old name.
const validitySourceTooLarge = testdiscovery.ValiditySourceTooLarge

// validitySourceUnparsed is testdiscovery.ValiditySourceUnparsed, kept under its old name.
const validitySourceUnparsed = testdiscovery.ValiditySourceUnparsed

// validitySerialNotLiteral is testdiscovery.ValiditySerialNotLiteral, kept under its old name.
const validitySerialNotLiteral = testdiscovery.ValiditySerialNotLiteral

// validityRSpecUnbalanced is testdiscovery.ValidityRSpecUnbalanced, kept under its old name.
const validityRSpecUnbalanced = testdiscovery.ValidityRSpecUnbalanced

// validityRSpecTooDeep is testdiscovery.ValidityRSpecTooDeep, kept under its old name.
const validityRSpecTooDeep = testdiscovery.ValidityRSpecTooDeep

// ValidityReport is the static test-validity reading of one committed test file: the source rule
// findings and, when the reader could not place some of the file, Uncertain with the first reason.
// An uncertain report may be missing findings; it never invents one.
type ValidityReport struct {
	Findings  []testvalidity.SourceFinding `json:"findings"`
	Uncertain bool                         `json:"uncertain"`
	Reason    string                       `json:"reason,omitempty"`
	// Abstentions are the advisory readings withheld because what decides them is declared in
	// another file (TSA-V0-300).
	Abstentions []testvalidity.RuleAbstention `json:"abstentions,omitempty"`
	// Exemptions are the readings a justified annotation exempted, each with the annotation's
	// reason (TSA-V0-870).
	Exemptions []testvalidity.RuleExemption `json:"exemptions,omitempty"`
}

// ValidityFindings reads one test file's text for the static test-validity rules: a JS/TS test
// file (IsTestFile) for ISOLATED-IN-SERIAL (TSA-V0-045..047) and an RSpec `_spec.rb` file for
// MEMOIZED-BLOCK-EXPECTATION (TSA-V0-048..050) and BLOCK-NOT-CALLED (TSA-V0-053..055). Any other
// file has no applicable rule. A JS/TS test file is also read for the outcome, cleanup, input and
// vacuity rules (TSA-V0-160..176), another JS/TS file (a fixture file) for CLEANUP-HANDLE-OUTLIVED,
// and a behaviour contract (Markdown, YAML or JSON) for EITHER-OR-OUTCOME wording.
func ValidityFindings(name, text string) ValidityReport {
	report := ValidityReport{Findings: []testvalidity.SourceFinding{}}
	if len(text) > MaxFileBytes {
		report.Uncertain, report.Reason = true, validitySourceTooLarge
		return report
	}
	switch {
	case strings.HasSuffix(name, "_spec.rb"):
		return rspecValidity(name, text)
	case IsTestFile(name):
		s := parseSource(name, text)
		report = isolatedInSerial(s)
		report.Findings = append(report.Findings, outcomeFindings(s)...)
		report.Abstentions = s.Abstentions
		report.Exemptions = s.Exemptions
	case webExtensions[path.Ext(name)]:
		s := parseSource(name, text)
		report.Findings = append(report.Findings, chFindings(s)...)
		report.Abstentions = s.Abstentions
		if s.Uncertain {
			report.Uncertain, report.Reason = true, validitySourceUnparsed
		}
	case eoIsContract(name):
		findings, exemptions := eoContractFindings(name, text)
		report.Findings = append(report.Findings, findings...)
		report.Exemptions = exemptions
	}
	return report
}

// outcomeFindings reads one test file for EITHER-OR-OUTCOME, CLEANUP-HANDLE-OUTLIVED,
// UNSTUBBED-OUTBOUND-INPUT and VACUOUS-EMPTY-EQUALITY, in that order.
func outcomeFindings(s *source) []testvalidity.SourceFinding {
	out := eoFindings(s)
	out = append(out, chFindings(s)...)
	out = append(out, uoFindings(s)...)
	return append(out, veFindings(s)...)
}

// isolatedInSerial reports every test carrying an isolation annotation under a serial-mode
// ancestor (TSA-V0-045). A serial ancestor is a `describe.serial` declaration, a describe whose
// own body calls `test.describe.configure({ mode: 'serial' })`, or such a call outside every
// describe, which makes the whole file serial. The outermost serial ancestor is named.
func isolatedInSerial(s *source) ValidityReport {
	report := ValidityReport{Findings: []testvalidity.SourceFinding{}}
	if s.Uncertain {
		report.Uncertain, report.Reason = true, validitySourceUnparsed
	}
	serialAt := map[int]bool{} // describe token index -> serial
	fileSerial := 0            // line of a file-level serial configure, 0 when none
	for i := range s.Tokens {
		mode, ok := configureMode(s, i)
		if !ok {
			continue
		}
		if mode == "" {
			if !report.Uncertain {
				report.Uncertain, report.Reason = true, validitySerialNotLiteral
			}
			continue
		}
		if mode != "serial" {
			continue
		}
		if owner := innermostDescribe(s, i); owner >= 0 {
			serialAt[owner] = true
		} else if fileSerial == 0 {
			fileSerial = s.Tokens[i].Line
		}
	}
	for _, d := range s.Decls {
		if d.Kind == "describe" && d.Modifiers["serial"] {
			serialAt[d.At] = true
		}
	}
	for _, t := range s.Decls {
		if t.Kind != "test" {
			continue
		}
		isolated := sourceIsolated(s, t)
		ancestor := 0
		if fileSerial > 0 {
			ancestor = fileSerial
		}
		for _, d := range s.Decls {
			if d.Kind != "describe" || d.At >= t.At || !within(t.At, d.Body) {
				continue
			}
			isolated = isolated || sourceIsolated(s, d)
			if ancestor == 0 && serialAt[d.At] {
				ancestor = d.Line
			}
		}
		if !isolated || ancestor == 0 {
			continue
		}
		where := fmt.Sprintf("%s:%d", s.Path, t.Line)
		cause := fmt.Sprintf("%s:%d", s.Path, ancestor)
		report.Findings = append(report.Findings, testvalidity.SourceFinding{
			Rule: testvalidity.RuleIsolatedInSerial, Severity: testvalidity.SeverityAdvisory,
			Test: where, Title: t.Title, Location: where, Cause: cause,
			Message: fmt.Sprintf("isolated test runs under the serial suite at %s, so a failure of an earlier serial test skips it and a targeted run proves nothing; move the test out of the serial suite, or drop serial mode if the suite's tests are independent", cause),
		})
	}
	return report
}

// configureMode reads `test.describe.configure({ ... mode: X ... })` starting at token i: ok is
// false when token i does not start that call, and mode is X's literal value, or empty when the
// call names a mode that is not a string literal. A call without a mode key returns "default".
func configureMode(s *source, i int) (mode string, ok bool) {
	if !s.Ident(i, "test") || s.MemberAccess(i) || !s.Punct(i+1, ".") || !s.Ident(i+2, "describe") ||
		!s.Punct(i+3, ".") || !s.Ident(i+4, "configure") || !s.Punct(i+5, "(") || s.Match[i+5] < 0 {
		return "", false
	}
	for j := i + 6; j < s.Match[i+5]; j++ {
		if !sourceKey(s, j, "mode") {
			continue
		}
		if s.LoneString([2]int{j + 2, j + 3}) && s.Tokens[j+2].Literal && (s.Punct(j+3, ",") || s.Punct(j+3, "}")) {
			return s.Tokens[j+2].Text, true
		}
		return "", true
	}
	return "default", true
}

// innermostDescribe returns the token index of the innermost describe whose body holds token i,
// or -1 when none does.
func innermostDescribe(s *source, i int) int {
	owner := -1
	for _, d := range s.Decls {
		if d.Kind == "describe" && within(i, d.Body) && d.At > owner {
			owner = d.At
		}
	}
	return owner
}

// isolated reports a declaration annotated as isolated: an `@isolated` tag in its title or in its
// details' `tag` value, or a details annotation whose `type` is the literal `isolated`
// (TSA-V0-045). Other details text, such as an annotation's description, is not a tag.
func sourceIsolated(s *source, d decl) bool {
	if d.Titled && hasIsolatedTag(d.Title) {
		return true
	}
	if len(d.Args) < 3 {
		return false
	}
	for _, arg := range d.Args[1 : len(d.Args)-1] {
		for j := arg[0]; j < arg[1]; j++ {
			switch {
			case sourceKey(s, j, "tag") && s.Punct(j+2, "[") && s.Match[j+2] > j+2:
				for k := j + 3; k < s.Match[j+2]; k++ {
					if literalString(s, k, "@isolated") {
						return true
					}
				}
			case sourceKey(s, j, "tag") && literalString(s, j+2, "@isolated"),
				sourceKey(s, j, "type") && literalString(s, j+2, "isolated"):
				return true
			}
		}
	}
	return false
}

// key reports an object key named name at token i, bare or as a string literal, followed by `:`.
func sourceKey(s *source, i int, name string) bool {
	quoted := i >= 0 && i < len(s.Tokens) && s.Tokens[i].Kind == appmap.JSString && s.Tokens[i].Literal && s.Tokens[i].Text == name
	return (s.Ident(i, name) || quoted) && s.Punct(i+1, ":")
}

// literalString reports a string literal token at i whose text is text, ignoring case.
func literalString(s *source, i int, text string) bool {
	return i >= 0 && i < len(s.Tokens) && s.Tokens[i].Kind == appmap.JSString && s.Tokens[i].Literal &&
		strings.EqualFold(s.Tokens[i].Text, text)
}

// hasIsolatedTag reports an `@isolated` word in a test title.
func hasIsolatedTag(title string) bool {
	for _, word := range strings.Fields(title) {
		if strings.EqualFold(word, "@isolated") {
			return true
		}
	}
	return false
}
