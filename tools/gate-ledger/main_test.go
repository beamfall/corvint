package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureRepository is one committed git worktree the test runs inside, with a
// private ledger directory, so every key derives from fixture content only.
func fixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{"docs/decisions/0001.md": "one\n", "main.go": "package main\n"}
	for file, body := range files {
		name := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "fixture")
	t.Setenv("CORVINT_GATE_LEDGER", "")
	t.Setenv("CORVINT_GATE_LEDGER_DIR", filepath.Join(t.TempDir(), "ledger"))
	t.Chdir(root)
	return root
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, root, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(file)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ledgerRun invokes the tool as `make` does and returns its exit code and log.
func ledgerRun(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code := run(args, &out, &out)
	return code, out.String()
}

// runs counts how often the counting command executed.
func runs(t *testing.T, root string) int {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(root, "COUNT"))
	return strings.Count(string(data), "\n")
}

func records(t *testing.T) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.Getenv("CORVINT_GATE_LEDGER_DIR"), "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return len(matches)
}

var counting = []string{"--", "sh", "-c", "echo ran >> COUNT"}

// TestRunStepSkipsOnlyRecordedIdenticalInputs replays GL-V0-001 to GL-V0-003: a
// step runs once per distinct in-scope content, a change outside its scope still
// hits, a failure and an undeclared step leave no record, and plan reports
// without running.
func TestRunStepSkipsOnlyRecordedIdenticalInputs(t *testing.T) {
	root := fixtureRepository(t)
	step := append([]string{"run", "decision-numbers-check"}, counting...)
	if code, out := ledgerRun(t, step...); code != 0 || !strings.Contains(out, "RUN decision-numbers-check") || !strings.Contains(out, "RECORD decision-numbers-check") {
		t.Fatalf("first run: code %d, %q", code, out)
	}
	if code, out := ledgerRun(t, step...); code != 0 || !strings.Contains(out, "HIT decision-numbers-check") {
		t.Fatalf("identical rerun: code %d, %q", code, out)
	}
	write(t, root, "main.go", "package main\n\nvar out = 1\n")
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "HIT decision-numbers-check") {
		t.Fatalf("a change outside the scope missed: %q", out)
	}
	if got := runs(t, root); got != 1 {
		t.Fatalf("the step ran %d times, want 1", got)
	}
	write(t, root, "docs/decisions/0001.md", "two\n")
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN decision-numbers-check: no recorded pass") {
		t.Fatalf("a change inside the scope hit: %q", out)
	}
	if got := runs(t, root); got != 2 {
		t.Fatalf("the step ran %d times after an in-scope change, want 2", got)
	}
	before := records(t)
	if code, _ := ledgerRun(t, "run", "go-format-check", "--", "sh", "-c", "exit 3"); code != 3 {
		t.Fatalf("a failing step exited %d, want 3", code)
	}
	if code, out := ledgerRun(t, "run", "nosuchstep", "--", "sh", "-c", "exit 4"); code != 4 || !strings.Contains(out, "RUN nosuchstep: no declared input scope") {
		t.Fatalf("an undeclared step: code %d, %q", code, out)
	}
	if got := records(t); got != before {
		t.Fatalf("a failure or an undeclared step recorded: %d records, want %d", got, before)
	}
	if _, out := ledgerRun(t, "plan", "decision-numbers-check", "go-format-check", "nosuchstep"); !strings.Contains(out, "HIT decision-numbers-check") || !strings.Contains(out, "RUN go-format-check: no recorded pass") || !strings.Contains(out, "RUN nosuchstep") {
		t.Fatalf("plan: %q", out)
	}
	if got := runs(t, root); got != 2 {
		t.Fatalf("plan ran a step: %d runs", got)
	}
}

// TestRunStepRefusesWhatItCannotDigest replays GL-V0-005 and GL-V0-006: a
// worktree git status cannot see fully, an ignored Go file the build compiles,
// and a ledger directory another user could read all run the step and record
// nothing; CORVINT_GATE_LEDGER=off runs it silently.
func TestRunStepRefusesWhatItCannotDigest(t *testing.T) {
	root := fixtureRepository(t)
	step := append([]string{"run", "decision-numbers-check"}, counting...)
	git(t, root, "update-index", "--skip-worktree", "main.go")
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN decision-numbers-check: a tracked file is skip-worktree or assume-unchanged") {
		t.Fatalf("skip-worktree: %q", out)
	}
	git(t, root, "update-index", "--no-skip-worktree", "main.go")
	if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, root, ".gitignore", "build/\n")
	write(t, root, "build/build.go", "package build\n")
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN decision-numbers-check: an ignored Go file is inside the build: build/build.go") {
		t.Fatalf("ignored Go file: %q", out)
	}
	if err := os.RemoveAll(filepath.Join(root, "build")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(os.Getenv("CORVINT_GATE_LEDGER_DIR"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN decision-numbers-check: ledger directory is group- or world-accessible") {
		t.Fatalf("open ledger directory: %q", out)
	}
	if got := records(t); got != 0 {
		t.Fatalf("a refused run recorded %d records", got)
	}
	t.Setenv("CORVINT_GATE_LEDGER", "off")
	if code, out := ledgerRun(t, step...); code != 0 || out != "" {
		t.Fatalf("off: code %d, %q", code, out)
	}
	if got := runs(t, root); got != 4 {
		t.Fatalf("the step ran %d times, want 4", got)
	}
}

// TestGoTestFallsBackToOneUncachedRun replays GL-V0-004's fallback: when the
// partition is unavailable every package runs with -count=1 under the tree key,
// and an identical tree hits. The log lives outside the worktree because the
// tree key sees every untracked file.
func TestGoTestFallsBackToOneUncachedRun(t *testing.T) {
	fixtureRepository(t)
	log := filepath.Join(t.TempDir(), "ARGS")
	args := []string{"go-test", "--", "sh", "-c", `echo "$@" >> "$0"`, log}
	code, out := ledgerRun(t, args...)
	if code != 0 || !strings.Contains(out, "PARTITION unavailable") || !strings.Contains(out, "RECORD go-test-unresolved") {
		t.Fatalf("fallback: code %d, %q", code, out)
	}
	data, _ := os.ReadFile(log)
	if strings.TrimSpace(string(data)) != "-count=1 ./..." {
		t.Fatalf("go test received %q", data)
	}
	if _, out := ledgerRun(t, args...); !strings.Contains(out, "HIT go-test-unresolved") {
		t.Fatalf("identical rerun: %q", out)
	}
}

// TestRunStepRecordsFromLinkedWorktree replays GL-V0-001's cross-worktree
// promise: a pass recorded in a `git worktree add` checkout, whose index lives
// under the main repository's `.git/worktrees/`, is hit from the main worktree.
func TestRunStepRecordsFromLinkedWorktree(t *testing.T) {
	root := fixtureRepository(t)
	linked := filepath.Join(t.TempDir(), "linked")
	git(t, root, "worktree", "add", "-q", "--detach", linked, "HEAD")
	t.Chdir(linked)
	step := append([]string{"run", "decision-numbers-check"}, counting...)
	if code, out := ledgerRun(t, step...); code != 0 || !strings.Contains(out, "RECORD decision-numbers-check") {
		t.Fatalf("linked worktree: code %d, %q", code, out)
	}
	t.Chdir(root)
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "HIT decision-numbers-check") {
		t.Fatalf("main worktree after a linked record: %q", out)
	}
}
