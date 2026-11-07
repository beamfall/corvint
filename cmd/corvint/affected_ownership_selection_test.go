// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

func ratchetSelectionRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":                           "module example.test/m\n\ngo 1.27.0\n",
		affected.OwnershipAuditTestPath:    "package affected_test\nimport \"testing\"\nfunc TestOwnershipAuditRatchet_AZU_V0_002(t *testing.T) {}\n",
		"tools/ci-reuse-plan/main.go":      "package main\nfunc main() {}\n",
		"tools/ci-reuse-plan/main_test.go": "package main\nimport \"testing\"\nfunc TestPlan(t *testing.T) {}\n",
		"web/test.ts":                      "import missing from 'missing';\nexport const x = missing;\n",
		"data/old.txt":                     "old\n",
		affected.RuntimeReadersPath:        `{"profile":"corvint-runtime-readers/0","entries":[]}`,
		affected.DocumentationPathsPath:    `{"profile":"corvint-documentation-paths/0","paths":["README.md"]}`,
		affected.NonSourcePathsPath:        `{"profile":"corvint-non-source-paths/0","entries":[{"path":"evidence/","reason":"output"}]}`,
	} {
		cemWrite(t, root, name, body)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.test"}, {"config", "gc.auto", "0"}, {"config", "maintenance.auto", "false"}, {"add", "."}, {"commit", "-qm", "base"}} {
		affectedGit(t, root, args...)
	}
	return root, strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
}

func ratchetPlan(t *testing.T, root, base string) affected.Plan {
	t.Helper()
	r, err := compileAffected(context.Background(), affectedInvocation{Root: root, Base: base, NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	return r.Plan
}

func assertRatchetSelected(t *testing.T, p affected.Plan) {
	t.Helper()
	found := false
	for _, s := range p.Selected {
		if !slices.Contains(s.Tests, affected.OwnershipAuditTestPath) {
			continue
		}
		if s.Witness.Kind == affected.WitnessOwnershipAudit && strings.Contains(strings.Join(s.Argv, " "), affected.OwnershipAuditTest) {
			found = true
		}
		for _, must := range p.MustRun {
			if must.UnitID == s.UnitID && strings.Contains(must.Reason, affected.OwnershipAuditTest) {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("ratchet absent or unexplained: selected=%+v mustRun=%+v", p.Selected, p.MustRun)
	}
}

// AZU-V0-9900: membership changes, staged additions and each declaration edit
// select the named gate; renames exercise both the old and new tracked names.
func TestOwnershipMembershipSelection_AZU_V0_9900(t *testing.T) {
	for _, change := range []string{"add", "stage", "stage-worktree-delete", "untracked", "delete", "worktree-delete", "rename", affected.RuntimeReadersPath, affected.DocumentationPathsPath, affected.NonSourcePathsPath} {
		t.Run(change, func(t *testing.T) {
			root, base := ratchetSelectionRepo(t)
			switch change {
			case "add", "stage", "stage-worktree-delete", "untracked":
				cemWrite(t, root, "data/new.txt", "new\n")
			case "delete", "worktree-delete":
				if err := os.Remove(filepath.Join(root, "data/old.txt")); err != nil {
					t.Fatal(err)
				}
			case "rename":
				if err := os.Rename(filepath.Join(root, "data/old.txt"), filepath.Join(root, "data/renamed.txt")); err != nil {
					t.Fatal(err)
				}
			default:
				file := filepath.Join(root, filepath.FromSlash(change))
				body, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, append(body, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if change != "untracked" && change != "worktree-delete" {
				affectedGit(t, root, "add", ".")
			}
			if change == "stage-worktree-delete" {
				if err := os.Remove(filepath.Join(root, "data/new.txt")); err != nil {
					t.Fatal(err)
				}
				if got := affectedGit(t, root, "ls-files", "--cached", "data/new.txt"); strings.TrimSpace(got) != "data/new.txt" {
					t.Fatalf("addition not indexed: %q", got)
				}
			}
			if change != "stage" && change != "stage-worktree-delete" && change != "untracked" && change != "worktree-delete" {
				affectedGit(t, root, "commit", "-qm", "change")
			}
			p := ratchetPlan(t, root, base)
			if change == "untracked" {
				if slices.ContainsFunc(p.Selected, func(s affected.Selection) bool { return slices.Contains(s.Tests, affected.OwnershipAuditTestPath) }) {
					t.Fatalf("untracked file selected ratchet: %+v", p.Selected)
				}
				return
			}
			assertRatchetSelected(t, p)
			if change == "add" {
				graph, err := affected.Build(context.Background(), root, languages.All()...)
				if err != nil {
					t.Fatal(err)
				}
				graph = affectedOwnershipInputs(context.Background(), affectedPlanInputs{gitExecutable: "git", repository: root, tree: root, base: base, revision: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))}, graph, p.Dirty)
				slices := testGoSlices(context.Background(), p, graph, root, base)
				if got := slices["example.test/m/internal/liveverify/affected"]; got.Pattern != "^("+affected.OwnershipAuditTest+")$" || !strings.Contains(got.Reason, "AZU-V0-9900") {
					t.Fatalf("dry-run slice %+v", got)
				}
			}
		})
	}
}

// AZU-V0-9901: replay the exact three added data paths of 0c8ae2da9.
// A selected Go host consumes the fixture; its missing declaration is retained,
// while unrelated language frontiers do not cascade from those paths.
func TestOwnershipReplay0c8ae2da9_AZU_V0_9901(t *testing.T) {
	root, base := ratchetSelectionRepo(t)
	for _, path := range []string{"jobs/42106672783.json", "logs/127535388505.txt", "runs.json"} {
		cemWrite(t, root, "tools/ci-reuse-plan/testdata/record-admission/"+path, "{}\n")
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "replay 0c8ae2da9")
	p := ratchetPlan(t, root, base)
	assertRatchetSelected(t, p)
	count := 0
	for _, u := range p.Unknown {
		if u.Reason == affected.UnknownUnownedDirtyPath {
			count++
			for _, declaration := range []string{affected.RuntimeReadersPath, affected.DocumentationPathsPath, affected.NonSourcePathsPath} {
				if !strings.Contains(u.Hint, declaration) {
					t.Fatalf("missing declaration hint: %+v", u)
				}
			}
		}
		if u.Reason == affected.UnknownLanguageFrontier && strings.Contains(u.Hint, "no plugin owns") {
			t.Fatalf("fixture cascaded: %+v", u)
		}
	}
	if count != 3 {
		t.Fatalf("unowned=%d, want 3; %+v", count, p.Unknown)
	}
	// A real independent frontier still bears when its source is also dirty.
	cemWrite(t, root, "web/test.ts", "import other from 'other';\nexport const x = other;\n")
	p = ratchetPlan(t, root, base)
	if !slices.ContainsFunc(p.Unknown, func(u affected.Unknown) bool {
		return u.Reason == affected.UnknownLanguageFrontier && strings.HasPrefix(u.Detail, "typescript:")
	}) {
		t.Fatalf("real frontier lost: %+v", p.Unknown)
	}
}

// AZU-V0-9900: measuring content-only edits adds no selection metadata and
// leaves canonical plan bytes identical to the same graph without the input.
func TestOwnershipContentOnlyBytes_AZU_V0_9900(t *testing.T) {
	for _, path := range []string{"tools/ci-reuse-plan/main.go", "data/old.txt"} {
		t.Run(path, func(t *testing.T) {
			root, base := ratchetSelectionRepo(t)
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			content := string(body) + "\n"
			if path == "tools/ci-reuse-plan/main.go" {
				content = "package main\nfunc main() { println(1) }\n"
			}
			cemWrite(t, root, path, content)
			affectedGit(t, root, "add", ".")
			affectedGit(t, root, "commit", "-qm", "content only")
			graph, err := affected.Build(context.Background(), root, languages.All()...)
			if err != nil {
				t.Fatal(err)
			}
			dirty := []string{path}
			measured := affectedOwnershipInputs(context.Background(), affectedPlanInputs{gitExecutable: "git", repository: root, tree: root, base: base, revision: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))}, graph, dirty)
			before, _ := json.Marshal(affected.Select(graph, dirty))
			after, _ := json.Marshal(affected.Select(measured, dirty))
			if string(before) != string(after) {
				t.Fatalf("content-only plan changed:\n%s\n%s", before, after)
			}
			// testFillPlan uses this compact receipt digest for test --dry-run.
			digest := func(plan affected.Plan) string {
				t.Helper()
				compact, err := compactAffectedReceipt(affectedReceipt{Plan: plan})
				if err != nil {
					t.Fatal(err)
				}
				value, err := sessiontest.Digest(compact)
				if err != nil {
					t.Fatal(err)
				}
				return value
			}
			if a, b := digest(affected.Select(graph, dirty)), digest(affected.Select(measured, dirty)); a != b {
				t.Fatalf("content-only dry-run planDigest changed: %s != %s", a, b)
			}
		})
	}
}

// AZU-V0-9901: JSON can be read by an unchanged test's computed require even
// when its selected Go host also consumes it. The dynamic frontier must remain.
func TestOwnershipFixtureComputedImport_AZU_V0_9901(t *testing.T) {
	root, _ := ratchetSelectionRepo(t)
	cemWrite(t, root, "web/data.test.ts", "import { test } from 'node:test';\ntest('data', () => { require(process.env.DATA_FILE); });\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "computed reader")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	fixture := "tools/ci-reuse-plan/testdata/new.json"
	cemWrite(t, root, fixture, "{}\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "new fixture")
	p := ratchetPlan(t, root, base)
	assertRatchetSelected(t, p)
	if !slices.ContainsFunc(p.Selected, func(s affected.Selection) bool {
		return slices.Contains(s.Tests, "tools/ci-reuse-plan/main_test.go")
	}) {
		t.Fatalf("Go fixture host not selected: %+v", p.Selected)
	}
	if !slices.ContainsFunc(p.Unknown, func(u affected.Unknown) bool {
		return u.Reason == affected.UnknownLanguageFrontier && strings.HasPrefix(u.Detail, "typescript:dynamic-import:web/data.test.ts:") && strings.Contains(u.Hint, fixture)
	}) {
		t.Fatalf("computed import uncertainty lost: %+v", p.Unknown)
	}
}

// AZU-V0-9901: replay with a terminal foreign reader of the modified Go
// source. Its source-bound import reason cannot observe the added fixtures;
// a computed data load still can, and keeps its original unknown reason.
func TestOwnershipReplayTerminalReader_AZU_V0_9901(t *testing.T) {
	root, _ := ratchetSelectionRepo(t)
	reader := "web/reader.test.ts"
	cemWrite(t, root, reader, "import { test } from 'node:test';\nimport missing from 'missing';\ntest('read', () => { console.log('tools/ci-reuse-plan/main.go'); require(process.env.DATA); });\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "terminal reader")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for _, path := range []string{"jobs/42106672783.json", "logs/127535388505.txt", "runs.json"} {
		cemWrite(t, root, "tools/ci-reuse-plan/testdata/record-admission/"+path, "{}\n")
	}
	cemWrite(t, root, "tools/ci-reuse-plan/main.go", "package main\nfunc main() { println(1) }\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "replay fixtures and source edit")
	p := ratchetPlan(t, root, base)
	assertRatchetSelected(t, p)
	if !slices.ContainsFunc(p.Selected, func(s affected.Selection) bool {
		return slices.Contains(s.Tests, reader) && s.Witness.Kind == affected.WitnessPathLiteralReader
	}) {
		t.Fatalf("terminal reader not selected: %+v", p.Selected)
	}
	dynamic := false
	for _, u := range p.Unknown {
		if u.Reason != affected.UnknownLanguageFrontier {
			continue
		}
		if strings.HasPrefix(u.Detail, "typescript:bare-import-undeclared:") {
			t.Fatalf("source-bound reason promoted through fixture: %+v", u)
		}
		if strings.HasPrefix(u.Detail, "typescript:dynamic-import:") {
			dynamic = true
		}
	}
	if !dynamic {
		t.Fatalf("computed reader unknown lost: %+v", p.Unknown)
	}
}

// AZU-V0-9902: the CLI and the ratchet classify identical tracked content;
// a baseline is outstanding debt, never a second ownership rule.
func TestOwnershipCLIAndRatchetAgree_AZU_V0_9902(t *testing.T) {
	root, _ := ratchetSelectionRepo(t)
	report, err := compileAffectedOwnershipAudit(context.Background(), affectedInvocation{Root: root, NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	tracked := strings.Split(strings.TrimSuffix(affectedGit(t, root, "ls-files", "-z"), "\x00"), "\x00")
	unowned, err := affected.AuditOwnership(context.Background(), root, tracked, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.Unowned, unowned) || !slices.Contains(unowned, "data/old.txt") || slices.Contains(unowned, affected.OwnershipAuditTestPath) {
		t.Fatalf("CLI %q ratchet %q", report.Unowned, unowned)
	}
}
