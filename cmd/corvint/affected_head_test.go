// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// AFP-V0-9989: --head plans the commit pair from Git objects alone. A dirty,
// untracked and staged worktree does not reach the plan, which is the --base
// plan of the clean checkout of HEAD byte for byte; the scratch is removed.
func TestAffectedHeadPlansTheCommitPairFromGitObjects_AFPV09989(t *testing.T) {
	scratch := t.TempDir()
	t.Setenv("TMPDIR", scratch)
	root := impactCLIRepository(t)
	base := coreFreezeParent(t, root)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	code, clean, stderr := runCLI(t, "--root", root, "affected", "--base", base, "--no-cache")
	if code != 0 {
		t.Fatalf("clean --base: exit %d: %s", code, stderr)
	}
	// Dirty the checkout every way the --base plan reads it.
	cemWrite(t, root, "pkg/main.go", "package main\n\nfunc StableValue() string { return \"dirty\" }\n")
	cemWrite(t, root, "pkg/extra.go", "package main\n\nfunc Extra() {}\n")
	cemWrite(t, root, "other/staged.go", "package other\n")
	affectedGit(t, root, "add", "other/staged.go")
	code, dirty, stderr := runCLI(t, "--root", root, "affected", "--base", base, "--no-cache")
	if code != 0 || dirty == clean {
		t.Fatalf("control: the dirty --base plan must differ from the clean one (exit %d): %s", code, stderr)
	}
	for _, revision := range []string{head, head[:12], "HEAD"} {
		code, replay, stderr := runCLI(t, "--root", root, "affected", "--base", base, "--head", revision, "--no-cache")
		if code != 0 {
			t.Fatalf("--head %s: exit %d: %s", revision, code, stderr)
		}
		if replay != clean {
			t.Fatalf("--head %s read the worktree or differs from the clean --base plan:\n%s\nwant\n%s", revision, replay, clean)
		}
	}
	// An older HEAD: the empty range BASE..BASE at revision BASE.
	code, older, stderr := runCLI(t, "--root", root, "affected", "--base", base, "--head", base, "--no-cache")
	if code != 0 {
		t.Fatalf("--head BASE: exit %d: %s", code, stderr)
	}
	var document struct {
		Revision string `json:"revision"`
		Summary  struct {
			Dirty    int `json:"dirty"`
			Worktree int `json:"worktree"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(older), &document); err != nil {
		t.Fatal(err)
	}
	if document.Revision != base || document.Summary.Dirty != 0 || document.Summary.Worktree != 0 {
		t.Fatalf("--head BASE planned %+v, want revision %s and nothing dirty", document, base)
	}
	if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
		t.Fatalf("scratch left behind in %s: %v %v", scratch, entries, err)
	}
	if status := affectedGit(t, root, "status", "--porcelain"); !strings.Contains(status, "pkg/main.go") || !strings.Contains(status, "A  other/staged.go") {
		t.Fatalf("--head touched the checkout: %q", status)
	}
}

// AFP-V0-9989: every option whose input lies outside Git objects, --head
// without --base, and a tree whose content is not all blobs are refused
// before or instead of a plan.
func TestAffectedHeadRefusesInputsOutsideGitObjects_AFPV09989(t *testing.T) {
	t.Parallel()
	base := strings.Repeat("a", 40)
	if _, err := parseAffectedOptions([]string{"--head", base}); err == nil || !strings.Contains(err.Error(), "--head requires --base") {
		t.Errorf("--head without --base: %v", err)
	}
	for _, value := range []string{"-x", "a..b", ""} {
		if _, err := parseAffectedOptions([]string{"--base", base, "--head=" + value}); err == nil || !strings.Contains(err.Error(), "--head requires exactly one commit") {
			t.Errorf("--head %q: %v", value, err)
		}
	}
	if _, err := parseAffectedOptions([]string{"--base", base, "--head", base, "--head", base}); err == nil {
		t.Error("a repeated --head was accepted")
	}
	for name, extra := range map[string][]string{
		"--path":                 {"--path", "a.go"},
		"--paths-from":           {"--paths-from", "list.txt"},
		"--require-clean":        {"--require-clean"},
		"--fail-fast-order":      {"--fail-fast-order"},
		"--provider":             {"--provider", "record.json"},
		"--appmap-impact":        {"--appmap-impact", "report.json"},
		"--playwright-config":    {"--playwright-config", "playwright.config.ts"},
		"--line-index":           {"--line-index", "linecov.idx"},
		"--line-exec":            {"--line-exec", "wrapper"},
		"--line-test-timeout":    {"--line-test-timeout", "1m"},
		"--playwright-discovery": {"--playwright-discovery", "d.json"},
	} {
		arguments := append([]string{"--base", base, "--head", base}, extra...)
		_, err := parseAffectedOptions(arguments)
		if err == nil || !strings.Contains(err.Error(), "--head cannot be combined with "+name) || !strings.Contains(err.Error(), "reads no file, worktree or local record") {
			t.Errorf("%q: %v", arguments, err)
		}
	}
	for _, invocation := range []affectedInvocation{
		{Head: base, Base: base, Worktree: true},
		{Head: base, Base: base, Snapshot: "s.json"},
		{Head: base, Base: base, ByPath: true},
		{Head: base, Base: base, PlaywrightEnv: []string{"A=1"}},
	} {
		if err := affectedHeadOptionRefusal(invocation); err == nil || !strings.Contains(err.Error(), "--head cannot be combined with") {
			t.Errorf("%+v: %v", invocation, err)
		}
	}

	root := impactCLIRepository(t)
	parent := coreFreezeParent(t, root)
	code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", parent, "--head", "no-such-ref")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--head no-such-ref is not a commit in this repository") {
		t.Fatalf("unknown --head: exit %d stdout %q stderr %s", code, stdout, stderr)
	}
	blob := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD:go.mod"))
	commit := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for name, cacheinfo := range map[string]string{"gitlink": "160000," + commit + ",vendor/sub", "symlink": "120000," + blob + ",link"} {
		affectedGit(t, root, "update-index", "--add", "--cacheinfo", cacheinfo)
		affectedGit(t, root, "commit", "-qm", "add a "+name)
		head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", parent, "--head", head)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "unsupported-affected-head") || !strings.Contains(stderr, "holds the "+name) {
			t.Fatalf("%s tree: exit %d stdout %q stderr %s", name, code, stdout, stderr)
		}
		affectedGit(t, root, "rm", "-q", "--cached", strings.Split(cacheinfo, ",")[2])
		affectedGit(t, root, "commit", "-qm", "drop the "+name)
	}
}
