package store

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// intentFix appends the CTW-V0-007 repair text to an intent-location refusal
// of a writer entry point: an INTENT_BRANCH_MISMATCH or INTENT_DIVERGED error,
// or a refused report coded with either. The code, outcome and store are
// unchanged; nothing changes at all when resolution recorded no fix and the
// intent root is the primary (CTW-V0-009).
func intentFix(repo *intent.Repository, report *Report, err error) (*Report, error) {
	if repo == nil {
		return report, err
	}
	if err != nil {
		return report, repo.WithIntentRepair(err)
	}
	if report == nil {
		return report, nil
	}
	for _, code := range []string{wire.CodeIntentBranchMismatch, wire.CodeIntentDiverged} {
		if !report.Outcome.HasCode(code) {
			continue
		}
		if fix := repo.IntentRepair(code); fix != "" {
			if report.Detail == "" {
				report.Detail = fix
			} else {
				report.Detail += "; " + fix
			}
		}
		break
	}
	return report, nil
}
