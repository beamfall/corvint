package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type pair struct{ Base, Target string }
type corpus struct {
	Schema   string
	Identity identity
	Pairs    []pair
}
type row struct {
	Pair     pair
	Identity identity
	Valid    bool
	Error    string
	Misses   []string
	Files    map[string]string
}
type qualification struct {
	Schema    string
	Identity  identity
	CorpusSHA string
	Rows      []row
	Verdict   string
}

var rowFiles = []string{"plan.json", "selector.txt", "selection.json", "expected.json", "go.json", "go.stderr", "execution.json"}

func freeze(ctx context.Context, o options) error {
	if err := prepareCache(o); err != nil {
		return err
	}
	id, err := toolIdentity(ctx, o)
	if err != nil {
		return err
	}
	if !oid.MatchString(o.target) {
		return errors.New("freeze requires full target OID")
	}
	history, err := git(ctx, o, "rev-list", "--first-parent", "--max-count=201", o.target)
	if err != nil {
		return err
	}
	commits := strings.Fields(history)
	if len(commits) != 201 {
		return errors.New("exactly 201 commits required for 200 distinct pairs")
	}
	c := corpus{Schema: schema, Identity: id}
	for i := 199; i >= 0; i-- {
		c.Pairs = append(c.Pairs, pair{Base: commits[i+1], Target: commits[i]})
	}
	return writeJSON(filepath.Join(o.out, "corpus.json"), c)
}
func validateCorpus(c corpus) error {
	if c.Schema != schema || len(c.Pairs) != 200 {
		return errors.New("corpus must contain exactly 200 pairs")
	}
	seen := map[string]bool{}
	for i, p := range c.Pairs {
		if !oid.MatchString(p.Base) || !oid.MatchString(p.Target) || p.Base == p.Target || seen[p.Target] {
			return errors.New("invalid or duplicate frozen pair")
		}
		if i > 0 && p.Base != c.Pairs[i-1].Target {
			return errors.New("corpus is not contiguous and ordered")
		}
		seen[p.Target] = true
	}
	return nil
}
func shadow(ctx context.Context, o options) error {
	indices, err := shadowIndices(o)
	if err != nil {
		return err
	}
	var c corpus
	if err := readJSON(o.corpus, &c); err != nil {
		return err
	}
	if err := validateCorpus(c); err != nil {
		return err
	}
	if err := prepareCache(o); err != nil {
		return err
	}
	id, err := toolIdentity(ctx, o)
	if err != nil {
		return err
	}
	if !equal(id, c.Identity) {
		return errors.New("frozen tool/profile identity mismatch")
	}
	// The output directory owns this checkout. Never reset or clean the input repository.
	checkout := shadowCheckout(o)
	marker := filepath.Join(o.out, "checkout-owner.json")
	if _, err = os.Stat(checkout); os.IsNotExist(err) {
		if err = cloneRepository(ctx, o, checkout); err != nil {
			return err
		}

		if err = writeJSON(marker, map[string]string{"source": o.root, "checkout": checkout}); err != nil {
			return err
		}
	}
	var owner map[string]string
	if err = readJSON(marker, &owner); err != nil {
		return errors.New("refusing an unowned existing checkout")
	}
	if owner["source"] != o.root || owner["checkout"] != checkout {
		return errors.New("checkout owner marker mismatch")
	}
	origin, err := capture(ctx, options{root: checkout, out: o.out, runtime: o.runtime}, "git", "remote", "get-url", "origin")
	if err != nil || origin != o.root {
		return errors.New("owned checkout origin mismatch")
	}
	for _, i := range indices {
		p := c.Pairs[i]
		if ctx.Err() != nil {
			return ctx.Err()
		}
		dir := filepath.Join(o.out, fmt.Sprintf("row-%03d", i+1))
		result := filepath.Join(dir, "row.json")
		if _, err = os.Stat(result); err == nil {
			var r row
			if err = readJSON(result, &r); err != nil {
				return err
			}
			if err = validateRow(dir, r, p, id); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		ro := o
		ro.root = checkout
		ro.out = dir
		ro.base = p.Base
		ro.target = p.Target
		ro.head = ""
		if err = resetPrivateRuntime(ro); err != nil {
			return err
		}
		if _, err = git(ctx, ro, "checkout", "--detach", "--force", p.Target); err != nil {
			return err
		}
		if _, err = git(ctx, ro, "clean", "-ffdqx"); err != nil {
			return err
		}
		r := observeRow(ctx, ro, id)
		if err = writeJSON(result, r); err != nil {
			return err
		}
		if !r.Valid {
			return fmt.Errorf("row %d invalid (retained, no replacement): %s", i+1, r.Error)
		}
		if len(r.Misses) > 0 {
			return fmt.Errorf("row %d selected-set miss: %v", i+1, r.Misses)
		}
	}
	return nil
}
func observeRow(ctx context.Context, o options, id identity) row {
	r := row{Pair: pair{o.base, o.target}, Identity: id, Files: map[string]string{}, Misses: []string{}}
	s := plan(ctx, o, id, false)
	if s.Tree == "" {
		r.Error = "historical topology invalid: " + s.Reason
		return r
	}
	if err := writeJSON(filepath.Join(o.out, "selection.json"), s); err != nil {
		r.Error = err.Error()
		return r
	}
	universe, err := capture(ctx, o, "go", "list", "./...")
	if err != nil {
		r.Error = "package universe: " + err.Error()
		return r
	}
	expected := strings.Fields(universe)
	if len(expected) == 0 {
		r.Error = "empty package universe"
		return r
	}
	if err = writeJSON(filepath.Join(o.out, "expected.json"), expected); err != nil {
		r.Error = err.Error()
		return r
	}
	tree, err := topology(ctx, o, false)
	if err != nil || tree != s.Tree {
		r.Error = "source drift after package universe"
		return r
	}
	// Freeze the candidate before one full invocation. No selected execution is claimed.
	full := s
	full.Packages = []string{"./..."}
	full.Reason = "historical shadow: full-suite observation"
	_, err = execute(ctx, o, full)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	if err = writeJSON(filepath.Join(o.out, "selection.json"), s); err != nil {
		r.Error = err.Error()
		return r
	}
	tree, err = topology(ctx, o, false)
	if err != nil || tree != s.Tree {
		r.Error = "source drift during full observation"
		return r
	}
	var e execution
	if err = readJSON(filepath.Join(o.out, "execution.json"), &e); err != nil {
		r.Error = err.Error()
		return r
	}
	failed, err := outcomes(filepath.Join(o.out, "go.json"), expected, e)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	selected := map[string]bool{}
	for _, p := range s.Packages {
		selected[p] = true
	}
	for _, p := range failed {
		if !selected["./..."] && !selected[p] {
			r.Misses = append(r.Misses, p)
		}
	}
	// Any omitted failure kills promotion; UNKNOWN is preserved, never an automatic waiver.
	for _, name := range rowFiles {
		h, err := digestFile(filepath.Join(o.out, name))
		if err != nil {
			if name == "plan.json" || name == "selector.txt" {
				continue
			}
			r.Error = err.Error()
			return r
		}
		r.Files[name] = h
	}
	r.Valid = true
	return r
}
func outcomes(path string, expected []string, e execution) ([]string, error) {
	if e.Error != "" || e.Exit < 0 || e.Exit > 1 {
		return nil, errors.New("infrastructure/timeout exit")
	}
	want := map[string]bool{}
	for _, p := range expected {
		if want[p] {
			return nil, errors.New("duplicate expected package")
		}
		want[p] = true
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 65536), 8<<20)
	terminal := map[string]string{}
	failed := []string{}
	for scan.Scan() {
		var event struct{ Action, Package, Test, Output string }
		if err = json.Unmarshal(scan.Bytes(), &event); err != nil {
			return nil, errors.New("malformed test JSON")
		}
		if event.Action == "build-fail" || strings.Contains(event.Output, "[build failed]") {
			return nil, errors.New("unbuildable historical target")
		}
		if event.Test != "" {
			continue
		}
		if !want[event.Package] && (event.Action == "pass" || event.Action == "skip" || event.Action == "fail") {
			return nil, errors.New("unexpected package terminal")
		}
		if !want[event.Package] {
			continue
		}
		if event.Action != "pass" && event.Action != "skip" && event.Action != "fail" {
			continue
		}
		if terminal[event.Package] != "" {
			return nil, errors.New("duplicate package terminal")
		}
		terminal[event.Package] = event.Action
		if event.Action == "fail" {
			failed = append(failed, event.Package)
		}
	}
	if err = scan.Err(); err != nil {
		return nil, err
	}
	if len(terminal) != len(want) {
		return nil, errors.New("incomplete package-universe terminal outcomes")
	}
	if (e.Exit == 1) != (len(failed) > 0) {
		return nil, errors.New("exit/outcome mismatch")
	}
	return sorted(failed), nil
}
func validateRow(dir string, r row, p pair, id identity) error {
	if !r.Valid || r.Error != "" || len(r.Misses) > 0 || r.Pair != p || !equal(r.Identity, id) {
		return errors.New("invalid/mismatched row or selected-set miss")
	}
	for _, name := range []string{"selection.json", "expected.json", "go.json", "go.stderr", "execution.json"} {
		if r.Files[name] == "" {
			return errors.New("missing required row digest")
		}
	}
	for name, h := range r.Files {
		allowed := false
		for _, known := range rowFiles {
			if name == known {
				allowed = true
			}
		}
		if !allowed {
			return errors.New("unexpected row filename")
		}
		actual, err := digestFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if h != actual {
			return errors.New("row bytes changed")
		}
	}
	var expected []string
	var e execution
	var s selection
	if err := readJSON(filepath.Join(dir, "expected.json"), &expected); err != nil {
		return err
	}
	if err := readJSON(filepath.Join(dir, "execution.json"), &e); err != nil {
		return err
	}
	if err := readJSON(filepath.Join(dir, "selection.json"), &s); err != nil {
		return err
	}
	if s.Base != p.Base || s.Target != p.Target || !equal(s.Identity, id) || !equal(e.Args, append(append([]string{}, testArgs...), "./...")) {
		return errors.New("row source/profile mismatch")
	}

	if e.Selection.Base != p.Base || e.Selection.Target != p.Target || e.Selection.Tree != s.Tree || e.Selection.ObservedTarget != p.Target || e.Selection.ObservedTree != s.Tree || !equal(e.Selection.Identity, id) || !equal(e.Selection.Packages, []string{"./..."}) || e.Selection.Reason != "historical shadow: full-suite observation" || !equal(e.Env, id.Env) {
		return errors.New("execution selection/environment link mismatch")
	}
	for _, link := range []struct{ name, hash string }{{"plan.json", s.PlanSHA}, {"selector.txt", s.AuditSHA}} {
		actual, present := r.Files[link.name]
		if link.hash != "" && (!present || actual != link.hash) {
			return errors.New("raw plan/audit link mismatch")
		}
		if link.hash == "" && (present || s.Reason == "") {
			return errors.New("unexplained missing plan/audit link")
		}
	}
	if e.Selection.PlanSHA != s.PlanSHA || e.Selection.AuditSHA != s.AuditSHA {
		return errors.New("execution plan/audit link mismatch")
	}
	failed, err := outcomes(filepath.Join(dir, "go.json"), expected, e)
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, pkg := range s.Packages {
		selected[pkg] = true
	}
	for _, pkg := range failed {
		if !selected["./..."] && !selected[pkg] {
			return errors.New("recomputed selected-set miss")
		}
	}
	return nil
}
func qualify(o options) error {
	var c corpus
	if err := readJSON(o.corpus, &c); err != nil {
		return err
	}
	if err := validateCorpus(c); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	selfHash, err := digestFile(self)
	if err != nil {
		return err
	}
	if o.profile != "" {
		var p containerProfile
		if err := readJSON(o.profile, &p); err != nil {
			return err
		}
		if err := p.validate(); err != nil {
			return err
		}
		if !equal(c.Identity.Container, &p) {
			return errors.New("qualifier container profile mismatch")
		}
	} else if c.Identity.Container != nil {
		return errors.New("container qualification requires inspected profile")
	}
	if selfHash != c.Identity.Driver {
		return errors.New("qualifier differs from frozen runner")
	}
	h, err := digestFile(o.corpus)
	if err != nil {
		return err
	}
	q := qualification{Schema: schema, Identity: c.Identity, CorpusSHA: h, Verdict: "PASS"}
	for i, p := range c.Pairs {
		rowsRoot := o.evidence
		if rowsRoot == "" {
			rowsRoot = o.out
		}
		dir := filepath.Join(rowsRoot, fmt.Sprintf("row-%03d", i+1))
		var r row
		if err = readJSON(filepath.Join(dir, "row.json"), &r); err != nil {
			return err
		}
		if err = validateRow(dir, r, p, c.Identity); err != nil {
			return fmt.Errorf("row %d: %w", i+1, err)
		}
		q.Rows = append(q.Rows, r)
	}
	return writeJSON(filepath.Join(o.out, "qualification.json"), q)
}
func admit(o options, id identity) error {
	if len(o.qualificationSHA) != 64 || o.qualification == "" {
		return errors.New("no trusted qualification pin")
	}
	resolved, err := filepath.EvalSymlinks(o.qualification)
	if err != nil {
		return err
	}
	if within(o.root, resolved) {
		return errors.New("qualification belongs to untrusted checkout")
	}
	h, err := digestFile(o.qualification)
	if err != nil {
		return err
	}
	if h != o.qualificationSHA {
		return errors.New("qualification digest mismatch")
	}
	var q qualification
	if err = readJSON(o.qualification, &q); err != nil {
		return err
	}
	if q.Schema != schema || q.Verdict != "PASS" || len(q.Rows) != 200 || !equal(id, q.Identity) {
		return errors.New("qualification profile/verdict mismatch")
	}
	c := corpus{Schema: schema}
	for _, r := range q.Rows {
		if !r.Valid || r.Error != "" || len(r.Misses) > 0 || !equal(r.Identity, id) {
			return errors.New("invalid qualification row")
		}
		for _, f := range []string{"selection.json", "expected.json", "go.json", "go.stderr", "execution.json"} {
			if len(r.Files[f]) != 64 {
				return errors.New("missing row digest")
			}
		}
		c.Pairs = append(c.Pairs, r.Pair)
	}
	return validateCorpus(c)
}

func shadowIndices(o options) ([]int, error) {
	if o.row != 0 {
		if o.row < 1 || o.row > 200 || o.rowsSet {
			return nil, errors.New("one --row 1..200 required, exclusive with --rows")
		}
		return []int{o.row - 1}, nil
	}
	if o.rows < 1 || o.rows > 200 {
		return nil, errors.New("rows must be 1..200")
	}
	indices := make([]int, o.rows)
	for i := range indices {
		indices[i] = i
	}
	return indices, nil
}

func shadowCheckout(o options) string {
	if o.profile != "" {
		return containerCheckout
	}
	return filepath.Join(o.out, "checkout")
}
