// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestGoOutcomesFailClosed_FIH_V1_005(t *testing.T) {
	got, err := goOutcomes([]byte("admission log\n{\"Action\":\"pass\",\"Package\":\"p\",\"Test\":\"TestA\"}\n{\"Action\":\"fail\",\"Package\":\"p\",\"Test\":\"TestB/sub\"}\n{\"Action\":\"fail\",\"Package\":\"p\",\"Test\":\"TestB\"}\n{\"Action\":\"skip\",\"Package\":\"p\",\"Test\":\"TestC\"}\n{\"Action\":\"fail\",\"Package\":\"p\"}\n"))
	want := RunResult{"go:p#TestA": Pass, "go:p#TestB": Fail, "go:p#TestC": Skip}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("outcomes %v %v", got, err)
	}
	for name, raw := range map[string]string{
		"build failure":     "{\"Action\":\"fail\",\"Package\":\"bad\"}\n",
		"build beside kill": "{\"Action\":\"fail\",\"Package\":\"p\",\"Test\":\"TestA\"}\n{\"Action\":\"fail\",\"Package\":\"p\"}\n{\"Action\":\"fail\",\"Package\":\"b\"}\n",
		"timeout":           "{\"Action\":\"fail\",\"Package\":\"p\",\"Test\":\"TestA\"}\n{\"Action\":\"output\",\"Package\":\"p\",\"Output\":\"panic: test timed out after 1m0s\\n\"}\n",
		"panic":             "{\"Action\":\"output\",\"Package\":\"p\",\"Test\":\"TestA\",\"Output\":\"panic: boom\\n\"}\n{\"Action\":\"fail\",\"Package\":\"p\",\"Test\":\"TestA\"}\n",
		"malformed":         "{\"Action\":\n",
	} {
		if _, err := goOutcomes([]byte(raw)); err == nil {
			t.Fatalf("%s accepted as outcomes", name)
		}
	}
}

func TestAdapterContract_FIH_V1_008(t *testing.T) {
	if a, err := adapterFor("go", adapterOptions{}); err != nil || a.Runner() != "go" {
		t.Fatalf("go adapter: %v", err)
	}
	// pytest has an adapter since PYR-V1-010 (TestPYRV1010PytestHarnessAdapter).
	if a, err := adapterFor("pytest", adapterOptions{}); err != nil || a.Runner() != "pytest" {
		t.Fatalf("pytest adapter: %v", err)
	}
	for _, r := range []string{"vitest", "cypress"} {
		if _, err := adapterFor(r, adapterOptions{}); refusalCode(err) != "runner-adapter-missing" {
			t.Fatalf("%s: %v", r, err)
		}
	}
	entries := []planEntry{{Unit: "go:a"}, {Unit: "go:b", Run: []string{"TestX"}}}
	for id, want := range map[string]bool{"go:a#TestY": true, "go:b#TestX": true, "go:b#TestY": false, "go:c#TestX": false, "go:a": false} {
		if selects(entries, id) != want {
			t.Fatalf("selects %s", id)
		}
	}
}

// TestImports_FIH_V1_013 is the import check: the harness imports neither
// removed mutation package, nor internal/liveverify/mutate, which no G26 ticket
// has re-owned under this spec.
func TestImports_FIH_V1_013(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil || len(names) == 0 {
		t.Fatal(names, err)
	}
	for _, name := range names {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, banned := range []string{"internal/liveverify/pymutate", "internal/liveverify/rbmutate", "internal/liveverify/mutate"} {
				if strings.HasSuffix(p, banned) {
					t.Fatalf("%s imports %s", name, p)
				}
			}
		}
	}
}

func TestChangedLineSites_FIH_V1_003(t *testing.T) {
	diff := "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -3 +3,2 @@\n-a\n+b\n+c\n@@ -9,0 +11 @@\n+d\n--- a/gone.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n"
	if got := changedLines(diff); !reflect.DeepEqual(got, map[string]lineSet{"x.go": {3: true, 4: true, 11: true}}) {
		t.Fatalf("changed lines %v", got)
	}
	src := []byte("package p\n\nfunc F(a, b int) bool {\n\tif a < b && !ok() {\n\t\tlog(a + 1)\n\t}\n\treturn a == b || true\n}\n")
	all := lineSet{4: true, 5: true, 7: true}
	sites, err := goSites("p.go", src, all, operatorFamilies)
	if err != nil {
		t.Fatal(err)
	}
	ops := map[string]int{}
	for _, s := range sites {
		ops[s.Operator]++
		if string(src[s.Offset:s.Offset+len(s.Before)]) != s.Before {
			t.Fatalf("site does not match source: %+v", s)
		}
	}
	if !reflect.DeepEqual(ops, map[string]int{"relational-swap": 2, "boolean-negation": 3, "arithmetic-swap": 1, "constant-replacement": 2, "statement-removal": 1}) {
		t.Fatalf("operator sites %v", ops)
	}
	// Unchanged line: a site outside the PR's changed lines is skipped.
	only7, err := goSites("p.go", src, lineSet{7: true}, operatorFamilies)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range only7 {
		if s.Line != 7 {
			t.Fatalf("site on unchanged line %d", s.Line)
		}
	}
	if len(only7) != 3 {
		t.Fatalf("line 7 sites %d", len(only7))
	}
	// Only the pre-registered families are generated.
	rel, _ := goSites("p.go", src, all, []string{"relational-swap"})
	if len(rel) != 2 {
		t.Fatalf("family filter %v", rel)
	}
	a, err := pickMutants(sites, 7, 12, 3, 5)
	b, _ := pickMutants(append(sites[3:], sites[:3]...), 7, 12, 3, 5)
	if err != nil || len(a) != 5 || !reflect.DeepEqual(a, b) {
		t.Fatalf("seeded pick not deterministic: %v %v", a, err)
	}
	if _, err := pickMutants(sites[:2], 7, 12, 3, 5); err == nil {
		t.Fatal("two sites accepted for a 3..5 mutant PR")
	}
}

func TestFaultsOnChangedLinesInGit_FIH_V1_003(t *testing.T) {
	ctx := context.Background()
	r := newFixtureRepo(t)
	r.write("lib.go", "package lib\n\nfunc A(x int) bool { return x > 0 }\n\nfunc B(x int) int { return x * 2 }\n")
	r.write("lib_test.go", "package lib\n")
	base := r.commit("base")
	r.write("lib.go", "package lib\n\nfunc A(x int) bool { return x > 0 }\n\nfunc B(x int) int { return x*2 + 1 - x }\n\nfunc C(x int) bool { return x != 3 && x <= 9 }\n")
	head := r.commit("pr")
	faults, err := generateGoMutants(ctx, r.root, base, head, 42, 1, operatorFamilies)
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) < 3 || len(faults) > 5 {
		t.Fatalf("mutants %d", len(faults))
	}
	for _, f := range faults {
		if f.Line == 3 {
			t.Fatalf("mutant on the unchanged line of A: %+v", f)
		}
		if err := applyFault(ctx, r.root, f); err != nil {
			t.Fatal(err)
		}
		r.git("checkout", "--quiet", "--", "lib.go")
	}
	bad := faults[0]
	bad.Offset++
	if err := applyFault(ctx, r.root, bad); err == nil {
		t.Fatal("fault off its recorded site accepted")
	}
	// A revert of a fix restores the pre-fix lines at a later pin.
	r.write("lib.go", "package lib\n\nfunc A(x int) bool { return x >= 0 }\n\nfunc B(x int) int { return x*2 + 1 - x }\n\nfunc C(x int) bool { return x != 3 && x <= 9 }\n")
	fix := r.commit("fix: A accepts zero")
	r.write("later.go", "package lib\n")
	r.commit("later")
	rev := fault{ID: "revert", Operator: "revert", Revert: &revertFault{FixBase: head, FixHead: fix, Paths: []string{"lib.go"}}}
	if err := applyFault(ctx, r.root, rev); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(r.root, "lib.go"))
	if !strings.Contains(string(body), "return x > 0 }") {
		t.Fatalf("revert did not restore the pre-fix line:\n%s", body)
	}
	if err := applyFault(ctx, r.root, rev); err == nil {
		t.Fatal("revert applied twice")
	}
}

func TestKillConfirmationAndMiss_FIH_V1_005_006(t *testing.T) {
	p := func(o ...Outcome) []RunResult {
		var runs []RunResult
		for _, x := range o {
			runs = append(runs, RunResult{"go:a#T": x, "go:a#U": Pass, "go:b#V": Fail})
		}
		return runs
	}
	base := p(Pass, Pass, Pass)
	// Two of three: a test failing twice in three faulted runs is no kill.
	if k := confirmKills(base, p(Fail, Pass, Fail)); len(k) != 0 {
		t.Fatalf("two of three counted: %v", k)
	}
	if k := confirmKills(base, p(Fail, Fail, Fail)); !reflect.DeepEqual(k, []string{"go:a#T"}) {
		t.Fatalf("kills %v", k)
	}
	// go:b#V fails in the baseline, so it never kills; a baseline with a
	// failed run of T makes T no kill either.
	if k := confirmKills(p(Pass, Fail, Pass), p(Fail, Fail, Fail)); len(k) != 0 {
		t.Fatalf("baseline-failing test counted: %v", k)
	}
	if k := confirmKills(base[:2], p(Fail, Fail, Fail)); k != nil {
		t.Fatal("two baseline runs accepted")
	}
	kills := []string{"go:a#T"}
	// Killing test not selected: the case is MISS.
	if m := findMisses(kills, []planEntry{{Unit: "go:b"}}, RunResult{"go:b#V": Fail}); !reflect.DeepEqual(m, kills) {
		t.Fatalf("unselected killer not a miss: %v", m)
	}
	// Selected but not observed failing (a flaky pass) is still a miss.
	if m := findMisses(kills, []planEntry{{Unit: "go:a"}}, RunResult{"go:a#T": Pass}); !reflect.DeepEqual(m, kills) {
		t.Fatalf("selected pass waived: %v", m)
	}
	if m := findMisses(kills, []planEntry{{Unit: "go:a", Run: []string{"U"}}}, RunResult{"go:a#U": Pass}); !reflect.DeepEqual(m, kills) {
		t.Fatalf("filtered-out killer not a miss: %v", m)
	}
	if m := findMisses(kills, []planEntry{{Unit: "go:a"}}, RunResult{"go:a#T": Fail}); len(m) != 0 {
		t.Fatalf("observed kill reported as miss: %v", m)
	}
	var r receipt
	if err := decodeStrict([]byte(`{"ok":true,"plan":{"scope":"BOUNDED","selected":[],"unknown":[]},"summary":{"line":"x"}}`), &r); err != nil {
		t.Fatal(err)
	}
	if k := planKind(r, []planEntry{{Unit: "go:a"}}, base); k != "NARROWED" {
		t.Fatal(k)
	}
	if k := planKind(r, []planEntry{{Unit: "go:a"}, {Unit: "go:b"}}, base); k != "FULL" {
		t.Fatal(k)
	}
	r.Plan.Scope = "UNKNOWN"
	if k := planKind(r, nil, base); k != "UNKNOWN" {
		t.Fatal(k)
	}
}
