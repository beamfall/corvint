package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedSummary is the one-glance head of an affected-plan/1 document
// (AFP-V0-510): the plan's scope and counts, the unknowns grouped by reason,
// and the same as one text line, so a reader needs neither the 60 KB plan
// nor a JSON tool to learn what the plan decided. It restates plan members
// and never changes the selection. Dirty is plan.dirty, which with --base
// unions the committed range (AFP-V0-010); Worktree and Committed split it
// so a clean tree with a range reads worktree=0 (AFP-V0-820).
type affectedSummary struct {
	Committed       int            `json:"committed"`
	Dirty           int            `json:"dirty"`
	Excluded        int            `json:"excluded"`
	Line            string         `json:"line"`
	Scope           string         `json:"scope"`
	Selected        int            `json:"selected"`
	Unknown         int            `json:"unknown"`
	UnknownByReason map[string]int `json:"unknownByReason"`
	Worktree        int            `json:"worktree"`
}

// summarizeAffectedPlan counts the full plan; excluded is the compact
// exclusion count, which equals len(plan.Excluded); worktree is the worktree
// status set's size and committed the range's.
func summarizeAffectedPlan(plan affected.Plan, excluded, worktree, committed int) affectedSummary {
	byReason := make(map[string]int, len(plan.Unknown))
	for _, unknown := range plan.Unknown {
		byReason[unknown.Reason]++
	}
	reasons := make([]string, 0, len(byReason))
	for reason := range byReason {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	line := fmt.Sprintf("scope=%s dirty=%d selected=%d excluded=%d unknown=%d", plan.Scope, len(plan.Dirty), len(plan.Selected), excluded, len(plan.Unknown))
	for _, reason := range reasons {
		line += fmt.Sprintf(" %s=%d", reason, byReason[reason])
	}
	return affectedSummary{
		Committed:       committed,
		Dirty:           len(plan.Dirty),
		Excluded:        excluded,
		Line:            strings.TrimSpace(line),
		Scope:           plan.Scope,
		Selected:        len(plan.Selected),
		Unknown:         len(plan.Unknown),
		UnknownByReason: byReason,
		Worktree:        worktree,
	}
}
