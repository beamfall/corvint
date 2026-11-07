// SPDX-License-Identifier: Apache-2.0

package jobinputs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const closureDeclaration = `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/ci.yml","job":"archive","goPackages":{"packages":["./cmd/x"],"platforms":["` + runtime.GOOS + `/` + runtime.GOARCH + `"]}}]}`

func evaluateClosure(t *testing.T, root string, changed ...string) JobResult {
	t.Helper()
	declaration, err := Parse([]byte(closureDeclaration))
	if err != nil {
		t.Fatal(err)
	}
	evidence := Evidence{Changed: changed, Tree: map[string]TreeEntry{}, PlanScope: "BOUNDED", GoDirectories: func(p GoPackages) (map[string]string, error) {
		return GoDirectories(context.Background(), root, "", fixtureEnv(), p)
	}}
	return Evaluate(declaration, DeclarationPath, evidence)[0]
}

// buildClosure builds the fixture packages at root, as goTest does in
// internal/testreuse: the targets are the fixture's, not the repository's.
func buildClosure(t *testing.T, root string, packages ...string) error {
	t.Helper()
	build := exec.Command("go", append([]string{"build", "-a", "-buildvcs=false", "-o", os.DevNull}, packages...)...)
	build.Dir = root
	build.Env = fixtureEnv()
	out, err := build.CombinedOutput()
	if err != nil {
		t.Logf("build failed as expected:\n%s", out)
	}
	return err
}

// TestJobDependencyModuleLanguageFault is AFP-V0-2342: the effective go.mod
// and go.sum of a closure module (an in-repository replacement) are job
// inputs, so moving its language version from Go 1.22 to 1.21, which breaks
// integer range, runs the job.
func TestJobDependencyModuleLanguageFault(t *testing.T) {
	root := goFixture(t, map[string]string{
		"go.mod":                       "module example.com/m\n\ngo 1.27\nrequire example.com/dependency v0.0.0\nreplace example.com/dependency => ./dependency\n",
		"dependency/go.mod":            "module example.com/dependency\n\ngo 1.22\n",
		"dependency/subpkg/value.go":   "package subpkg\n\nfunc Value() int { n := 0; for range 1 { n++ }; return n }\n",
		"cmd/x/main.go":                "package main\n\nimport \"example.com/dependency/subpkg\"\n\nfunc main() { _ = subpkg.Value() }\n",
		"internal/unrelated/unrelated": "x\n",
	})
	if result := evaluateClosure(t, root, "internal/unrelated/unrelated"); result.Decision != DecisionSkip {
		t.Fatalf("unrelated change: %+v", result)
	}
	if err := buildClosure(t, root, "./cmd/x"); err != nil {
		t.Fatalf("baseline build: %v", err)
	}
	for _, changed := range []string{"dependency/go.mod", "dependency/go.sum"} {
		result := evaluateClosure(t, root, changed)
		if result.Decision != DecisionRun || result.Reason != ReasonGoModuleChanged || result.Witnesses[0].Path != changed {
			t.Fatalf("closure module input %s: %+v", changed, result)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "dependency/go.mod"), []byte("module example.com/dependency\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result := evaluateClosure(t, root, "dependency/go.mod"); result.Decision != DecisionRun {
		t.Fatalf("language fault: %+v", result)
	}
	if err := buildClosure(t, root, "./cmd/x"); err == nil {
		t.Fatal("the go 1.21 manifest did not fail the build")
	}
}

// TestJobEscapingIncludeFault is AFP-V0-2342: a closure package with
// assembly or cgo can include a header outside every listed directory, so
// the job runs on any change rather than skipping.
func TestJobEscapingIncludeFault(t *testing.T) {
	files := map[string]string{
		"go.mod":          "module example.com/m\n\ngo 1.27\n",
		"shared/value.h":  "#define VALUE 1\n",
		"cmd/x/main.go":   "package main\n\nimport \"example.com/m/value\"\n\nfunc main() { _ = value.B() }\n",
		"value/value.go":  "package value\n\nfunc B() int\n",
		"unrelated/notes": "x\n",
	}
	sources := map[string]string{
		"amd64": "MOVQ $VALUE, ret+0(FP)\n",
		"arm64": "MOVD $VALUE, R0\nMOVD R0, ret+0(FP)\n",
	}
	body, assembly := sources[runtime.GOARCH]
	if assembly {
		files["value/value_"+runtime.GOARCH+".s"] = "#include \"../shared/value.h\"\nTEXT ·B(SB),$0-8\n" + body + "RET\n"
	} else {
		files["value/value.go"] = "package value\n\n// #include \"../shared/value.h\"\nimport \"C\"\n\nfunc B() int { return C.VALUE }\n"
	}
	root := goFixture(t, files)
	for _, changed := range []string{"shared/value.h", "unrelated/notes"} {
		result := evaluateClosure(t, root, changed)
		if result.Decision != DecisionRun || result.Reason != ReasonEvaluationError {
			t.Fatalf("escaping include closure, change %s: %+v", changed, result)
		}
		if detail := result.Witnesses[0].Detail; !strings.Contains(detail, "assembly-input") && !strings.Contains(detail, "cgo-input") {
			t.Fatalf("refusal names no include class: %q", detail)
		}
	}
	if !assembly {
		return
	}
	if err := buildClosure(t, root, "./cmd/x"); err != nil {
		t.Fatalf("baseline build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "shared/value.h"), []byte("#define VALUE (\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := buildClosure(t, root, "./cmd/x"); err == nil {
		t.Fatal("the invalid escaping header did not fail a fresh build")
	}
}

// TestCheckoutAttributesRunEveryJob covers AFP-V0-2342: a changed
// .gitattributes, at the root or in any directory, runs every job, because
// a checkout transformation changes the bytes a job reads from its declared
// paths without changing their blobs. A documentation-only change is the
// negative control. The fault injection commits only a root attribute that
// checks interop sources out with CRLF; the job runs and its gofmt check then
// reports the file.
func TestCheckoutAttributesRunEveryJob(t *testing.T) {
	declaration := parsed(t)
	if result := decisions(Evaluate(declaration, DeclarationPath, boundedEvidence("docs/readme.md")))["go-interop"]; result.Decision != DecisionSkip {
		t.Fatalf("documentation only: %+v", result)
	}
	for _, path := range []string{".gitattributes", "interop/.gitattributes", "docs/.gitattributes"} {
		for job, result := range decisions(Evaluate(declaration, DeclarationPath, boundedEvidence(path))) {
			if result.Decision != DecisionRun || result.Reason != "CHECKOUT_ATTRIBUTES_CHANGED" {
				t.Errorf("%s: job %s: %+v", path, job, result)
			}
		}
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := goFixture(t, map[string]string{"interop/m/go.mod": "module example.com/i\n\ngo 1.27\n", "interop/m/a.go": "package i\n"})
	gitRun := func(arguments ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, arguments...)...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	gitRun("init", "-q")
	gitRun("add", "-A")
	gitRun("commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("interop/m/*.go text eol=crlf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "-A")
	gitRun("commit", "-qm", "attributes")
	changed := strings.Fields(gitRun("diff", "--name-only", "HEAD~1", "HEAD"))
	if len(changed) != 1 || changed[0] != ".gitattributes" {
		t.Fatalf("changed %v, want only .gitattributes", changed)
	}
	if err := os.Remove(filepath.Join(root, "interop/m/a.go")); err != nil {
		t.Fatal(err)
	}
	gitRun("checkout", "-q", "--", "interop/m/a.go")
	result := decisions(Evaluate(declaration, DeclarationPath, boundedEvidence(changed...)))["go-interop"]
	if result.Decision != DecisionRun {
		t.Fatalf("attribute-only change: %+v", result)
	}
	gofmt := exec.Command("gofmt", "-l", ".")
	gofmt.Dir = filepath.Join(root, "interop/m")
	out, err := gofmt.Output()
	if err != nil || strings.TrimSpace(string(out)) != "a.go" {
		t.Fatalf("gofmt -l did not report the CRLF checkout: %q %v", out, err)
	}
}
