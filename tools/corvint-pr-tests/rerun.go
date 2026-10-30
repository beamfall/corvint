package main

// TSP-V0-007 (V1-3050): a PR's second CI run reuses the package results its prior run
// recorded. A package runs again when it did not pass in the prior run's tested merge M0,
// or when the trusted planner selects it for the tree delta M0..M1. Every unproven input
// leaves the rerun selection at the whole universe with a named reason.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/.github/cishards"
)

const priorSchema = "corvint-pr-tests-prior/0"
const rerunSchema = "corvint-pr-tests-rerun/0"

// priorRecord is what one completed PR run proves: the packages that passed whole in
// the tested merge Target of PR head Head, under Identity.
type priorRecord struct {
	Schema, Head, Target, Tree string
	Identity                   identity
	Passed                     []string
}

// rerunSelection is the TSP-V0-007 rerun.json. Packages is NotPassed plus Affected, or
// ["./..."] with a Reason. It never replaces selection.json.
type rerunSelection struct {
	Schema, Head, Target, Tree        string
	PriorHead, PriorTarget, PriorTree string
	Reason                            string
	NotPassed, Affected, Packages     []string
	PlanSHA, AuditSHA                 string
}

// priorIdentity is the identity a prior record must share with the rerun: everything
// except the shard count, which does not change a package's result.
func priorIdentity(id identity) identity {
	id.Shards = 0
	return id
}

func readPrior(path string) (priorRecord, error) {
	var p priorRecord
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 8<<20+1))
	if err != nil {
		return p, err
	}
	if len(b) > 8<<20 {
		return p, errors.New("prior record exceeds bound")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&p); err != nil {
		return p, err
	}
	if d.More() {
		return p, errors.New("trailing data")
	}
	if p.Schema != priorSchema || !oid.MatchString(p.Head) || !oid.MatchString(p.Target) || !oid.MatchString(p.Tree) || p.Passed == nil {
		return p, errors.New("invalid prior record")
	}
	for _, name := range p.Passed {
		if !packageName.MatchString(name) || strings.Contains(name, "..") {
			return p, errors.New("invalid passed package")
		}
	}
	return p, nil
}

// rerun computes rerun.json for the checked-out tested merge o.target. s is the deciding
// selection of the same invocation; idOK reports that id is the complete tool identity.
func rerun(ctx context.Context, o options, id identity, idOK bool, s selection) rerunSelection {
	r := rerunSelection{Schema: rerunSchema, Head: o.head, Target: o.target, Tree: s.Tree, Packages: []string{"./..."}}
	if !idOK {
		r.Reason = "tool profile unavailable"
		return r
	}
	if s.Tree == "" {
		r.Reason = "plan: " + s.Reason
		return r
	}
	p, err := readPrior(o.prior)
	if err != nil {
		r.Reason = "prior-record-unreadable"
		return r
	}
	r.PriorHead, r.PriorTarget, r.PriorTree = p.Head, p.Target, p.Tree
	if !equal(priorIdentity(p.Identity), priorIdentity(id)) {
		r.Reason = "prior-identity-mismatch"
		return r
	}
	tree, err := git(ctx, o, "rev-parse", "--verify", "--end-of-options", p.Target+"^{tree}")
	if err != nil || tree != p.Tree {
		r.Reason = "prior-record-mismatch"
		return r
	}
	parents, err := git(ctx, o, "show", "-s", "--format=%P", p.Target)
	if pp := strings.Fields(parents); err != nil || len(pp) != 2 || pp[1] != p.Head {
		r.Reason = "prior-record-mismatch"
		return r
	}
	if _, err = git(ctx, o, "merge-base", "--is-ancestor", p.Head, o.head); err != nil {
		r.Reason = "prior-head-not-ancestor"
		return r
	}
	delta, err := git(ctx, o, "diff", "--no-renames", "--name-only", "-z", p.Target, o.target)
	if err != nil {
		r.Reason = "prior-delta-unreadable"
		return r
	}
	for _, path := range strings.Split(delta, "\x00") {
		if path == ".github" || strings.HasPrefix(path, ".github/") {
			r.Reason = "prior-delta-touches-github"
			return r
		}
	}
	affected, planSHA, auditSHA, reason := selectSince(ctx, o, p.Target, "rerun-")
	r.PlanSHA, r.AuditSHA = planSHA, auditSHA
	if reason != "" {
		r.Reason = reason
		return r
	}
	listed, err := capture(ctx, o, "go", "list", "./...")
	if err != nil {
		r.Reason = "package universe unavailable"
		return r
	}
	universe, err := cishards.Packages([]byte(listed))
	if err != nil {
		r.Reason = "package universe unavailable"
		return r
	}
	o.base = s.Base
	if check, err := topology(ctx, o, true); err != nil || check != s.Tree {
		r.Reason = "source drift after planning"
		return r
	}
	passed := map[string]bool{}
	for _, name := range p.Passed {
		passed[name] = true
	}
	run := map[string]bool{}
	r.NotPassed = []string{}
	for _, name := range universe {
		if !passed[name] {
			r.NotPassed = append(r.NotPassed, name)
			run[name] = true
		}
	}
	r.Affected = sorted(affected)
	for _, name := range affected {
		run[name] = true
	}
	r.Packages = []string{}
	for name := range run {
		r.Packages = append(r.Packages, name)
	}
	sort.Strings(r.Packages)
	return r
}

// priorEvent is the part of a `go test -json` event that a prior record reads.
type priorEvent struct {
	Action, Package, Test string
}

// buildPrior writes prior.json from the retained output directories of one completed
// run: o.evidence holds execution.json and go.json directly, or one directory per shard.
// A package is passed only when some shard reports a package-level pass and none
// reports a fail; a package any shard ran in slices is never passed.
func buildPrior(o options) error {
	dirs := []string{o.evidence}
	if _, err := os.Stat(filepath.Join(o.evidence, "execution.json")); os.IsNotExist(err) {
		entries, err := os.ReadDir(o.evidence)
		if err != nil {
			return err
		}
		dirs = nil
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, filepath.Join(o.evidence, e.Name()))
			}
		}
	}
	if len(dirs) == 0 || len(dirs) > cishards.MaxShards {
		return errors.New("prior evidence needs 1..64 run directories")
	}
	var p priorRecord
	pass, fail, sliced := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i, dir := range dirs {
		var e execution
		if err := readJSON(filepath.Join(dir, "execution.json"), &e); err != nil {
			return fmt.Errorf("%s: %w", dir, err)
		}
		if e.Error != "" {
			return fmt.Errorf("%s: execution error %q", dir, e.Error)
		}
		s := e.Selection
		record := priorRecord{Schema: priorSchema, Head: s.Head, Target: s.Target, Tree: s.Tree, Identity: priorIdentity(s.Identity)}
		if s.ObservedTarget != s.Target || s.ObservedTree != s.Tree || !oid.MatchString(s.Head) || !oid.MatchString(s.Target) || !oid.MatchString(s.Tree) {
			return fmt.Errorf("%s: execution did not observe its tested merge", dir)
		}
		if i == 0 {
			p = record
		} else if !equal(p, record) {
			return fmt.Errorf("%s: run directories disagree on head, merge, tree, or identity", dir)
		}
		for _, slice := range s.ShardSlices {
			sliced[slice.Package] = true
		}
		for _, run := range e.Slices {
			sliced[run.Package] = true
		}
		if err := readPriorEvents(filepath.Join(dir, "go.json"), pass, fail); err != nil {
			return fmt.Errorf("%s: %w", dir, err)
		}
	}
	p.Passed = []string{}
	for name := range pass {
		if !fail[name] && !sliced[name] && packageName.MatchString(name) {
			p.Passed = append(p.Passed, name)
		}
	}
	sort.Strings(p.Passed)
	return writeJSON(filepath.Join(o.out, "prior.json"), p)
}

func readPriorEvents(path string, pass, fail map[string]bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	lines := bufio.NewScanner(io.LimitReader(f, maxStdout+1))
	lines.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for lines.Scan() {
		var ev priorEvent
		if err := json.Unmarshal(lines.Bytes(), &ev); err != nil {
			return errors.New("unreadable test event")
		}
		if ev.Test != "" || ev.Package == "" {
			continue
		}
		switch ev.Action {
		case "pass":
			pass[ev.Package] = true
		case "fail":
			fail[ev.Package] = true
		}
	}
	return lines.Err()
}
