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

type docsCheckResult struct {
	Profile string `json:"profile"`
	Commit  string `json:"commit"`
	Status  string `json:"status"`
	Parts   []struct {
		Part       string          `json:"part"`
		State      string          `json:"state"`
		Reason     string          `json:"reason"`
		Checked    int             `json:"checked"`
		Stale      []docsCheckItem `json:"stale"`
		NotChecked []docsCheckItem `json:"not_checked"`
	} `json:"parts"`
	Worktree docworktree.Status `json:"worktree"`
}

func runDocsCheckCLI(t *testing.T, root string) (int, docsCheckResult, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "docs", "check"}, strings.NewReader(""), &out, &errOut)
	var report docsCheckResult
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("docs check %d %q %s: %v", code, out.String(), errOut.String(), err)
	}
	return code, report, errOut.String()
}

// DMF-V1-002: with no markers, no flows manifest and no citations, every part is NOT_CHECKED with
// reason none-found and the exit is 0.
func TestDMFV1002DocsCheckNothingFound(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	code, report, stderr := runDocsCheckCLI(t, root)
	if code != 0 || report.Profile != "corvint-docs-check/0" || report.Status != "PASS" || len(report.Commit) < 40 || len(report.Parts) != 3 || report.Worktree.State != "clean" {
		t.Fatalf("nothing found %d %+v %s", code, report, stderr)
	}
	for i, name := range []string{"markers", "flows", "citations"} {
		if p := report.Parts[i]; p.Part != name || p.State != "NOT_CHECKED" || p.Reason != "none-found" {
			t.Fatalf("part %d %+v", i, p)
		}
	}
	var out, errOut bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "docs", "check", "docs/page.md"}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Fatalf("docs check accepted a per-page argument: %d", code)
	}
	for _, args := range [][]string{{"help", "docs", "check"}, {"docs", "check", "--help"}} {
		out.Reset()
		if code := runContext(context.Background(), args, strings.NewReader(""), &out, &errOut); code != 0 || out.String() != docsCheckHelp {
			t.Fatalf("%v printed %d %q", args, code, out.String())
		}
	}
}

// DMF-V1-002: a repository with a fresh marker block, a fresh committed flows manifest whose
// recorded output directory matches, and a fresh citation passes every part; once the block's cited
// source changes at HEAD, the check exits 1 and lists that block's page and line under STALE while
// the other parts still pass.
func TestDMFV1002DocsCheckPassThenOneStaleBlock(t *testing.T) {
	root := docsRepository(t)
	ruby := "class PaymentsController\n  def create\n    authorize(user)\n  end\nend\n"
	for name, body := range map[string]string{
		"app/controller.rb": ruby,
		"docs/page.md":      "# Page\n\nSee `owner.md:4`.\n\n<!-- corvint:docmaintain:insertion-point -->\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	docsCommit(t, root)
	head := func() string {
		out, err := fixtureGitCommand("-C", root, "rev-parse", "HEAD").CombinedOutput()
		if err != nil {
			t.Fatalf("rev-parse: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run := func(args ...string) {
		t.Helper()
		var out, errOut bytes.Buffer
		if code := runContext(context.Background(), append([]string{"--root", root}, args...), strings.NewReader(""), &out, &errOut); code != 0 {
			t.Fatalf("%v %d %s", args, code, errOut.String())
		}
	}
	run("docs", "maintain", "--page", "docs/page.md", "--source", "owner.md", "--package", "cache", "--enable", "--apply")
	run("docs", "flows", "generate", "--revision", head(), "--scope", "app", "--output-dir", filepath.Join(root, "docs", "flows"))
	docsCommit(t, root)
	code, report, stderr := runDocsCheckCLI(t, root)
	if code != 0 || report.Status != "PASS" {
		t.Fatalf("all pass %d %+v %s", code, report, stderr)
	}
	for _, p := range report.Parts {
		if p.State != "PASS" || p.Checked == 0 {
			t.Fatalf("part %+v", p)
		}
	}
	owner, err := os.ReadFile(filepath.Join(root, "owner.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "owner.md"), append(owner, []byte("A new requirement line.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	docsCommit(t, root)
	code, report, stderr = runDocsCheckCLI(t, root)
	if code != 1 || report.Status != "STALE" {
		t.Fatalf("one stale block %d %+v %s", code, report, stderr)
	}
	markers, flows, citations := report.Parts[0], report.Parts[1], report.Parts[2]
	if markers.State != "STALE" || len(markers.Stale) != 1 || markers.Stale[0].Path != "docs/page.md" || markers.Stale[0].Line != 6 || !strings.HasPrefix(markers.Stale[0].Detail, "stale") {
		t.Fatalf("markers %+v", markers)
	}
	if flows.State != "PASS" || citations.State != "PASS" {
		t.Fatalf("other parts %+v %+v", flows, citations)
	}
}

// DMF-V1-013: a draft bound at commit A, consumed after HEAD moved to commit B with a changed tree,
// is refused naming both commits and trees and the command that drafts again.
func TestDMFV1013ConsumeStaleNamesBothRevisions(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	revision := func(rev string) string {
		out, err := fixtureGitCommand("-C", root, "rev-parse", rev).CombinedOutput()
		if err != nil {
			t.Fatalf("rev-parse: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commitA, treeA := revision("HEAD"), revision("HEAD^{tree}")
	code, draft, stderr := runDocsFixture(t, root, "draft", "")
	if code != 0 {
		t.Fatalf("draft %d %s", code, stderr)
	}
	owner, err := os.ReadFile(filepath.Join(root, "owner.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "owner.md"), append(owner, []byte("Another requirement.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	docsCommit(t, root)
	commitB, treeB := revision("HEAD"), revision("HEAD^{tree}")
	code, out, stderr := runDocsFixture(t, root, "consume", draft, "--task", "Split")
	if code != 2 || out != "" || !strings.Contains(stderr, "stale-documentation-draft") {
		t.Fatalf("stale consume %d %q %s", code, out, stderr)
	}
	for _, want := range []string{
		"bound at commit " + commitA + " (tree " + treeA + ")",
		"now at commit " + commitB + " (tree " + treeB + ")",
		"re-draft with corvint-docs docs draft --source owner.md --package cache",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stale message lacks %q: %s", want, stderr)
		}
	}
	code, draft, _ = runDocsFixture(t, root, "draft", "")
	if code != 0 {
		t.Fatal("redraft")
	}
	edited := strings.Replace(draft, "Corvint capability orientation", "Edited orientation", 1)
	if _, _, stderr = runDocsFixture(t, root, "consume", edited, "--task", "Split"); !strings.Contains(stderr, "names the current commit "+commitB) {
		t.Fatalf("edited draft at the current commit: %s", stderr)
	}
}
