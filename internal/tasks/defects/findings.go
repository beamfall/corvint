package defects

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// Preflight finding kinds of register references (TDR-V0-007, TDR-V0-008).
const (
	FindingUnknown     = "DEFECT_UNKNOWN"
	FindingNotABug     = "DEFECT_NOT_A_BUG"
	FindingEnvironment = "DEFECT_ENVIRONMENT"
)

const (
	unknownRemedy     = "record the defect with `corvint-tasks defects add`, or correct the id"
	notABugRemedy     = "drop the reference: the register closed that defect as NOT_A_BUG"
	environmentRemedy = "cite a PRODUCT defect, or fix the environment and witness or reopen the obligation"
)

// Findings checks the register-prefix defect ids that obligation titles and
// reasons, and expected-fail lines (an expected-fail title or a test.fail(
// call) of the spec files, cite (TDR-V0-007): an
// id the register does not hold, or one it closed NOT_A_BUG, is a finding. A
// DEFECT or BLOCKED obligation whose cited register defects resolve only to
// ENVIRONMENT defects is a DEFECT_ENVIRONMENT finding (TDR-V0-008).
func Findings(reg *Register, prefix string, l *ticket.ObligationLedger, files map[string][]byte) []obligation.Finding {
	var out []obligation.Finding
	ledgerPrefix := ""
	if l != nil {
		ledgerPrefix = l.Prefix
	}
	check := func(id, path string, line int, where string) *Defect {
		d := reg.Resolve(id)
		switch {
		case reg.Get(id) == nil || d == nil:
			out = append(out, obligation.Finding{Kind: FindingUnknown, ID: id, Path: path, Line: line,
				Detail: where + " cites " + id + ", which the defect register does not hold", Remedy: unknownRemedy})
			return nil
		case d.Status == StatusNotABug:
			out = append(out, obligation.Finding{Kind: FindingNotABug, ID: id, Path: path, Line: line,
				Detail: where + " cites " + id + ", which the register closed as NOT_A_BUG", Remedy: notABugRemedy})
			return nil
		}
		return d
	}
	if l != nil {
		for _, e := range l.Entries {
			text := e.Title
			if e.Reason != nil {
				text += "\n" + *e.Reason
			}
			cited, product, environment, foreign := 0, 0, 0, 0
			for _, id := range obligation.DefectIDs(text, ledgerPrefix) {
				if !strings.HasPrefix(id, prefix+"-") {
					foreign++
					continue
				}
				cited++
				if d := check(id, "", 0, "obligation "+e.ID); d != nil {
					if d.Category == CategoryEnvironment {
						environment++
					} else {
						product++
					}
				}
			}
			blocked := e.State == ticket.ObligationDefect || e.State == ticket.ObligationBlocked
			if blocked && cited > 0 && environment > 0 && product == 0 && foreign == 0 {
				out = append(out, obligation.Finding{Kind: FindingEnvironment, ID: e.ID,
					Detail: "obligation " + e.ID + " is " + e.State + " only on ENVIRONMENT defects, which do not satisfy a blocked-by-defect obligation",
					Remedy: environmentRemedy})
			}
		}
	}
	for path, b := range files {
		for n, line := range strings.Split(string(b), "\n") {
			if !obligation.SaysExpectedFail(line) && !strings.Contains(line, "test.fail(") {
				continue
			}
			for _, id := range obligation.DefectIDs(line, ledgerPrefix) {
				if strings.HasPrefix(id, prefix+"-") {
					check(id, path, n+1, "an expected-fail annotation")
				}
			}
		}
	}
	return out
}
