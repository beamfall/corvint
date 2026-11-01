// SPDX-License-Identifier: Apache-2.0
// Command selection-baseline replays the affected planner and the CI PR-tier
// selector over a fixed set of real changes of this repository and measures how
// often CI would narrow, and whether the narrowed selection kept every test that
// failed in that change's CI run (V1-2892).
//
//	selection-baseline --root DIR --changes FILE --corvint BIN --selector BIN [--out FILE] [--check]
//
// FILE (profile corvint-selection-baseline-changes/0) lists each change as a
// base and head commit and, for a change whose CI failed, the failing packages.
// The replay clones DIR's repository into a temporary directory with --shared
// (DIR's index, worktrees and refs are not written), checks out each head
// detached and clean, and runs `BIN affected --base BASE`, the selector on that
// plan, and `BIN test --base BASE --dry-run --manifest M`. A change narrows in CI
// when the selector's verdict is `run` or `NOTHING` and no `.github/` path
// changed, the rules the CI PR tier applies (AFP-V0-098, AFP-V0-933); any other
// verdict is FULL. A failing package of a narrowed change that the verdict does
// not name is a miss. Costs are each selected package's share of the summed
// milliseconds in DIR's .github/cishards/package-costs.json.
//
// --out writes the corvint-selection-baseline/0 results. --check fails (exit 1)
// when the number of narrowed merged changes is below the file's floor or any
// miss appears, so a regression in either is caught on the same fixed set.
// Every change whose CI scope or verdict moved from the one recorded beside it
// in FILE is named with its fallback reason (TSP-V0-710).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	changesProfile = "corvint-selection-baseline-changes/0"
	resultsProfile = "corvint-selection-baseline/0"
	commandTimeout = 5 * time.Minute
)

type changeSet struct {
	Profile    string   `json:"profile"`
	Repository string   `json:"repository"`
	Note       string   `json:"note"`
	Floor      floor    `json:"floor"`
	Changes    []change `json:"changes"`
}

type floor struct {
	NarrowMerged int `json:"narrowMerged"`
	Misses       int `json:"misses"`
}

type change struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	Base     string    `json:"base"`
	Head     string    `json:"head"`
	PR       int       `json:"pr,omitempty"`
	Failures []failure `json:"failures,omitempty"`
	// Recorded is the CI scope and verdict of the replay the floor was last
	// measured on (TSP-V0-710).
	Recorded *recorded `json:"recorded,omitempty"`
}

type recorded struct {
	CIScope string `json:"ciScope"`
	Verdict string `json:"verdict"`
}

type failure struct {
	Run          int64  `json:"run"`
	Job          int64  `json:"job"`
	Package      string `json:"package,omitempty"`
	Unattributed string `json:"unattributed,omitempty"`
}

// plan is the part of an affected-plan receipt the measure reads.
type plan struct {
	Plan struct {
		Dirty    []string `json:"dirty"`
		Scope    string   `json:"scope"`
		Selected []struct {
			UnitID  string `json:"unitId"`
			Witness struct {
				Kind string `json:"kind"`
			} `json:"witness"`
		} `json:"selected"`
		Excluded struct {
			Count int `json:"count"`
		} `json:"excluded"`
		Unknown []struct {
			Reason string `json:"reason"`
			Detail string `json:"detail"`
		} `json:"unknown"`
	} `json:"plan"`
	Provider struct {
		Go struct {
			State string `json:"state"`
		} `json:"go"`
	} `json:"provider"`
}

type manifest struct {
	Scope  string         `json:"scope"`
	Reason string         `json:"reason"`
	Units  []manifestUnit `json:"units"`
}

type manifestUnit struct {
	Adapter string `json:"adapter"`
	Unit    string `json:"unit"`
	Slice   string `json:"slice"`
	Reason  string `json:"reason"`
}

// Result is one replayed change.
type Result struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	Base           string         `json:"base"`
	Head           string         `json:"head"`
	PR             int            `json:"pr,omitempty"`
	PlanScope      string         `json:"planScope"`
	Dirty          int            `json:"dirty"`
	Selected       int            `json:"selected"`
	Unknown        int            `json:"unknown"`
	Universe       int            `json:"universe"`
	UnknownKinds   map[string]int `json:"unknownKinds"`
	WitnessKinds   map[string]int `json:"witnessKinds"`
	Verdict        string         `json:"verdict"`
	VerdictReason  string         `json:"verdictReason,omitempty"`
	GithubChange   bool           `json:"githubChange"`
	CIScope        string         `json:"ciScope"`
	Packages       []string       `json:"packages"`
	PackageShare   float64        `json:"packageShare"`
	CostShare      float64        `json:"costShare"`
	TestScope      string         `json:"testScope"`
	WholePackages  int            `json:"wholePackages"`
	SlicedPackages int            `json:"slicedPackages"`
	WholeWithTests int            `json:"wholeSourceAndTestsChanged"`
	Failures       []failure      `json:"failures,omitempty"`
	Misses         []string       `json:"misses,omitempty"`
	MissCauses     []string       `json:"missCauses,omitempty"`
	Unattributed   int            `json:"unattributedFailures,omitempty"`
}

// Summary restates the results; narrow rates are only reported with misses.
type Summary struct {
	Merged                int     `json:"merged"`
	NarrowMerged          int     `json:"narrowMerged"`
	NarrowRateMerged      float64 `json:"narrowRateMerged"`
	BoundedMerged         int     `json:"boundedMerged"`
	FailedCI              int     `json:"failedCI"`
	FailingPackages       int     `json:"failingPackages"`
	Misses                int     `json:"misses"`
	MedianCostShareMerged float64 `json:"medianCostShareMerged"`
	MeanCostShareMerged   float64 `json:"meanCostShareMerged"`
	WholePackagesMerged   int     `json:"wholePackagesMerged"`
	SlicedPackagesMerged  int     `json:"slicedPackagesMerged"`
	WholeWithTestsMerged  int     `json:"wholeSourceAndTestsChangedMerged"`
}

type results struct {
	Profile  string   `json:"profile"`
	Changes  string   `json:"changes"`
	Planner  string   `json:"planner"`
	Floor    floor    `json:"floor"`
	Summary  Summary  `json:"summary"`
	Verdicts []string `json:"check"`
	Results  []Result `json:"results"`
}

func main() {
	root := flag.String("root", ".", "repository whose history holds every change")
	changesFile := flag.String("changes", "benchmarks/selection-baseline/changes.json", "fixed change set")
	corvint := flag.String("corvint", "", "corvint binary under measure")
	selector := flag.String("selector", "", "gate-affected-select binary under measure")
	out := flag.String("out", "", "write results JSON here")
	check := flag.Bool("check", false, "fail when the narrow count drops below the floor or a miss appears")
	flag.Parse()
	if *corvint == "" || *selector == "" {
		fmt.Fprintln(os.Stderr, "selection-baseline: --corvint and --selector are required")
		os.Exit(2)
	}
	set, err := readChanges(*changesFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "selection-baseline:", err)
		os.Exit(2)
	}
	r, err := replay(context.Background(), *root, set, *changesFile, abs(*corvint), abs(*selector))
	if err != nil {
		fmt.Fprintln(os.Stderr, "selection-baseline:", err)
		os.Exit(2)
	}
	for _, line := range r.Verdicts {
		fmt.Println(line)
	}
	if *out != "" {
		data, _ := json.MarshalIndent(r, "", "  ")
		if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "selection-baseline:", err)
			os.Exit(2)
		}
	}
	if *check && len(Failures(r.Summary, set.Floor)) > 0 {
		os.Exit(1)
	}
}

func abs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

func readChanges(name string) (changeSet, error) {
	var set changeSet
	data, err := os.ReadFile(name)
	if err != nil {
		return set, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&set); err != nil {
		return set, fmt.Errorf("%s: %w", name, err)
	}
	if set.Profile != changesProfile {
		return set, fmt.Errorf("%s: profile %q, want %q", name, set.Profile, changesProfile)
	}
	seen := map[string]bool{}
	for _, c := range set.Changes {
		if c.ID == "" || seen[c.ID] || !fullOID(c.Base) || !fullOID(c.Head) || (c.Kind != "merged" && c.Kind != "failed-ci") {
			return set, fmt.Errorf("%s: invalid or repeated change %q", name, c.ID)
		}
		seen[c.ID] = true
	}
	return set, nil
}

func fullOID(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func replay(ctx context.Context, root string, set changeSet, changesFile, corvint, selector string) (results, error) {
	r := results{Profile: resultsProfile, Changes: filepath.ToSlash(changesFile), Floor: set.Floor}
	costs, err := readCosts(filepath.Join(root, ".github", "cishards", "package-costs.json"))
	if err != nil {
		return r, err
	}
	version, err := run(ctx, root, corvint, "--version")
	if err != nil {
		return r, err
	}
	r.Planner = strings.TrimSpace(version)
	module, err := moduleOf(filepath.Join(root, "go.mod"))
	if err != nil {
		return r, err
	}
	tmp, err := os.MkdirTemp("", "selection-baseline-")
	if err != nil {
		return r, err
	}
	defer os.RemoveAll(tmp)
	// ASD-V0-925 (V1-3469): the changes replay on jobs workers, each in its own shared clone, so
	// doc-gates stays inside a lane tool window; results and their lines keep the change-set order.
	jobs := replayJobs(len(set.Changes))
	clones := make(chan string, jobs)
	for i := range jobs {
		clone := filepath.Join(tmp, fmt.Sprintf("repo%d", i))
		if _, err := run(ctx, root, "git", "clone", "--quiet", "--shared", "--no-checkout", ".", clone); err != nil {
			return r, err
		}
		clones <- clone
	}
	measured := make([]Result, len(set.Changes))
	errs := make([]error, len(set.Changes))
	var wg sync.WaitGroup
	for i, c := range set.Changes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clone := <-clones
			defer func() { clones <- clone }()
			measured[i], errs[i] = measure(ctx, clone, tmp, module, corvint, selector, c, costs)
		}()
	}
	wg.Wait()
	for i, res := range measured {
		if errs[i] != nil {
			return r, fmt.Errorf("%s: %w", set.Changes[i].ID, errs[i])
		}
		r.Results = append(r.Results, res)
		fmt.Fprintf(os.Stderr, "selection-baseline: %s plan=%s verdict=%s ci=%s packages=%d cost=%.3f whole=%d misses=%d\n",
			res.ID, res.PlanScope, res.Verdict, res.CIScope, len(res.Packages), res.CostShare, res.WholePackages, len(res.Misses))
	}
	r.Summary = Summarize(r.Results)
	r.Verdicts = append(Moved(set.Changes, r.Results), verdictLines(r.Summary, set.Floor)...)
	return r, nil
}

// replayJobs is SELECTION_BASELINE_JOBS when it is a positive integer, else max(1, CPUs/3), at
// most 4 and at most n.
func replayJobs(n int) int {
	jobs := max(1, runtime.NumCPU()/3)
	if v, err := strconv.Atoi(os.Getenv("SELECTION_BASELINE_JOBS")); err == nil && v > 0 {
		jobs = v
	} else {
		jobs = min(jobs, 4)
	}
	return max(1, min(jobs, n))
}

func measure(ctx context.Context, clone, tmp, module, corvint, selector string, c change, costs map[string]float64) (Result, error) {
	res := Result{ID: c.ID, Kind: c.Kind, Base: c.Base, Head: c.Head, PR: c.PR, Failures: c.Failures}
	if _, err := run(ctx, clone, "git", "checkout", "--quiet", "--force", "--detach", c.Head); err != nil {
		return res, err
	}
	if _, err := run(ctx, clone, "git", "clean", "-ffdxq"); err != nil {
		return res, err
	}
	raw, err := run(ctx, clone, corvint, "affected", "--base", c.Base)
	if err != nil {
		return res, err
	}
	var p plan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return res, fmt.Errorf("plan: %w", err)
	}
	planPath := filepath.Join(tmp, c.ID+".plan.json")
	if err := os.WriteFile(planPath, []byte(raw), 0o600); err != nil {
		return res, err
	}
	audit, err := run(ctx, clone, selector, planPath, module, clone)
	if err != nil {
		return res, err
	}
	manifestPath := filepath.Join(tmp, c.ID+".manifest.json")
	if _, err := run(ctx, clone, corvint, "test", "--base", c.Base, "--dry-run", "--manifest", manifestPath); err != nil {
		return res, err
	}
	var m manifest
	data, err := os.ReadFile(manifestPath)
	if err == nil {
		err = json.Unmarshal(data, &m)
	}
	if err != nil {
		return res, fmt.Errorf("manifest: %w", err)
	}
	diff, err := run(ctx, clone, "git", "diff", "--name-only", "--no-renames", "-z", c.Base, c.Head)
	if err != nil {
		return res, err
	}
	for _, name := range strings.Split(diff, "\x00") {
		if strings.HasPrefix(name, ".github/") {
			res.GithubChange = true
		}
	}
	fillPlan(&res, p)
	fillVerdict(&res, audit, costs)
	fillManifest(&res, m)
	checkFailures(&res)
	return res, nil
}

// fillPlan records the planner's scope, counts and the kinds that widened it.
func fillPlan(res *Result, p plan) {
	res.PlanScope = p.Plan.Scope
	res.Dirty = len(p.Plan.Dirty)
	res.Selected = len(p.Plan.Selected)
	res.Unknown = len(p.Plan.Unknown)
	res.Universe = len(p.Plan.Selected) + p.Plan.Excluded.Count
	res.UnknownKinds = map[string]int{}
	for _, u := range p.Plan.Unknown {
		res.UnknownKinds[UnknownKind(u.Reason, u.Detail)]++
	}
	res.WitnessKinds = map[string]int{}
	for _, s := range p.Plan.Selected {
		res.WitnessKinds[s.Witness.Kind]++
	}
}

// UnknownKind names an unknown by its reason and, for a language frontier,
// the frontier's name without its per-site detail.
func UnknownKind(reason, detail string) string {
	if reason != "LANGUAGE_FRONTIER" {
		return reason
	}
	parts := strings.SplitN(detail, ":", 3)
	if len(parts) >= 2 {
		return reason + " " + parts[0] + ":" + parts[1]
	}
	return reason + " " + detail
}

// fillVerdict applies the CI PR-tier rule to the selector's last line.
func fillVerdict(res *Result, audit string, costs map[string]float64) {
	lines := strings.Split(strings.TrimRight(audit, "\n"), "\n")
	last := strings.Fields(lines[len(lines)-1])
	res.Packages = []string{}
	if len(last) > 0 {
		res.Verdict = last[0]
	}
	if res.Verdict != "run" && len(last) > 1 {
		res.VerdictReason = strings.Join(last[1:], " ")
	}
	if res.Verdict == "run" {
		res.Packages = append(res.Packages, last[1:]...)
		sort.Strings(res.Packages)
	}
	res.CIScope = CIScope(res.Verdict, res.GithubChange)
	switch res.CIScope {
	case "FULL":
		res.PackageShare, res.CostShare = 1, 1
	case "NARROW":
		res.PackageShare, res.CostShare = shares(res.Packages, costs)
	}
}

// CIScope is the CI PR tier's decision: a `.github/` change or any verdict
// other than `run` or `NOTHING` runs FULL.
func CIScope(verdict string, githubChange bool) string {
	switch {
	case githubChange:
		return "FULL"
	case verdict == "run":
		return "NARROW"
	case verdict == "NOTHING":
		return "NOTHING"
	}
	return "FULL"
}

func shares(packages []string, costs map[string]float64) (float64, float64) {
	if len(costs) == 0 {
		return 0, 0
	}
	var total, chosen float64
	for _, ms := range costs {
		total += ms
	}
	for _, p := range packages {
		chosen += costs[p]
	}
	return round(float64(len(packages)) / float64(len(costs))), round(chosen / total)
}

func round(f float64) float64 { return float64(int64(f*1000+0.5)) / 1000 }

// fillManifest counts the Go packages `corvint test` runs whole and in slices,
// and the whole ones kept because their source and test files both changed.
func fillManifest(res *Result, m manifest) {
	res.TestScope = m.Scope
	for _, u := range m.Units {
		if u.Adapter != "go" {
			continue
		}
		if u.Slice == "whole" {
			res.WholePackages++
			if strings.Contains(u.Reason, "DIRECT_SOURCE_CHANGE") && strings.Contains(u.Reason, "changed too") {
				res.WholeWithTests++
			}
		} else {
			res.SlicedPackages++
		}
	}
}

// checkFailures lists each failing package a narrowed change does not select.
func checkFailures(res *Result) {
	selected := map[string]bool{}
	for _, p := range res.Packages {
		selected[p] = true
	}
	for _, f := range res.Failures {
		if f.Package == "" {
			res.Unattributed++
			continue
		}
		if res.CIScope == "FULL" || selected[f.Package] {
			continue
		}
		res.Misses = append(res.Misses, f.Package)
		cause := "the selector's run set does not name it"
		if res.CIScope == "NOTHING" {
			cause = "the selector selected nothing"
		}
		res.MissCauses = append(res.MissCauses, f.Package+": "+cause)
	}
}

// Summarize totals the results; cost shares are of merged changes only.
func Summarize(rs []Result) Summary {
	var s Summary
	var costs []float64
	failing := map[string]bool{}
	for _, r := range rs {
		s.Misses += len(r.Misses)
		for _, f := range r.Failures {
			if f.Package != "" {
				failing[r.ID+" "+f.Package] = true
			}
		}
		if r.Kind == "failed-ci" {
			s.FailedCI++
			continue
		}
		s.Merged++
		if r.CIScope != "FULL" {
			s.NarrowMerged++
		}
		if r.PlanScope == "BOUNDED" {
			s.BoundedMerged++
		}
		costs = append(costs, r.CostShare)
		s.WholePackagesMerged += r.WholePackages
		s.SlicedPackagesMerged += r.SlicedPackages
		s.WholeWithTestsMerged += r.WholeWithTests
	}
	s.FailingPackages = len(failing)
	if s.Merged > 0 {
		s.NarrowRateMerged = round(float64(s.NarrowMerged) / float64(s.Merged))
		sort.Float64s(costs)
		var sum float64
		for _, c := range costs {
			sum += c
		}
		s.MeanCostShareMerged = round(sum / float64(len(costs)))
		mid := len(costs) / 2
		s.MedianCostShareMerged = costs[mid]
		if len(costs)%2 == 0 {
			s.MedianCostShareMerged = round((costs[mid-1] + costs[mid]) / 2)
		}
	}
	return s
}

// Moved names each change whose replayed CI scope or verdict differs from the
// one recorded for it, with the selector's reason, and counts the changes
// without a record (TSP-V0-710).
func Moved(changes []change, rs []Result) []string {
	byID := make(map[string]Result, len(rs))
	for _, r := range rs {
		byID[r.ID] = r
	}
	var lines []string
	unrecorded := 0
	for _, c := range changes {
		r, ok := byID[c.ID]
		if !ok {
			continue
		}
		if c.Recorded == nil {
			unrecorded++
			continue
		}
		if c.Recorded.CIScope == r.CIScope && c.Recorded.Verdict == r.Verdict {
			continue
		}
		reason := ""
		if r.VerdictReason != "" {
			reason = ": " + r.VerdictReason
		}
		lines = append(lines, fmt.Sprintf("selection-baseline: MOVED %s ci %s -> %s (verdict %s -> %s%s)",
			c.ID, c.Recorded.CIScope, r.CIScope, c.Recorded.Verdict, r.Verdict, reason))
	}
	if unrecorded > 0 {
		lines = append(lines, fmt.Sprintf("selection-baseline: UNRECORDED %d change(s) carry no recorded verdict", unrecorded))
	}
	return lines
}

// Failures names each way the summary breaks the floor.
func Failures(s Summary, f floor) []string {
	var out []string
	if s.NarrowMerged < f.NarrowMerged {
		out = append(out, fmt.Sprintf("narrowed merged changes %d below the floor %d", s.NarrowMerged, f.NarrowMerged))
	}
	if s.Misses > f.Misses {
		out = append(out, fmt.Sprintf("%d failing packages missed by a narrowed selection (allowed %d)", s.Misses, f.Misses))
	}
	return out
}

func verdictLines(s Summary, f floor) []string {
	lines := []string{fmt.Sprintf("selection-baseline: narrow=%d/%d merged (rate %.3f) bounded=%d misses=%d over %d failing packages of %d failed CI runs; median cost %.3f; whole=%d sliced=%d whole-source-and-tests=%d",
		s.NarrowMerged, s.Merged, s.NarrowRateMerged, s.BoundedMerged, s.Misses, s.FailingPackages, s.FailedCI, s.MedianCostShareMerged, s.WholePackagesMerged, s.SlicedPackagesMerged, s.WholeWithTestsMerged)}
	fails := Failures(s, f)
	for _, line := range fails {
		lines = append(lines, "selection-baseline: FAIL "+line)
	}
	if len(fails) == 0 {
		lines = append(lines, fmt.Sprintf("selection-baseline: OK floor narrow>=%d misses<=%d", f.NarrowMerged, f.Misses))
	}
	return lines
}

func readCosts(name string) (map[string]float64, error) {
	var doc struct {
		Milliseconds map[string]float64 `json:"milliseconds"`
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return doc.Milliseconds, nil
}

func moduleOf(name string) (string, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "module" {
			return f[1], nil
		}
	}
	return "", errors.New("go.mod names no module")
}

// run runs one command in dir and returns its stdout; stderr joins the error.
func run(ctx context.Context, dir, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg, _ := io.ReadAll(io.LimitReader(&stderr, 400))
		return "", fmt.Errorf("%s %s: %w: %s", filepath.Base(name), strings.Join(args, " "), err, strings.TrimSpace(string(msg)))
	}
	return stdout.String(), nil
}
