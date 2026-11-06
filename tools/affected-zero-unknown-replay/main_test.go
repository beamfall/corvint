// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func replayFixture() (safetyReceipt, plan, plan, map[string]map[int]bool) {
	old := plan{Selected: []selection{{UnitID: "go:a", Tests: []string{"a_test.go"}}}, Excluded: []selection{{UnitID: "go:b"}}, Unknown: []unknown{}}
	now := old
	r := safetyReceipt{Merge: strings.Repeat("a", 40), Parent: strings.Repeat("b", 40), MutationComplete: true,
		ParentSuite: suite{Revision: strings.Repeat("b", 40), Complete: true, Inventory: []string{"go:a", "go:b"}, Results: []observation{{"go:a", "TestA", "pass"}, {"go:b", "TestB", "pass"}}},
		MergeSuite:  suite{Revision: strings.Repeat("a", 40), Complete: true, Inventory: []string{"go:a", "go:b"}, Results: []observation{{"go:a", "TestA", "pass"}, {"go:b", "TestB", "pass"}}},
		Mutants:     []mutant{{ID: "mutant-1", Side: "merge", Path: "a.go", Line: 2, KilledBy: []observation{{Unit: "go:a", Test: "TestA"}}}},
	}
	return r, old, now, map[string]map[int]bool{"merge:a.go": {2: true}, "parent:a.go": {2: true}}
}

// AZU-V0-8903: additive/symbol exclusions are part of complete external
// suite inventories, and remain subject to the same unsafe-drop check.
func TestSafetyIncludesAdditiveExclusions_AZU_V0_8903(t *testing.T) {
	r, old, _, lines := replayFixture()
	var now plan
	if err := json.Unmarshal([]byte(`{"selected":[{"unitId":"go:a"}],"excluded":[],"goAdditiveExcluded":[{"unitId":"go:b","through":"go:a"}],"unknown":[]}`), &now); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(r)
	if got := safety(body, r.Merge, r.Parent, old, now, lines); got.Status != "OBSERVED_NO_MISS" {
		t.Fatalf("complete additive inventory refused: %+v", got)
	}
	r.MergeSuite.Results[1].Outcome = "fail"
	body, _ = json.Marshal(r)
	if got := safety(body, r.Merge, r.Parent, old, now, lines); got.Status != "UNSAFE_DROP" {
		t.Fatalf("additively excluded differing outcome admitted: %+v", got)
	}
}

// AZU-V0-009/8903: an extended frozen list through its newest merge must
// not append that merge twice, while duplicate supplied rows still refuse.
func TestReplayGrowingFrozenList_AZU_V0_8903(t *testing.T) {
	root := t.TempDir()
	gitOK := func(args ...string) string {
		t.Helper()
		out, err := git(root, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	gitOK("init", "--initial-branch=main")
	gitOK("config", "user.name", "Russell Lewis")
	gitOK("config", "user.email", "me@russe11.com")
	gitOK("commit", "--allow-empty", "-m", "root")
	merge := func(branch string) string {
		t.Helper()
		gitOK("checkout", "-b", branch)
		gitOK("commit", "--allow-empty", "-m", branch)
		gitOK("checkout", "main")
		gitOK("merge", "--no-ff", "-m", "merge "+branch, branch)
		return gitOK("rev-parse", "HEAD")
	}
	anchor, newest := merge("first"), merge("second")
	for _, refs := range [][]string{{anchor}, {anchor, newest}} {
		got, tip, err := replayMerges(root, refs, newest)
		if err != nil || tip != newest || !slices.Equal(got, []string{anchor, newest}) {
			t.Fatalf("growing list %v: got %v through %s: %v", refs, got, tip, err)
		}
	}
	if _, _, err := replayMerges(root, []string{anchor, newest, newest}, newest); err == nil || err.Error() != "duplicate frozen merge "+newest {
		t.Fatalf("supplied duplicate accepted: %v", err)
	}
}

// AZU-V0-009, AZU-V0-8903: mutating selection must fail the same checker CI
// uses, both for changed full-suite outcomes and changed-line mutant killers.
func TestSafetyRejectsUnsafeDrop_AZU_V0_8903(t *testing.T) {
	for _, kind := range []string{"outcome", "mutant", "both"} {
		t.Run(kind, func(t *testing.T) {
			r, old, now, lines := replayFixture()
			if kind == "outcome" || kind == "both" {
				r.MergeSuite.Results[1].Outcome = "fail"
			}
			if kind == "mutant" || kind == "both" {
				r.Mutants[0].KilledBy = append(r.Mutants[0].KilledBy, observation{Unit: "go:b", Test: "TestB"})
			}
			body, _ := json.Marshal(r)
			got := safety(body, r.Merge, r.Parent, old, now, lines)
			if got.Status != "UNSAFE_DROP" || len(got.Misses) == 0 {
				t.Fatalf("unsafe selection admitted: %+v", got)
			}
			now.Selected = append(now.Selected, selection{UnitID: "go:b"})
			now.Excluded = []selection{}
			body, _ = json.Marshal(r)
			if fixed := safety(body, r.Merge, r.Parent, old, now, lines); fixed.Status != "OBSERVED_NO_MISS" {
				t.Fatalf("repair rejected: %+v", fixed)
			}
		})
	}
}

// AZU-V0-8903: missing, incomplete, wrong-revision, unbound mutation and
// unknown test evidence cannot turn an unobserved safety run into acceptance.
func TestSafetyRefusesIncompleteEvidence_AZU_V0_8903(t *testing.T) {
	for _, kind := range []string{"missing-suite", "wrong-revision", "missing-mutation", "unchanged-line", "unknown-killer", "duplicate-result", "missing-result", "dropped-test-in-unit", "wrong-inventory"} {
		t.Run(kind, func(t *testing.T) {
			r, old, now, lines := replayFixture()
			switch kind {
			case "missing-suite":
				r.MergeSuite.Complete = false
			case "wrong-revision":
				r.Parent = "other"
			case "missing-mutation":
				r.MutationComplete = false
			case "unchanged-line":
				r.Mutants[0].Line = 3
			case "unknown-killer":
				r.Mutants[0].KilledBy[0].Unit = "go:ghost"
			case "duplicate-result":
				r.MergeSuite.Results = append(r.MergeSuite.Results, r.MergeSuite.Results[0])
			case "missing-result":
				r.MergeSuite.Results = r.MergeSuite.Results[:1]
			case "dropped-test-in-unit":
				r.ParentSuite.Results = append(r.ParentSuite.Results, observation{Unit: "go:a", Test: "TestDisappeared", Outcome: "pass"})
			case "wrong-inventory":
				r.MergeSuite.Inventory = []string{"go:a"}
			}
			body, _ := json.Marshal(r)
			got := safety(body, strings.Repeat("a", 40), strings.Repeat("b", 40), old, now, lines)
			if got.Status != "NOT_PRODUCED" || got.Detail == "" {
				t.Fatalf("incomplete evidence admitted: %+v", got)
			}
		})
	}
}

func TestChangedLineCoordinates_AZU_V0_8903(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,0 +2,2 @@\n+x\n+y\n@@ -8 +10 @@\n-z\n+q\ndiff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1,0 @@\n-x\n"
	lines := changedLines(diff)
	if len(lines["merge:a.go"]) != 3 || !lines["merge:a.go"][2] || !lines["merge:a.go"][3] || !lines["merge:a.go"][10] || len(lines["merge:b.go"]) != 0 {
		t.Fatalf("wrong changed lines: %+v", lines)
	}
}

// AZU-V0-009, AZU-V0-8903: use Git's actual quoting and header delimiters
// to bind valid mutants, including the parent coordinates of deleted files.
func TestChangedLineGitPaths_AZU_V0_8903(t *testing.T) {
	for _, name := range []string{"café.go", "with space.go", "with\ttab.go"} {
		for _, deleted := range []bool{false, true} {
			operation := "modify"
			if deleted {
				operation = "delete"
			}
			t.Run(name+"/"+operation, func(t *testing.T) {
				if runtime.GOOS == "windows" && strings.Contains(name, "\t") {
					t.Skip("Windows file names cannot contain tabs")
				}
				root := t.TempDir()
				gitOK := func(args ...string) string {
					t.Helper()
					out, err := git(root, args...)
					if err != nil {
						t.Fatal(err)
					}
					return out
				}
				gitOK("init", "--initial-branch=main")
				gitOK("config", "user.name", "Russell Lewis")
				gitOK("config", "user.email", "me@russe11.com")
				gitOK("config", "core.quotePath", "true")
				file := filepath.Join(root, name)
				// These content lines resemble headers after Git's +/- prefix.
				if err := os.WriteFile(file, []byte("-- a/phantom.go\n"), 0600); err != nil {
					t.Fatal(err)
				}
				gitOK("add", "--", name)
				gitOK("commit", "-m", "parent")
				if deleted {
					if err := os.Remove(file); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(file, []byte("++ b/phantom.go\n"), 0600); err != nil {
					t.Fatal(err)
				}
				diff := gitOK("diff", "--no-ext-diff", "--no-textconv", "--unified=0", "--", name)
				lines := changedLines(diff)
				if !lines["parent:"+name][1] || len(lines["parent:"+name]) != 1 {
					t.Fatalf("parent coordinates lost: %+v\n%s", lines, diff)
				}
				wantFiles := 2
				if deleted {
					wantFiles = 1
				} else if !lines["merge:"+name][1] || len(lines["merge:"+name]) != 1 {
					t.Fatalf("merge coordinates lost: %+v\n%s", lines, diff)
				}
				if len(lines) != wantFiles {
					t.Fatalf("unexpected changed paths: %+v\n%s", lines, diff)
				}
				r, old, now, _ := replayFixture()
				r.Mutants[0].Path, r.Mutants[0].Line = name, 1
				if deleted {
					r.Mutants[0].Side = "parent"
				}
				body, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				if got := safety(body, r.Merge, r.Parent, old, now, lines); got.Status != "OBSERVED_NO_MISS" {
					t.Fatalf("bound changed-line mutant rejected: %+v\n%s", got, diff)
				}
			})
		}
	}
}

// AZU-V0-009, AZU-V0-8903: local diff presentation settings and textconv
// drivers cannot change the source coordinates used to admit mutation receipts.
func TestSourceDiffConfiguration_AZU_V0_8903(t *testing.T) {
	root := t.TempDir()
	gitOK := func(args ...string) string {
		t.Helper()
		out, err := git(root, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	gitOK("init", "--initial-branch=main")
	gitOK("config", "user.name", "Russell Lewis")
	gitOK("config", "user.email", "me@russe11.com")
	file := filepath.Join(root, "a.go")
	if err := os.WriteFile(file, []byte("unchanged\nold\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.go diff=source-test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOK("add", ".")
	gitOK("commit", "-m", "parent")
	parent := gitOK("rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("unchanged\nnew\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOK("commit", "-am", "merge")
	ref := gitOK("rev-parse", "HEAD")
	for _, config := range []string{"diff.noprefix", "diff.mnemonicPrefix", "diff.source-test.textconv"} {
		t.Run(config, func(t *testing.T) {
			value := "true"
			if config == "diff.source-test.textconv" {
				// Both sides convert to the same Git version, hiding the edit.
				value = "git --version"
			}
			gitOK("config", config, value)
			t.Cleanup(func() { gitOK("config", "--unset", config) })
			diff, err := sourceDiff(root, parent, ref)
			if err != nil {
				t.Fatal(err)
			}
			lines := changedLines(diff)
			if len(lines) != 2 || len(lines["parent:a.go"]) != 1 || !lines["parent:a.go"][2] || len(lines["merge:a.go"]) != 1 || !lines["merge:a.go"][2] {
				t.Fatalf("source coordinates changed by %s: %+v\n%s", config, lines, diff)
			}
			r, old, now, _ := replayFixture()
			r.Merge, r.Parent = ref, parent
			r.ParentSuite.Revision, r.MergeSuite.Revision = parent, ref
			body, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if got := safety(body, r.Merge, r.Parent, old, now, lines); got.Status != "OBSERVED_NO_MISS" {
				t.Fatalf("valid source-line receipt rejected under %s: %+v", config, got)
			}
		})
	}
}

// AZU-V0-009, AZU-V0-8903: inter-hunk context is unchanged source,
// never mutation evidence, on either side of a real Git diff.
func TestInterHunkContextRejectsUnchangedMutants_AZU_V0_8903(t *testing.T) {
	root := t.TempDir()
	gitOK := func(args ...string) string {
		t.Helper()
		out, err := git(root, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	gitOK("init", "--initial-branch=main")
	gitOK("config", "user.name", "Russell Lewis")
	gitOK("config", "user.email", "me@russe11.com")
	file := filepath.Join(root, "a.go")
	if err := os.WriteFile(file, []byte("old\nunchanged\nold\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOK("add", ".")
	gitOK("commit", "-m", "parent")
	parent := gitOK("rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("new\nunchanged\nnew\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOK("commit", "-am", "merge")
	ref := gitOK("rev-parse", "HEAD")
	gitOK("config", "diff.interHunkContext", "20")
	// Exercise both the parser's handling of real context and sourceDiff's
	// independent guarantee that local configuration cannot introduce it.
	contextDiff := gitOK("diff", "--unified=0", parent, ref)
	if !strings.Contains(contextDiff, "\n unchanged\n") {
		t.Fatalf("fixture has no inter-hunk context: %s", contextDiff)
	}
	canonical, err := sourceDiff(root, parent, ref)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(canonical, "\n unchanged\n") {
		t.Errorf("sourceDiff retained configured inter-hunk context: %s", canonical)
	}
	for _, tc := range []struct{ name, diff string }{{"context", contextDiff}, {"canonical", canonical}} {
		t.Run(tc.name, func(t *testing.T) {
			lines := changedLines(tc.diff)
			for _, side := range []string{"parent", "merge"} {
				if len(lines[side+":a.go"]) != 2 || !lines[side+":a.go"][1] || !lines[side+":a.go"][3] || lines[side+":a.go"][2] {
					t.Errorf("unchanged coordinate admitted on %s: %+v", side, lines)
				}
				r, old, now, _ := replayFixture()
				r.Merge, r.Parent = ref, parent
				r.ParentSuite.Revision, r.MergeSuite.Revision = parent, ref
				r.Mutants[0].Side, r.Mutants[0].Line = side, 2
				body, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				if got := safety(body, ref, parent, old, now, lines); got.Status != "NOT_PRODUCED" {
					t.Fatalf("unchanged-line mutant admitted: %+v", got)
				}
			}
		})
	}
}

func TestBeforeAfterNamesEveryRemovedUnknown_AZU_V0_8903(t *testing.T) {
	old := plan{Selected: []selection{{UnitID: "kept"}, {UnitID: "dropped"}}, Unknown: []unknown{{"LANGUAGE_FRONTIER", "shell:foreign"}}}
	now := plan{Selected: []selection{{UnitID: "kept"}, {UnitID: "added"}}, Unknown: []unknown{}}
	var r row
	compare(&r, old, now)
	if len(r.RemovedUnknown) != 1 || strings.Join(r.AddedSelections, ",") != "added" || strings.Join(r.DroppedSelections, ",") != "dropped" {
		t.Fatalf("incomplete delta: %+v", r)
	}
}

// AZU-V0-8903: the frozen acceptance set cannot be shortened or repointed.
func TestFrozenMergePrefix_AZU_V0_8903(t *testing.T) {
	if !validFrozenPrefix(frozenMergePrefix) {
		t.Fatal("contractual prefix rejected")
	}
	if validFrozenPrefix(frozenMergePrefix[:7]) {
		t.Fatal("missing frozen merge accepted")
	}
	altered := append([]string(nil), frozenMergePrefix...)
	altered[1] = strings.Repeat("0", 40)
	if validFrozenPrefix(altered) {
		t.Fatal("repointed frozen merge accepted")
	}
}

// AZU-V0-009, AZU-V0-8903: one qualified merge cannot accept an incomplete
// replay. Missing receipts and planning errors retain an explicit failed row.
func TestReplayIncompleteRemainsUnaccepted_AZU_V0_8903(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture planner is a POSIX shell executable")
	}
	root := t.TempDir()
	gitOK := func(args ...string) string {
		t.Helper()
		out, err := git(root, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	write := func(path string, body []byte, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, body, mode); err != nil {
			t.Fatal(err)
		}
	}
	gitOK("init", "--initial-branch=main")
	gitOK("config", "user.name", "Russell Lewis")
	gitOK("config", "user.email", "me@russe11.com")
	gitOK("commit", "--allow-empty", "-m", "root")
	merge := func(branch string) string {
		t.Helper()
		gitOK("checkout", "-b", branch)
		write(filepath.Join(root, "change.go"), []byte(branch+"\n"), 0600)
		gitOK("add", "change.go")
		gitOK("commit", "-m", branch)
		gitOK("checkout", "main")
		gitOK("merge", "--no-ff", "-m", "merge "+branch, branch)
		return gitOK("rev-parse", "HEAD")
	}
	refs := []string{merge("first"), merge("second")}
	scratch := filepath.Join(t.TempDir(), "clone")
	if err := cloneSnapshot(root, root, scratch); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "planner")
	write(binary, []byte(`#!/bin/sh
if [ -n "$REPLAY_FAIL_REV" ] && [ "$(git -C "$2" rev-parse HEAD)" = "$REPLAY_FAIL_REV" ]; then
    exit 1
fi
printf '%s\n' '{"ok":true,"plan":{"selected":[{"unitId":"go:a","tests":["a_test.go"]}],"excluded":[{"unitId":"go:b"}],"unknown":[]}}'
`), 0700)
	safetyDir := t.TempDir()
	writeReceipt := func(ref string) {
		t.Helper()
		r, _, _, _ := replayFixture()
		r.Merge, r.Parent = ref, gitOK("rev-parse", ref+"^1")
		r.ParentSuite.Revision, r.MergeSuite.Revision = r.Parent, r.Merge
		r.Mutants[0].Path, r.Mutants[0].Line = "change.go", 1
		body, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(safetyDir, ref+".json"), body, 0600)
	}
	writeReceipt(refs[0])
	for _, kind := range []string{"missing-receipt", "planning-error", "complete"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("REPLAY_FAIL_REV", "")
			if kind == "planning-error" {
				t.Setenv("REPLAY_FAIL_REV", refs[1])
			}
			if kind == "complete" {
				writeReceipt(refs[1])
			}
			out := t.TempDir()
			err := replayRevisions(root, scratch, binary, "", out, safetyDir, refs, report{Through: refs[1], Merges: []row{}})
			body, readErr := os.ReadFile(filepath.Join(out, "report.json"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			var got report
			if decodeErr := json.Unmarshal(body, &got); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if len(got.Merges) != 2 || got.Merges[0].Safety.Status != "OBSERVED_NO_MISS" {
				t.Fatalf("missing qualifying first merge or incomplete second row: %s", body)
			}
			if kind == "complete" {
				if err != nil || !got.Accepted || got.Merges[1].Safety.Status != "OBSERVED_NO_MISS" {
					t.Fatalf("complete replay not accepted: %v\n%s", err, body)
				}
				return
			}
			if err == nil || got.Accepted || got.Merges[1].Merge != refs[1] || got.Merges[1].Safety.Status != "NOT_PRODUCED" || !strings.Contains(got.Merges[1].Safety.Detail, "replay incomplete:") {
				t.Fatalf("incomplete replay accepted or error lost: %v\n%s", err, body)
			}
		})
	}
}

// AZU-V0-8903: the replay snapshot clone is self-contained, so the
// candidate's immutable Git reader sees the same objects as a normal checkout.
func TestSnapshotCloneHasNoAlternates_AZU_V0_8903(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.name", "t"}, {"config", "user.email", "t@example.invalid"}, {"commit", "-q", "--allow-empty", "-m", "root"}} {
		if _, err := git(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	scratch := filepath.Join(t.TempDir(), "clone")
	if err := cloneSnapshot(root, root, scratch); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scratch, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
		t.Fatalf("snapshot clone has an alternates file (err=%v)", err)
	}
	if _, err := git(scratch, "cat-file", "-e", "HEAD^{commit}"); err != nil {
		t.Fatalf("snapshot clone cannot read HEAD without alternates: %v", err)
	}
}
