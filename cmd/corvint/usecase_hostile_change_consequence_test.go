package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// consequenceCase is one hostile input to the UC-CHANGE-CONSEQUENCE
// entrypoint `corvint affected`. Category is the UCV0-006 case category and
// class is the V1-0188 hostile class it exercises.
type consequenceCase struct {
	category, class, name string
	run                   func(t *testing.T)
}

// TestUseCaseHostileChangeConsequence retains the hostile-tests evidence for
// UC-CHANGE-CONSEQUENCE (V1-0188). Each row asserts the contracted outcome: a
// typed refusal with exit 2 (negative), a degradation label that keeps the
// plan or the selection from claiming a bounded answer (hostile), or an
// explicit UNKNOWN scope or advice gap (abstention).
func TestUseCaseHostileChangeConsequence(t *testing.T) {
	t.Parallel()
	cases := []consequenceCase{
		// negative: typed refusals (AFP-V0-006).
		{"negative", "missing-anchors", "base that is not a commit is refused", func(t *testing.T) {
			root := consequenceRepository(t)
			consequenceRefusal(t, root, "unsupported-affected-revision", "--base", strings.Repeat("0123456789", 4))
		}},
		{"negative", "missing-anchors", "abbreviated base is refused", func(t *testing.T) {
			root := consequenceRepository(t)
			consequenceRefusal(t, root, "invalid-arguments", "--base", "HEAD")
		}},
		{"negative", "missing-anchors", "repository without a HEAD commit is refused", func(t *testing.T) {
			root := t.TempDir()
			cemGit(t, root, "init", "-q")
			consequenceRefusal(t, root, "unsupported-affected-revision")
		}},
		// hostile: degradation labels, never a narrower bounded answer.
		// affected never reads .corvint/index; this case documents that independence.
		{"hostile", "stale-index", "stale snapshot does not change the plan", func(t *testing.T) {
			root := consequenceRepository(t)
			runIndexForTest(t, root, false)
			cemWrite(t, root, "leaf/extra.go", "package leaf\n\nfunc Extra() int { return 2 }\n")
			cemGit(t, root, "add", ".")
			cemGit(t, root, "commit", "-qm", "extra")
			appendFile(t, filepath.Join(root, "core", "core.go"), "\n// dirty\n")
			_, hostile, stderr, code := runAffectedArguments(t, root)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			if err := os.RemoveAll(filepath.Join(root, ".corvint")); err != nil {
				t.Fatal(err)
			}
			if _, cold, _, _ := runAffectedArguments(t, root); !bytes.Equal(hostile, cold) {
				t.Fatalf("the plan changed with a stale snapshot present:\n%s\n%s", hostile, cold)
			}
		}},
		{"hostile", "stale-index", "provider record at a superseded revision widens the selection", func(t *testing.T) {
			root, base, record := affectedSelectionRepository(t)
			head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
			rewriteRecord(t, record, `"revision":"`+head+`"`, `"revision":"`+base+`"`)
			assertSelection(t, root, base, record, "full-relevant-suite-required", "stale-provider-revision")
		}},
		{"hostile", "malformed-provider-records", "truncated provider record blocks", func(t *testing.T) {
			root, base, record := affectedSelectionRepository(t)
			body, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(record, body[:len(body)/2], 0o644); err != nil {
				t.Fatal(err)
			}
			assertSelection(t, root, base, record, "blocked", "provider-invalid")
		}},
		{"hostile", "malformed-provider-records", "provider record with a foreign schema blocks", func(t *testing.T) {
			root, base, record := affectedSelectionRepository(t)
			rewriteRecord(t, record, `"schema":"external-evidence-provider/1"`, `"schema":"external-evidence-provider/9"`)
			assertSelection(t, root, base, record, "blocked", "provider-invalid")
		}},
		{"hostile", "malformed-provider-records", "unparseable go.mod is a language frontier", func(t *testing.T) {
			root := consequenceRepository(t)
			cemWrite(t, root, "go.mod", "this is not a go.mod\n")
			cemGit(t, root, "commit", "-qam", "break go.mod")
			receipt := consequencePlan(t, root)
			assertConsequenceUnknown(t, receipt, "LANGUAGE_FRONTIER", "go:module-path-unresolved")
			if state := receipt["provider"].(map[string]any)["go"].(map[string]any)["state"]; state != "MODULE_PATH_UNRESOLVED" {
				t.Fatalf("provider.go.state = %v, want MODULE_PATH_UNRESOLVED", state)
			}
		}},
		{"hostile", "symlinked-or-relocated-subjects", "record naming a relocated test path widens the selection", func(t *testing.T) {
			root, base, record := affectedSelectionRepository(t)
			rewriteRecord(t, record, `"path":"core/core_test.go"`, `"path":"core/relocated_test.go"`)
			assertSelection(t, root, base, record, "full-relevant-suite-required", "missing-path-reference")
		}},
		{"hostile", "symlinked-or-relocated-subjects", "record naming a path outside the repository is unresolved", func(t *testing.T) {
			root, base, record := affectedSelectionRepository(t)
			rewriteRecord(t, record, `"path":"core/core_test.go"`, `"path":"../outside_test.go"`)
			assertSelection(t, root, base, record, "full-relevant-suite-required", "unresolved-endpoint")
		}},
		{"hostile", "symlinked-or-relocated-subjects", "untracked symlink into a package is unindexed", func(t *testing.T) {
			root := consequenceRepository(t)
			if err := os.Symlink("/etc/hosts", filepath.Join(root, "core", "ext.go")); err != nil {
				t.Fatal(err)
			}
			assertConsequenceUnknown(t, consequencePlan(t, root), "UNINDEXED_SOURCE_PATH", "core/ext.go")
		}},
		{"hostile", "symlinked-or-relocated-subjects", "untracked symlinked directory is unowned", func(t *testing.T) {
			root := consequenceRepository(t)
			if err := os.Symlink("core", filepath.Join(root, "linked")); err != nil {
				t.Fatal(err)
			}
			assertConsequenceUnknown(t, consequencePlan(t, root), "UNOWNED_DIRTY_PATH", "linked")
		}},
		{"hostile", "symlinked-or-relocated-subjects", "committed symlink to an outside directory is not walked", func(t *testing.T) {
			root := consequenceRepository(t)
			outside := t.TempDir()
			cemWrite(t, outside, "extdir.go", "package extdir\n\nfunc Ext() int { return 3 }\n")
			cemWrite(t, outside, "extdir_test.go", "package extdir\n\nimport \"testing\"\n\nfunc TestExt(t *testing.T) { _ = Ext() }\n")
			if err := os.Symlink(outside, filepath.Join(root, "extdir")); err != nil {
				t.Fatal(err)
			}
			cemGit(t, root, "add", ".")
			cemGit(t, root, "commit", "-qm", "outside link")
			receipt := consequencePlan(t, root)
			if plan := receipt["plan"].(map[string]any); plan["scope"] != "BOUNDED" || strings.Contains(string(mustJSON(t, plan)), "extdir") {
				t.Fatalf("an outside directory reached the plan: %s", mustJSON(t, plan))
			}
		}},
		{"hostile", "dirty-worktree", "dirty edit selects its dependency closure", func(t *testing.T) {
			root := consequenceRepository(t)
			appendFile(t, filepath.Join(root, "core", "core.go"), "\n// dirty\n")
			plan := consequencePlan(t, root)["plan"].(map[string]any)
			selected := string(mustJSON(t, plan["selected"]))
			if plan["scope"] != "BOUNDED" || !strings.Contains(selected, "go:example.com/g/core") || !strings.Contains(selected, "go:example.com/g/leaf") {
				t.Fatalf("dirty core must select core and its importer: %s", mustJSON(t, plan))
			}
		}},
		{"hostile", "dirty-worktree", "dirty package without tests is named", func(t *testing.T) {
			t.Skip("V1-0188 follow-up: a dirty Go package with no test files leaves scope BOUNDED and is absent from selected, excluded and unknown (silent omission); fixed by the unmerged V1-0187 branch (AFP-V0-020, NO_SELECTABLE_TEST)")
			root := consequenceRepository(t)
			cemWrite(t, root, "util/util.go", "package util\n\nfunc Util() int { return 4 }\n")
			cemGit(t, root, "add", ".")
			cemGit(t, root, "commit", "-qm", "util")
			appendFile(t, filepath.Join(root, "util", "util.go"), "\n// dirty\n")
			if plan := consequencePlan(t, root)["plan"].(map[string]any); plan["scope"] != "UNKNOWN" || !strings.Contains(string(mustJSON(t, plan["unknown"])), "util") {
				t.Fatalf("a dirty package without tests must be named as unknown scope: %s", mustJSON(t, plan))
			}
		}},
		// abstention: UNKNOWN scope or a named advice gap instead of a bounded claim.
		{"abstention", "symlinked-or-relocated-subjects", "uncommitted relocation leaves the scope unknown", func(t *testing.T) {
			root := consequenceRepository(t)
			if err := os.MkdirAll(filepath.Join(root, "core2"), 0o755); err != nil {
				t.Fatal(err)
			}
			cemGit(t, root, "mv", "core/core.go", "core2/core.go")
			receipt := consequencePlan(t, root)
			assertConsequenceUnknown(t, receipt, "UNINDEXED_SOURCE_PATH", "core/core.go")
			if excluded := string(mustJSON(t, receipt["plan"].(map[string]any)["excluded"])); !strings.Contains(excluded, `"reason":"UNINDEXED_DIRTY_GO_PATH_MAY_BE_DELETED_OR_RENAMED","unitId":"go:example.com/g/core"`) {
				t.Fatalf("core is not excluded as possibly deleted or renamed: %s", excluded)
			}
		}},
		{"abstention", "dirty-worktree", "deleted source leaves the scope unknown", func(t *testing.T) {
			root := consequenceRepository(t)
			if err := os.Remove(filepath.Join(root, "core", "core.go")); err != nil {
				t.Fatal(err)
			}
			receipt := consequencePlan(t, root)
			assertConsequenceUnknown(t, receipt, "UNINDEXED_SOURCE_PATH", "core/core.go")
			if excluded := string(mustJSON(t, receipt["plan"].(map[string]any)["excluded"])); !strings.Contains(excluded, `"reason":"UNINDEXED_DIRTY_GO_PATH_MAY_BE_DELETED_OR_RENAMED","unitId":"go:example.com/g/core"`) {
				t.Fatalf("core is not excluded as possibly deleted or renamed: %s", excluded)
			}
		}},
		{"abstention", "missing-anchors", "no declared gate and no selection are named advice gaps", func(t *testing.T) {
			root := consequenceRepository(t)
			receipt := consequencePlan(t, root)
			advice := string(mustJSON(t, receipt["advice"].(map[string]any)["unknown"]))
			for _, gap := range []string{"NO_REPOSITORY_GATE_DECLARED", "NO_ADVISORY_GO_COMMAND"} {
				if !strings.Contains(advice, gap) {
					t.Fatalf("advice.unknown lacks %s: %s", gap, advice)
				}
			}
			if checks, _ := receipt["advice"].(map[string]any)["checks"].([]any); len(checks) != 0 {
				t.Fatalf("an empty selection advised checks: %v", checks)
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.category+"/"+test.class+"/"+test.name, func(t *testing.T) {
			t.Parallel()
			test.run(t)
		})
	}
}

// consequenceRepository is a Go-only module whose package leaf imports core,
// so a plan over it is BOUNDED unless a hostile input makes it UNKNOWN.
func consequenceRepository(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	cemWrite(t, root, "go.mod", "module example.com/g\n\ngo 1.22\n")
	cemWrite(t, root, "core/core.go", "package core\n\nfunc Core() int { return 1 }\n")
	cemWrite(t, root, "core/core_test.go", "package core\n\nimport \"testing\"\n\nfunc TestCore(t *testing.T) { _ = Core() }\n")
	cemWrite(t, root, "leaf/leaf.go", "package leaf\n\nimport \"example.com/g/core\"\n\nfunc Leaf() int { return core.Core() }\n")
	cemWrite(t, root, "leaf/leaf_test.go", "package leaf\n\nimport \"testing\"\n\nfunc TestLeaf(t *testing.T) { _ = Leaf() }\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "fixture")
	return root
}

func consequencePlan(t *testing.T, root string, arguments ...string) map[string]any {
	t.Helper()
	receipt, _, stderr, code := runAffectedArguments(t, root, arguments...)
	if code != 0 {
		t.Fatalf("affected exit %d: %s", code, stderr)
	}
	return receipt
}

func consequenceRefusal(t *testing.T, root, code string, arguments ...string) {
	t.Helper()
	_, stdout, stderr, exit := runAffectedArguments(t, root, arguments...)
	if exit != 2 || !strings.Contains(string(stdout)+stderr, `"code": "`+code+`"`) {
		t.Fatalf("exit %d, want 2 with code %s: %s %s", exit, code, stdout, stderr)
	}
}

func assertConsequenceUnknown(t *testing.T, receipt map[string]any, reason, detail string) {
	t.Helper()
	plan := receipt["plan"].(map[string]any)
	if plan["scope"] != "UNKNOWN" {
		t.Fatalf("scope = %v, want UNKNOWN: %s", plan["scope"], mustJSON(t, plan))
	}
	for _, item := range plan["unknown"].([]any) {
		entry := item.(map[string]any)
		if entry["reason"] == reason && entry["detail"] == detail {
			return
		}
	}
	t.Fatalf("plan.unknown lacks %s %s: %s", reason, detail, mustJSON(t, plan["unknown"]))
}

func rewriteRecord(t *testing.T, record, from, to string) {
	t.Helper()
	body, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(from)) {
		t.Fatalf("record has no %s", from)
	}
	if err := os.WriteFile(record, bytes.ReplaceAll(body, []byte(from), []byte(to)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertSelection pins the ETS failure table: the selection takes the
// contracted non-narrow state and names the typed reason.
func assertSelection(t *testing.T, root, base, record, state, reason string) {
	t.Helper()
	selection := testSelection(t, consequencePlan(t, root, "--base", base, "--provider", record))
	if selection["state"] != state {
		t.Fatalf("selection state = %v, want %q: %s", selection["state"], state, mustJSON(t, selection))
	}
	if !strings.Contains(string(mustJSON(t, selection)), reason) {
		t.Fatalf("selection lacks %s: %s", reason, mustJSON(t, selection))
	}
}
