package dogfoodflow

import "strings"

// rerunChange is the recovery for evidence the check no longer finds bound to
// this BASE and HEAD: dogfood change rewrites the report and its evidence.
const rerunChange = "the recorded evidence no longer matches this base and HEAD; rerun corvint dogfood change BASE on this HEAD, then rerun dogfood check"

// failRecoveries is the default fix line for a dogfood-check FAIL whose call
// names none (ASD-V0-998). tools/flow-guide-check reads this map as the hint
// for each code (ASD-V0-997), so keep it a literal map.
var failRecoveries = map[string]string{
	"aggregate-binding-drift":           rerunChange,
	"aggregate-prior-evidence-drift":    rerunChange,
	"context-abstention-evidence-drift": rerunChange,
	"dogfood-report-drift":              rerunChange,
	"dogfood-report-missing":            "run corvint dogfood change BASE on this HEAD until it reports complete",
	"aggregate-schema-invalid":          rerunChange,
	"local-outcome-evidence-drift":      rerunChange,
	"query-abstention-evidence-drift":   rerunChange,
	"missing-intent-scope":              "rerun corvint dogfood change BASE on this HEAD; it writes .corvint/change.ocm-intents and .corvint/change.ocm-status.json",
	"cem-map-missing":                   "bind the change first: commit the .corvint/change.cem.json the binding step wrote (docs/DOGFOOD.md section 4), then rerun corvint dogfood change BASE and dogfood check",
	"cem-policy":                        "resolve each policy issue the status line above names (cite or reason every unknown hunk), rebind, then rerun dogfood check",
	"ocm-policy":                        "run corvint dogfood-ocm status --expected-base BASE --target HEAD to see the refusal, fix the intent scopes it names, then rerun corvint dogfood change BASE",
	"verifier-disagreement":             "the base and tree verifiers disagree on this range; rerun corvint dogfood change BASE on this HEAD from a clean worktree, and report both verifier outputs if it repeats (docs/DOGFOOD.md section 4)",
	"dogfood-report-write":              "make .corvint and the report path writable, then rerun dogfood check",
}

// recoveryLine returns the default fix line for reason, or "" when the call
// already names one or the code has none.
func recoveryLine(reason, base string, lines []string) string {
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "fix:") {
			return ""
		}
	}
	recovery, found := failRecoveries[reason]
	if !found {
		return ""
	}
	return "  fix: " + strings.ReplaceAll(recovery, "BASE", base)
}
