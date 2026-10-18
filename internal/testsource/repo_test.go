package testsource

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitFiles writes files (an empty text removes the file) and commits them.
func commitFiles(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		gitTest(t, root, "init", "-q")
	}
	for name, text := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if text == "" {
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	return gitTest(t, root, "rev-parse", "HEAD")
}

const oldSpec = `import { test, expect } from '@playwright/test';

test('legacy page', async ({ page }) => {
  await page.goto('/legacy');
  await page.waitForTimeout(1000);
});
`

// TestLintChangedLinesOnly proves the default reports only findings on lines changed since the
// base, --all-lines reports every finding, and only test and page-object files are read
// (TSA-V0-012, TSA-V0-014).
func TestLintChangedLinesOnly(t *testing.T) {
	root := t.TempDir()
	base := commitFiles(t, root, map[string]string{"e2e/legacy.spec.ts": oldSpec, "src/app.ts": "export const x = 1;\n"})
	head := commitFiles(t, root, map[string]string{
		"e2e/legacy.spec.ts": oldSpec + `
test('new page', async ({ page }) => {
  await page.goto('/new');
});
`,
		"e2e/pages/login.ts": "export async function open(page) { await page.waitForTimeout(10); }\n",
		"src/app.ts":         "export const x = 2; // page.waitForTimeout(5)\n",
	})
	repo := OpenRepo(context.Background(), root)
	report, err := Lint(repo, LintOptions{Base: base, Revision: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Revision != head || report.Base != base || report.Scope != "changed-lines" {
		t.Fatalf("report header = %+v", report)
	}
	if strings.Join(report.Files, ",") != "e2e/legacy.spec.ts,e2e/pages/login.ts" {
		t.Fatalf("files = %v", report.Files)
	}
	got := []string{}
	for _, f := range report.Findings {
		got = append(got, f.Path+":"+f.Rule)
	}
	if strings.Join(got, ",") != "e2e/pages/login.ts:fixed-wait,e2e/legacy.spec.ts:no-assertion" &&
		strings.Join(got, ",") != "e2e/legacy.spec.ts:no-assertion,e2e/pages/login.ts:fixed-wait" {
		t.Fatalf("changed-line findings = %v", got)
	}
	all, err := Lint(repo, LintOptions{Base: base, Revision: head, AllLines: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Findings) != 4 {
		t.Fatalf("all-lines findings = %+v", all.Findings)
	}
	if _, err := Lint(repo, LintOptions{Base: "no-such-rev", Revision: head}); !hasCode(err, "tests-invalid-revision") {
		t.Fatalf("bad base error = %v", err)
	}
}

func hasCode(err error, code string) bool {
	var typed *Error
	return errors.As(err, &typed) && typed.Code == code
}

const (
	cartA = `test.describe('cart', () => {
  test('adds an item', async ({ page }) => {
    await expect(page.getByTestId('count')).toHaveText('1');
    await expect(page.getByTestId('total')).toHaveText('$5');
    await expect(page).toHaveURL('/cart');
  });
});
`
	cartB = `test('basket gains an item', async ({ page }) => {
  await expect(page.getByTestId('count'))
    .toHaveText('1');
  await expect(page.getByTestId('total')).toHaveText("$5");
  await expect(page).toHaveURL('/cart');
});
test('unrelated', async ({ page }) => {
  await expect(page).toHaveURL('/cart');
  await expect(page.getByTestId('x')).toHaveText('y');
});
`
)

func duplicateOptions() DuplicatesOptions {
	return DuplicatesOptions{Revision: "HEAD", Threshold: 0.5, MaxLineTests: 2}
}

// TestDuplicatesReport proves normalised assertion lines pair tests across files, a line held by
// more tests than the cutoff is excluded, and the report says pairs are candidates
// (TSA-V0-020, TSA-V0-023).
func TestDuplicatesReport(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"e2e/a.spec.ts": cartA, "e2e/b.spec.ts": cartB,
		"e2e/c.spec.ts": strings.ReplaceAll(cartA, "'cart'", "'cart copy'")})
	report, err := Duplicates(OpenRepo(context.Background(), root), duplicateOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.Note != CandidateNote || report.Tests != 4 {
		t.Fatalf("report = %+v", report)
	}
	// toHaveURL('/cart') appears in four tests, above the cutoff of 2... and the count/total lines
	// in three, so with cutoff 2 every line is excluded and nothing pairs.
	if len(report.Unresolved) != 0 || report.ExcludedLines != 3 {
		t.Fatalf("cutoff 2: unresolved=%+v excluded=%d", report.Unresolved, report.ExcludedLines)
	}
	options := duplicateOptions()
	options.MaxLineTests = 3
	report, err = Duplicates(OpenRepo(context.Background(), root), options)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Unresolved) != 3 || report.ExcludedLines != 1 {
		t.Fatalf("cutoff 3: unresolved=%+v excluded=%d", report.Unresolved, report.ExcludedLines)
	}
	first := report.Unresolved[0]
	if first.Score != 1 || len(first.SharedLines) != 2 || first.Test != "e2e/a.spec.ts :: cart > adds an item" ||
		first.Other != "e2e/b.spec.ts :: basket gains an item" {
		t.Fatalf("first pair = %+v", first)
	}
	if first.SharedLines[0] != `expect(page.getByTestId("count")).toHaveText("1")` {
		t.Fatalf("normalised line = %q", first.SharedLines[0])
	}
	encoded, _ := json.Marshal(report)
	again, _ := Duplicates(OpenRepo(context.Background(), root), options)
	if encodedAgain, _ := json.Marshal(again); string(encoded) != string(encodedAgain) {
		t.Fatal("duplicates report is not deterministic")
	}
}

// TestDuplicatesGateAndDecisions proves gate mode considers only pairs with an added or changed
// test, and that a reasoned distinct-from or an obligation-bearing covered-by resolves a pair
// while a reasonless one does not (TSA-V0-021, TSA-V0-022).
func TestDuplicatesGateAndDecisions(t *testing.T) {
	root := t.TempDir()
	third := strings.ReplaceAll(cartA, "'cart'", "'cart copy'")
	base := commitFiles(t, root, map[string]string{"e2e/a.spec.ts": cartA, "e2e/c.spec.ts": third})
	commitFiles(t, root, map[string]string{"e2e/b.spec.ts": cartB})
	repo := OpenRepo(context.Background(), root)
	options := duplicateOptions()
	options.MaxLineTests = 3
	options.Base, options.Gate = base, "V1-1592"
	report, err := Duplicates(repo, options)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.ChangedTests, ",") != "e2e/b.spec.ts :: basket gains an item,e2e/b.spec.ts :: unrelated" {
		t.Fatalf("changed tests = %v", report.ChangedTests)
	}
	if len(report.Unresolved) != 2 {
		t.Fatalf("gate pairs = %+v", report.Unresolved)
	}
	for _, p := range report.Unresolved {
		if !strings.HasPrefix(p.Test, "e2e/b.spec.ts") && !strings.HasPrefix(p.Other, "e2e/b.spec.ts") {
			t.Fatalf("gate kept a pair without a changed test: %+v", p)
		}
	}
	decisions := `{"schema":"corvint-test-duplicate-decisions/0","decisions":[
 {"decision":"distinct-from","test":"e2e/b.spec.ts :: basket gains an item","other":"e2e/a.spec.ts :: cart > adds an item","reason":"b checks the guest basket, a the signed-in cart"},
 {"decision":"distinct-from","test":"e2e/c.spec.ts :: cart copy > adds an item","other":"e2e/b.spec.ts :: basket gains an item"}
]}`
	commitFiles(t, root, map[string]string{DecisionsPath: decisions})
	report, err = Duplicates(repo, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Resolved) != 1 || report.Resolved[0].Decision != "distinct-from" || len(report.Unresolved) != 1 {
		t.Fatalf("after distinct-from: resolved=%+v unresolved=%+v", report.Resolved, report.Unresolved)
	}
	if len(report.IgnoredDecisions) != 1 || report.IgnoredDecisions[0].Problem != "missing-reason" {
		t.Fatalf("ignored = %+v", report.IgnoredDecisions)
	}
	covered := strings.Replace(decisions, `"other":"e2e/b.spec.ts :: basket gains an item"}`,
		`"other":"e2e/b.spec.ts :: basket gains an item","reason":"x"},
 {"decision":"covered-by","test":"e2e/c.spec.ts :: cart copy > adds an item","other":"e2e/b.spec.ts :: basket gains an item","obligation":"CART-001"}`, 1)
	covered = strings.Replace(covered, `"decision":"distinct-from","test":"e2e/c.spec.ts`, `"decision":"covered-by","test":"e2e/c.spec.ts`, 1)
	commitFiles(t, root, map[string]string{DecisionsPath: covered})
	report, err = Duplicates(repo, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Unresolved) != 0 || len(report.Resolved) != 2 {
		t.Fatalf("after covered-by: resolved=%+v unresolved=%+v ignored=%+v", report.Resolved, report.Unresolved, report.IgnoredDecisions)
	}
	commitFiles(t, root, map[string]string{DecisionsPath: `{"schema":"corvint-test-duplicate-decisions/0","decisions":[],"extra":1}`})
	if _, err := Duplicates(repo, options); !hasCode(err, "tests-decisions-invalid") {
		t.Fatalf("malformed decisions error = %v", err)
	}
	options.Base = ""
	if _, err := Duplicates(repo, options); !hasCode(err, "tests-invalid-option") {
		t.Fatalf("gate without base error = %v", err)
	}
}

// TestLintFollowsImportedHelpers proves helpers resolve into unchanged, relatively imported
// modules read at the revision (a `.js` specifier naming a `.ts` file, and an index module), that
// a helper whose module is not relatively imported is UNKNOWN, and that the advisory findings are
// not blocking (TSA-V0-030, TSA-V0-032).
func TestLintFollowsImportedHelpers(t *testing.T) {
	root := t.TempDir()
	base := commitFiles(t, root, map[string]string{
		"e2e/pages/panel.ts": `import { badge } from './parts/index.js';
export class Panel {
  constructor(page) { this.page = page; }
  async hasBadge() { return badge(this.page); }
}
`,
		"e2e/pages/parts/index.ts": "export const badge = async (page) => page.getByTestId('badge').isVisible();\n",
		"src/app.ts":               "export const x = 1;\n",
	})
	commitFiles(t, root, map[string]string{"e2e/panel.spec.ts": `import { test, expect } from '@playwright/test';
import { Panel } from './pages/panel.js';
import { other } from 'some-package';
test('panel', async ({ page }) => {
  const panel = new Panel(page);
  expect(await panel.hasBadge()).toBe(true);
  expect(await other(page)).toBe(true);
});
`})
	report, err := Lint(OpenRepo(context.Background(), root), LintOptions{Base: base, Revision: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Files, ",") != "e2e/panel.spec.ts" || len(report.Findings) != 2 {
		t.Fatalf("files = %v findings = %+v", report.Files, report.Findings)
	}
	if f := report.Findings[0]; f.Line != 6 || f.Resolution != "" ||
		!strings.Contains(f.Message, "isVisible() read (e2e/pages/parts/index.ts:1) returned through hasBadge (e2e/pages/panel.ts:4) -> badge (e2e/pages/parts/index.ts:1)") {
		t.Fatalf("imported helper finding = %+v", f)
	}
	if f := report.Findings[1]; f.Line != 7 || f.Resolution != "UNKNOWN" || !strings.Contains(f.Message, "no definition of other") {
		t.Fatalf("unresolved helper finding = %+v", f)
	}
	if report.Blocking() != 0 || report.CrossVariant.Reason != ReasonNoRunEvidence {
		t.Fatalf("blocking = %d cross-variant = %+v", report.Blocking(), report.CrossVariant)
	}
}

// TestLintMultiLineChangeScope proves a change on a later line of a multi-line assertion or locator
// brings its advisory finding into the changed-line scope, anchored on the first line and spanning
// the call (TSA-V0-012, TSA-V0-029, TSA-V0-031).
func TestLintMultiLineChangeScope(t *testing.T) {
	root := t.TempDir()
	spec := func(read, exact string) string {
		return `import { test, expect } from '@playwright/test';
test('shows the name', async ({ page }) => {
  expect(
    await page.getByTestId('rows').` + read + `
  ).toBe(3);
  await expect(page.getByText('Ada',
    { exact: ` + exact + ` })).toBeVisible();
});
`
	}
	base := commitFiles(t, root, map[string]string{"e2e/a.spec.ts": spec("evaluate((e) => e.childElementCount)", "false")})
	head := commitFiles(t, root, map[string]string{"e2e/a.spec.ts": spec("count()", "true")})
	runs := []RunEvidence{{Revision: head, Tests: []RunTest{
		{Project: "a", File: "a.spec.ts", TitlePath: []string{"shows the name"}, Status: "expected", Executed: true},
		{Project: "b", File: "a.spec.ts", TitlePath: []string{"shows the name"}, Status: "unexpected", Executed: true}}}}
	report, err := Lint(OpenRepo(context.Background(), root), LintOptions{Base: base, Revision: "HEAD", Runs: runs})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, f := range report.Findings {
		got = append(got, f.Rule+"@"+strconv.Itoa(f.Line)+"-"+strconv.Itoa(f.EndLine))
	}
	if want := []string{RuleOneShotUIState + "@3-5", RuleExactTextVariant + "@6-7"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %v, want %v: %+v", got, want, report.Findings)
	}
}

// TestLintImportedHelperUncertainty proves a relatively imported helper module the lexer cannot
// fully place is listed as uncertain, in path order, though it is not itself linted (TSA-V0-010).
func TestLintImportedHelperUncertainty(t *testing.T) {
	root := t.TempDir()
	base := commitFiles(t, root, map[string]string{
		"e2e/helpers/z.ts":  "export const late = async (page) => { return page.getByTestId('z').isVisible();\n",
		"e2e/helpers/a.ts":  "export const early = async (page) => { return page.getByTestId('a').count();\n",
		"e2e/helpers/ok.ts": "export const fine = 1;\n",
	})
	commitFiles(t, root, map[string]string{"e2e/a.spec.ts": `import { late } from './helpers/z';
import { early } from './helpers/a';
import { fine } from './helpers/ok';
test('t', async ({ page }) => {
  expect(await late(page)).toBe(true);
});
`})
	report, err := Lint(OpenRepo(context.Background(), root), LintOptions{Base: base, Revision: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Files, ",") != "e2e/a.spec.ts" || strings.Join(report.Uncertain, ",") != "e2e/helpers/a.ts,e2e/helpers/z.ts" {
		t.Fatalf("files = %v uncertain = %v", report.Files, report.Uncertain)
	}
}

// TestLintImportBudgetIsCumulative proves the linted files and every level of imported helper
// modules draw on one file and byte budget, so the aggregate cannot exceed it (TSA-V0-012,
// TSA-V0-030).
func TestLintImportBudgetIsCumulative(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"e2e/helpers/one.ts": "import { two } from './two';\nexport const one = async (page) => two(page);\n",
		"e2e/helpers/two.ts": "export const two = async (page) => page.getByTestId('x').isVisible();\n",
	}
	spec := "import { one } from './helpers/one';\ntest('t', async ({ page }) => {\n  expect(await one(page)).toBe(true);\n});\n"
	base := commitFiles(t, root, files)
	commitFiles(t, root, map[string]string{"e2e/a.spec.ts": spec})
	r, options := OpenRepo(context.Background(), root), LintOptions{Base: base, Revision: "HEAD"}
	total := len(spec) + len(files["e2e/helpers/one.ts"]) + len(files["e2e/helpers/two.ts"]) + 3*128
	for _, tc := range []struct {
		budget readBudget
		ok     bool
	}{{readBudget{files: 3, bytes: total}, true}, {readBudget{files: 2, bytes: total}, false}, {readBudget{files: 3, bytes: total - 1}, false}} {
		budget := tc.budget
		report, err := lint(r, options, &budget)
		if tc.ok {
			if err != nil || len(report.Findings) != 1 || report.Findings[0].Resolution != "" {
				t.Fatalf("budget %+v: err = %v findings = %+v", tc.budget, err, report.Findings)
			}
			continue
		}
		if !hasCode(err, "tests-bound-exceeded") {
			t.Fatalf("budget %+v: err = %v, want tests-bound-exceeded", tc.budget, err)
		}
	}
}
