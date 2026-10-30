package witness

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gotestrun"
)

// goEvidenceFixture commits a repository whose Go module lives under svc/ with
// module path example.test/svc, changes a package and its test in the
// reported range, and runs the real gotestrun runner, which runs the real
// go test -json, against the head. The real stream carries package-qualified
// import paths and subtests; the document maps them to svc/calc through the
// module path. It returns the head index, the base and the document.
func goEvidenceFixture(t *testing.T) (*contextindex.Index, string, gotestrun.Document) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
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
	write := func(name, body string) {
		t.Helper()
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("svc/go.mod", "module example.test/svc\n\ngo 1.21\n")
	write("svc/calc/calc.go", "package calc\n\nfunc Add(a, b int) int { return a + b }\n")
	write("svc/calc/calc_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"sum\")\n\t}\n}\n")
	fixtureGit("add", ".")
	fixtureGit("commit", "--quiet", "-m", "base")
	base := fixtureGit("rev-parse", "HEAD")
	write("svc/calc/calc.go", "package calc\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Neg(a int) int { return a }\n")
	write("svc/calc/calc_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"sum\")\n\t}\n}\n\nfunc TestNeg(t *testing.T) {\n\tt.Run(\"positive\", func(t *testing.T) {\n\t\tif Neg(2) != -2 {\n\t\t\tt.Fatal(\"neg\")\n\t\t}\n\t})\n}\n")
	fixtureGit("commit", "--quiet", "-am", "change")

	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	document, err := gotestrun.Run(ctx, gotestrun.Config{Dir: filepath.Join(resolved, "svc"), Timeout: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index, base, document
}

// TestGoEvidenceBindsToTheHeadTree pins AGW-V0-012: a corvint-go-test-run/0
// document from a clean run at the head binds, each test (subtests included)
// attaches a non-closing CALLER_REPORTED witness to the _test.go file that
// declares its top-level function, and nothing attaches to the package file
// the document does not name.
func TestGoEvidenceBindsToTheHeadTree(t *testing.T) {
	index, base, document := goEvidenceFixture(t)
	if document.Infrastructure != nil || document.Module != "example.test/svc" || document.ModuleDirectory != "svc" || len(document.Tests) != 3 {
		encoded, _ := json.Marshal(document)
		t.Fatalf("the runner produced an unexpected document: %s", encoded)
	}
	report, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{writeEvidence(t, document)}})
	if err != nil {
		t.Fatal(err)
	}
	source := report.Sources[1]
	if source.Status != "BOUND" || source.Basis != 3 || source.Unattached != 0 || source.Hunks != 3 {
		t.Fatalf("source = %+v, want BOUND with three attached tests", source)
	}
	relations := map[string]string{}
	for _, candidate := range evidenceWitnesses(report, "svc/calc/calc_test.go") {
		if candidate.Closing || candidate.Authority != "CALLER_REPORTED" || candidate.EvidencePath != "svc/calc/calc_test.go" {
			t.Fatalf("witness = %+v, want a non-closing CALLER_REPORTED witness on the test file", candidate)
		}
		relations[candidate.Test] = candidate.Relation
	}
	if relations["example.test/svc/calc.TestAdd"] != "test-run:pass" || relations["example.test/svc/calc.TestNeg"] != "test-run:fail" || relations["example.test/svc/calc.TestNeg/positive"] != "test-run:fail" {
		t.Fatalf("relations = %v", relations)
	}
	if found := evidenceWitnesses(report, "svc/calc/calc.go"); len(found) != 0 {
		t.Fatalf("the package file the document does not name has evidence %+v", found)
	}
}

// TestGoEvidenceNotBoundToTheHeadTreeAttachesNothing pins the AGW-V0-012
// failure modes, the RSpec conditions: each mismatch is UNBOUND and attaches
// nothing, and a document that names the profile but breaks its closed shape
// refuses the report.
func TestGoEvidenceNotBoundToTheHeadTreeAttachesNothing(t *testing.T) {
	index, base, real := goEvidenceFixture(t)
	other := strings.Repeat("ab", 20)
	mutate := func(change func(*gotestrun.Document)) gotestrun.Document {
		var document gotestrun.Document
		data, err := json.Marshal(real)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		change(&document)
		return document
	}
	cases := []struct {
		name     string
		document gotestrun.Document
		reason   string
	}{
		{"test blob", mutate(func(d *gotestrun.Document) {
			for position := range d.Tests {
				d.Tests[position].Blob = other
			}
		}), "blob ID for svc/calc/calc_test.go differs from the head blob"},
		{"absent file", mutate(func(d *gotestrun.Document) { d.Tests[0].File = "svc/calc/missing_test.go" }), "svc/calc/missing_test.go, which is not a blob in the head tree"},
		{"tree", mutate(func(d *gotestrun.Document) {
			d.RepositoryBefore.Tree, d.RepositoryAfter.Tree = other, other
		}), "not the report head tree"},
		{"revision", mutate(func(d *gotestrun.Document) {
			d.RepositoryBefore.Revision, d.RepositoryAfter.Revision = other, other
		}), "not the report head " + index.CommitRevision},
		{"dirty", mutate(func(d *gotestrun.Document) {
			d.RepositoryBefore.DirtyState, d.RepositoryBefore.DirtyDigest = "dirty", "sha256:"+strings.Repeat("0", 64)
			*d.RepositoryAfter = *d.RepositoryBefore
		}), "not clean during the go test run"},
		{"moved", mutate(func(d *gotestrun.Document) { d.RepositoryAfter.Revision = other }), "changed between the start and the end"},
		{"no before", mutate(func(d *gotestrun.Document) { d.RepositoryBefore = nil }), "both before and after"},
		{"infrastructure", mutate(func(d *gotestrun.Document) {
			d.Infrastructure = &gotestrun.Infrastructure{Reason: "timeout", Detail: "go test did not finish"}
		}), "infrastructure failure (timeout)"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			report, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{writeEvidence(t, testCase.document)}})
			if err != nil {
				t.Fatal(err)
			}
			source := report.Sources[1]
			if source.Status != "UNBOUND" || !strings.Contains(source.Detail, testCase.reason) || source.Basis != 0 {
				t.Fatalf("source = %+v, want UNBOUND naming %q", source, testCase.reason)
			}
			if found := evidenceWitnesses(report, "svc/calc/calc_test.go"); len(found) != 0 {
				t.Fatalf("an unbound document attached %+v", found)
			}
		})
	}
	open := mutate(func(d *gotestrun.Document) { d.Tests[0].File, d.Tests[0].Unbound = "", "" })
	if _, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{writeEvidence(t, open)}}); err == nil || !strings.Contains(err.Error(), "neither a file nor an unbound reason") {
		t.Fatalf("a malformed go test run document did not refuse: %v", err)
	}
}

// TestGoEvidenceUnanchoredTestAttachesNothing pins that a test the provider
// could not anchor to one declaring file attaches nothing and is counted.
func TestGoEvidenceUnanchoredTestAttachesNothing(t *testing.T) {
	document := &gotestrun.Document{Tests: []gotestrun.Test{
		{Package: "example.test/other", Name: "TestX", Action: "pass", Unbound: "the package is outside module example.test/svc"},
		{Package: "example.test/svc", Name: "TestY", Action: "pass", File: "svc/y_test.go", Blob: strings.Repeat("ab", 20)},
	}}
	obligations := []Obligation{{ID: "change:svc/y_test.go", Kind: KindChange, Path: "svc/y_test.go", Witnesses: []Witness{}}}
	attached, unattached := attachEvidence(obligations, goTests(document))
	if attached != 1 || unattached != 1 || obligations[0].Witnesses[0].Test != "example.test/svc.TestY" {
		t.Fatalf("attached=%d unattached=%d witnesses=%+v", attached, unattached, obligations[0].Witnesses)
	}
}
