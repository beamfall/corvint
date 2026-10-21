package testsource

import (
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Uncertainty reasons of a ValidityReport (TSA-V0-046, TSA-V0-050).
const (
	validitySourceTooLarge   = "source-too-large"
	validitySourceUnparsed   = "source-unparsed"
	validitySerialNotLiteral = "serial-mode-not-literal"
	validityRSpecUnbalanced  = "rspec-block-unbalanced"
	validityRSpecTooDeep     = "rspec-depth-exceeded"
)

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
		report = s.isolatedInSerial()
		report.Findings = append(report.Findings, s.outcomeFindings()...)
		report.Abstentions = s.abstentions
	case webExtensions[path.Ext(name)]:
		s := parseSource(name, text)
		report.Findings = append(report.Findings, s.chFindings()...)
		report.Abstentions = s.abstentions
		if s.uncertain {
			report.Uncertain, report.Reason = true, validitySourceUnparsed
		}
	case eoIsContract(name):
		report.Findings = append(report.Findings, eoContractFindings(name, text)...)
	}
	return report
}

// outcomeFindings reads one test file for EITHER-OR-OUTCOME, CLEANUP-HANDLE-OUTLIVED,
// UNSTUBBED-OUTBOUND-INPUT and VACUOUS-EMPTY-EQUALITY, in that order.
func (s *source) outcomeFindings() []testvalidity.SourceFinding {
	out := s.eoFindings()
	out = append(out, s.chFindings()...)
	out = append(out, s.uoFindings()...)
	return append(out, s.veFindings()...)
}

// isolatedInSerial reports every test carrying an isolation annotation under a serial-mode
// ancestor (TSA-V0-045). A serial ancestor is a `describe.serial` declaration, a describe whose
// own body calls `test.describe.configure({ mode: 'serial' })`, or such a call outside every
// describe, which makes the whole file serial. The outermost serial ancestor is named.
func (s *source) isolatedInSerial() ValidityReport {
	report := ValidityReport{Findings: []testvalidity.SourceFinding{}}
	if s.uncertain {
		report.Uncertain, report.Reason = true, validitySourceUnparsed
	}
	serialAt := map[int]bool{} // describe token index -> serial
	fileSerial := 0            // line of a file-level serial configure, 0 when none
	for i := range s.tokens {
		mode, ok := s.configureMode(i)
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
		if owner := s.innermostDescribe(i); owner >= 0 {
			serialAt[owner] = true
		} else if fileSerial == 0 {
			fileSerial = s.tokens[i].Line
		}
	}
	for _, d := range s.decls {
		if d.kind == "describe" && d.modifiers["serial"] {
			serialAt[d.at] = true
		}
	}
	for _, t := range s.decls {
		if t.kind != "test" {
			continue
		}
		isolated := s.isolated(t)
		ancestor := 0
		if fileSerial > 0 {
			ancestor = fileSerial
		}
		for _, d := range s.decls {
			if d.kind != "describe" || d.at >= t.at || !within(t.at, d.body) {
				continue
			}
			isolated = isolated || s.isolated(d)
			if ancestor == 0 && serialAt[d.at] {
				ancestor = d.line
			}
		}
		if !isolated || ancestor == 0 {
			continue
		}
		where := fmt.Sprintf("%s:%d", s.path, t.line)
		cause := fmt.Sprintf("%s:%d", s.path, ancestor)
		report.Findings = append(report.Findings, testvalidity.SourceFinding{
			Rule: testvalidity.RuleIsolatedInSerial, Severity: testvalidity.SeverityAdvisory,
			Test: where, Title: t.title, Location: where, Cause: cause,
			Message: fmt.Sprintf("isolated test runs under the serial suite at %s, so a failure of an earlier serial test skips it and a targeted run proves nothing; move the test out of the serial suite, or drop serial mode if the suite's tests are independent", cause),
		})
	}
	return report
}

// configureMode reads `test.describe.configure({ ... mode: X ... })` starting at token i: ok is
// false when token i does not start that call, and mode is X's literal value, or empty when the
// call names a mode that is not a string literal. A call without a mode key returns "default".
func (s *source) configureMode(i int) (mode string, ok bool) {
	if !s.ident(i, "test") || s.memberAccess(i) || !s.punct(i+1, ".") || !s.ident(i+2, "describe") ||
		!s.punct(i+3, ".") || !s.ident(i+4, "configure") || !s.punct(i+5, "(") || s.match[i+5] < 0 {
		return "", false
	}
	for j := i + 6; j < s.match[i+5]; j++ {
		if !s.key(j, "mode") {
			continue
		}
		if s.loneString([2]int{j + 2, j + 3}) && s.tokens[j+2].Literal && (s.punct(j+3, ",") || s.punct(j+3, "}")) {
			return s.tokens[j+2].Text, true
		}
		return "", true
	}
	return "default", true
}

// innermostDescribe returns the token index of the innermost describe whose body holds token i,
// or -1 when none does.
func (s *source) innermostDescribe(i int) int {
	owner := -1
	for _, d := range s.decls {
		if d.kind == "describe" && within(i, d.body) && d.at > owner {
			owner = d.at
		}
	}
	return owner
}

// isolated reports a declaration annotated as isolated: an `@isolated` tag in its title or in its
// details' `tag` value, or a details annotation whose `type` is the literal `isolated`
// (TSA-V0-045). Other details text, such as an annotation's description, is not a tag.
func (s *source) isolated(d decl) bool {
	if d.titled && hasIsolatedTag(d.title) {
		return true
	}
	if len(d.args) < 3 {
		return false
	}
	for _, arg := range d.args[1 : len(d.args)-1] {
		for j := arg[0]; j < arg[1]; j++ {
			switch {
			case s.key(j, "tag") && s.punct(j+2, "[") && s.match[j+2] > j+2:
				for k := j + 3; k < s.match[j+2]; k++ {
					if s.literalString(k, "@isolated") {
						return true
					}
				}
			case s.key(j, "tag") && s.literalString(j+2, "@isolated"),
				s.key(j, "type") && s.literalString(j+2, "isolated"):
				return true
			}
		}
	}
	return false
}

// key reports an object key named name at token i, bare or as a string literal, followed by `:`.
func (s *source) key(i int, name string) bool {
	quoted := i >= 0 && i < len(s.tokens) && s.tokens[i].Kind == appmap.JSString && s.tokens[i].Literal && s.tokens[i].Text == name
	return (s.ident(i, name) || quoted) && s.punct(i+1, ":")
}

// literalString reports a string literal token at i whose text is text, ignoring case.
func (s *source) literalString(i int, text string) bool {
	return i >= 0 && i < len(s.tokens) && s.tokens[i].Kind == appmap.JSString && s.tokens[i].Literal &&
		strings.EqualFold(s.tokens[i].Text, text)
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
