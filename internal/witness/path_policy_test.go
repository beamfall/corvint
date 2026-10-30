package witness

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// policyFixture commits base files (with policy, when non-empty, at the
// base), then the head files, where an empty body deletes, and compiles the report over the range.
func policyFixture(t *testing.T, policy string, base, head map[string]string) *Report {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := initTestRepo(t, git)
	fixtureGit := func(args ...string) string {
		t.Helper()
		command := exec.Command(git, append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test"}, args...)...)
		command.Dir, command.Env = root, gitEnvironment()
		raw, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %v: %s", args, err, raw)
		}
		return strings.TrimSpace(string(raw))
	}
	write := func(files map[string]string) {
		for name, body := range files {
			if body == "" {
				if err := os.Remove(filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
				t.Fatal(err)
			}
			if target, link := strings.CutPrefix(body, symlinkTo); link {
				if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	files := map[string]string{
		"go.mod":                 "module example.com/fixture\n\ngo 1.22\n",
		"pkg/a/a.go":             "package a\n\nfunc A() int { return 1 }\n",
		"pkg/a/a_test.go":        "package a\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestA(t *testing.T) { _, _ = os.ReadFile(\"testdata/in.json\"); _ = A() }\n",
		"pkg/a/testdata/in.json": "{}\n",
		"docs/guide.md":          "# Guide\n",
		"config/app.yaml":        "k: v\n",
	}
	for name, body := range base {
		files[name] = body
	}
	if policy != "" {
		files[PathPolicyPath] = policy
	}
	write(files)
	fixtureGit("add", ".")
	fixtureGit("commit", "--quiet", "-m", "base")
	baseCommit := fixtureGit("rev-parse", "HEAD")
	write(head)
	fixtureGit("add", ".")
	fixtureGit("commit", "--quiet", "-m", "change")
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Compile(context.Background(), index, Options{Base: baseCommit})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// symlinkTo prefixes a fixture body that commits a symbolic link to the rest.
const symlinkTo = "\x00symlink:"

// issueRange is the GitHub #1962 shape: a Go change with Markdown, JSON and
// YAML changes beside it.
var issueRange = map[string]string{
	"pkg/a/a.go":             "package a\n\nfunc A() int { return 2 }\n",
	"pkg/a/testdata/in.json": "{\"a\": 1}\n",
	"docs/guide.md":          "# Guide, revised\n",
	"config/app.yaml":        "k: w\n",
}

const issuePolicy = `{
  "profile": "corvint-witness-paths/0",
  "rules": [
    {"glob": "docs/**", "class": "inert"},
    {"glob": "config/*.yaml", "class": "data", "readers": ["pkg/a/a.go"]},
    {"glob": "**/testdata/*.json", "class": "data"},
    {"glob": "**", "class": "inert"}
  ]
}
`

func obligation(t *testing.T, report *Report, id string) Obligation {
	t.Helper()
	for _, item := range report.Obligations {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("obligation %s missing from %+v", id, report.Obligations)
	return Obligation{}
}

// TestAGWV0041PolicyIsReadFromTheBaseTree: a policy committed only in the
// head tree classifies nothing and the report has no pathPolicy member, so a
// range cannot exempt its own paths; undeclared paths keep NOT_RUN.
func TestAGWV0041PolicyIsReadFromTheBaseTree(t *testing.T) {
	head := map[string]string{PathPolicyPath: issuePolicy}
	for name, body := range issueRange {
		head[name] = body
	}
	report := policyFixture(t, "", nil, head)
	if report.PathPolicy != nil {
		t.Fatalf("a head-only policy was read: %+v", report.PathPolicy)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "pathPolicy") || strings.Contains(string(encoded), "declaration") || strings.Contains(Render(report), "PATH POLICY") {
		t.Fatal("a report without a base policy changed its bytes")
	}
	for _, id := range []string{"change:docs/guide.md", "change:config/app.yaml", "change:pkg/a/testdata/in.json", "change:" + PathPolicyPath} {
		if got := obligation(t, report, id); got.Verdict != NotRun || got.Reason != "LANGUAGE_COVERAGE_NONE" {
			t.Fatalf("%s = %s %s, want NOT_RUN LANGUAGE_COVERAGE_NONE", id, got.Verdict, got.Reason)
		}
	}
}

// TestAGWV0041InvalidPolicyAppliesNoRule: each defect makes the policy
// INVALID, applies no rule and names WITNESS-P9.
func TestAGWV0041InvalidPolicyAppliesNoRule(t *testing.T) {
	for name, policy := range map[string]string{
		"malformed":      `{"profile":`,
		"profile":        `{"profile":"corvint-witness-paths/9","rules":[]}`,
		"unknown member": `{"profile":"corvint-witness-paths/0","rules":[{"glob":"docs/**","class":"inert","why":"x"}]}`,
		"class":          `{"profile":"corvint-witness-paths/0","rules":[{"glob":"docs/**","class":"code"}]}`,
		"glob":           `{"profile":"corvint-witness-paths/0","rules":[{"glob":"../docs/**","class":"inert"}]}`,
		"inert readers":  `{"profile":"corvint-witness-paths/0","rules":[{"glob":"docs/**","class":"inert","readers":["pkg/a/a.go"]}]}`,
		"no rules":       `{"profile":"corvint-witness-paths/0"}`,
		"case-folded":    `{"PROFILE":"corvint-witness-paths/0","rules":[{"glob":"docs/**","class":"inert"}]}`,
		"rule member":    `{"profile":"corvint-witness-paths/0","rules":[{"Glob":"docs/**","class":"inert"}]}`,
		"duplicate":      "{\"profile\":\"corvint-witness-paths/0\",\n\"rules\":[{\"glob\":\"x/**\",\"class\":\"inert\"}],\n\"rules\":[{\"glob\":\"docs/**\",\"class\":\"inert\"}]}",
	} {
		t.Run(name, func(t *testing.T) {
			report := policyFixture(t, policy, nil, issueRange)
			if report.PathPolicy == nil || report.PathPolicy.Status != "INVALID" || report.PathPolicy.Detail == "" {
				t.Fatalf("path policy %+v, want INVALID with a detail", report.PathPolicy)
			}
			if got := obligation(t, report, "change:docs/guide.md"); got.Verdict != NotRun || got.Declaration != nil {
				t.Fatalf("an invalid policy classified docs/guide.md: %+v", got)
			}
			if !strings.Contains(identifiers(report.Preconditions), "WITNESS-P9-PATH-POLICY-INVALID") {
				t.Fatalf("preconditions %s lack WITNESS-P9", identifiers(report.Preconditions))
			}
		})
	}
}

// TestAGWV0042FirstMatchingRuleNamesFileAndLine: the first rule in file order
// applies, the declaration names the policy file and the rule's line, and a
// rule never reclassifies a path an extractor reads or a deletion.
func TestAGWV0042FirstMatchingRuleNamesFileAndLine(t *testing.T) {
	head := map[string]string{"docs/old.md": ""}
	for name, body := range issueRange {
		head[name] = body
	}
	report := policyFixture(t, issuePolicy, map[string]string{"docs/old.md": "# Old\n"}, head)
	if report.PathPolicy == nil || report.PathPolicy.Status != "BOUND" || report.PathPolicy.Rules != 4 {
		t.Fatalf("path policy %+v, want BOUND with 4 rules", report.PathPolicy)
	}
	for id, want := range map[string]Declaration{
		"change:docs/guide.md":          {File: PathPolicyPath, Line: 4, Glob: "docs/**", Class: ClassInert},
		"change:config/app.yaml":        {File: PathPolicyPath, Line: 5, Glob: "config/*.yaml", Class: ClassData},
		"change:pkg/a/testdata/in.json": {File: PathPolicyPath, Line: 6, Glob: "**/testdata/*.json", Class: ClassData},
	} {
		got := obligation(t, report, id).Declaration
		if got == nil || got.File != want.File || got.Line != want.Line || got.Glob != want.Glob || got.Class != want.Class {
			t.Fatalf("%s declaration %+v, want %+v", id, got, want)
		}
	}
	for _, item := range report.Obligations {
		if item.ID == "impact:data-reader:pkg/a/a.go" {
			t.Fatal("a reader the range changes opened a second obligation")
		}
	}
	if got := obligation(t, report, "change:pkg/a/a.go"); got.Declaration != nil || got.Coverage != "DEEP" {
		t.Fatalf("the catch-all inert rule reclassified a Go path: %+v", got)
	}
	if !strings.Contains(Render(report), "declared inert by .corvint/witness-paths.json:4 glob=docs/**") {
		t.Fatalf("text report does not print the declaration:\n%s", Render(report))
	}
	if got := obligation(t, report, "change:docs/old.md"); got.Verdict != Exempt || got.Declaration == nil || got.Declaration.Line != 4 {
		t.Fatalf("a deleted inert path was not exempt: %+v", got)
	}
	edited := policyFixture(t, issuePolicy, nil, map[string]string{PathPolicyPath: strings.Replace(issuePolicy, "docs/**", "docs/*", 1)})
	if got := obligation(t, edited, "change:"+PathPolicyPath); got.Verdict != NotRun || got.Declaration != nil {
		t.Fatalf("the catch-all inert rule classified the policy itself: %+v", got)
	}
	link := policyFixture(t, issuePolicy, map[string]string{"docs/link.md": symlinkTo + "guide.md"}, map[string]string{"docs/link.md": ""})
	if got := obligation(t, link, "change:docs/link.md"); got.Verdict != NotRun || got.Declaration != nil {
		t.Fatalf("a deleted symbolic link was exempt: %+v", got)
	}
	deleted := policyFixture(t, issuePolicy, nil, map[string]string{"pkg/a/testdata/in.json": ""})
	if got := obligation(t, deleted, "change:pkg/a/testdata/in.json"); got.Verdict != NotRun || got.Declaration != nil || got.Reason == "" {
		t.Fatalf("a deleted data path was reclassified: %+v", got)
	}
}

// TestAGWV0043InertPathIsExempt: an inert path is EXEMPT, counted in exempt
// and not in notRun, unproven or unwitnessed, and never closed.
func TestAGWV0043InertPathIsExempt(t *testing.T) {
	report := policyFixture(t, issuePolicy, nil, map[string]string{"docs/guide.md": "# Guide, revised\n"})
	got := obligation(t, report, "change:docs/guide.md")
	if got.Verdict != Exempt || got.Reason != "DECLARED_INERT" {
		t.Fatalf("docs/guide.md = %s %s, want EXEMPT DECLARED_INERT", got.Verdict, got.Reason)
	}
	summary := report.Summary
	if summary.Opened != 1 || summary.Exempt != 1 || summary.NotRun != 0 || summary.Unproven != 0 || summary.Unwitnessed != 0 || summary.Closed != 0 || summary.ProvenPerMille != 0 {
		t.Fatalf("summary %+v, want one exempt obligation and nothing else", summary)
	}
}

// TestAGWV0044DataPathNeedsAReader: a data path with a declared or found
// reader is admitted text-only and opens a data-reader obligation; without
// one, or with only an untracked declared reader, it stays NOT_RUN.
func TestAGWV0044DataPathNeedsAReader(t *testing.T) {
	report := policyFixture(t, issuePolicy, nil, map[string]string{"config/app.yaml": "k: w\n", "pkg/a/testdata/in.json": "{\"a\": 1}\n"})
	yaml := obligation(t, report, "change:config/app.yaml")
	if yaml.Verdict != Unproven || yaml.Coverage != "GENERIC" || len(yaml.Declaration.Readers) != 1 || yaml.Declaration.Readers[0] != (Reader{Path: "pkg/a/a.go", Origin: "declared"}) {
		t.Fatalf("declared-reader data path %+v", yaml)
	}
	if reader := obligation(t, report, "impact:data-reader:pkg/a/a.go"); reader.Kind != KindImpact || reader.Verdict != Unproven {
		t.Fatalf("data-reader obligation %+v", reader)
	}
	json := obligation(t, report, "change:pkg/a/testdata/in.json")
	if json.Verdict != Unproven || len(json.Declaration.Readers) != 1 || json.Declaration.Readers[0] != (Reader{Path: "pkg/a/a_test.go", Origin: "found"}) {
		t.Fatalf("found-reader data path %+v", json.Declaration)
	}
	if report.Summary.NotRun != 0 {
		t.Fatalf("summary %+v, want no NOT_RUN", report.Summary)
	}

	unread := `{"profile":"corvint-witness-paths/0","rules":[{"glob":"config/*.yaml","class":"data","readers":["pkg/missing.go"]},{"glob":"**/*.json","class":"data"},{"glob":"self/*.yaml","class":"data","readers":["self/a.yaml","docs/guide.md"]}]}`
	report = policyFixture(t, unread, map[string]string{"data/table.json": "[]\n", "self/a.yaml": "a: 1\n"}, map[string]string{"config/app.yaml": "k: w\n", "data/table.json": "[1]\n", "self/a.yaml": "a: 2\n"})
	if got := obligation(t, report, "change:self/a.yaml").Declaration.Readers; len(got) != 2 || got[0] != (Reader{Path: "self/a.yaml", Origin: "self"}) || got[1] != (Reader{Path: "docs/guide.md", Origin: "no-extractor"}) {
		t.Fatalf("self and no-extractor readers %+v", got)
	}
	for _, id := range []string{"change:config/app.yaml", "change:data/table.json", "change:self/a.yaml"} {
		if got := obligation(t, report, id); got.Verdict != NotRun || got.Reason != "DATA_READER_MISSING" {
			t.Fatalf("%s = %s %s, want NOT_RUN DATA_READER_MISSING", id, got.Verdict, got.Reason)
		}
	}
	if got := obligation(t, report, "change:config/app.yaml").Declaration.Readers; len(got) != 1 || got[0] != (Reader{Path: "pkg/missing.go", Origin: "untracked"}) {
		t.Fatalf("untracked declared reader %+v", got)
	}
}

// TestAGWV0044UnusableReaderDoesNotAdmitData: a declared reader the index did
// not admit as a source (a generated path or header), a symbolic link, or a
// source whose extraction was refused counts as no reader, so the data path
// stays NOT_RUN and --check=admitted cannot pass; the declaration keeps the
// reader's refusal reason, and no data-reader obligation is opened for it.
func TestAGWV0044UnusableReaderDoesNotAdmitData(t *testing.T) {
	for name, test := range map[string]struct {
		reader, body, reason string
	}{
		"generated path":   {"pkg/gen/reader.go", "package gen\n\nfunc Read() string { return \"config/app.yaml\" }\n", "NOT_ADMITTED"},
		"generated header": {"pkg/b/reader.go", "// Code generated by tool. DO NOT EDIT.\n\npackage b\n\nfunc Read() string { return \"config/app.yaml\" }\n", "NOT_ADMITTED"},
		"symbolic link":    {"pkg/b/link.go", symlinkTo + "../a/a.go", "SYMLINK_OUT_OF_PROFILE"},
		"parse refused":    {"pkg/b/bad.go", "package b\n\nfunc Read( string { return \"config/app.yaml\"\n", "GO_SOURCE_UNPARSED"},
	} {
		t.Run(name, func(t *testing.T) {
			policy := `{"profile":"corvint-witness-paths/0","rules":[{"glob":"config/*.yaml","class":"data","readers":["` + test.reader + `"]}]}`
			report := policyFixture(t, policy, map[string]string{test.reader: test.body}, map[string]string{"config/app.yaml": "k: w\n"})
			got := obligation(t, report, "change:config/app.yaml")
			if got.Verdict != NotRun || got.Reason != "DATA_READER_MISSING" {
				t.Fatalf("data path with an unusable reader = %s %s, want NOT_RUN DATA_READER_MISSING", got.Verdict, got.Reason)
			}
			readers := got.Declaration.Readers
			if len(readers) != 1 || readers[0].Path != test.reader || readers[0].Origin != "unusable" || !strings.Contains(readers[0].Reason, test.reason) {
				t.Fatalf("readers %+v, want %s unusable with a reason naming %s", readers, test.reader, test.reason)
			}
			for _, item := range report.Obligations {
				if item.Relation == "data-reader" {
					t.Fatalf("an unusable reader opened %+v", item)
				}
			}
			if report.Summary.NotRun == 0 {
				t.Fatalf("summary %+v admits the range", report.Summary)
			}
		})
	}
}
