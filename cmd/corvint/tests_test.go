package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testsource"
	"github.com/Beamfall/corvint/internal/testvalidity"
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
	if code, out, _ := runCLI(t, "--help"); code != 0 || !strings.Contains(out, "\n    tests  ") || !strings.Contains(out, " (TSA-V0)\n") {
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
	// --all-lines needs no base (TSA-V0-650).
	if code, out, _ := runCLI(t, "--root", root, "tests", "lint", "--all-lines"); code != 1 || !strings.Contains(out, "base none") ||
		!strings.Contains(out, "e2e/basket.spec.ts:7: fixed-wait:") {
		t.Fatalf("lint --all-lines without --base: code=%d out=%q", code, out)
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

const testsSerialSpec = `test.describe('alpha', () => {
  test.describe.configure({ mode: 'serial' });
  test('alpha-01 existing', async () => { expect(1).toBe(1); });
  test('alpha-02 new row', { tag: '@isolated' }, async () => { expect(2).toBe(2); });
});
`

const testsMemoizedSpec = `RSpec.describe Widget do
  subject(:widget) { create(:widget) }
  before { widget }
  it 'creates a widget' do
    expect { widget }.to change(Widget, :count)
  end
end
`

// TestTestsValidityCLI: `corvint tests validity` reports the ISOLATED-IN-SERIAL and
// MEMOIZED-BLOCK-EXPECTATION findings of the named committed files and refuses an example with an
// invalid finding as proof (exit 1, hygiene INELIGIBLE); a JS/TS file lists every example with
// its strength; advisory-only files exit 0; bad arguments refuse with exit 2 (TSA-V0-071,
// TSA-V0-081, V1-1787).
func TestTestsValidityCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	head := testsCommit(t, root, map[string]string{"e2e/alpha.spec.ts": testsSerialSpec, "spec/widget_spec.rb": testsMemoizedSpec})
	code, out, errb := runCLI(t, "--root", root, "tests", "validity", "--files", "e2e/alpha.spec.ts,spec/widget_spec.rb")
	var report struct {
		Schema, Revision string
		Files            []struct {
			Path     string
			Findings []struct{ Rule, Severity, Test string }
		}
		Tests []struct {
			Test     string
			Hygiene  struct{ State, Reason string }
			Strength *struct{ State, Reason string }
		}
	}
	if code != 1 || json.Unmarshal([]byte(out), &report) != nil || report.Schema != "corvint-test-source-validity/0" || report.Revision != head || len(report.Files) != 2 {
		t.Fatalf("validity: code=%d out=%s err=%s", code, out, errb)
	}
	if f := report.Files[0].Findings; len(f) != 3 || f[0].Rule != "ISOLATED-IN-SERIAL" || f[0].Severity != "advisory" || f[1].Rule != "JS-CONSTANT-EXPECTATION" || f[2].Rule != "JS-CONSTANT-EXPECTATION" {
		t.Errorf("serial findings %+v", f)
	}
	if f := report.Files[1].Findings; len(f) != 1 || f[0].Rule != "MEMOIZED-BLOCK-EXPECTATION" || f[0].Test != "spec/widget_spec.rb:4" {
		t.Errorf("memoized findings %+v", f)
	}
	// One entry per JS example in line order, each with a NOT_MEASURED strength (TSA-V0-081).
	if len(report.Tests) != 3 || report.Tests[0].Test != "e2e/alpha.spec.ts:3" || report.Tests[1].Test != "e2e/alpha.spec.ts:4" ||
		report.Tests[0].Hygiene.State != "ELIGIBLE" || report.Tests[1].Hygiene.State != "ELIGIBLE" ||
		report.Tests[1].Strength == nil || report.Tests[1].Strength.State != "NOT_MEASURED" || report.Tests[1].Strength.Reason != "constant-expectations-only" ||
		report.Tests[2].Strength != nil || report.Tests[2].Hygiene.State != "INELIGIBLE" || report.Tests[2].Hygiene.Reason != "memoized-block-expectation" {
		t.Errorf("tests %+v", report.Tests)
	}
	if code, _, errb := runCLI(t, "--root", root, "tests", "validity", "--files", "e2e/alpha.spec.ts", "--rev", head); code != 0 {
		t.Errorf("advisory only: code=%d err=%s", code, errb)
	}
	for _, args := range [][]string{{}, {"--files", "../x_spec.rb"}, {"--files", "spec/absent_spec.rb"}, {"--files", "a_spec.rb", "extra"}} {
		if code, out, _ := runCLI(t, append([]string{"--root", root, "tests", "validity"}, args...)...); code != 2 || out != "" {
			t.Errorf("%v: code=%d out=%q", args, code, out)
		}
	}
	if code, out, _ := runCLI(t, "tests", "--help"); code != 0 || !strings.Contains(out, "tests validity --files") {
		t.Errorf("help: code=%d", code)
	}
}

// TestTSAV0108SeedParityCLI: --legacy NEW=LEGACY (repeatable) adds the advisory seedParity member
// without --files and never changes the exit code; a malformed link refuses with exit 2
// (TSA-V0-100, TSA-V0-108).
func TestTSAV0108SeedParityCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	testsCommit(t, root, map[string]string{
		"spec/legacy_spec.rb": "RSpec.describe A do\n  it 'a' do\n    create_list(:item, 2)\n    expect(Item.count).to eq(2)\n  end\nend\n",
		"spec/new_spec.rb":    "RSpec.describe A do\n  it 'a' do\n    create(:item)\n    expect(Item.count).to eq(1)\n  end\nend\n",
		"spec/skip_spec.rb":   "RSpec.describe A do\n  xit 'a' do\n    create_list(:item, 2)\n  end\nend\n",
	})
	code, out, errb := runCLI(t, "--root", root, "tests", "validity", "--legacy", "spec/new_spec.rb:2=spec/legacy_spec.rb:2",
		"--legacy", "spec/new_spec.rb:2=spec/new_spec.rb:2")
	var report struct {
		Files      []json.RawMessage
		SeedParity []struct {
			New, Legacy, Status string
			Findings            []struct{ Rule, Severity, Location, Cause, Message string }
		}
	}
	if code != 0 || json.Unmarshal([]byte(out), &report) != nil || len(report.Files) != 0 || len(report.SeedParity) != 2 {
		t.Fatalf("seed parity: code=%d out=%s err=%s", code, out, errb)
	}
	p := report.SeedParity[0]
	if p.New != "spec/new_spec.rb:2" || p.Legacy != "spec/legacy_spec.rb:2" || p.Status != "diverges" || len(p.Findings) != 1 ||
		p.Findings[0].Rule != "SEED-PARITY-DIVERGES" || p.Findings[0].Severity != "advisory" || p.Findings[0].Location != "spec/new_spec.rb:3" ||
		p.Findings[0].Message != "legacy seeds 2 item, new seeds 1; asserting lines reading item: legacy spec/legacy_spec.rb:4; new spec/new_spec.rb:4" {
		t.Errorf("link %+v", p)
	}
	if p := report.SeedParity[1]; p.Status != "parity" || len(p.Findings) != 0 {
		t.Errorf("self link %+v", p)
	}
	// A skipped new test still exits 1 from its hygiene alone; seed parity adds nothing to it.
	if code, _, errb := runCLI(t, "--root", root, "tests", "validity", "--files", "spec/skip_spec.rb", "--legacy", "spec/skip_spec.rb:2=spec/legacy_spec.rb:2"); code != 1 {
		t.Errorf("hygiene exit: code=%d err=%s", code, errb)
	}
	for _, args := range [][]string{{"--legacy", "spec/new_spec.rb:2"}, {"--legacy", "=spec/legacy_spec.rb:2"}, {"--legacy", "spec/new_spec.rb=spec/legacy_spec.rb:2"}} {
		if code, out, _ := runCLI(t, append([]string{"--root", root, "tests", "validity"}, args...)...); code != 2 || out != "" {
			t.Errorf("%v: code=%d out=%q", args, code, out)
		}
	}
	if code, out, _ := runCLI(t, "tests", "--help"); code != 0 || !strings.Contains(out, "--legacy NEW=LEGACY") {
		t.Errorf("help: code=%d", code)
	}
}

// TestTestsCitationsCLI: `corvint tests citations` prints the corvint-source-citations/1 report,
// exits 1 while a citation is unsupported and 0 when every citation holds; bad arguments refuse
// with exit 2 (TSA-V0-255).
func TestTestsCitationsCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	testsCommit(t, root, map[string]string{"a/b.go": "package a\n\nfunc B() {}\n"})
	head := testsCommit(t, root, map[string]string{
		"docs/bad.md":  "See `a/b.go:2-9` and `a/b.go:3#B`.\n",
		"docs/good.md": "See `a/b.go:1-3#B`.\n",
		"docs/sub.md":  "<sub>a/b.go line two</sub>\n",
	})
	code, out, errb := runCLI(t, "--root", root, "tests", "citations", "--docs", "docs/bad.md")
	var report struct {
		Schema, Revision string
		Checked          int
		Findings         []struct {
			Doc, Citation, Reason string
			Line                  int
		}
	}
	if code != 1 || json.Unmarshal([]byte(out), &report) != nil || report.Schema != "corvint-source-citations/1" || report.Revision != head || report.Checked != 2 ||
		len(report.Findings) != 1 || report.Findings[0].Reason != "range-out-of-bounds" || report.Findings[0].Line != 1 {
		t.Fatalf("citations: code=%d out=%s err=%s", code, out, errb)
	}
	if code, _, errb := runCLI(t, "--root", root, "tests", "citations", "--docs", "docs/good.md", "--rev", head); code != 0 {
		t.Errorf("valid: code=%d err=%s", code, errb)
	}
	// V1-1972: a glob selects documents, a profile or pattern reads them, totals are per document and
	// an unparseable citation fails the run (TSA-V0-380..386).
	var selected testsource.CitationReport
	if code, out, errb := runCLI(t, "--root", root, "tests", "citations", "--glob", "docs/*.md", "--syntax", "html-sub"); code != 1 || json.Unmarshal([]byte(out), &selected) != nil ||
		len(selected.Unparseable) != 1 || selected.Unparseable[0].Doc != "docs/sub.md" || selected.Unparseable[0].Citation != "a/b.go line two" ||
		len(selected.Totals) != 3 || selected.Totals[2] != (testsource.CitationDocumentTotal{Doc: "docs/sub.md", Unparseable: 1}) {
		t.Errorf("html-sub: code=%d out=%s err=%s", code, out, errb)
	}
	selected = testsource.CitationReport{}
	if code, out, errb := runCLI(t, "--root", root, "tests", "citations", "--docs", "docs/good.md", "--pattern", "`(?P<path>[a-z/.]+):(?P<start>[0-9]+)-(?P<end>[0-9]+)#B`", "--repository", "core"); code != 0 ||
		json.Unmarshal([]byte(out), &selected) != nil || selected.Syntax != "pattern" || selected.Checked != 1 {
		t.Errorf("pattern: code=%d out=%s err=%s", code, out, errb)
	}
	for _, args := range [][]string{{}, {"--docs", "../x.md"}, {"--docs", "docs/absent.md"}, {"--docs", "docs/good.md", "extra"},
		{"--glob", "docs/*.md", "--syntax", "nosuch"}, {"--glob", "docs/*.md", "--pattern", "(?P<x>y)"}, {"--glob", "docs/*.md", "--syntax", "html-sub", "--syntax", "html-sub"}} {
		if code, out, _ := runCLI(t, append([]string{"--root", root, "tests", "citations"}, args...)...); code != 2 || out != "" {
			t.Errorf("%v: code=%d out=%q", args, code, out)
		}
	}
	if code, out, _ := runCLI(t, "tests", "--help"); code != 0 || !strings.Contains(out, "tests citations --docs") {
		t.Errorf("help: code=%d", code)
	}
}

const exampleJoinSpec = `RSpec.describe User do
  subject(:user) { build(:user) }

  it 'is valid' do
    expect(user).to be_valid
  end
  xit 'is skipped' do
    expect(user).to be_valid
  end
  it 'has a name' do
    expect(user.name).to eq('u')
  end
end
`

// TestTestsExampleJoinCLI pins TSA-V0-410, TSA-V0-413, TSA-V0-417 and TSA-V0-418 at the CLI: one
// corvint-example-join/0 record per receipt example with caller-asserted requirement IDs from
// --select and --requirements, and typed refusals for a malformed or unknown selector and a receipt
// run at another revision.
func TestTestsExampleJoinCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	earlier := testsCommit(t, root, map[string]string{"README.md": "fixture\n"})
	helper := "RSpec.configure { }\n"
	head := testsCommit(t, root, map[string]string{"spec/user_spec.rb": exampleJoinSpec, "spec/spec_helper.rb": helper})
	repository := &jstestprovider.ApplicationRepositoryIdentity{RootCommit: earlier, Revision: head, Tree: cemGit(t, root, "rev-parse", head+"^{tree}"), DirtyState: "clean"}
	helpers := []rubytestprovider.BoundFile{{Path: "spec/spec_helper.rb", Blob: rubytestprovider.BlobID("sha1", []byte(helper))}}
	document := rubytestprovider.Document{
		Profile: rubytestprovider.Profile, Runner: "rspec", ProjectDirectory: ".", ObjectFormat: "sha1",
		RepositoryBefore: repository, RepositoryAfter: repository, HelperBasis: rubytestprovider.HelperBasis,
		Helpers: helpers, HelperSet: rubytestprovider.HelperSetDigest(helpers), Examples: []rubytestprovider.Example{}, JUnit: rubytestprovider.JUnit{State: "NOT_REQUESTED"},
	}
	for index, e := range []struct {
		line   int
		status string
	}{{4, "passed"}, {7, "pending"}, {10, "failed"}} {
		id := "./spec/user_spec.rb[1:" + strconv.Itoa(index+1) + "]"
		document.Examples = append(document.Examples, rubytestprovider.Example{
			ID: "spec/user_spec.rb:" + strconv.Itoa(e.line), RSpecID: id, File: "spec/user_spec.rb", Line: e.line, Status: e.status,
			SpecBlob: rubytestprovider.BlobID("sha1", []byte(exampleJoinSpec)), Helpers: document.HelperSet,
		})
	}
	evidence, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	receipt, mapping := filepath.Join(scratch, "receipt.json"), filepath.Join(scratch, "requirements.txt")
	if err := os.WriteFile(receipt, evidence, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mapping, []byte("# caller-asserted\nspec/user_spec.rb:10=TSA-V0-419\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errb := runCLI(t, "--root", root, "tests", "example-join", "--receipt", receipt, "--select", "spec/user_spec.rb[1:1]=TSA-V0-417,TSA-V0-410", "--requirements", mapping)
	var report testsource.ExampleJoinReport
	if code != 0 || json.Unmarshal([]byte(out), &report) != nil || report.Schema != testsource.ExampleJoinSchema || report.Revision != head || len(report.Examples) != 3 {
		t.Fatalf("example-join: code=%d out=%s err=%s", code, out, errb)
	}
	first, second, third := report.Examples[0], report.Examples[1], report.Examples[2]
	if first.Example != "spec/user_spec.rb[1:1]" || first.Outcome != "passed" || third.Outcome != "failed" || second.Requirements != nil ||
		first.Verdict.Hygiene.State == testvalidity.HygieneAbstained || len(first.Source.Files) != 2 || first.Source.Files[1].Path != "spec/spec_helper.rb" {
		t.Fatalf("records = %+v", report.Examples)
	}
	if r := first.Requirements; r == nil || r.Basis != "caller-asserted" || strings.Join(r.IDs, ",") != "TSA-V0-410,TSA-V0-417" {
		t.Fatalf("first requirements = %+v", r)
	}
	if r := third.Requirements; r == nil || strings.Join(r.IDs, ",") != "TSA-V0-419" || strings.Join(r.Selectors, ",") != "spec/user_spec.rb:10" {
		t.Fatalf("third requirements = %+v", r)
	}
	for _, c := range []struct {
		args []string
		code string
	}{
		{[]string{"--select", "spec/user_spec.rb"}, "tests-join-selector-malformed"},
		{[]string{"--select", "spec/user_spec.rb[1:9]=REQ-1"}, "tests-join-selector-unknown"},
		{[]string{"--rev", earlier}, "tests-join-revision-mismatch"},
		{[]string{"--requirements", filepath.Join(scratch, "missing.txt")}, "tests-source-unavailable"},
	} {
		code, _, errb := runCLI(t, append([]string{"--root", root, "tests", "example-join", "--receipt", receipt}, c.args...)...)
		if code != 2 || !strings.Contains(errb, `"code":"`+c.code+`"`) {
			t.Errorf("%v: code=%d err=%s, want %s", c.args, code, errb, c.code)
		}
	}
	if code, _, errb := runCLI(t, "--root", root, "tests", "example-join"); code != 2 || !strings.Contains(errb, "tests-invalid-arguments") {
		t.Errorf("no receipt: code=%d err=%s", code, errb)
	}
}
