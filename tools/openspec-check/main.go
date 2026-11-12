// SPDX-License-Identifier: Apache-2.0

// Command openspec-check checks the OpenSpec change folders under docs/specs/openspec/changes/
// and their propose, apply and archive stages (docs/specs/openspec-check-v1.md, OSC-V1-001 to
// OSC-V1-011). Run from the repository root, it checks the layout of every active change folder,
// the requirement IDs of every spec delta, tick order and the canonical spec of every ticked task,
// the archive state and history of every change, the one OpenSpec root, the Health: line of every
// spec under docs/specs/ and the acceptance link of every proposal. With --base REV it also checks
// every non-merge commit of REV..HEAD: each commit that touches a path outside docs/ binds to a
// change whose folder its first parent already holds, or names an accepted requirement, when it
// opts in to a change. A finding listed in the baseline file by rule, subject and finding key is
// reported and does not fail; the baseline only shrinks. It reads files and Git history only; it writes nothing and calls no network service,
// no npm and no OpenSpec command.
//
// Exit status: 0 when nothing fails, 1 when a rule fails, 2 on a usage error or a refusal
// (history-unavailable, requirements-stale).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	changesDir      = "docs/specs/openspec/changes"
	openspecRoot    = "docs/specs/openspec"
	requirementsTSV = "docs/specs/REQUIREMENTS.tsv"
	acceptanceLog   = "docs/specs/openspec/acceptance-log.tsv"
	defaultBaseline = "tools/openspec-check/baseline.txt"
	generator       = "script/gen-spec-requirements.sh"
)

// finding is one rule result. subject is "change NAME", "spec PATH" or empty for a finding no
// baseline can hold (a boundary or commit finding).
type finding struct {
	req     string
	subject string
	msg     string
}

func (f finding) String() string {
	if f.subject == "" {
		return f.req + " " + f.msg
	}
	return f.req + " " + f.subject + ": " + f.msg
}

// report is one listed status that never fails and never counts as a pass.
type report struct {
	status  string
	subject string
	msg     string
}

// refusal stops the check before any rule runs; code names it.
type refusal struct {
	code string
	msg  string
}

func (r *refusal) Error() string { return r.code + ": " + r.msg }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("openspec-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	base := flags.String("base", "", "check the commits of BASE..HEAD as well (OSC-V1-003, OSC-V1-004)")
	baselinePath := flags.String("baseline", defaultBaseline, "baseline file, relative to the root")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: go run ./tools/openspec-check [--root DIR] [--base REV] [--baseline FILE]")
		fmt.Fprintln(stderr, "example: go run ./tools/openspec-check --base origin/main")
		flags.PrintDefaults()
	}
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	result, err := check(*root, *base, *baselinePath)
	if err != nil {
		var refused *refusal
		if errors.As(err, &refused) {
			fmt.Fprintf(stderr, "openspec-check: REFUSE %s: %s\n", refused.code, refused.msg)
			return 2
		}
		fmt.Fprintln(stderr, "openspec-check:", err)
		return 2
	}
	return result.print(stdout)
}

type result struct {
	failures  []finding
	baselined []finding
	reports   []report
	notRun    []string
}

func (r *result) print(w io.Writer) int {
	for _, n := range r.notRun {
		fmt.Fprintf(w, "openspec-check: NOT_RUN %s\n", n)
	}
	for _, rep := range r.reports {
		fmt.Fprintf(w, "openspec-check: REPORT %s %s: %s\n", rep.status, rep.subject, rep.msg)
	}
	for _, f := range r.baselined {
		fmt.Fprintf(w, "openspec-check: BASELINE %s\n", f)
	}
	for _, f := range r.failures {
		fmt.Fprintf(w, "openspec-check: FAIL %s\n", f)
	}
	subjects := map[string]bool{}
	for _, f := range r.baselined {
		subjects[f.subject] = true
	}
	summary := fmt.Sprintf("%d failure(s), %d baselined finding(s) in %d baselined subject(s), %d report(s)",
		len(r.failures), len(r.baselined), len(subjects), len(r.reports))
	if len(r.failures) > 0 {
		fmt.Fprintf(w, "openspec-check: FAIL (%s); fix each FAIL line, or for a stale baseline entry delete "+
			"its line from the baseline file (it only shrinks)\n", summary)
		return 1
	}
	fmt.Fprintf(w, "openspec-check: PASS (%s)\n", summary)
	return 0
}

// check runs every rule. It refuses before any rule when the history it needs is unavailable or
// REQUIREMENTS.tsv is stale (OSC-V1-011).
func check(root, base, baselinePath string) (*result, error) {
	g := gitRepo{root: root}
	if err := g.preflight(base); err != nil {
		return nil, err
	}
	res := &result{}
	if stale, ran, err := requirementsStale(root); err != nil {
		return nil, err
	} else if !ran {
		res.notRun = append(res.notRun, "requirements-stale check: "+generator+" is absent")
	} else if stale != "" {
		return nil, &refusal{"requirements-stale", stale}
	}
	repo, err := loadRepo(root)
	if err != nil {
		return nil, err
	}
	var findings []finding
	findings = append(findings, boundaryRules(root)...)
	findings = append(findings, layoutRules(repo)...)
	idFindings, err := idRules(repo, g)
	if err != nil {
		return nil, err
	}
	findings = append(findings, idFindings...)
	applyFindings, reports := applyRules(repo)
	findings = append(findings, applyFindings...)
	res.reports = append(res.reports, reports...)
	findings = append(findings, archiveStateRules(repo)...)
	historyFindings, err := archiveHistoryRules(repo, g)
	if err != nil {
		return nil, err
	}
	findings = append(findings, historyFindings...)
	findings = append(findings, healthRules(root)...)
	findings = append(findings, acceptanceRules(repo)...)
	if base == "" {
		res.notRun = append(res.notRun, "commit rules OSC-V1-003 and OSC-V1-004: no --base given")
	} else {
		commitFindings, err := commitRules(repo, g, base)
		if err != nil {
			return nil, err
		}
		findings = append(findings, commitFindings...)
	}
	entries, baselineFindings, err := loadBaseline(root, baselinePath)
	if err != nil {
		return nil, err
	}
	findings = append(findings, baselineFindings...)
	if base != "" {
		grown, err := baselineGrowth(g, base, baselinePath, entries)
		if err != nil {
			return nil, err
		}
		findings = append(findings, grown...)
	}
	matched := map[string]bool{}
	for _, f := range findings {
		if key := baselineKey(f); key != "" && entries[key] {
			res.baselined = append(res.baselined, f)
			matched[key] = true
			continue
		}
		res.failures = append(res.failures, f)
	}
	stale := []string{}
	for entry := range entries {
		if !matched[entry] {
			stale = append(stale, entry)
		}
	}
	sort.Strings(stale)
	for _, entry := range stale {
		res.failures = append(res.failures, finding{req: "OSC-V1-011", msg: fmt.Sprintf(
			"baseline entry %q no longer fails: delete its line from %s (the baseline only shrinks)",
			entry, baselinePath)})
	}
	return res, nil
}

// requirementsStale regenerates REQUIREMENTS.tsv from the staged specs and compares it with the
// file. ran is false when the generator is absent (a fixture repository).
func requirementsStale(root string) (stale string, ran bool, err error) {
	if _, statErr := os.Stat(filepath.Join(root, generator)); statErr != nil {
		return "", false, nil
	}
	out, runErr := runCommand(root, "sh", generator)
	if runErr != nil {
		return "", true, &refusal{"requirements-stale", fmt.Sprintf(
			"%s failed (%v); fix the spec it names, then run make spec-requirements", generator, runErr)}
	}
	current, readErr := os.ReadFile(filepath.Join(root, requirementsTSV))
	if readErr != nil {
		return "", true, &refusal{"requirements-stale", fmt.Sprintf("cannot read %s: %v", requirementsTSV, readErr)}
	}
	if string(current) != out {
		return fmt.Sprintf("%s differs from what %s generates from the staged specs; "+
			"stage the specs and run make spec-requirements", requirementsTSV, generator), true, nil
	}
	return "", true, nil
}

func subjectChange(name string) string { return "change " + name }
func subjectSpec(path string) string   { return "spec " + path }

// short trims a commit object name for messages.
func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func listPaths(paths []string) string {
	const limit = 5
	if len(paths) <= limit {
		return strings.Join(paths, ", ")
	}
	return strings.Join(paths[:limit], ", ") + fmt.Sprintf(" and %d more", len(paths)-limit)
}
