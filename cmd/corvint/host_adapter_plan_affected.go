package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const (
	// planAffectedProfile names the framed affected-checks object an ExitPlanMode answer carries
	// (AHI-632, V1-0878).
	planAffectedProfile = "corvint-plan-affected/0"
	// planAffectedPathLimit bounds how many plan-named paths reach the affected plan.
	planAffectedPathLimit = 16
	// planAffectedUnitLimit bounds how many selected unit IDs the answer names.
	planAffectedUnitLimit = 8
	// planAffectedReserve is kept back from the host-kill deadline to write the answer.
	planAffectedReserve = 150 * time.Millisecond
	// planAffectedResultAllowance is the encoded room, beyond the paths' NOT_PRODUCED object,
	// kept back from the context packet's budget for a PRODUCED object's summary, units,
	// revision and worktree warning (AHI-632).
	planAffectedResultAllowance = 1500
	// planAffectedHeader introduces the framed object; it is evidence, never approval.
	planAffectedHeader = "Corvint plan affected checks (evidence for the plan's named paths, not approval):\n"
)

// planAffectedPaths is the sorted, duplicate-free set of existing project files a plan names,
// at most planAffectedPathLimit: each whitespace-separated token, stripped of quotes, backticks
// and trailing punctuation, that names a regular file inside root through no symbolic link, so a
// symlinked ancestor cannot admit a file outside the project.
func planAffectedPaths(root, plan string) []string {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil
	}
	var paths []string
	for _, token := range strings.Fields(plan) {
		token = strings.Trim(token, "`'\"()[]{}<>,;:!?*")
		token = strings.TrimSuffix(token, ".")
		token = strings.TrimPrefix(token, "./")
		if token == "" || filepath.IsAbs(token) || !affected.ValidRelativePath(token) {
			continue
		}
		candidate := filepath.Join(resolvedRoot, filepath.FromSlash(token))
		if info, err := os.Lstat(candidate); err != nil || !info.Mode().IsRegular() {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(candidate); err != nil || resolved != candidate {
			continue
		}
		paths = affected.NormalizePaths(append(paths, token))
		if len(paths) == planAffectedPathLimit {
			break
		}
	}
	return paths
}

// planAffectedContextReserve is the encoded additionalContext room the plan's affected object
// may take: the context packet is compiled against the adapter bound less this reserve, so
// appending the object never pushes the answer past adapterOutputLimit (AHI-632).
func planAffectedContextReserve(paths []string) int {
	return promptBoundReserve("\n\n"+renderPlanAffected(planAffectedRefusal(paths, "plan-affected-over-budget"))) + planAffectedResultAllowance
}

// planAffectedRefusal is the NOT_PRODUCED object for paths with the named reason.
func planAffectedRefusal(paths []string, reason string) map[string]any {
	return map[string]any{"profile": planAffectedProfile, "paths": paths, "state": "NOT_PRODUCED", "reason": reason}
}

// renderPlanAffected frames object as repository data under the evidence header, or "" when it
// cannot be encoded.
func renderPlanAffected(object map[string]any) string {
	raw, err := json.Marshal(object)
	if err != nil {
		return ""
	}
	framed, err := repoenvelope.Frame(string(raw))
	if err != nil {
		return ""
	}
	return planAffectedHeader + framed
}

// planAffectedResult is a finished plan and the instant it finished.
type planAffectedResult struct {
	object map[string]any
	at     time.Time
}

// startPlanAffected plans the affected checks for paths against the working tree, concurrently
// with the plan's context compile, and returns a wait function that gives the framed answer's
// object, or the NOT_PRODUCED degradation when the plan fails, or plan-affected-deadline when it
// does not finish before ctx's deadline less planAffectedReserve. The cutoff is fixed at launch
// and bounds the plan's own context, so a result finished after it is never PRODUCED. The plan
// reads only; it never approves or blocks the plan.
func startPlanAffected(ctx context.Context, root string, paths []string) func() map[string]any {
	var child context.Context
	var cancel context.CancelFunc
	cutoff, bounded := ctx.Deadline()
	if bounded {
		cutoff = cutoff.Add(-planAffectedReserve)
		child, cancel = context.WithDeadline(ctx, cutoff)
	} else {
		child, cancel = context.WithCancel(ctx)
	}
	done := make(chan planAffectedResult, 1)
	go func() {
		object := planAffectedObject(child, root, paths)
		done <- planAffectedResult{object: object, at: time.Now()}
	}()
	return func() map[string]any {
		defer cancel()
		var wait <-chan time.Time
		if bounded {
			timer := time.NewTimer(time.Until(cutoff))
			defer timer.Stop()
			wait = timer.C
		}
		select {
		case result := <-done:
			if bounded && !result.at.Before(cutoff) {
				return planAffectedRefusal(paths, "plan-affected-deadline")
			}
			return result.object
		case <-wait:
			return planAffectedRefusal(paths, "plan-affected-deadline")
		}
	}
}

// planAffectedObject is the working-tree affected plan for paths. A PRODUCED object keeps the
// receipt's observed HEAD, its worktree warning and its degradations (worktree-path-plan,
// ACX-V0-851): it reads uncommitted bytes and so is not pinned evidence.
func planAffectedObject(ctx context.Context, root string, paths []string) map[string]any {
	invocation := affectedInvocation{Root: root, PathList: paths, Worktree: true, sharedVia: true}
	if err := resolveAffectedBatchAndCache(&invocation); err != nil {
		return planAffectedRefusal(paths, "plan-affected-refused")
	}
	receipt, err := compileAffected(ctx, invocation)
	if err != nil {
		return planAffectedRefusal(paths, "plan-affected-refused")
	}
	summary := summarizeAffectedPlan(receipt.Plan, len(receipt.Plan.Excluded))
	units := make([]string, 0, min(len(receipt.Plan.Selected), planAffectedUnitLimit))
	for _, selection := range receipt.Plan.Selected[:min(len(receipt.Plan.Selected), planAffectedUnitLimit)] {
		units = append(units, selection.UnitID)
	}
	object := map[string]any{"profile": planAffectedProfile, "paths": paths, "state": "PRODUCED", "summary": summary.Line, "selectedUnits": units}
	object["omittedSelectedUnits"] = len(receipt.Plan.Selected) - len(units)
	object["revision"], object["warning"], object["degradations"] = receipt.Revision, receipt.Warning, receipt.Degradations
	return object
}

// adapterContextReserveKey carries the encoded additionalContext room a caller appends after the
// Claude adapter's packet, which runClaudeAdapter holds back from the packet's budget (AHI-632).
type adapterContextReserveKey struct{}

func withAdapterContextReserve(ctx context.Context, reserve int) context.Context {
	return context.WithValue(ctx, adapterContextReserveKey{}, reserve)
}

func adapterContextReserve(ctx context.Context) int {
	reserve, _ := ctx.Value(adapterContextReserveKey{}).(int)
	return max(reserve, 0)
}
