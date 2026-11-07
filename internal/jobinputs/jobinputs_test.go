// SPDX-License-Identifier: Apache-2.0

package jobinputs

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testDeclaration = `{
  "profile": "corvint-ci-job-inputs/0",
  "jobs": [
    {"workflow": ".github/workflows/ci.yml", "job": "go-interop", "paths": ["interop/m/"]},
    {"workflow": ".github/workflows/ci.yml", "job": "archive", "paths": ["Makefile", "script/gate"],
     "goPackages": {"packages": ["./cmd/x"], "platforms": ["linux/amd64", "windows/amd64"]}, "exportedTree": true},
    {"workflow": ".github/workflows/intake.yml", "job": "containment", "goTests": ["./internal/intake"]}
  ]
}`

func parsed(t *testing.T) Declaration {
	t.Helper()
	declaration, err := Parse([]byte(testDeclaration))
	if err != nil {
		t.Fatal(err)
	}
	return declaration
}

func boundedEvidence(changed ...string) Evidence {
	return Evidence{
		Changed:   changed,
		Tree:      map[string]TreeEntry{},
		PlanScope: "BOUNDED",
		Selected:  map[string]string{},
		Module:    "example.com/m",
		GoDirectories: func(GoPackages) (map[string]string, error) {
			return map[string]string{"cmd/x": "example.com/m/cmd/x linux/amd64", "internal/winonly": "example.com/m/internal/winonly windows/amd64"}, nil
		},
	}
}

func decisions(results []JobResult) map[string]JobResult {
	byJob := map[string]JobResult{}
	for _, result := range results {
		byJob[result.Job] = result
	}
	return byJob
}

// TestEvaluateSkipsOnlyWithoutDeclaredInputChange is AFP-V0-2341 and
// AFP-V0-2342: each declared input class is a negative control that runs the
// job, and only a change outside every input skips it.
func TestEvaluateSkipsOnlyWithoutDeclaredInputChange(t *testing.T) {
	declaration := parsed(t)
	cases := []struct {
		name     string
		evidence Evidence
		want     map[string]string
	}{
		{"documentation only", boundedEvidence("docs/a.md"), map[string]string{"go-interop": ReasonNoDeclaredInputChange, "archive": ReasonNoDeclaredInputChange, "containment": ReasonNoDeclaredInputChange}},
		{"interop source", boundedEvidence("interop/m/a.go"), map[string]string{"go-interop": ReasonInputPathChanged, "archive": ReasonNoDeclaredInputChange}},
		{"interop directory itself", boundedEvidence("interop/m"), map[string]string{"go-interop": ReasonInputPathChanged}},
		{"sibling of a declared directory", boundedEvidence("interop/mx/a.go"), map[string]string{"go-interop": ReasonNoDeclaredInputChange}},
		{"workflow", boundedEvidence(".github/workflows/ci.yml"), map[string]string{"go-interop": ReasonWorkflowChanged, "archive": ReasonWorkflowChanged, "containment": ReasonNoDeclaredInputChange}},
		{"declaration", boundedEvidence(DeclarationPath), map[string]string{"go-interop": ReasonDeclarationChanged, "archive": ReasonDeclarationChanged, "containment": ReasonDeclarationChanged}},
		{"declared file", boundedEvidence("Makefile"), map[string]string{"archive": ReasonInputPathChanged, "go-interop": ReasonNoDeclaredInputChange}},
		{"go.sum", boundedEvidence("go.sum"), map[string]string{"archive": ReasonGoModuleChanged, "containment": ReasonGoModuleChanged, "go-interop": ReasonNoDeclaredInputChange}},
		{"package of the closure", boundedEvidence("cmd/x/main.go"), map[string]string{"archive": ReasonGoPackageDirChanged}},
		{"package only another platform builds", boundedEvidence("internal/winonly/w.go"), map[string]string{"archive": ReasonGoPackageDirChanged}},
		{"deleted file of a closure package", boundedEvidence("cmd/x/gone.go"), map[string]string{"archive": ReasonGoPackageDirChanged}},
		{"embedded subtree of a closure package", boundedEvidence("cmd/x/reporters/nested/r.js"), map[string]string{"archive": ReasonGoPackageDirChanged}},
		{"sibling of a closure package", boundedEvidence("cmd/xy/main.go"), map[string]string{"archive": ReasonNoDeclaredInputChange}},
	}
	selected := boundedEvidence("internal/intake/a.go")
	selected.Selected["go:example.com/m/internal/intake"] = "DIRECT_SOURCE_CHANGE"
	cases = append(cases, struct {
		name     string
		evidence Evidence
		want     map[string]string
	}{"selected go test", selected, map[string]string{"containment": ReasonGoTestSelected, "go-interop": ReasonNoDeclaredInputChange}})
	unknown := boundedEvidence("docs/a.md")
	unknown.PlanScope = "UNKNOWN"
	cases = append(cases, struct {
		name     string
		evidence Evidence
		want     map[string]string
	}{"unknown plan scope", unknown, map[string]string{"containment": ReasonPlanScopeUnknown, "go-interop": ReasonNoDeclaredInputChange, "archive": ReasonNoDeclaredInputChange}})
	for _, c := range cases {
		got := decisions(Evaluate(declaration, DeclarationPath, c.evidence))
		for job, reason := range c.want {
			result := got[job]
			if result.Reason != reason {
				t.Errorf("%s: job %s: %s %s %+v, want %s", c.name, job, result.Decision, result.Reason, result.Witnesses, reason)
			}
			if (result.Decision == DecisionSkip) != (reason == ReasonNoDeclaredInputChange) || len(result.Witnesses) == 0 {
				t.Errorf("%s: job %s: decision %s with %d witnesses", c.name, job, result.Decision, len(result.Witnesses))
			}
		}
	}
}

// TestEvaluateFailsClosed is AFP-V0-2342: a failed listing, a failed plan or
// a missing module runs the jobs that need them.
func TestEvaluateFailsClosed(t *testing.T) {
	declaration := parsed(t)
	evidence := boundedEvidence("docs/a.md")
	evidence.GoDirectories = func(GoPackages) (map[string]string, error) { return nil, errors.New("boom") }
	evidence.PlanError = "planner failed"
	got := decisions(Evaluate(declaration, DeclarationPath, evidence))
	if got["archive"].Reason != ReasonEvaluationError || got["containment"].Reason != ReasonEvaluationError || got["go-interop"].Decision != DecisionSkip {
		t.Fatalf("failed evidence: %+v", got)
	}
	evidence = boundedEvidence("docs/a.md")
	evidence.GoDirectories = nil
	evidence.Module = ""
	got = decisions(Evaluate(declaration, DeclarationPath, evidence))
	if got["archive"].Reason != ReasonEvaluationError || got["containment"].Reason != ReasonEvaluationError {
		t.Fatalf("absent evidence: %+v", got)
	}
}

// TestEvaluateExportedTree is AFP-V0-2343.
func TestEvaluateExportedTree(t *testing.T) {
	declaration := parsed(t)
	evidence := boundedEvidence("docs/link", "docs/big.bin", "docs/plain.md", "docs/deleted.md")
	evidence.Tree = map[string]TreeEntry{"docs/link": {Mode: "120000", Size: 4}, "docs/big.bin": {Mode: "100644", Size: 200 << 20}, "docs/plain.md": {Mode: "100644", Size: 10}}
	got := decisions(Evaluate(declaration, DeclarationPath, evidence))
	archive := got["archive"]
	if archive.Reason != ReasonOversizedTreeEntry || len(archive.Witnesses) != 2 || archive.Witnesses[1].Kind != ReasonIrregularTreeEntry {
		t.Fatalf("archive: %+v", archive)
	}
	if got["go-interop"].Decision != DecisionSkip {
		t.Fatalf("a job without exportedTree ran: %+v", got["go-interop"])
	}
	evidence = boundedEvidence("docs/plain.md")
	evidence.ListingBytes = maxListingBytes + 1
	if got := decisions(Evaluate(declaration, DeclarationPath, evidence)); got["archive"].Reason != ReasonTreeListingLarge {
		t.Fatalf("large listing: %+v", got["archive"])
	}
}

func TestEvaluateBoundsWitnesses(t *testing.T) {
	declaration := parsed(t)
	var changed []string
	for i := 0; i < MaxWitnesses+5; i++ {
		changed = append(changed, "interop/m/f"+strings.Repeat("x", i)+".go")
	}
	result := decisions(Evaluate(declaration, DeclarationPath, boundedEvidence(changed...)))["go-interop"]
	if len(result.Witnesses) != MaxWitnesses || result.Omitted != 5 {
		t.Fatalf("witnesses %d omitted %d", len(result.Witnesses), result.Omitted)
	}
}

func TestParseRefusesInvalidDeclarations(t *testing.T) {
	for name, body := range map[string]string{
		"unknown member":    `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j","paths":["a"],"extra":1}]}`,
		"wrong profile":     `{"profile":"x","jobs":[{"workflow":".github/workflows/a.yml","job":"j","paths":["a"]}]}`,
		"no jobs":           `{"profile":"corvint-ci-job-inputs/0","jobs":[]}`,
		"unsorted paths":    `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j","paths":["b","a"]}]}`,
		"workflow path":     `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":"ci.yml","job":"j","paths":["a"]}]}`,
		"duplicate":         `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j","paths":["a"]},{"workflow":".github/workflows/a.yml","job":"j","paths":["b"]}]}`,
		"no input":          `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j"}]}`,
		"escaping path":     `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j","paths":["../a"]}]}`,
		"bad platform":      `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j","goPackages":{"packages":["./a"],"platforms":["linux"]}}]}`,
		"bad package":       `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j","goPackages":{"packages":["a"],"platforms":["linux/amd64"]}}]}`,
		"wildcard go test":  `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j","goTests":["./a/..."]}]}`,
		"no platforms":      `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j","goPackages":{"packages":["./a"],"platforms":[]}}]}`,
		"trailing garbage":  `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"j","paths":["a"]}]} x`,
		"job name with gap": `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/a.yml","job":"a b","paths":["a"]}]}`,
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// TestRepositoryDeclarationParses keeps the shipped declaration valid.
func TestRepositoryDeclarationParses(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(DeclarationPath)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(body); err != nil {
		t.Fatal(err)
	}
}

func goFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func fixtureEnv() []string {
	return append(os.Environ(), "GOFLAGS=", "GOWORK=off", "GOTOOLCHAIN=local")
}

// TestGoDirectoriesUnionsPlatforms is AFP-V0-2342: a package only one
// declared platform imports is still an input, and an injected compile error
// in it runs the job, whose build on that platform then fails (fault
// injection).
func TestGoDirectoriesUnionsPlatforms(t *testing.T) {
	root := goFixture(t, map[string]string{
		"go.mod":                  "module example.com/m\n\ngo 1.27\n",
		"cmd/x/main.go":           "package main\n\nfunc main() {}\n",
		"cmd/x/win_windows.go":    "package main\n\nimport _ \"example.com/m/internal/winonly\"\n",
		"internal/winonly/w.go":   "package winonly\n",
		"internal/unrelated/u.go": "package unrelated\n",
	})
	packages := GoPackages{Packages: []string{"./cmd/x"}, Platforms: []string{"linux/amd64"}}
	linux, err := GoDirectories(context.Background(), root, "", fixtureEnv(), packages)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := linux["internal/winonly"]; ok || linux["cmd/x"] == "" {
		t.Fatalf("linux closure %v", linux)
	}
	packages.Platforms = []string{"linux/amd64", "windows/amd64"}
	both, err := GoDirectories(context.Background(), root, "", fixtureEnv(), packages)
	if err != nil {
		t.Fatal(err)
	}
	if both["internal/winonly"] != "example.com/m/internal/winonly windows/amd64" {
		t.Fatalf("union closure %v", both)
	}
	if _, ok := both["internal/unrelated"]; ok {
		t.Fatalf("an unimported package joined the closure: %v", both)
	}
	declaration, err := Parse([]byte(`{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/ci.yml","job":"archive","goPackages":{"packages":["./cmd/x"],"platforms":["linux/amd64","windows/amd64"]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/winonly/w.go"), []byte("package winonly\n\nvar broken int = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	evidence := Evidence{Changed: []string{"internal/winonly/w.go"}, Tree: map[string]TreeEntry{}, PlanScope: "BOUNDED", GoDirectories: func(p GoPackages) (map[string]string, error) {
		return GoDirectories(context.Background(), root, "", fixtureEnv(), p)
	}}
	result := Evaluate(declaration, DeclarationPath, evidence)[0]
	if result.Decision != DecisionRun || result.Reason != ReasonGoPackageDirChanged {
		t.Fatalf("injected change: %+v", result)
	}
	build := exec.Command("go", "build", "-o", os.DevNull, "example.com/m/cmd/x")
	build.Dir = root
	build.Env = append(fixtureEnv(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err == nil {
		t.Fatalf("the injected compile error did not fail the windows build:\n%s", out)
	}
	if _, err := GoDirectories(context.Background(), root, "", fixtureEnv(), GoPackages{Packages: []string{"./missing"}, Platforms: []string{"linux/amd64"}}); err == nil {
		t.Fatal("a missing package listed without error")
	}
}

// TestInteropFormattingFaultIsRun is the paths-class fault injection: an
// unformatted file in the declared directory runs the job, and the job's
// gofmt check then fails.
func TestInteropFormattingFaultIsRun(t *testing.T) {
	root := goFixture(t, map[string]string{"interop/m/go.mod": "module example.com/i\n\ngo 1.27\n", "interop/m/a.go": "package i\n"})
	if err := os.WriteFile(filepath.Join(root, "interop/m/a.go"), []byte("package i\nfunc  F( ) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := decisions(Evaluate(parsed(t), DeclarationPath, boundedEvidence("interop/m/a.go")))["go-interop"]
	if result.Decision != DecisionRun {
		t.Fatalf("injected interop change: %+v", result)
	}
	gofmt := exec.Command("gofmt", "-l", ".")
	gofmt.Dir = filepath.Join(root, "interop/m")
	out, err := gofmt.Output()
	if err != nil || strings.TrimSpace(string(out)) != "a.go" {
		t.Fatalf("gofmt -l did not report the injected file: %q %v", out, err)
	}
}
