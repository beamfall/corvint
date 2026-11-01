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
)

// planAffectedPaths is the sorted, duplicate-free set of existing project files a plan names,
// at most planAffectedPathLimit: each whitespace-separated token, stripped of quotes, backticks
// and trailing punctuation, that resolves to a regular file inside root.
func planAffectedPaths(root, plan string) []string {
	var paths []string
	for _, token := range strings.Fields(plan) {
		token = strings.Trim(token, "`'\"()[]{}<>,;:!?*")
		token = strings.TrimSuffix(token, ".")
		token = strings.TrimPrefix(token, "./")
		if token == "" || filepath.IsAbs(token) || !affected.ValidRelativePath(token) {
			continue
		}
		if info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(token))); err != nil || !info.Mode().IsRegular() {
			continue
		}
		paths = affected.NormalizePaths(append(paths, token))
		if len(paths) == planAffectedPathLimit {
			break
		}
	}
	return paths
}

// startPlanAffected plans the affected checks for paths against the working tree, concurrently
// with the plan's context compile, and returns a wait function that gives the framed answer, or
// the framed NOT_PRODUCED degradation when the plan fails or does not finish before ctx's
// deadline less planAffectedReserve. The plan reads only; it never approves or blocks the plan.
func startPlanAffected(ctx context.Context, root string, paths []string) func() string {
	child, cancel := context.WithCancel(ctx)
	done := make(chan map[string]any, 1)
	go func() {
		done <- planAffectedObject(child, root, paths)
	}()
	return func() string {
		defer cancel()
		var object map[string]any
		var wait <-chan time.Time
		if deadline, ok := ctx.Deadline(); ok {
			timer := time.NewTimer(time.Until(deadline) - planAffectedReserve)
			defer timer.Stop()
			wait = timer.C
		}
		select {
		case object = <-done:
		case <-wait:
			object = map[string]any{"profile": planAffectedProfile, "paths": paths, "state": "NOT_PRODUCED", "reason": "plan-affected-deadline"}
		}
		raw, err := json.Marshal(object)
		if err != nil {
			return ""
		}
		framed, err := repoenvelope.Frame(string(raw))
		if err != nil {
			return ""
		}
		return "Corvint plan affected checks (evidence for the plan's named paths, not approval):\n" + framed
	}
}

func planAffectedObject(ctx context.Context, root string, paths []string) map[string]any {
	object := map[string]any{"profile": planAffectedProfile, "paths": paths}
	invocation := affectedInvocation{Root: root, PathList: paths, Worktree: true, sharedVia: true}
	if err := resolveAffectedBatchAndCache(&invocation); err != nil {
		object["state"], object["reason"] = "NOT_PRODUCED", "plan-affected-refused"
		return object
	}
	receipt, err := compileAffected(ctx, invocation)
	if err != nil {
		object["state"], object["reason"] = "NOT_PRODUCED", "plan-affected-refused"
		return object
	}
	summary := summarizeAffectedPlan(receipt.Plan, len(receipt.Plan.Excluded))
	units := make([]string, 0, min(len(receipt.Plan.Selected), planAffectedUnitLimit))
	for _, selection := range receipt.Plan.Selected[:min(len(receipt.Plan.Selected), planAffectedUnitLimit)] {
		units = append(units, selection.UnitID)
	}
	object["state"], object["summary"], object["selectedUnits"] = "PRODUCED", summary.Line, units
	object["omittedSelectedUnits"] = len(receipt.Plan.Selected) - len(units)
	return object
}
