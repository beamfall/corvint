package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/docworktree"
	"github.com/Beamfall/corvint/internal/flowdocs"
)

// flowsDocsFixture is a committed one-controller Ruby repository for docs flows tests. It returns
// the root, a git runner, a docs flows runner and the source commit.
func flowsDocsFixture(t *testing.T) (string, func(...string) string, func(...string) (int, []byte, string), string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		out, e := fixtureGitCommand(append([]string{"-C", root}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %v %s", args, e, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.invalid")
	if err = os.Mkdir(filepath.Join(root, "app"), 0o700); err != nil {
		t.Fatal(err)
	}
	ruby := "class PaymentsController\n  def create\n    authorize(user)\n    payment.save!\n  end\nend\n"
	if err = os.WriteFile(filepath.Join(root, "app", "controller.rb"), []byte(ruby), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-qm", "source")
	run := func(args ...string) (int, []byte, string) {
		t.Helper()
		var out, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root, "docs", "flows"}, args...), strings.NewReader(""), &out, &stderr)
		return code, out.Bytes(), stderr.String()
	}
	return root, git, run, git("rev-parse", "HEAD")
}

func decodeFlowsCheck(t *testing.T, raw []byte) flowdocs.Check {
	t.Helper()
	var check flowdocs.Check
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Fatalf("check output %q: %v", raw, err)
	}
	return check
}

// firstPage is one generated README page of the output directory, repository-relative to dir.
func firstPage(t *testing.T, dir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*", "README.md"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no generated page under %s: %v", dir, err)
	}
	rel, err := filepath.Rel(dir, matches[0])
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

// DMF-V1-005: docs flows check reports a clean worktree, then `dirty` naming the one generated
// page edited and not committed.
func TestDMFV1005DocsFlowsCheckWorktree(t *testing.T) {
	root, git, run, source := flowsDocsFixture(t)
	destination := filepath.Join(root, "generated")
	if code, _, stderr := run("generate", "--revision", source, "--scope", "app", "--output-dir", destination); code != 0 {
		t.Fatalf("generate %d %s", code, stderr)
	}
	git("add", "generated")
	git("commit", "-qm", "generated")
	previous := filepath.Join(destination, "generation.json")
	code, raw, stderr := run("check", "--revision", source, "--previous", previous, "--output-dir", destination)
	check := decodeFlowsCheck(t, raw)
	if code != 0 || check.Worktree == nil || check.Worktree.State != docworktree.Clean {
		t.Fatalf("clean %d %+v %s", code, check.Worktree, stderr)
	}
	page := firstPage(t, destination)
	if err := os.WriteFile(filepath.Join(destination, filepath.FromSlash(page)), []byte("# Hand edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, raw, _ = run("check", "--revision", source, "--previous", previous, "--output-dir", destination)
	check = decodeFlowsCheck(t, raw)
	want := docworktree.Status{State: docworktree.Dirty, Changed: 1, Paths: []string{"generated/" + page}}
	if code != 1 || check.Worktree == nil || check.Worktree.State != want.State || check.Worktree.Changed != 1 || !slices.Equal(check.Worktree.Paths, want.Paths) {
		t.Fatalf("dirty %d %+v", code, check.Worktree)
	}
}

// DMF-V1-005: docs maintain --check and docs citations check --json carry the worktree member:
// clean on a committed tree, dirty with the edited page once it changes in the worktree.
func TestDMFV1005MaintainAndCitationsWorktree(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "page.md"), []byte("# Page\n\nSee `owner.md:4`.\n\n<!-- corvint:docmaintain:insertion-point -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (int, map[string]any) {
		t.Helper()
		var out, errOut bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root, "docs"}, args...), strings.NewReader(""), &out, &errOut)
		var report map[string]any
		if err := json.Unmarshal(out.Bytes(), &report); err != nil && code != 2 {
			t.Fatalf("%v output %q %s: %v", args, out.String(), errOut.String(), err)
		}
		return code, report
	}
	if code, _ := run("maintain", "--page", "docs/page.md", "--source", "owner.md", "--package", "cache", "--enable", "--apply"); code != 0 {
		t.Fatalf("apply %d", code)
	}
	docsCommit(t, root)
	worktree := func(report map[string]any) map[string]any {
		w, _ := report["worktree"].(map[string]any)
		return w
	}
	for _, args := range [][]string{{"maintain", "--check", "--page", "docs/page.md"}, {"citations", "check", "--json", "docs/page.md"}} {
		if code, report := run(args...); code != 0 || worktree(report)["state"] != "clean" {
			t.Fatalf("clean %v %d %v", args, code, report)
		}
	}
	page, err := os.ReadFile(filepath.Join(root, "docs", "page.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "page.md"), append(page, []byte("Uncommitted line.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"maintain", "--check", "--page", "docs/page.md"}, {"citations", "check", "--json", "docs/page.md"}} {
		code, report := run(args...)
		w := worktree(report)
		paths, _ := w["paths"].([]any)
		if code != 0 || w["state"] != "dirty" || w["changed"] != float64(1) || len(paths) != 1 || paths[0] != "docs/page.md" {
			t.Fatalf("dirty %v %d %v", args, code, w)
		}
	}
	var out, errOut bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "docs", "citations", "check", "docs/page.md"}, strings.NewReader(""), &out, &errOut); code != 0 ||
		!strings.Contains(out.String(), "worktree dirty: 1 changed tracked path(s) read (docs/page.md)") {
		t.Fatalf("text worktree line %d %q", code, out.String())
	}
}

// DMF-V1-007: generate into docs/flows records it; check without --output-dir compares it and
// reports a hand-edited page. Generated outside the root, nothing is recorded and check without
// --output-dir reports output NOT_CHECKED and clean false.
func TestDMFV1007DocsFlowsCheckRecordedOutput(t *testing.T) {
	root, _, run, source := flowsDocsFixture(t)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "docs", "flows")
	code, raw, stderr := run("generate", "--revision", source, "--scope", "app", "--output-dir", destination)
	var m flowdocs.Manifest
	if err := json.Unmarshal(raw, &m); code != 0 || err != nil || m.OutputDir != "docs/flows" {
		t.Fatalf("generate %d %v %q %s", code, err, m.OutputDir, stderr)
	}
	previous := filepath.Join(destination, "generation.json")
	code, raw, stderr = run("check", "--revision", source, "--previous", previous)
	check := decodeFlowsCheck(t, raw)
	if code != 0 || !check.Clean || check.Output != flowdocs.OutputChecked || check.OutputDir != "docs/flows" {
		t.Fatalf("recorded clean %d %+v %s", code, check, stderr)
	}
	page := firstPage(t, destination)
	if err := os.WriteFile(filepath.Join(destination, filepath.FromSlash(page)), []byte("# Hand edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, raw, _ = run("check", "--revision", source, "--previous", previous)
	check = decodeFlowsCheck(t, raw)
	if code != 1 || check.Clean || !slices.Equal(check.OutputDifferences, []string{page}) {
		t.Fatalf("hand edit %d %+v", code, check)
	}
	outside := filepath.Join(t.TempDir(), "out")
	if code, _, stderr = run("generate", "--revision", source, "--scope", "app", "--output-dir", outside); code != 0 {
		t.Fatalf("generate outside %d %s", code, stderr)
	}
	code, raw, _ = run("check", "--revision", source, "--previous", filepath.Join(outside, "generation.json"))
	check = decodeFlowsCheck(t, raw)
	if code != 1 || check.Clean || check.Output != flowdocs.OutputNotChecked || check.OutputReason != flowdocs.OutputNoDir {
		t.Fatalf("no recorded directory %d %+v", code, check)
	}
}
