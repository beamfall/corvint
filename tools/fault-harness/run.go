// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// confirmRuns is the number of faulted runs a kill needs, and of baseline runs
// (FIH-V1-004, FIH-V1-005).
const confirmRuns = 3

// harnessCase is one sealed PR: the faults put into the tree at HeadSHA, whose
// plan is computed against BaseSHA.
type harnessCase struct {
	ID             string  `json:"id"`
	Kind           string  `json:"kind"` // revert, mutants or non-code
	PR             int     `json:"pr"`
	BaseSHA        string  `json:"baseSha"`
	HeadSHA        string  `json:"headSha"`
	FillsShortfall bool    `json:"fillsShortfall,omitempty"`
	Faults         []fault `json:"faults"`
}

// Fault statuses.
const (
	faultKilled = "KILLED"  // at least one confirmed kill
	faultNoKill = "NO_KILL" // no confirmed kill, or the faulted tree did not build or run
	faultError  = "ERROR"   // a failed measurement after a confirmed kill
)

type faultResult struct {
	ID             string      `json:"id"`
	Status         string      `json:"status"`
	Planned        bool        `json:"planned"`
	PlanKind       string      `json:"planKind,omitempty"` // NARROWED, FULL or UNKNOWN
	Tier           string      `json:"tier,omitempty"`
	Plan           []planEntry `json:"plan,omitempty"`
	DependentUnits []string    `json:"dependentUnits,omitempty"`
	Kills          []string    `json:"kills,omitempty"`
	Misses         []string    `json:"misses,omitempty"`
	Detail         string      `json:"detail,omitempty"`
}

// Case statuses.
const (
	caseCounted  = "KILLED"   // at least one fault has a confirmed kill
	caseDropped  = "NO_KILL"  // no fault was killed (FIH-V1-005)
	caseExcluded = "EXCLUDED" // a pinned input could not be reproduced
	caseError    = "ERROR"
)

type caseResult struct {
	ID             string        `json:"id"`
	Kind           string        `json:"kind"`
	PR             int           `json:"pr"`
	HeadSHA        string        `json:"headSha"`
	Status         string        `json:"status"`
	ExcludedReason string        `json:"excludedReason,omitempty"`
	Error          string        `json:"error,omitempty"`
	Faults         []faultResult `json:"faults"`
}

// baselineRuns holds the three full-suite runs at one head and their
// evaluation.
type baselineRuns struct {
	SHA    string
	Runs   []RunResult
	Record baselineRecord
}

// passedEveryRun lists the tests that passed in all runs.
func passedEveryRun(runs []RunResult) map[string]bool {
	clean := map[string]bool{}
	if len(runs) == 0 {
		return clean
	}
	for id, o := range runs[0] {
		if o != Pass {
			continue
		}
		ok := true
		for _, r := range runs[1:] {
			ok = ok && r[id] == Pass
		}
		if ok {
			clean[id] = true
		}
	}
	return clean
}

// confirmKills returns the tests that failed in every faulted run and passed
// in every baseline run (FIH-V1-005). Two failures in three are no kill.
func confirmKills(baseline, faulted []RunResult) []string {
	if len(baseline) != confirmRuns || len(faulted) != confirmRuns {
		return nil
	}
	clean := passedEveryRun(baseline)
	var kills []string
	for id, o := range faulted[0] {
		if o != Fail || !clean[id] {
			continue
		}
		ok := true
		for _, r := range faulted[1:] {
			ok = ok && r[id] == Fail
		}
		if ok {
			kills = append(kills, id)
		}
	}
	slices.Sort(kills)
	return kills
}

// findMisses returns each confirmed kill the selected run did not execute and
// see fail (FIH-V1-006). A miss is never waived as a flake.
func findMisses(kills []string, entries []planEntry, selected RunResult) []string {
	var misses []string
	for _, k := range kills {
		if !selects(entries, k) || selected[k] != Fail {
			misses = append(misses, k)
		}
	}
	return misses
}

// planEntries keeps the runner's own units of a receipt. Other runners' units,
// driver targets and CI jobs belong to other measurements.
func planEntries(runner string, sel []selection) []planEntry {
	var out []planEntry
	for _, s := range sel {
		if s.Kind != "" || !strings.HasPrefix(s.UnitID, runner+":") {
			continue
		}
		out = append(out, planEntry{Unit: s.UnitID, Run: slices.Clone(s.Run)})
	}
	slices.SortFunc(out, func(a, b planEntry) int { return strings.Compare(a.Unit, b.Unit) })
	return out
}

// planKind classifies a receipt: UNKNOWN when it is not bounded or names
// unknowns, FULL when it runs every unit of the suite whole, else NARROWED.
func planKind(r receipt, entries []planEntry, suite []RunResult) string {
	if r.Plan.Scope == "UNKNOWN" || len(r.Plan.Unknown) > 0 {
		return "UNKNOWN"
	}
	whole := map[string]bool{}
	for _, e := range entries {
		if len(e.Run) == 0 {
			whole[e.Unit] = true
		}
	}
	for _, run := range suite {
		for id := range run {
			if !whole[testUnit(id)] {
				return "NARROWED"
			}
		}
	}
	return "FULL"
}

type runConfig struct {
	Source  string // read-only source repository
	Clones  string // existing private parent for per-case clones
	Binary  string // planner binary
	Adapter Adapter
	// Unrunnable names tests declared unrunnable in the pinned image.
	Unrunnable []string
}

// harness carries the baselines measured so far, one per head.
type harness struct {
	cfg       runConfig
	baselines map[string]baselineRuns
	order     []string // heads in the order their baselines ran
}

// baselineRecords returns the evaluated baselines in the order they ran.
func (h *harness) baselineRecords() []baselineRecord {
	out := []baselineRecord{}
	for _, sha := range h.order {
		out = append(out, h.baselines[sha].Record)
	}
	return out
}

func newHarness(cfg runConfig) *harness {
	return &harness{cfg: cfg, baselines: map[string]baselineRuns{}}
}

// runCase clones the source at the case head, measures the head's baseline
// once, then puts each fault in and confirms kills and misses. The clone is
// removed afterwards; the source is never written.
func (h *harness) runCase(ctx context.Context, c harnessCase) (r caseResult) {
	r = caseResult{ID: c.ID, Kind: c.Kind, PR: c.PR, HeadSHA: c.HeadSHA, Status: caseError, Faults: []faultResult{}}
	err := func() error {
		root, cleanup, err := h.clone(ctx, c.ID, c.HeadSHA)
		if err != nil {
			if refusalCode(err) == "pin-unavailable" {
				r.Status, r.ExcludedReason = caseExcluded, "pin-unavailable"
				return nil
			}
			return err
		}
		defer func() {
			if err := cleanup(); err != nil {
				r.Status, r.Error = caseError, "clone cleanup: "+err.Error()
			}
		}()
		base, ok := h.baselines[c.HeadSHA]
		if !ok {
			base = baselineRuns{SHA: c.HeadSHA}
			for i := 0; i < confirmRuns; i++ {
				run, err := h.cfg.Adapter.Run(ctx, root, nil)
				if err != nil {
					return fmt.Errorf("baseline run %d: %w", i+1, err)
				}
				base.Runs = append(base.Runs, run)
			}
			if base.Record, err = evaluateBaseline(c.HeadSHA, base.Runs, h.cfg.Unrunnable); err != nil {
				return err
			}
			h.baselines[c.HeadSHA] = base
			h.order = append(h.order, c.HeadSHA)
		}
		killed := false
		for _, f := range c.Faults {
			fr := h.runFault(ctx, root, c, f, base)
			r.Faults = append(r.Faults, fr)
			switch fr.Status {
			case faultError:
				return fmt.Errorf("fault %s: %s", f.ID, fr.Detail)
			case faultKilled:
				killed = true
			}
		}
		r.Status = caseDropped
		if killed {
			r.Status = caseCounted
		}
		return nil
	}()
	if err != nil {
		r.Status, r.Error = caseError, err.Error()
	}
	return r
}

func (h *harness) runFault(ctx context.Context, root string, c harnessCase, f fault, base baselineRuns) (fr faultResult) {
	fr = faultResult{ID: f.ID, Status: faultError}
	fail := func(err error) faultResult { fr.Detail = err.Error(); return fr }
	if _, err := git(ctx, root, "reset", "--quiet", "--hard", c.HeadSHA); err != nil {
		return fail(err)
	}
	if _, err := git(ctx, root, "clean", "-fdxq"); err != nil {
		return fail(err)
	}
	if err := applyFault(ctx, root, f); err != nil {
		return fail(err)
	}
	paths, err := commitAll(ctx, root, "fault "+f.ID)
	if err != nil {
		return fail(err)
	}
	if len(paths) == 0 {
		return fail(errors.New("fault changed no tracked path (possibly ignored file)"))
	}
	// The plan and the dependent units are recorded for every fault, killed
	// or not, so a quarantined test the plan omits is counted (FIH-V1-004).
	p, err := plan(ctx, root, h.cfg.Binary, c.BaseSHA)
	if err != nil {
		return fail(err)
	}
	fr.Tier = p.Plan.Tier
	fr.Plan = planEntries(h.cfg.Adapter.Runner(), p.Plan.Selected)
	fr.PlanKind = planKind(p, fr.Plan, base.Runs)
	diff, err := git(ctx, root, "diff", "--name-only", "--no-renames", c.BaseSHA, "HEAD")
	if err != nil {
		return fail(err)
	}
	if diff != "" {
		if fr.DependentUnits, err = h.cfg.Adapter.DependentUnits(ctx, root, strings.Split(diff, "\n")); err != nil {
			return fail(err)
		}
	}
	fr.Planned = true
	var faulted []RunResult
	for i := 0; i < confirmRuns; i++ {
		run, err := h.cfg.Adapter.Run(ctx, root, nil)
		if err != nil {
			// A fault that does not build, crashes or times out is not
			// counted (G3, AEO-V0-004): it joins the dropped count.
			fr.Status, fr.Detail = faultNoKill, fmt.Sprintf("faulted run %d: %v", i+1, err)
			return fr
		}
		faulted = append(faulted, run)
	}
	fr.Kills = confirmKills(base.Runs, faulted)
	if len(fr.Kills) == 0 {
		fr.Status, fr.Detail = faultNoKill, "no test failed in all faulted runs after passing every baseline run"
		return fr
	}
	if fr.PlanKind == "NARROWED" {
		selected := RunResult{}
		if len(fr.Plan) > 0 {
			if selected, err = h.cfg.Adapter.Run(ctx, root, fr.Plan); err != nil {
				return fail(fmt.Errorf("selected run: %w", err))
			}
		}
		fr.Misses = findMisses(fr.Kills, fr.Plan, selected)
	}
	fr.Status = faultKilled
	return fr
}

// clone makes a private clone of the source at sha under the clone parent.
func (h *harness) clone(ctx context.Context, id, sha string) (string, func() error, error) {
	source, err := filepath.EvalSymlinks(h.cfg.Source)
	if err != nil {
		return "", nil, err
	}
	parent, err := filepath.EvalSymlinks(h.cfg.Clones)
	if err != nil {
		return "", nil, err
	}
	if rel, err := filepath.Rel(source, parent); err != nil || rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return "", nil, errors.New("clone parent must be outside source repository")
	}
	root := filepath.Join(parent, id)
	// Reserve the destination atomically so a concurrent run sharing the
	// parent can neither reuse nor delete this case's clone.
	if err := os.Mkdir(root, 0o700); err != nil {
		return "", nil, fmt.Errorf("clone destination already exists or is unreadable: %s: %w", root, err)
	}
	cleanup := func() error { return os.RemoveAll(root) }
	if _, err := git(ctx, parent, "clone", "--quiet", "--no-hardlinks", "--shared", "--no-checkout", source, root); err != nil {
		return "", nil, errors.Join(err, cleanup())
	}
	if _, err := git(ctx, root, "checkout", "--quiet", "--detach", sha); err != nil {
		return "", nil, errors.Join(refuse("pin-unavailable", "%s: %v", sha, err), cleanup())
	}
	if head, err := git(ctx, root, "rev-parse", "HEAD"); err != nil || head != sha {
		return "", nil, errors.Join(refuse("pin-unavailable", "clone head %s is not %s", head, sha), err, cleanup())
	}
	return root, cleanup, nil
}
