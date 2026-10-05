package postmergehost

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// PCH-V0-015: the reference pipeline's resolve and delta scripts, run
// unmodified under bash and POSIX sh against a local fixture with a locally
// built corvint, replay a merged change (merge or not) by its full id into
// exactly the record that corvint delta emits for its first parent. A change
// without a first parent or outside the checkout fails the step and leaves no
// record.
func TestTemplateDeltaReplayConformance(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	workspace, bin := t.TempDir(), t.TempDir()
	product := filepath.Join(workspace, "product")
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "core.hooksPath=/dev/null", "-c", "init.defaultBranch=main"}, args...)...)
		cmd.Dir = product
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %q: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	build := exec.Command("go", "build", "-o", filepath.Join(bin, "corvint"), "./cmd/corvint")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build corvint: %v %s", err, out)
	}
	if err := os.MkdirAll(product, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(product, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "--quiet")
	write("calc.go", "package calc\n\nfunc Add(a, b int) int { return a + b }\n")
	git("add", ".")
	git("commit", "--quiet", "-m", "base")
	root := git("rev-parse", "HEAD")
	git("checkout", "--quiet", "-b", "feature")
	write("calc.go", "package calc\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Sub(a, b int) int { return a - b }\n")
	write("calc_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestSub(t *testing.T) {\n\tif Sub(3, 1) != 2 {\n\t\tt.Fatal(\"sub\")\n\t}\n}\n")
	git("add", ".")
	git("commit", "--quiet", "-m", "feature")
	git("checkout", "--quiet", "main")
	write("README.md", "calc\n")
	git("add", ".")
	git("commit", "--quiet", "-m", "main moves on")
	parent := git("rev-parse", "HEAD")
	git("merge", "--quiet", "--no-ff", "-m", "merge feature", "feature")
	change := git("rev-parse", "HEAD")

	direct := func(base, head, paths string) []byte {
		t.Helper()
		out, err := exec.Command(filepath.Join(bin, "corvint"), "--root", product, "delta", "--base", base, "--head", head).Output()
		if err != nil {
			t.Fatalf("direct corvint delta %s..%s: %v", base, head, err)
		}
		var record struct {
			Schema, Base, Head, Tree string
			ChangedPaths             []string
		}
		if err := json.Unmarshal(out, &record); err != nil {
			t.Fatal(err)
		}
		if record.Schema != "corvint-delta/0" || record.Base != base || record.Head != head ||
			record.Tree != git("rev-parse", head+"^{tree}") || strings.Join(record.ChangedPaths, ",") != paths {
			t.Fatalf("direct record %s", out)
		}
		return out
	}
	mergeRecord := direct(parent, change, "calc.go,calc_test.go")
	linearRecord := direct(root, parent, "README.md")

	steps, err := ParseYAML([]byte(template(t, "postmerge.yml")))
	if err != nil {
		t.Fatal(err)
	}
	if name := scalar(steps.Get("jobs").Get("delta").Get("steps").Items[4].Get("name")); name != "Compile the immutable delta of the merged change" {
		t.Fatalf("delta step 4 is %q", name)
	}
	path := "PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")
	for _, tc := range []struct {
		name, requested string
		expected        []byte
	}{
		{"merge", change, mergeRecord},
		{"non-merge change", parent, linearRecord},
		{"root commit has no first parent", root, nil},
		{"change outside the checkout", strings.Repeat("ab", 20), nil},
	} {
		temp := t.TempDir()
		output := filepath.Join(temp, "output")
		resolved := ""
		runStep(t, "postmerge.yml", "resolve", 0, func(shell string, err error) {
			data, _ := os.ReadFile(output)
			_ = os.Remove(output)
			if err != nil || !strings.HasPrefix(string(data), "change=") {
				t.Fatalf("%s %s: resolve err %v output %q", tc.name, shell, err, data)
			}
			resolved, _, _ = strings.Cut(strings.TrimPrefix(string(data), "change="), "\n")
		}, "REQUESTED_CHANGE="+tc.requested, "REQUESTED_MODE=dry-run", "GITHUB_OUTPUT="+output, "GITHUB_WORKSPACE="+workspace)
		runStep(t, "postmerge.yml", "delta", 4, func(shell string, err error) {
			got, readErr := os.ReadFile(filepath.Join(temp, "delta", "delta.json"))
			_ = os.RemoveAll(filepath.Join(temp, "delta"))
			switch {
			case tc.expected != nil && (err != nil || string(got) != string(tc.expected)):
				t.Errorf("%s %s: err %v record %q, want %q", tc.name, shell, err, got, tc.expected)
			case tc.expected == nil && (err == nil || readErr == nil):
				t.Errorf("%s %s: err %v, record present %v", tc.name, shell, err, readErr == nil)
			}
		}, "CHANGE="+resolved, "RUNNER_TEMP="+temp, "GITHUB_WORKSPACE="+workspace, path)
	}
}
