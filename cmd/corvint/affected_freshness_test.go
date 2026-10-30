package main

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func appendAffectedLine(t *testing.T, root, name, line string) {
	t.Helper()
	path := filepath.Join(root, name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, []byte("\n"+line+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// AFP-V0-044: a --base range path outside the worktree dirty set, planned over
// a dirty worktree, carries a warning naming HEAD and the dirty count; every
// other receipt keeps its member set.
func TestAffectedMixedRangeWarning(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	appendAffectedLine(t, root, "core/core.go", "// committed edit")
	affectedGit(t, root, "commit", "-qam", "edit core")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))

	clean, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 || clean["warning"] != nil {
		t.Fatalf("clean range: exit=%d warning=%v stderr=%s", code, clean["warning"], stderr)
	}

	appendAffectedLine(t, root, "leaf/leaf.go", "// MUTABLE_CANARY")
	for _, form := range [][]string{{"--base", base}, {"--full", "--base", base}} {
		receipt, raw, stderr, code := runAffectedCLI(t, root, form...)
		warning, _ := receipt["warning"].(string)
		if code != 0 || !strings.Contains(warning, head) || !strings.Contains(warning, "1 dirty worktree paths") || bytes.Contains(raw, []byte("MUTABLE_CANARY")) {
			t.Fatalf("%v: exit=%d stderr=%s receipt=%s", form, code, stderr, raw)
		}
	}
	// AFP-V0-928: --format text carries the same warning on its own line.
	if text := affectedTextOutput(t, root, "--base", base, "--format", "text"); !strings.Contains(text, "\nwarning ") ||
		!strings.Contains(text, head) || !strings.Contains(text, "1 dirty worktree paths") {
		t.Fatalf("--format text dropped the mixed warning:\n%s", text)
	}
	worktreeOnly, _, _, code := runAffectedCLI(t, root)
	if code != 0 || worktreeOnly["warning"] != nil {
		t.Fatalf("worktree-only form warned: %v", worktreeOnly["warning"])
	}

	affectedGit(t, root, "checkout", "-q", "--", "leaf/leaf.go")
	appendAffectedLine(t, root, "core/core.go", "// dirty again")
	inside, _, _, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 || inside["warning"] != nil {
		t.Fatalf("range inside the dirty set warned: %v", inside["warning"])
	}
}

// AFP-V0-045: --require-clean leaves a clean receipt byte-identical and
// refuses a dirty worktree with a typed error before any provider runs.
func TestAffectedRequireClean(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	_, ordinary, _, code := runAffectedCLI(t, root)
	_, strict, stderr, strictCode := runAffectedCLI(t, root, "--require-clean")
	if code != 0 || strictCode != 0 || !bytes.Equal(ordinary, strict) {
		t.Fatalf("clean strict receipt changed: exit=%d/%d stderr=%s", code, strictCode, stderr)
	}
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, form := range [][]string{
		{"--require-clean"},
		{"--full", "--require-clean", "--base", head},
		// A provider that ran would refuse with its own code instead.
		{"--require-clean", "--provider", "absent-provider.json"},
	} {
		before := treeDigest(t, root)
		_, raw, stderr, code := runAffectedCLI(t, root, form...)
		if code != 2 || len(raw) != 0 || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") || !strings.Contains(stderr, head) || !strings.Contains(stderr, "1 dirty worktree paths") {
			t.Fatalf("%v: exit=%d stdout=%s stderr=%s", form, code, raw, stderr)
		}
		if treeDigest(t, root) != before {
			t.Fatalf("%v wrote repository state", form)
		}
	}
	for _, form := range [][]string{
		{"--require-clean=true"},
		{"--require-clean", "--require-clean"},
		{"--require-clean", "--snapshot", "receipt.json"},
		{"discovery", "--require-clean", "--playwright-config", "a.ts", "--playwright-list", "b.json"},
	} {
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), append([]string{"--root", root, "affected"}, form...), strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "invalid-arguments") {
			t.Fatalf("%v: exit=%d stdout=%s stderr=%s", form, code, stdout.String(), stderr.String())
		}
	}
	if !strings.Contains(affectedHelp, "--require-clean") || !strings.Contains(affectedHelp, "unsupported-affected-mixed-worktree") {
		t.Fatal("affected help omits --require-clean")
	}
}

// playwrightFreshnessFixture is the committed two-project Playwright fixture.
func playwrightFreshnessFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writePlaywrightCLIFile(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	writePlaywrightCLIFile(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  projects: [
    { name: "angular", grep: /@angular/, use: { browserName: "chromium" } },
    { name: "react", grep: /@react/, use: { browserName: "chromium" } },
  ],
})
`)
	writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "page"`)
	writePlaywrightCLIFile(t, root, "src/other.ts", `export const other = "other"`)
	writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; import { page } from "../src/page"; test("@angular @react page", () => page)`)
	affectedGit(t, root, "init", "-q")
	affectedGit(t, root, "config", "user.email", "corvint@example.test")
	affectedGit(t, root, "config", "user.name", "Corvint Test")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "fixture")
	return root
}

func runPlaywrightFreshnessCLI(t *testing.T, root, discovery string, extra ...string) ([]byte, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	arguments := append([]string{"--root", root, "affected", "--playwright-config", "playwright.config.ts", "--playwright-discovery", discovery}, extra...)
	code := runContext(t.Context(), arguments, strings.NewReader(""), &stdout, &stderr)
	return stdout.Bytes(), stderr.String(), code
}

// AFP-V0-044/045 on the Playwright receipt: --require-clean keeps a clean
// receipt byte-identical and refuses a dirty worktree, and a --base range path
// outside the dirty set carries the MIXED warning.
func TestAffectedPlaywrightFreshness(t *testing.T) {
	t.Parallel()
	root := playwrightFreshnessFixture(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "committed"`)
	affectedGit(t, root, "commit", "-qam", "edit page")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	discovery := writePlaywrightDiscoveryFile(t, root)

	ordinary, stderr, code := runPlaywrightFreshnessCLI(t, root, discovery, "--base", base)
	strict, strictStderr, strictCode := runPlaywrightFreshnessCLI(t, root, discovery, "--base", base, "--require-clean")
	if code != 0 || strictCode != 0 || !bytes.Equal(ordinary, strict) || bytes.Contains(ordinary, []byte(`"warning"`)) {
		t.Fatalf("clean receipt: exit=%d/%d stderr=%s/%s\n%s\n%s", code, strictCode, stderr, strictStderr, ordinary, strict)
	}

	writePlaywrightCLIFile(t, root, "src/other.ts", `export const other = "MUTABLE_CANARY"`)
	before := treeDigest(t, root)
	raw, stderr, code := runPlaywrightFreshnessCLI(t, root, discovery, "--base", base, "--require-clean")
	if code != 2 || len(raw) != 0 || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") || !strings.Contains(stderr, head) || !strings.Contains(stderr, "1 dirty worktree paths") {
		t.Fatalf("dirty strict: exit=%d stdout=%s stderr=%s", code, raw, stderr)
	}
	if treeDigest(t, root) != before {
		t.Fatal("refusal wrote repository state")
	}

	raw, stderr, code = runPlaywrightFreshnessCLI(t, root, discovery, "--base", base)
	var receipt map[string]any
	if code != 0 || json.Unmarshal(raw, &receipt) != nil {
		t.Fatalf("mixed: exit=%d stderr=%s", code, stderr)
	}
	warning, _ := receipt["warning"].(string)
	if !strings.Contains(warning, head) || !strings.Contains(warning, "1 dirty worktree paths") || bytes.Contains(raw, []byte("MUTABLE_CANARY")) {
		t.Fatalf("mixed warning=%q receipt=%s", warning, raw)
	}
	if raw, _, code := runPlaywrightFreshnessCLI(t, root, discovery); code != 0 || bytes.Contains(raw, []byte(`"warning"`)) {
		t.Fatalf("worktree-only Playwright form warned: exit=%d %s", code, raw)
	}
}

// AFP-V0-356, V1-2053: a path ignored only by the global excludes file
// (core.excludesFile) is dirty to affected, which runs Git without global
// configuration. The --require-clean and --path refusals and the --base MIXED
// warning name that path and state the cause and the fix. It sets
// GIT_CONFIG_GLOBAL, so it is not parallel.
func TestAFPV0356MixedNamesGloballyExcludedPath(t *testing.T) {
	root := affectedFixtureRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	appendAffectedLine(t, root, "core/core.go", "// committed edit")
	affectedGit(t, root, "commit", "-qam", "edit core")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	dir := t.TempDir()
	excludes := filepath.Join(dir, "ignore")
	global := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(excludes, []byte("scratch.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global, []byte("[core]\n\texcludesFile = "+excludes+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	if err := os.WriteFile(filepath.Join(root, "scratch.log"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := affectedGit(t, root, "status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Fatalf("host Git with the global excludes still reports %q", status)
	}
	named := func(text string) bool {
		return strings.Contains(text, "scratch.log") && strings.Contains(text, "core.excludesFile") &&
			strings.Contains(text, ".git/info/exclude") && strings.Contains(text, "tracked .gitignore")
	}
	for _, form := range [][]string{{"--require-clean"}, {"--path", "core/core.go"}} {
		_, raw, stderr, code := runAffectedCLI(t, root, form...)
		if code != 2 || len(raw) != 0 || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") || !strings.Contains(stderr, head) || !named(stderr) {
			t.Fatalf("%v: exit=%d stdout=%s stderr=%s", form, code, raw, stderr)
		}
	}
	receipt, raw, stderr, code := runAffectedCLI(t, root, "--base", base)
	if warning, _ := receipt["warning"].(string); code != 0 || !named(warning) {
		t.Fatalf("--base: exit=%d stderr=%s receipt=%s", code, stderr, raw)
	}
	if text := affectedTextOutput(t, root, "--base", base, "--format", "text"); !named(text) {
		t.Fatalf("--format text dropped the excludes recovery:\n%s", text)
	}
}

// affectedTextOutput runs `corvint affected` with arguments that select
// --format text and returns its stdout, failing on a nonzero exit.
func affectedTextOutput(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), append([]string{"--root", root, "affected"}, arguments...), strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("%q exit %d: %s", arguments, code, stderr.String())
	}
	return stdout.String()
}

// TestAFPV0507PathRefusalNamesRecovery: AFP-V0-507 (V1-2818). --path plans a
// clean worktree at HEAD; on a dirty one the refusal names the two exact
// commands that plan the change instead, with HEAD's full commit id, and
// writes no plan.
func TestAFPV0507PathRefusalNamesRecovery(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	receipt, raw, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go")
	if code != 0 || receipt["plan"] == nil {
		t.Fatalf("clean --path: exit=%d stderr=%s receipt=%s", code, stderr, raw)
	}
	appendAffectedLine(t, root, "core/core.go", "// local edit")
	_, raw, stderr, code = runAffectedCLI(t, root, "--path", "core/core.go")
	for _, want := range []string{"unsupported-affected-mixed-worktree", "run `corvint affected` without --path", "commit the dirty paths and run `corvint affected --base " + head + "`"} {
		if code != 2 || len(raw) != 0 || !strings.Contains(stderr, want) {
			t.Fatalf("dirty --path: exit=%d stdout=%s stderr=%s lacks %q", code, raw, stderr, want)
		}
	}
}

// TestAFPV0356DirtyDetailIsBounded: AFP-V0-356. The detail names at most
// affectedDirtyPathsShown sorted paths and counts the rest.
func TestAFPV0356DirtyDetailIsBounded(t *testing.T) {
	t.Parallel()
	paths := []string{}
	for i := 9; i >= 0; i-- {
		paths = append(paths, "p"+strconv.Itoa(i)+".log")
	}
	got := affectedDirtyDetail(paths)
	if !strings.HasPrefix(got, `dirty: "p0.log", "p1.log",`) || !strings.Contains(got, `"p7.log" (+2 more)`) || strings.Contains(got, "p8.log") {
		t.Fatalf("bounded list: %q", got)
	}
}
