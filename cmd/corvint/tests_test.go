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
	t.Parallel()
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
	testsCommit(t, root, map[string]string{
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

	for _, args := range [][]string{{"tests", "nope"}, {"tests"}} {
		code, out, errb := runCLI(t, append([]string{"--root", root}, args...)...)
		if code != 2 || out != "" || !strings.Contains(errb, `"ok":false`) {
			t.Fatalf("%v: code=%d out=%q err=%q", args, code, out, errb)
		}
	}

}

// TestTestsCitationsCLI checks committed source citations.
func TestTestsCitationsCLI(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
