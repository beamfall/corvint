package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	git(t, root, "update-index", "--assume-unchanged", "main.go")
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN decision-numbers-check: a tracked file is skip-worktree or assume-unchanged") {
		t.Fatalf("assume-unchanged: %q", out)
	}
	git(t, root, "update-index", "--no-assume-unchanged", "main.go")
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
	if got := runs(t, root); got != 5 {
		t.Fatalf("the step ran %d times, want 5", got)
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

func TestWorktreeDigestIgnoresCachedStat(t *testing.T) {
	t.Run("GL-V0-001 exact content", func(t *testing.T) {
		for _, newerIndex := range []bool{false, true} {
			t.Run(fmt.Sprintf("newer-index-%t", newerIndex), func(t *testing.T) {
				root := fixtureRepository(t)
				git(t, root, "config", "core.checkStat", "minimal")
				git(t, root, "config", "core.trustctime", "false")
				fixed := time.Unix(1700000000, 0)
				path := filepath.Join(root, "docs/decisions/0001.md")
				setMtime(t, path, fixed)
				git(t, root, "add", "-A")
				index := filepath.Join(root, ".git/index")
				indexTime := fixed
				if newerIndex {
					indexTime = fixed.Add(time.Hour)
				}
				setMtime(t, index, indexTime)
				assertIndexUnchanged(t, index)
				step := append([]string{"run", "decision-numbers-check"}, counting...)
				if code, out := ledgerRun(t, step...); code != 0 || !strings.Contains(out, "RECORD decision-numbers-check") {
					t.Fatalf("initial record: %d %s", code, out)
				}
				write(t, root, "docs/decisions/0001.md", "two\n")
				setMtime(t, path, fixed)
				if code, out := ledgerRun(t, step...); code != 0 || !strings.Contains(out, "RUN decision-numbers-check: no recorded pass") {
					t.Fatalf("same-size restored-time edit: %d %s", code, out)
				}
				tree, _, reason := worktreeDigest(root)
				if reason != "" {
					t.Fatal(reason)
				}
				assertTreeBody(t, root, tree, "docs/decisions/0001.md", "two\n")
				if code, out := ledgerRun(t, step...); code != 0 || !strings.Contains(out, "HIT decision-numbers-check") {
					t.Fatalf("unchanged content: %d %s", code, out)
				}
			})
		}
	})
}

func setMtime(t *testing.T, path string, stamp time.Time) {
	t.Helper()
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

func assertIndexUnchanged(t *testing.T, path string) {
	t.Helper()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		afterStat, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) || !stat.ModTime().Equal(afterStat.ModTime()) {
			t.Error("repository index bytes or mtime changed")
		}
	})
}

func assertTreeBody(t *testing.T, root, tree, path, want string) {
	t.Helper()
	got, err := gitOutput(root, "show", tree+":"+path)
	if err != nil || got != want {
		t.Fatalf("tree %s path %q: got %q, want %q, error %v", tree, path, got, want, err)
	}
}

func TestWorktreeDigestPreservesMembershipAndPaths(t *testing.T) {
	root := fixtureRepository(t)
	for _, path := range []string{"ignored.txt", "staged.txt", "gone.txt", "recreated-ignored.txt", "recreated.txt", "executable", "line\nbreak\t.txt"} {
		write(t, root, path, "old\n")
	}
	if err := os.Chmod(filepath.Join(root, "executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("ignored.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "-A")
	for _, path := range []string{"intent.txt", "intent-ignored.txt"} {
		write(t, root, path, "intent\n")
		git(t, root, "add", "-N", "--", path)
	}
	git(t, root, "rm", "--cached", "recreated.txt", "recreated-ignored.txt")
	write(t, root, ".gitignore", "ignored.txt\nintent-ignored.txt\nrecreated-ignored.txt\nexcluded.txt\n")
	write(t, root, "excluded.txt", "excluded\n")
	want := map[string]string{"ignored.txt": "new\n", "staged.txt": "new\n", "intent.txt": "new intent\n", "intent-ignored.txt": "new ignored intent\n", "recreated.txt": "recreated\n", "untracked.txt": "untracked\n", "line\nbreak\t.txt": "path bytes\n"}
	for path, body := range want {
		write(t, root, path, body)
	}
	if err := os.Remove(filepath.Join(root, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("staged.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	// These optimizations must not reintroduce valid cached stats in the private index.
	git(t, root, "config", "core.ignorestat", "true")
	git(t, root, "config", "core.fsmonitor", "true")
	assertIndexUnchanged(t, filepath.Join(root, ".git/index"))
	tree, entries, reason := worktreeDigest(root)
	if reason != "" {
		t.Fatal(reason)
	}
	for path, body := range want {
		assertTreeBody(t, root, tree, path, body)
	}
	assertTreeBody(t, root, tree, "link", "staged.txt")
	for _, path := range []string{"gone.txt", "excluded.txt", "recreated-ignored.txt"} {
		if _, err := gitOutput(root, "cat-file", "-e", tree+":"+path); err == nil {
			t.Errorf("unexpected tree member %q", path)
		}
	}
	modes := map[string]string{}
	for _, entry := range entries {
		modes[entry.path] = strings.Fields(entry.line)[0]
	}
	if modes["executable"] != "100755" || modes["link"] != "120000" {
		t.Fatalf("lost modes: %#v", modes)
	}
}

func TestWorktreeDigestWithoutIndex(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	write(t, root, "new.txt", "new\n")
	tree, _, reason := worktreeDigest(root)
	if reason != "" {
		t.Fatal(reason)
	}
	assertTreeBody(t, root, tree, "new.txt", "new\n")
	if _, err := os.Stat(filepath.Join(root, ".git/index")); !os.IsNotExist(err) {
		t.Fatalf("original unborn index created: %v", err)
	}
}

func TestWorktreeDigestImportFailureRunsWithoutRecord(t *testing.T) {
	t.Run("GL-V0-005 import failure", func(t *testing.T) {
		root := fixtureRepository(t)
		actualGit, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		bin := t.TempDir()
		wrapper := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = update-index ]; then exit 47; fi\ndone\nexec \"$ACTUAL_TEST_GIT\" \"$@\"\n"
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ACTUAL_TEST_GIT", actualGit)
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		private := t.TempDir()
		t.Setenv("TMPDIR", private)
		assertIndexUnchanged(t, filepath.Join(root, ".git/index"))
		step := append([]string{"run", "decision-numbers-check"}, counting...)
		code, out := ledgerRun(t, step...)
		if code != 0 || !strings.Contains(out, "RUN decision-numbers-check: private index import failed") || strings.Contains(out, "RECORD") || records(t) != 0 || runs(t, root) != 1 {
			t.Fatalf("import failure did not fail closed: %d %s", code, out)
		}
		left, err := filepath.Glob(filepath.Join(private, "corvint-gate-ledger-index.*"))
		if err != nil || len(left) != 0 {
			t.Fatalf("private index/lock residue: %v %v", left, err)
		}
	})
}

// TestGoTestKeysResolvedPackagesPerPackage replays GL-V0-009: a resolved
// package runs under a key over its proven bound and records how the bound
// was proven, the key is hit from a linked worktree, an edit inside the bound
// reruns the package and its dependents, and an edit outside it does not (the
// root package encloses every path, so only the nested packages are asserted).
func TestGoTestKeysResolvedPackagesPerPackage(t *testing.T) {
	files := map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.27\n",
		"core/core.go":          "package core\n",
		"core/core_test.go":     "package core\n\nimport \"testing\"\n\nfunc TestCore(t *testing.T) {}\n",
		"dep/dep.go":            "package dep\n\nimport _ \"example.com/fixture/core\"\n",
		"reader/reader_test.go": "package reader\n\nvar guide = \"docs/guide.md\"\n",
		"docs/guide.md":         "guide\n",
	}
	for _, name := range []string{"main.go", "readers.go"} {
		data, err := os.ReadFile(filepath.Join("..", "gate-affected-select", name))
		if err != nil {
			t.Fatal(err)
		}
		files["tools/gate-affected-select/"+name] = string(data)
	}
	root := fixtureRepository(t)
	for file, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(file))), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, root, file, body)
	}
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "packages")
	log := filepath.Join(t.TempDir(), "ARGS")
	args := []string{"go-test", "--", "sh", "-c", `echo "$@" >> "$0"`, log}
	packageLines := func(out string) string {
		var lines []string
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "go-test-package") {
				lines = append(lines, line[:strings.LastIndex(line, " ")])
			}
		}
		return strings.Join(lines, "\n")
	}
	code, out := ledgerRun(t, args...)
	if code != 0 || !strings.Contains(out, "RECORD go-test-package example.com/fixture/core ") || !strings.Contains(out, "RECORD go-test-package example.com/fixture/reader ") {
		t.Fatalf("first run: code %d, %q", code, out)
	}
	bound, _ := filepath.Glob(filepath.Join(os.Getenv("CORVINT_GATE_LEDGER_DIR"), "*.json"))
	for _, name := range bound {
		data, _ := os.ReadFile(name)
		if strings.Contains(string(data), `"step":"go-test-package"`) && !strings.Contains(string(data), `"bound":"go list -deps -test `) {
			t.Errorf("record without a bound proof: %s", data)
		}
	}
	linked := filepath.Join(t.TempDir(), "linked")
	git(t, root, "worktree", "add", "-q", "--detach", linked, "HEAD")
	t.Chdir(linked)
	if _, out := ledgerRun(t, args...); !strings.Contains(out, "HIT go-test-package example.com/fixture/core ") || strings.Contains(out, "RUN go-test-package") {
		t.Fatalf("linked worktree: %q", out)
	}
	t.Chdir(root)
	write(t, root, "core/core.go", "package core\n\nvar changed = true\n")
	_, out = ledgerRun(t, args...)
	if lines := packageLines(out); !strings.Contains(lines, "RUN go-test-package example.com/fixture/core:") || !strings.Contains(lines, "RUN go-test-package example.com/fixture/dep:") || !strings.Contains(lines, "HIT go-test-package example.com/fixture/reader") {
		t.Fatalf("edit inside core's bound: %q", out)
	}
	data, _ := os.ReadFile(log)
	if last := data[strings.LastIndex(strings.TrimSpace(string(data)), "\n")+1:]; !strings.Contains(string(last), "example.com/fixture/core example.com/fixture/dep") || strings.Contains(string(last), "reader") {
		t.Fatalf("go test received %q", last)
	}
	write(t, root, "docs/guide.md", "guide, edited\n")
	if _, out := ledgerRun(t, args...); !strings.Contains(out, "RUN go-test-package example.com/fixture/reader:") || !strings.Contains(out, "HIT go-test-package example.com/fixture/core ") {
		t.Fatalf("edit inside reader's bound: %q", out)
	}
	write(t, root, "docs/decisions/0001.md", "one, edited\n")
	if _, out := ledgerRun(t, args...); strings.Contains(out, "RUN go-test-package example.com/fixture/") {
		t.Fatalf("edit outside the bounds of core, dep and reader: %q", out)
	}
}
