// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/lineevidence"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// applyLineEvidence is additive over the file plan, including its must-run units.
// It never runs a collector. All line tiers remain disabled without FIH admission.
func applyLineEvidence(ctx context.Context, in affectedPlanInputs, graph *affected.Graph, plan *affected.Plan) {
	if len(plan.Selected) == 0 && len(plan.Dirty) == 0 {
		return
	}
	git := testinventory.Git{Executable: in.gitExecutable, Root: in.repository}
	store, err := lineevidence.Store(ctx, git)
	if err != nil {
		return
	}
	// The frozen baseline has no tier/reason members. Until its output migration
	// is accepted, activate the overlay only for an initialized private store or
	// an explicit watch index. Missing instance files inside it still widen.
	if in.lineEvidenceIndex == "" {
		if _, err := os.Lstat(store); os.IsNotExist(err) {
			return
		}
	}
	tree, err := testinventory.NewWorkTree(ctx, git)
	if err != nil {
		annotateLineFallback(plan, lineevidence.Stale)
		return
	}
	files, _ := tree.Files()
	declared := testinventory.Detect(files, tree.Read)
	base := in.base
	if base == "" {
		base = in.revision
	}
	// Local edits use HEAD as the evidence base on every one-shot surface.
	if dirty, err := affected.DirtyPaths(ctx, in.gitExecutable, in.repository); err != nil || len(dirty) != 0 {
		base = in.revision
	}
	committed, err := testinventory.NewCommitTree(ctx, git, base)
	if err != nil {
		annotateLineFallback(plan, lineevidence.Stale)
		return
	}
	baseFiles, _ := committed.Files()
	verdicts := map[string]lineevidence.Verdict{}
	for _, d := range declared {
		v := lineevidence.Verdict{Instance: d.Key, Tier: lineevidence.File, Reason: lineevidence.Absent}
		switch d.Runner {
		case testid.Pytest:
			v.Reason = "pytest-file-level"
		case testid.Playwright:
			v.Tier = lineevidence.Full
		}
		if d.Runner == testid.Pytest {
			verdicts[d.Key] = v
			continue
		}
		name := filepath.Join(store, lineevidence.Filename(d))
		if in.lineEvidenceIndex != "" && d.Runner == in.lineEvidenceFamily && d.Config == in.lineEvidenceConfig {
			name = in.lineEvidenceIndex
		}
		if _, err := os.Lstat(name); err == nil {
			native, nativeErr := lineevidence.NativeIdentity(ctx, in.repository, d)
			command := lineevidence.Command(d)
			runner := lineevidence.Binding(d, files, command, native)
			if nativeErr != nil {
				runner = ""
			}
			v = lineevidence.Read(lineevidence.Input{Store: store, Path: name, Instance: d, Runner: runner, Command: command,
				BaseFiles: baseFiles, CurrentFiles: files, Changed: plan.Dirty,
				Ancestor: func(commit string) bool {
					_, err := git.Output(ctx, "merge-base", "--is-ancestor", commit, base)
					return err == nil
				}})
		} else if !os.IsNotExist(err) {
			v.Reason = lineevidence.Damaged
		}
		verdicts[d.Key] = v
	}
	selected := map[string]bool{}
	for _, s := range plan.Selected {
		selected[s.UnitID] = true
	}
	for _, unitID := range graph.UnitIDs() {
		unit, _ := graph.Unit(unitID)
		if len(unit.Tests) == 0 {
			continue
		}
		d, ok := lineInstance(unit, declared)
		if !ok {
			continue
		}
		v := verdicts[d.Key]
		add := v.Tier == lineevidence.Full && v.Reason == lineevidence.Stale
		for _, idString := range v.Rank {
			id, e := testid.Parse(idString)
			if e != nil {
				continue
			}
			for _, file := range unit.Tests {
				if id.File == file || d.Runner == testid.Go && id.File == path.Dir(file) {
					add = true
				}
			}
		}
		if add && !selected[unitID] {
			plan.Selected = append(plan.Selected, affected.Selection{UnitID: unitID, Tests: unit.Tests,
				Witness: affected.Witness{Kind: "LINE_EVIDENCE", DirtyPath: firstChanged(plan.Dirty)}})
			selected[unitID] = true
		}
	}
	plan.Excluded = slices.DeleteFunc(plan.Excluded, func(e affected.Exclusion) bool { return selected[e.UnitID] })
	for i := range plan.Selected {
		s := &plan.Selected[i]
		unit, _ := graph.Unit(s.UnitID)
		d, ok := lineInstance(unit, declared)
		v := lineevidence.Verdict{Instance: s.UnitID, Tier: lineevidence.File, Reason: lineevidence.Absent}
		if ok {
			v = verdicts[d.Key]
		}
		if plan.Scope == "FULL" {
			v.Tier = lineevidence.Full
		}
		if ok && d.Runner == testid.Playwright {
			v.Tier = lineevidence.Full
			if len(s.Flows) != 0 {
				v.Tier = lineevidence.Flow
			}
		}
		exact := false
		s.Tier, s.Exact, s.Instance, s.Reason, s.Rank = string(v.Tier), &exact, v.Instance, v.Reason, v.Rank
	}
}

func firstChanged(paths []string) string {
	if len(paths) != 0 {
		return paths[0]
	}
	return ""
}

func annotateLineFallback(plan *affected.Plan, reason string) {
	for i := range plan.Selected {
		exact := false
		s := &plan.Selected[i]
		s.Tier, s.Exact, s.Instance, s.Reason = "file", &exact, s.UnitID, reason
	}
}

func lineInstance(unit affected.Unit, declared []testinventory.Declared) (testinventory.Declared, bool) {
	runner := ""
	switch {
	case strings.HasPrefix(unit.ID, "go:") || strings.HasPrefix(unit.ID, "go-module:"):
		runner = testid.Go
	case strings.HasPrefix(unit.ID, "python:"):
		runner = testid.Pytest
	default:
		id, ok := testid.FromUnit(unit.ID, nil)
		if ok {
			runner = id.Runner
		}
	}
	var best testinventory.Declared
	for _, d := range declared {
		if d.Runner != runner {
			continue
		}
		for _, file := range unit.Tests {
			if d.Root == "." || strings.HasPrefix(file, d.Root+"/") {
				if best.Key == "" || best.Root == "." && d.Root != "." || len(d.Root) > len(best.Root) {
					best = d
				}
			}
		}
	}
	return best, best.Key != ""
}
