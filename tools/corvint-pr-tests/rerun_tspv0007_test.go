//go:build darwin || linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/.github/cishards"
)

// TSP-V0-007: a prior record holds only whole-package passes that no run directory
// contradicts, and refuses evidence that does not describe one observed tested merge.
func TestTSPV0007PriorRecordFromRunEvidence(t *testing.T) {
	head, target, tree := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	id := identity{Driver: "d", GoVersion: "go1.27.3"}
	write := func(t *testing.T, dir string, e execution, events string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := writeJSON(filepath.Join(dir, "execution.json"), e); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.json"), []byte(events), 0600); err != nil {
			t.Fatal(err)
		}
	}
	shard := func(n int) execution {
		s := selection{Head: head, Target: target, Tree: tree, ObservedTarget: target, ObservedTree: tree, Identity: id, Shard: n, Shards: 2}
		s.Identity.Shards = 2
		return execution{Selection: s}
	}
	valid := func(t *testing.T) string {
		evidence := t.TempDir()
		zero := shard(0)
		zero.Selection.ShardSlices = []cishards.Slice{{Package: "m/c"}}
		write(t, filepath.Join(evidence, "shard-0"), zero, `{"Action":"pass","Package":"m/a"}
{"Action":"pass","Package":"m/c"}
{"Action":"pass","Package":"m/e","Test":"TestOnly"}
`)
		one := shard(1)
		one.Slices = []sliceRun{{Package: "m/f"}}
		write(t, filepath.Join(evidence, "shard-1"), one, `{"Action":"fail","Package":"m/d"}
{"Action":"pass","Package":"m/b"}
{"Action":"pass","Package":"m/b","Test":"TestX"}
{"Action":"pass","Package":"m/f"}
{"Action":"skip","Package":"m/g"}
`)
		return evidence
	}
	o := options{evidence: valid(t), out: t.TempDir()}
	if err := buildPrior(o); err != nil {
		t.Fatal(err)
	}
	p, err := readPrior(filepath.Join(o.out, "prior.json"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Head != head || p.Target != target || p.Tree != tree || p.Identity.Shards != 0 || !equal(p.Passed, []string{"m/a", "m/b"}) {
		t.Fatalf("prior record: %+v", p)
	}
	for _, tc := range []struct {
		name  string
		spoil func(t *testing.T, evidence string)
	}{
		{"empty", func(t *testing.T, evidence string) {
			for _, d := range []string{"shard-0", "shard-1"} {
				if err := os.RemoveAll(filepath.Join(evidence, d)); err != nil {
					t.Fatal(err)
				}
			}
		}},
		{"other-merge", func(t *testing.T, evidence string) {
			e := shard(1)
			e.Selection.Target, e.Selection.ObservedTarget = head, head
			write(t, filepath.Join(evidence, "shard-1"), e, "")
		}},
		{"unobserved-merge", func(t *testing.T, evidence string) {
			e := shard(1)
			e.Selection.ObservedTree = head
			write(t, filepath.Join(evidence, "shard-1"), e, "")
		}},
		{"other-identity", func(t *testing.T, evidence string) {
			e := shard(1)
			e.Selection.Identity.GoVersion = "go0"
			write(t, filepath.Join(evidence, "shard-1"), e, "")
		}},
		{"execution-error", func(t *testing.T, evidence string) {
			e := shard(1)
			e.Error = "test output limit exceeded"
			write(t, filepath.Join(evidence, "shard-1"), e, "")
		}},
		{"unreadable-event", func(t *testing.T, evidence string) {
			write(t, filepath.Join(evidence, "shard-1"), shard(1), "{\"Action\":\"pass\",\"Package\":\"m/b\"}\n{truncated")
		}},
		{"missing-events", func(t *testing.T, evidence string) {
			if err := os.Remove(filepath.Join(evidence, "shard-1", "go.json")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := options{evidence: valid(t), out: t.TempDir()}
			tc.spoil(t, o.evidence)
			if err := buildPrior(o); err == nil {
				t.Fatal("broken evidence produced a prior record")
			}
			if _, err := os.Stat(filepath.Join(o.out, "prior.json")); !os.IsNotExist(err) {
				t.Fatalf("broken evidence wrote prior.json: %v", err)
			}
		})
	}
}

// TSP-V0-007: the rerun runs the packages that did not pass in M0 plus those the
// planner selects for M0..M1, and every unproven input runs the whole universe.
func TestTSPV0007RerunReusesPriorPasses(t *testing.T) {
	o, id := fixture(t)
	o.mode = "plan"
	ctx := context.Background()
	g := func(args ...string) string {
		t.Helper()
		v, err := git(ctx, o, args...)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	edit := func(name, value string) {
		t.Helper()
		path := filepath.Join(o.root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		g("add", ".")
		g("commit", "-qm", name)
	}
	merge := func(head string) string {
		return g("commit-tree", g("rev-parse", head+"^{tree}"), "-p", o.base, "-p", head, "-m", "merge")
	}
	// Run 1: the PR changes a (whose test now fails) and b's test; b passes.
	g("checkout", "-q", "--detach", o.head)
	edit("b/b_test.go", "package b\nimport \"testing\"\nfunc TestOther(t *testing.T) {}\nfunc TestMore(t *testing.T) {}\n")
	h0 := g("rev-parse", "HEAD")
	m0 := merge(h0)
	// Run 2: the fix touches only a.
	edit("a/a.go", "package a\nfunc Value() int { return 2 - 1 }\n")
	h1 := g("rev-parse", "HEAD")
	m1 := merge(h1)
	g("checkout", "-q", "--detach", m1)
	o.head, o.target = h1, m1
	record := priorRecord{Schema: priorSchema, Head: h0, Target: m0, Tree: g("rev-parse", m0+"^{tree}"), Identity: priorIdentity(id), Passed: []string{"example.org/fixture/b"}}
	put := func(t *testing.T, p priorRecord) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "prior.json")
		if err := writeJSON(path, p); err != nil {
			t.Fatal(err)
		}
		return path
	}
	o.prior = put(t, record)
	if code, err := planOnly(ctx, o); err != nil || code != 0 {
		t.Fatalf("plan mode: %d %v", code, err)
	}
	var s selection
	if err := readJSON(filepath.Join(o.out, "selection.json"), &s); err != nil {
		t.Fatal(err)
	}
	if s.Reason != "" || !equal(s.Packages, []string{"example.org/fixture/a", "example.org/fixture/b"}) {
		t.Fatalf("the deciding selection must stay base-relative: %+v", s)
	}
	var r rerunSelection
	if err := readJSON(filepath.Join(o.out, "rerun.json"), &r); err != nil {
		t.Fatal(err)
	}
	a := []string{"example.org/fixture/a"}
	if r.Reason != "" || r.PriorTarget != m0 || !equal(r.Affected, a) || !equal(r.NotPassed, a) || !equal(r.Packages, a) || r.PlanSHA == "" || r.AuditSHA == "" {
		t.Fatalf("rerun must run only a: %+v", r)
	}

	valid := o
	full := []string{"./..."}
	both := []string{"example.org/fixture/a", "example.org/fixture/b"}
	for _, tc := range []struct{ passed, notPassed, packages []string }{
		{[]string{}, both, both}, // nothing passed: b runs although M0..M1 leaves it alone
		{both, []string{}, a},    // everything passed: only the changed a runs
		{a, []string{"example.org/fixture/b"}, both},
	} {
		p := record
		p.Passed = tc.passed
		o := valid
		o.prior = put(t, p)
		r := rerun(ctx, o, id, true, s)
		if r.Reason != "" || !equal(r.NotPassed, tc.notPassed) || !equal(r.Packages, tc.packages) {
			t.Fatalf("passed %v: %+v", tc.passed, r)
		}
	}
	sibling := func() priorRecord {
		g("checkout", "-q", "--detach", o.base)
		edit("a/a.go", "package a\nfunc Value() int { return 3 }\n")
		x := g("rev-parse", "HEAD")
		g("checkout", "-q", "--detach", m1)
		mx := merge(x)
		return priorRecord{Schema: priorSchema, Head: x, Target: mx, Tree: g("rev-parse", mx+"^{tree}"), Identity: priorIdentity(id), Passed: []string{}}
	}()
	selector := filepath.Join(t.TempDir(), "selector")
	if err := os.WriteFile(selector, []byte("#!/bin/sh\necho 'full UNKNOWN: LANGUAGE_FRONTIER'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, reason string
		prepare      func(t *testing.T, o *options, s *selection, idOK *bool)
	}{
		{"missing-record", "prior-record-unreadable", func(t *testing.T, o *options, _ *selection, _ *bool) {
			o.prior = filepath.Join(t.TempDir(), "absent.json")
		}},
		{"malformed-record", "prior-record-unreadable", func(t *testing.T, o *options, _ *selection, _ *bool) {
			o.prior = filepath.Join(t.TempDir(), "prior.json")
			if err := os.WriteFile(o.prior, []byte("{"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"unknown-field", "prior-record-unreadable", func(t *testing.T, o *options, _ *selection, _ *bool) {
			o.prior = filepath.Join(t.TempDir(), "prior.json")
			if err := os.WriteFile(o.prior, []byte(`{"Schema":"`+priorSchema+`","Failed":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"other-schema", "prior-record-unreadable", func(t *testing.T, o *options, _ *selection, _ *bool) {
			p := record
			p.Schema = "corvint-pr-tests-prior/1"
			o.prior = put(t, p)
		}},
		{"invalid-package", "prior-record-unreadable", func(t *testing.T, o *options, _ *selection, _ *bool) {
			p := record
			p.Passed = []string{"example.org/fixture/../b"}
			o.prior = put(t, p)
		}},
		{"other-identity", "prior-identity-mismatch", func(t *testing.T, o *options, _ *selection, _ *bool) {
			p := record
			p.Identity.GoVersion = "go0"
			o.prior = put(t, p)
		}},
		{"other-tree", "prior-record-mismatch", func(t *testing.T, o *options, _ *selection, _ *bool) {
			p := record
			p.Tree = g("rev-parse", h1+"^{tree}")
			o.prior = put(t, p)
		}},
		{"unresolvable-merge", "prior-record-mismatch", func(t *testing.T, o *options, _ *selection, _ *bool) {
			p := record
			p.Target = strings.Repeat("0", 40)
			o.prior = put(t, p)
		}},
		{"other-head", "prior-record-mismatch", func(t *testing.T, o *options, _ *selection, _ *bool) {
			p := record
			p.Head = h1
			o.prior = put(t, p)
		}},
		{"rewritten-head", "prior-head-not-ancestor", func(t *testing.T, o *options, _ *selection, _ *bool) {
			o.prior = put(t, sibling)
		}},
		{"planner-failure", "planner failed: ", func(t *testing.T, o *options, _ *selection, _ *bool) {
			o.planner = filepath.Join(t.TempDir(), "missing")
		}},
		{"unknown-verdict", "selector: full UNKNOWN: LANGUAGE_FRONTIER", func(t *testing.T, o *options, _ *selection, _ *bool) {
			o.selector = selector
		}},
		{"tool-profile", "tool profile unavailable", func(t *testing.T, _ *options, _ *selection, idOK *bool) {
			*idOK = false
		}},
		{"refused-plan", "plan: pr-base-not-ancestor", func(t *testing.T, _ *options, s *selection, _ *bool) {
			s.Tree, s.Reason = "", "pr-base-not-ancestor"
		}},
	}
	base := plan(ctx, valid, id, true)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, s, idOK := valid, base, true
			tc.prepare(t, &o, &s, &idOK)
			r := rerun(ctx, o, id, idOK, s)
			if !strings.HasPrefix(r.Reason, tc.reason) || !equal(r.Packages, full) {
				t.Fatalf("rerun: %+v, want %q and the whole universe", r, tc.reason)
			}
		})
	}
	// Last, because it moves the checkout: a .github path in M0..M1 runs everything.
	t.Run("github-delta", func(t *testing.T) {
		o := valid
		g("checkout", "-q", "--detach", h1)
		edit(".github/workflows/w.yml", "on: push\n")
		o.head = g("rev-parse", "HEAD")
		o.target = merge(o.head)
		g("checkout", "-q", "--detach", o.target)
		s := plan(ctx, o, id, true)
		if s.Tree == "" {
			t.Fatalf("topology: %+v", s)
		}
		if r := rerun(ctx, o, id, true, s); r.Reason != "prior-delta-touches-github" || !equal(r.Packages, full) {
			t.Fatalf("rerun: %+v", r)
		}
	})
}
