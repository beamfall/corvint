package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the compiled public CLI without the historical fixture's trace ignore rule.
func TestRecordFreshRepositoryWithoutIgnore(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "corvint")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	for _, linked := range []bool{false, true} {
		name := "main"
		if linked {
			name = "linked-worktree"
		}
		t.Run("LTPM-V0-001-"+name, func(t *testing.T) {
			root := t.TempDir()
			env := testEnvironment("GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
			run := func(program string, args ...string) []byte {
				t.Helper()
				command := exec.Command(program, args...)
				command.Dir, command.Env = root, env
				output, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("%s %v: %v\n%s", program, args, err, output)
				}
				return output
			}
			write := func(body string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			run("git", "init", "-q")
			run("git", "config", "user.name", "Corvint Test")
			run("git", "config", "user.email", "corvint@example.test")
			write("package main\n\nfunc main() {}\n")
			run("git", "add", "main.go")
			run("git", "commit", "-qm", "initial")
			if linked {
				worktree := filepath.Join(t.TempDir(), "linked")
				run("git", "worktree", "add", "-q", "-b", "linked", worktree)
				root = worktree
			}
			exclude := strings.TrimSpace(string(run("git", "rev-parse", "--git-path", "info/exclude")))
			if !filepath.IsAbs(exclude) {
				exclude = filepath.Join(root, exclude)
			}
			excludeBefore, err := os.ReadFile(exclude)
			if err != nil {
				t.Fatal(err)
			}
			if output := run(binary, "init", "--authority-id", "rr"); !bytes.Contains(output, []byte(`"mutates":false`)) {
				t.Fatalf("init did not report read-only: %s", output)
			}
			if _, err := os.Lstat(filepath.Join(root, ".context-corvint")); !os.IsNotExist(err) {
				t.Fatalf("init created private trace state: %v", err)
			}
			write("package main\n\nfunc main() { _ = 1 }\n")
			run("git", "commit", "-qam", "change")
			run(binary, "index", "--if-stale")
			if status := run("git", "status", "--porcelain"); len(status) != 0 {
				t.Fatalf("fresh repository is dirty: %s", status)
			}
			args := []string{"record", "--task", "change main", "--changed", "main.go", "--verify", "true", "--outcome", "passed"}
			invalid := exec.Command(binary, "record", "--task", "change main", "--changed", "missing.go", "--verify", "true", "--outcome", "passed")
			invalid.Dir, invalid.Env = root, env
			if output, err := invalid.CombinedOutput(); err == nil {
				t.Fatalf("invalid input accepted: %s", output)
			}
			if _, err := os.Lstat(filepath.Join(root, ".context-corvint")); !os.IsNotExist(err) {
				t.Fatalf("invalid record created private state: %v", err)
			}
			first := run(binary, args...)
			second := run(binary, args...)
			if !bytes.Equal(first, second) {
				t.Fatalf("repeat changed record output:\n%s\n%s", first, second)
			}
			files, err := filepath.Glob(filepath.Join(root, ".context-corvint", "traces", "*.jsonl"))
			if err != nil || len(files) != 1 {
				t.Fatalf("trace files=%v err=%v", files, err)
			}
			data, err := os.ReadFile(files[0])
			if err != nil || bytes.Count(data, []byte{'\n'}) != 1 {
				t.Fatalf("trace rows=%q err=%v", data, err)
			}
			var row struct {
				TraceID string `json:"trace_id"`
			}
			if err := json.Unmarshal(data, &row); err != nil || row.TraceID == "" || !bytes.Contains(first, []byte(row.TraceID)) {
				t.Fatalf("row identity=%q err=%v output=%s", row.TraceID, err, first)
			}
			excludeAfter, err := os.ReadFile(exclude)
			if err != nil || !bytes.Equal(excludeBefore, excludeAfter) {
				t.Fatalf("record changed Git exclude rules: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
				t.Fatalf("record wrote ignore rules: %v", err)
			}
		})
	}
}
