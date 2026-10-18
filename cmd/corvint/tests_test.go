package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testsCommit(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	for name, text := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cemGit(t, root, "add", "-A")
	cemGit(t, root, "commit", "-qm", "fixture")
	return cemGit(t, root, "rev-parse", "HEAD")
}

const testsCartSpec = `import { users } from '../setup/users.seed';
test('adds an item', async ({ page }) => {
  await expect(page.getByTestId('count')).toHaveText('1');
  await expect(page.getByTestId('total')).toHaveText('$5');
});
`

// TestTestsCLI drives `corvint tests` end to end: help and dispatch, setup-select in both
// formats, lint exit codes, and duplicates report and gate exit codes (TSA-V0-005, TSA-V0-016,
// TSA-V0-021, TSA-V0-023).
func TestTestsCLI(t *testing.T) {
	for _, args := range [][]string{{"tests", "--help"}, {"help", "tests"}} {
		code, out, _ := runCLI(t, args...)
		if code != 0 || !strings.Contains(out, "tests setup-select") || !strings.Contains(out, "test-source-analysis-v0.md") {
			t.Fatalf("%v: code=%d", args, code)
		}
	}
	if code, out, _ := runCLI(t, "--help"); code != 0 || !strings.Contains(out, "tests (TSA-V0)") {
		t.Fatalf("root help: code=%d", code)
	}
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	base := testsCommit(t, root, map[string]string{
		"setup/users.seed.ts":  "export const users = ['ada'];\n",
		"setup/other.setup.ts": "export const other = 1;\n",
		"e2e/cart.spec.ts":     testsCartSpec,
	})
	head := testsCommit(t, root, map[string]string{
		"e2e/basket.spec.ts": strings.Replace(testsCartSpec, "adds an item", "basket gains", 1) +
			"test('waits', async ({ page }) => {\n  await page.waitForTimeout(100);\n});\n",
	})

	code, out, errb := runCLI(t, "--root", root, "tests", "setup-select", "--specs", "e2e/cart.spec.ts")
	var selection struct {
		Revision, Mode, InputDigest string
		SetupFiles                  []string
	}
	if code != 0 || json.Unmarshal([]byte(out), &selection) != nil || selection.Revision != head || selection.Mode != "selected" ||
		strings.Join(selection.SetupFiles, ",") != "setup/users.seed.ts" || !strings.HasPrefix(selection.InputDigest, "sha256:") {
		t.Fatalf("setup-select json: code=%d out=%s err=%s", code, out, errb)
	}
	if code, out, _ := runCLI(t, "--root", root, "tests", "setup-select", "--specs", "e2e/absent.spec.ts", "--format", "lines"); code != 0 ||
		out != "setup/other.setup.ts\nsetup/users.seed.ts\n" {
		t.Fatalf("setup-select lines ALL: code=%d out=%q", code, out)
	}

	if code, out, _ := runCLI(t, "--root", root, "tests", "lint", "--base", head); code != 0 || !strings.Contains(out, "0 finding(s)") {
		t.Fatalf("lint clean: code=%d out=%q", code, out)
	}
	code, out, _ = runCLI(t, "--root", root, "tests", "lint", "--base", base)
	if code != 1 || !strings.Contains(out, "e2e/basket.spec.ts:7: fixed-wait:") || !strings.Contains(out, "e2e/basket.spec.ts:6: no-assertion:") {
		t.Fatalf("lint findings: code=%d out=%q", code, out)
	}
	code, first, _ := runCLI(t, "--root", root, "tests", "lint", "--base", base, "--json")
	_, second, _ := runCLI(t, "--root", root, "tests", "lint", "--base", base, "--json")
	if code != 1 || first != second || !strings.Contains(first, `"schema": "corvint-tests-lint/0"`) {
		t.Fatalf("lint json: code=%d deterministic=%v", code, first == second)
	}
	for _, args := range [][]string{{"tests", "lint"}, {"tests", "lint", "--base", "no-such-rev"}, {"tests", "nope"}, {"tests"}} {
		if code, out, errb := runCLI(t, append([]string{"--root", root}, args...)...); code != 2 || out != "" || !strings.Contains(errb, `"ok":false`) {
			t.Fatalf("%v: code=%d out=%q err=%q", args, code, out, errb)
		}
	}

	code, out, _ = runCLI(t, "--root", root, "tests", "duplicates", "--max-line-tests", "2")
	if code != 0 || !strings.Contains(out, "not verdicts") || !strings.Contains(out, `"e2e/basket.spec.ts :: basket gains"`) {
		t.Fatalf("duplicates report: code=%d out=%s", code, out)
	}
	if code, _, _ := runCLI(t, "--root", root, "tests", "duplicates", "--max-line-tests", "2", "--base", base, "--gate", "V1-1592"); code != 1 {
		t.Fatalf("duplicates gate with an unresolved pair: code=%d", code)
	}
	if code, _, _ := runCLI(t, "--root", root, "tests", "duplicates", "--max-line-tests", "2", "--base", head, "--gate", "V1-1592"); code != 0 {
		t.Fatalf("duplicates gate with no changed test: code=%d", code)
	}
	if code, _, errb := runCLI(t, "--root", root, "tests", "duplicates", "--threshold", "2"); code != 2 || !strings.Contains(errb, "tests-invalid-option") {
		t.Fatalf("bad threshold: code=%d err=%q", code, errb)
	}
}

// testsRunReport is a minimal Playwright JSON report declaring the test revision and root, with
// e2e/name.spec.ts :: "shows the name" passing in desktop and failing in mobile.
func testsRunReport(t *testing.T, path, revision string) {
	t.Helper()
	test := func(project, status, result string) map[string]any {
		return map[string]any{"projectName": project, "expectedStatus": "passed", "status": status, "annotations": []any{},
			"results": []any{map[string]any{"workerIndex": 0, "status": result, "duration": 5, "retry": 0, "errors": []any{}}}}
	}
	doc := map[string]any{
		"config": map[string]any{"version": "1.61.1", "rootDir": "/work/e2e", "workers": 1,
			"metadata": map[string]any{"corvintRunIdentity": map[string]any{"testRevision": revision, "testRootPath": "e2e"}},
			"projects": []any{map[string]any{"name": "desktop"}, map[string]any{"name": "mobile"}}},
		"suites": []any{map[string]any{"title": "name.spec.ts", "file": "name.spec.ts", "suites": []any{}, "specs": []any{
			map[string]any{"title": "shows the name", "file": "name.spec.ts", "line": 1, "column": 1, "tests": []any{
				test("desktop", "expected", "passed"), test("mobile", "unexpected", "failed")}}}}},
		"errors": []any{}, "stats": map[string]any{},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTestsLintAdvisoryCLI proves the advisory rules print labelled findings without changing the
// exit status, --runs joins a report at the linted revision into a PRODUCED cross-variant line,
// its absence is NOT_PRODUCED, and an unreadable report is refused (TSA-V0-031, TSA-V0-032).
func TestTestsLintAdvisoryCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	base := testsCommit(t, root, map[string]string{"src/app.ts": "export const x = 1;\n"})
	head := testsCommit(t, root, map[string]string{"e2e/name.spec.ts": `test('shows the name', async ({ page }) => {
  expect(await page.getByTestId('rows').count()).toBe(2);
  await expect(page.getByText('Ada Lovelace', { exact: true })).toBeVisible();
  expect(await nameShown(page)).toBe(true);
});
`})
	code, out, errb := runCLI(t, "--root", root, "tests", "lint", "--base", base)
	if code != 0 || !strings.Contains(out, "one-shot-ui-state-assertion (advisory)") ||
		!strings.Contains(out, "one-shot-ui-state-assertion (advisory, UNKNOWN)") || !strings.Contains(out, "cross-variant: NOT_PRODUCED (no-run-evidence)") {
		t.Fatalf("lint without runs: code=%d out=%q err=%q", code, out, errb)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	testsRunReport(t, report, head)
	code, out, errb = runCLI(t, "--root", root, "tests", "lint", "--base", base, "--runs", report)
	if code != 0 || !strings.Contains(out, "exact-text-cross-variant (advisory)") ||
		!strings.Contains(out, "cross-variant: PRODUCED from 1 run(s), projects desktop,mobile, 1 split test(s)") {
		t.Fatalf("lint with runs: code=%d out=%q err=%q", code, out, errb)
	}
	code, out, _ = runCLI(t, "--root", root, "tests", "lint", "--base", base, "--runs", report, "--json")
	if code != 0 || !strings.Contains(out, `"exactText": true`) || !strings.Contains(out, `"advisory": true`) {
		t.Fatalf("lint json with runs: code=%d out=%s", code, out)
	}
	code, out, errb = runCLI(t, "--root", root, "tests", "lint", "--base", base, "--runs", filepath.Join(root, "absent.json"))
	if code != 2 || out != "" || !strings.Contains(errb, "tests-source-unavailable") {
		t.Fatalf("lint with an unreadable report: code=%d out=%q err=%q", code, out, errb)
	}
}
