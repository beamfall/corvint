package main

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testplatform"
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
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
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

// TestRunStepKeysSaltedStepsByBaseScope replays GL-V0-051: NAME+SALT keys on
// NAME's declared scope with the salt in the key, a different salt never hits,
// an undeclared salted name keys on the whole tree, and an unsalted undeclared
// name still records nothing.
func TestRunStepKeysSaltedStepsByBaseScope(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	write(t, root, ".gitignore", "COUNT\n")
	salted := append([]string{"run", "decision-numbers-check+aaa"}, counting...)
	if code, out := ledgerRun(t, salted...); code != 0 || !strings.Contains(out, "RECORD decision-numbers-check+aaa") {
		t.Fatalf("first salted run: code %d, %q", code, out)
	}
	write(t, root, "main.go", "package main\n\nvar out = 1\n")
	if _, out := ledgerRun(t, salted...); !strings.Contains(out, "HIT decision-numbers-check+aaa") {
		t.Fatalf("a change outside the base scope missed: %q", out)
	}
	if _, out := ledgerRun(t, append([]string{"run", "decision-numbers-check+bbb"}, counting...)...); !strings.Contains(out, "RUN decision-numbers-check+bbb: no recorded pass") {
		t.Fatalf("a different salt hit: %q", out)
	}
	if _, out := ledgerRun(t, append([]string{"run", "decision-numbers-check"}, counting...)...); !strings.Contains(out, "RUN decision-numbers-check: no recorded pass") {
		t.Fatalf("the unsalted step hit a salted record: %q", out)
	}
	write(t, root, "docs/decisions/0001.md", "two\n")
	if _, out := ledgerRun(t, salted...); !strings.Contains(out, "RUN decision-numbers-check+aaa: no recorded pass") {
		t.Fatalf("a change inside the base scope hit: %q", out)
	}
	if got := runs(t, root); got != 4 {
		t.Fatalf("the salted steps ran %d times, want 4", got)
	}
	tree := append([]string{"run", "nosuchstep+ccc"}, counting...)
	if _, out := ledgerRun(t, tree...); !strings.Contains(out, "RECORD nosuchstep+ccc") {
		t.Fatalf("an undeclared salted step did not record: %q", out)
	}
	if _, out := ledgerRun(t, tree...); !strings.Contains(out, "HIT nosuchstep+ccc") {
		t.Fatalf("an undeclared salted step missed on an identical tree: %q", out)
	}
	write(t, root, "README", "any path\n")
	if _, out := ledgerRun(t, tree...); !strings.Contains(out, "RUN nosuchstep+ccc: no recorded pass") {
		t.Fatalf("an undeclared salted step hit after a tree change: %q", out)
	}
	before := records(t)
	for _, step := range []string{"nosuchstep", "nosuchstep+", "+ddd"} {
		if _, out := ledgerRun(t, append([]string{"run", step}, counting...)...); !strings.Contains(out, "RUN "+step+": no declared input scope") {
			t.Fatalf("%s: %q", step, out)
		}
	}
	if _, out := ledgerRun(t, append([]string{"run", "go-archive-gate+eee"}, counting...)...); !strings.Contains(out, "RUN go-archive-gate+eee: always runs") {
		t.Fatalf("a salted never step: %q", out)
	}
	if got := records(t); got != before {
		t.Fatalf("an unkeyed step recorded: %d records, want %d", got, before)
	}
}

// TestRunStepRefusesWhatItCannotDigest replays GL-V0-005 and GL-V0-006: a
// worktree git status cannot see fully, an ignored Go file the build compiles,
// and a ledger directory another user could read all run the step and record
// nothing; CORVINT_GATE_LEDGER=off runs it silently.
func TestRunStepRefusesWhatItCannotDigest(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
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
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
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
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
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
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
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
	testplatform.SkipPOSIXShell(t, "the git spy is a #!/bin/sh script and the ledger step is an sh -c command")
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
// packageRepository commits files and the partition's selector (its wrapper and
// internal/gateselect, rebased onto the fixture module) into a fixture
// repository and returns its root.
func packageRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	sources, err := selectorFixtureSources(filepath.Join("..", "..", "internal"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range sources {
		files[name] = body
	}
	wrapper, err := os.ReadFile(filepath.Join("..", "gate-affected-select", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	files["tools/gate-affected-select/main.go"] = strings.Replace(string(wrapper), "github.com/Beamfall/corvint/internal/gateselect", "example.com/fixture/internal/gateselect", 1)
	root := fixtureRepository(t)
	for file, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(file))), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, root, file, body)
	}
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "packages")
	return root
}

// selectorFixtureSources returns every non-test source of internal/gateselect and its one
// in-module import, the stdlib-only internal/taskprojection leaf, read under internal and rebased
// onto the fixture module, keyed by fixture path. The directory is listed, not named file by file,
// so a new gateselect source cannot leave the fixture unbuildable (V1-3476); a new in-module import
// fails here by name.
func selectorFixtureSources(internal string) (map[string]string, error) {
	sources, err := filepath.Glob(filepath.Join(internal, "gateselect", "*.go"))
	if err != nil || len(sources) == 0 {
		return nil, fmt.Errorf("list internal/gateselect: %v", err)
	}
	names := []string{"taskprojection/taskprojection.go"}
	for _, source := range sources {
		if !strings.HasSuffix(source, "_test.go") {
			names = append(names, "gateselect/"+filepath.Base(source))
		}
	}
	inModule := regexp.MustCompile(`"github.com/Beamfall/corvint/([^"]+)"`)
	files := map[string]string{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(internal, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		for _, imported := range inModule.FindAllStringSubmatch(string(data), -1) {
			if imported[1] != "internal/gateselect" && imported[1] != "internal/taskprojection" {
				return nil, fmt.Errorf("internal/%s imports %s, which this fixture does not copy; add it to selectorFixtureSources", name, imported[1])
			}
		}
		files["internal/"+name] = strings.ReplaceAll(string(data), "github.com/Beamfall/corvint/internal/", "example.com/fixture/internal/")
	}
	return files, nil
}

// TestSelectorFixtureCopiesANewGateselectSource pins V1-3476: a gateselect source added beside
// the others (such as gated_locators.go) enters the fixture without editing this test, while
// test files stay out and an unexpected in-module import is refused by name.
func TestSelectorFixtureCopiesANewGateselectSource(t *testing.T) {
	internal := t.TempDir()
	for name, body := range map[string]string{
		"taskprojection/taskprojection.go": "package taskprojection\n",
		"gateselect/select.go":             "package gateselect\n\nimport _ \"github.com/Beamfall/corvint/internal/taskprojection\"\n",
		"gateselect/gated_locators.go":     "package gateselect\n\nvar gatedLocatorsPath = \"x\"\n",
		"gateselect/select_test.go":        "package gateselect\n",
	} {
		path := filepath.Join(internal, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := selectorFixtureSources(internal)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["internal/gateselect/gated_locators.go"]; !ok || len(files) != 3 {
		t.Fatalf("fixture sources %v", slices.Sorted(maps.Keys(files)))
	}
	if !strings.Contains(files["internal/gateselect/select.go"], "example.com/fixture/internal/taskprojection") {
		t.Fatalf("import not rebased: %q", files["internal/gateselect/select.go"])
	}
	if err := os.WriteFile(filepath.Join(internal, "gateselect", "extra.go"), []byte("package gateselect\n\nimport _ \"github.com/Beamfall/corvint/internal/other\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := selectorFixtureSources(internal); err == nil || !strings.Contains(err.Error(), "internal/other") {
		t.Fatalf("unexpected import: err = %v", err)
	}
}

// TestGoTestRunsUnresolvedAfterResolvedFailure replays GL-V0-004's promise
// that ledger/go-test runs every package `go test ./...` would: a failing
// resolved batch still runs the unresolved packages, records their pass, and
// the step exits with the resolved batch's status.
func TestGoTestRunsUnresolvedAfterResolvedFailure(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	packageRepository(t, map[string]string{
		"go.mod":            "module example.com/fixture\n\ngo 1.27\n",
		"core/core.go":      "package core\n",
		"core/core_test.go": "package core\n\nimport \"testing\"\n\nfunc TestCore(t *testing.T) {}\n",
		"wd/wd_test.go":     "package wd\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestWd(t *testing.T) { os.Getwd() }\n",
	})
	log := filepath.Join(t.TempDir(), "ARGS")
	// The resolved batch, the one without -count=1, fails with 3.
	fail := `echo "$@" >> "$0"; case " $* " in *" -count=1 "*) exit 0;; esac; exit 3`
	code, out := ledgerRun(t, "go-test", "--", "sh", "-c", fail, log)
	if code != 3 || !strings.Contains(out, "resolved packages under per-package keys, 1 unresolved under the tree key") || !strings.Contains(out, "RECORD go-test-unresolved") || strings.Contains(out, "RECORD go-test-package") {
		t.Fatalf("resolved failure: code %d, %q", code, out)
	}
	data, _ := os.ReadFile(log)
	if lines := strings.Split(strings.TrimSpace(string(data)), "\n"); len(lines) != 2 || strings.Contains(lines[0], "-count=1") || lines[1] != "-count=1 example.com/fixture/wd" {
		t.Fatalf("go test received %q", data)
	}
}

// GL-V0-050 (V1-0634): a root-locating package the read-scope declaration
// bounds is keyed per package over its subtree, its declared entries and the
// declaration, not the tree: an unrelated edit hits its record, and an edit
// under a declared entry or to the declaration reruns it.
func TestGoTestKeysDeclaredPackagesByTheirScope(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := packageRepository(t, map[string]string{
		"go.mod":                         "module example.com/fixture\n\ngo 1.27\n",
		"walker/walker_test.go":          "package walker\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestWalk(t *testing.T) { os.Getwd() }\n",
		"docs/guide.md":                  "guide\n",
		"notes/other.md":                 "other\n",
		".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"walker":["docs/"]}}` + "\n",
	})
	log := filepath.Join(t.TempDir(), "ARGS")
	args := []string{"go-test", "--", "sh", "-c", `echo "$@" >> "$0"; for pkg in "$@"; do case "$pkg" in -*) continue;; esac; printf '{"Action":"start","Package":"%s"}\n{"Action":"pass","Package":"%s"}\n' "$pkg" "$pkg"; done`, log}
	code, out := ledgerRun(t, args...)
	if code != 0 || !strings.Contains(out, "RECORD go-test-package example.com/fixture/walker ") || strings.Contains(out, "go-test-unresolved") {
		t.Fatalf("first run: code %d, %q", code, out)
	}
	write(t, root, "notes/other.md", "other, edited\n")
	if _, out := ledgerRun(t, args...); !strings.Contains(out, "HIT go-test-package example.com/fixture/walker ") {
		t.Fatalf("edit outside the declared scope: %q", out)
	}
	write(t, root, "docs/guide.md", "guide, edited\n")
	if _, out := ledgerRun(t, args...); !strings.Contains(out, "RUN go-test-package example.com/fixture/walker:") {
		t.Fatalf("edit under a declared entry: %q", out)
	}
	write(t, root, ".corvint/test-read-scopes.json", `{"profile":"corvint-test-read-scopes/0","packages":{"walker":["docs/","notes/"]}}`+"\n")
	if _, out := ledgerRun(t, args...); !strings.Contains(out, "RUN go-test-package example.com/fixture/walker:") {
		t.Fatalf("edit to the declaration: %q", out)
	}
	write(t, root, ".corvint/test-read-scopes.json", "{}\n")
	if _, out := ledgerRun(t, args...); !strings.Contains(out, "go-test-unresolved") && !strings.Contains(out, "PARTITION unavailable") {
		t.Fatalf("an invalid declaration keyed the package by scope: %q", out)
	}
}

func TestGoTestKeysResolvedPackagesPerPackage(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	files := map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.27\n",
		"core/core.go":          "package core\n",
		"core/core_test.go":     "package core\n\nimport \"testing\"\n\nfunc TestCore(t *testing.T) {}\n",
		"dep/dep.go":            "package dep\n\nimport _ \"example.com/fixture/core\"\n",
		"reader/reader_test.go": "package reader\n\nvar guide = \"docs/guide.md\"\n",
		"docs/guide.md":         "guide\n",
	}
	root := packageRepository(t, files)
	log := filepath.Join(t.TempDir(), "ARGS")
	args := []string{"go-test", "--", "sh", "-c", `echo "$@" >> "$0"; for pkg in "$@"; do case "$pkg" in -*) continue;; esac; printf '{"Action":"start","Package":"%s"}\n{"Action":"pass","Package":"%s"}\n' "$pkg" "$pkg"; done`, log}
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

// GL-V0-002/009: actual Go JSON, package keys and unresolved whole-tree
// misses compose correctly on a same-tree retry after a failed batch.
func TestGoTestPartialPassRerun(t *testing.T) {
	for name, bad := range map[string]string{
		"test failure":  "package bad\nimport \"testing\"\nfunc TestBad(t *testing.T) { t.Fatal(\"expected failure\") }\n",
		"build failure": "package bad\nimport \"testing\"\nfunc TestBad(t *testing.T) { missingSymbol() }\n",
	} {
		t.Run(name, func(t *testing.T) { testGoTestPartialPassRerun(t, bad) })
	}
}

func testGoTestPartialPassRerun(t *testing.T, bad string) {
	packageRepository(t, map[string]string{
		"tools/gate-affected-select/main_test.go": "package main\nimport \"testing\"\nfunc TestSelector(t *testing.T) {}\n",
		"internal/gateselect/select_test.go":      "package gateselect\nimport \"testing\"\nfunc TestSelect(t *testing.T) {}\n",
		"internal/taskprojection/leaf_test.go":    "package taskprojection\nimport \"testing\"\nfunc TestLeaf(t *testing.T) {}\n",
		"go.mod":                                  "module example.com/fixture\n\ngo 1.27\n",
		"main_test.go":                            "package main\nimport \"testing\"\nfunc TestRoot(t *testing.T) {}\n",
		"good/good_test.go":                       "package good\nimport \"testing\"\nfunc TestGood(t *testing.T) {}\n",
		"bad/bad_test.go":                         bad,
		"wd/wd_test.go":                           "package wd\nimport (\"testing\"; \"os\")\nfunc TestWd(t *testing.T) { os.Getwd(); t.Fatal(\"unresolved failure\") }\n",
	})
	args := []string{"go-test", "--", "go", "test", "-timeout", "30s"}
	code, out := ledgerRun(t, args...)
	if code != 1 || !strings.Contains(out, "RECORD go-test-package example.com/fixture/good ") || strings.Contains(out, "RECORD go-test-package example.com/fixture/bad ") || strings.Contains(out, "RECORD go-test-unresolved") {
		t.Fatalf("first failed batch: code %d, %s", code, out)
	}
	keyed, reason := open(&bytes.Buffer{}, &bytes.Buffer{}).packageKeys([]string{"example.com/fixture/bad"})
	if reason != "" || len(keyed) != 1 || keyed[0].reason != "" {
		t.Fatalf("bad package key unavailable: %q, %+v", reason, keyed)
	}
	if _, hit := open(&bytes.Buffer{}, &bytes.Buffer{}).lookup(keyed[0].key); hit {
		t.Fatal("failed package recorded")
	}
	code, out = ledgerRun(t, args...)
	if code != 1 || !strings.Contains(out, "HIT go-test-package example.com/fixture/good ") || !strings.Contains(out, "RUN go-test-package example.com/fixture/bad:") || !strings.Contains(out, "RUN go-test-unresolved:") {
		t.Fatalf("same-tree rerun: code %d, %s", code, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "RUN go-test-package") && !strings.Contains(line, "example.com/fixture/bad:") {
			t.Fatalf("passed package repeated: %s", line)
		}
	}
}

// GL-V0-002: a package pass alone is insufficient if any requested outcome
// is missing, contradictory, malformed or truncated, even on exit zero.
func TestPackageOutcomesRefusesIncomplete(t *testing.T) {
	start := `{"Action":"start","Package":"good"}` + "\n"
	pass := `{"Action":"pass","Package":"good"}` + "\n"
	for name, stream := range map[string]string{
		"empty":                    "",
		"unfinished test":          start + `{"Action":"run","Package":"good","Test":"TestOpen"}` + "\n" + pass,
		"duplicate action":         start + `{"Action":"fail","Action":"pass","Package":"good"}` + "\n",
		"unicode duplicate test":   start + `{"Action":"run","Package":"good","Test":"TestOpen","Teſt":"BenchmarkOpen"}` + "\n" + pass,
		"build failure after pass": start + pass + `{"Action":"build-fail","ImportPath":"good"}` + "\n",
		"missing package":          start + pass,
		"truncated":                start + strings.TrimSpace(pass),
		"malformed":                start + pass + "garbled\n",
		"missing start":            pass,
		"test failure then pass":   start + `{"Action":"fail","Package":"good","Test":"TestBad"}` + "\n" + pass,
		"duplicate terminal":       start + pass + pass,
		"build failure then pass":  start + `{"Action":"build-fail","ImportPath":"good"}` + "\n" + pass,
		"oversize":                 start + strings.Repeat("x", (1<<20)+1),
	} {
		t.Run(name, func(t *testing.T) {
			pkgs := []packageKey{{pkg: "good"}}
			if name == "missing package" {
				pkgs = append(pkgs, packageKey{pkg: "bad"})
			}
			var out bytes.Buffer
			o := newPackageOutcomes(pkgs, &out)
			// Exercise arbitrary pipe-write boundaries, not only whole lines.
			for len(stream) > 0 {
				n := min(17, len(stream))
				_, _ = o.Write([]byte(stream[:n]))
				stream = stream[n:]
			}
			if got := o.passes(); got != nil {
				t.Fatalf("invalid stream admitted: %v", got)
			}
		})
	}
}

func TestPackageOutcomesPassAndFailure(t *testing.T) {
	var out bytes.Buffer
	o := newPackageOutcomes([]packageKey{{pkg: "good"}, {pkg: "bad"}}, &out)
	stream := `{"Action":"start","Package":"good"}
{"Action":"start","Package":"bad"}
{"Action":"output","Package":"good","Output":"ok output\n"}
{"Action":"pass","Package":"good"}
{"Action":"build-output","ImportPath":"bad","Output":"build diagnostic\n"}
{"Action":"build-fail","ImportPath":"bad"}
{"Action":"fail","Package":"bad"}
`
	_, _ = o.Write([]byte(stream))
	passes := o.passes()
	if !passes["good"] || passes["bad"] || out.String() != "ok output\nbuild diagnostic\n" {
		t.Fatalf("passes %v, output %q", passes, out.String())
	}
}

// GL-V0-002: every package terminal must close its tests, even when another
// package has passed and would otherwise be eligible for recording.
func TestPackageOutcomesRefusesUnfinishedTestsAtTerminal(t *testing.T) {
	for _, terminal := range []string{"pass", "fail", "skip"} {
		t.Run(terminal, func(t *testing.T) {
			var out bytes.Buffer
			o := newPackageOutcomes([]packageKey{{pkg: "good"}, {pkg: "bad"}}, &out)
			stream := `{"Action":"start","Package":"good"}
{"Action":"pass","Package":"good"}
{"Action":"start","Package":"bad"}
{"Action":"run","Package":"bad","Test":"TestOpen"}
{"Action":"run","Package":"bad","Test":"BenchmarkOpen"}
{"Action":"bench","Package":"bad","Test":"TestOpen"}
` + fmt.Sprintf("{\"Action\":%q,\"Package\":\"bad\"}\n", terminal)
			_, _ = o.Write([]byte(stream))
			if got := o.passes(); got != nil {
				t.Fatalf("unfinished test at package %s admitted records: %v", terminal, got)
			}
		})
	}
}

// GL-V0-002: framed benchmark runs, including subbenchmarks, need not emit
// individual terminals before the package outcome (Go's framebig.json).
func TestPackageOutcomesAcceptsBenchmarkRuns(t *testing.T) {
	for name, events := range map[string]string{
		"framed": `{"Action":"run","Package":"good","Test":"BenchmarkIndex"}
{"Action":"output","Package":"good","Test":"BenchmarkIndex","Output":"=== RUN   BenchmarkIndex\n","OutputType":"frame"}
{"Action":"output","Package":"good","Test":"BenchmarkIndex","Output":"BenchmarkIndex-16\t1\t4.697 ns/op\n"}
{"Action":"run","Package":"good","Test":"BenchmarkIndexAnyASCII"}
{"Action":"run","Package":"good","Test":"BenchmarkIndexAnyASCII/1:1"}
{"Action":"output","Package":"good","Test":"BenchmarkIndexAnyASCII/1:1","Output":"BenchmarkIndexAnyASCII/1:1-16\t1\t5.592 ns/op\n"}
`,
		"bench terminal": `{"Action":"run","Package":"good","Test":"BenchmarkGood"}
{"Action":"bench","Package":"good","Test":"BenchmarkGood"}
{"Action":"run","Package":"good","Test":"BenchmarkGood"}
{"Action":"bench","Package":"good","Test":"BenchmarkGood"}
`,
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			o := newPackageOutcomes([]packageKey{{pkg: "good"}}, &out)
			stream := `{"Action":"start","Package":"good"}
{"Action":"run","Package":"good","Test":"TestGood"}
{"Action":"pass","Package":"good","Test":"TestGood"}
` + events + `{"Action":"pass","Package":"good"}` + "\n"
			for len(stream) > 0 {
				n := min(17, len(stream))
				_, _ = o.Write([]byte(stream[:n]))
				stream = stream[n:]
			}
			if got := o.passes(); len(got) != 1 || !got["good"] {
				t.Fatalf("benchmark stream refused: %v, output %q", got, out.String())
			}
		})
	}
}

// GL-V0-002: benchmark failure remains a failure, even though a package
// terminal may close benchmark runs that have no individual terminal.
func TestPackageOutcomesBenchmarkFailure(t *testing.T) {
	for _, names := range [][2]string{
		{"BenchmarkBad", "BenchmarkBad"},
		{"BenchmarkBad", "BenchmarkBad-3"},
		{"BenchmarkBad/sub", "BenchmarkBad/sub-3"},
		{"BenchmarkBad/sub-3", "BenchmarkBad/sub-3"},
	} {
		t.Run(names[0]+"/"+names[1], func(t *testing.T) {
			var out bytes.Buffer
			o := newPackageOutcomes([]packageKey{{pkg: "good"}, {pkg: "bad"}}, &out)
			_, _ = o.Write([]byte(fmt.Sprintf(`{"Action":"start","Package":"good"}
{"Action":"pass","Package":"good"}
{"Action":"start","Package":"bad"}
{"Action":"run","Package":"bad","Test":%q}
{"Action":"fail","Package":"bad","Test":%q}
{"Action":"fail","Package":"bad"}
`, names[0], names[1])))
			if got := o.passes(); len(got) != 2 || !got["good"] || got["bad"] {
				t.Fatalf("passes %v", got)
			}
		})
	}
}

// GL-V0-002: repeated benchmark terminals under -count=2 need no new run,
// preserve unrelated package passes, and never undo an earlier failure.
func TestPackageOutcomesRepeatedBenchmarkTerminals(t *testing.T) {
	for _, names := range [][2]string{
		{"BenchmarkBad", "BenchmarkBad"},
		{"BenchmarkBad", "BenchmarkBad-3"},
		{"BenchmarkBad/sub", "BenchmarkBad/sub-3"},
		{"BenchmarkBad/sub-3", "BenchmarkBad/sub-3"},
	} {
		for _, repeat := range []string{"fail", "pass", "skip"} {
			for _, terminal := range []string{"fail", "pass"} {
				t.Run(names[0]+"/"+names[1]+"/"+repeat+"/"+terminal, func(t *testing.T) {
					o := newPackageOutcomes([]packageKey{{pkg: "good"}, {pkg: "bad"}}, io.Discard)
					_, _ = fmt.Fprintf(o, `{"Action":"start","Package":"good"}
{"Action":"pass","Package":"good"}
{"Action":"start","Package":"bad"}
{"Action":"run","Package":"bad","Test":%q}
{"Action":"fail","Package":"bad","Test":%q}
{"Action":%q,"Package":"bad","Test":%q}
{"Action":%q,"Package":"bad"}
`, names[0], names[1], repeat, names[1], terminal)
					got := o.passes()
					if terminal == "pass" {
						if got != nil {
							t.Fatalf("failed benchmark admitted package pass: %v", got)
						}
					} else if len(got) != 2 || !got["good"] || got["bad"] {
						t.Fatalf("repeated %s lost unrelated pass or admitted failure: %v", repeat, got)
					}
				})
			}
		}
	}
}

// GL-V0-002: CPU suffix matching is limited to benchmarks with a preceding run.
func TestPackageOutcomesRefusesUnmatchedBenchmarkTerminals(t *testing.T) {
	for _, names := range [][2]string{
		{"TestBad", "TestBad-3"},
		{"BenchmarkBad", "BenchmarkBad-cpu"},
		{"BenchmarkBad", "BenchmarkOther-3"},
		{"BenchmarkBad/sub", "BenchmarkBad-3"},
	} {
		t.Run(names[1], func(t *testing.T) {
			o := newPackageOutcomes([]packageKey{{pkg: "bad"}}, io.Discard)
			_, _ = fmt.Fprintf(o, "{\"Action\":\"start\",\"Package\":\"bad\"}\n{\"Action\":\"run\",\"Package\":\"bad\",\"Test\":%q}\n{\"Action\":\"fail\",\"Package\":\"bad\",\"Test\":%q}\n{\"Action\":\"fail\",\"Package\":\"bad\"}\n", names[0], names[1])
			if got := o.passes(); got != nil {
				t.Fatalf("unmatched terminal admitted records: %v", got)
			}
		})
	}
}

// GL-V0-002: FailedBuild contradicts a package pass, including when a dependency
// failed or the field appeared on an earlier event.
func TestPackageOutcomesRefusesFailedBuildPass(t *testing.T) {
	for _, action := range []string{"start", "output", "pass"} {
		t.Run(action, func(t *testing.T) {
			o := newPackageOutcomes([]packageKey{{pkg: "good"}, {pkg: "bad"}}, io.Discard)
			stream := `{"Action":"start","Package":"good"}
{"Action":"pass","Package":"good"}
`
			if action != "start" {
				stream += "{\"Action\":\"start\",\"Package\":\"bad\"}\n"
			}
			stream += fmt.Sprintf("{\"Action\":%q,\"Package\":\"bad\",\"FailedBuild\":\"dependency\"}\n", action)
			if action != "pass" {
				stream += "{\"Action\":\"pass\",\"Package\":\"bad\"}\n"
			}
			_, _ = o.Write([]byte(stream))
			if got := o.passes(); got != nil {
				t.Fatalf("FailedBuild followed by pass admitted records: %v", got)
			}
		})
	}
}

// GL-V0-002: every defined event field has its JSON type checked, even when
// the field is not needed for package lifecycle tracking. One bad event refuses
// the whole batch, including packages that already passed.
func TestPackageOutcomesRefusesMalformedFieldTypes(t *testing.T) {
	fields := []string{"Time", "Action", "Package", "ImportPath", "Test", "Output", "OutputType", "FailedBuild", "Key", "Value", "Path", "Elapsed", "Teſt", "Elapſed", "Key"}
	for _, field := range fields {
		values := []string{`null`, `true`, `{}`, `[]`}
		if field == "Elapsed" || field == "Elapſed" {
			values = append(values, `"invalid"`)
		} else {
			values = append(values, `42`)
			if field == "Time" {
				values = append(values, `"invalid"`, `"2026-10-10T1:00:00Z"`, `"2026-10-10T10:00:00,1Z"`, `"2026-10-10T10:00:00+24:00"`, `"2026-10-10T10:00:00-04:60"`)
			}
		}
		for _, value := range values {
			t.Run(field+"/"+value, func(t *testing.T) {
				var out bytes.Buffer
				o := newPackageOutcomes([]packageKey{{pkg: "good"}, {pkg: "bad"}}, &out)
				// A build event exercises the same decoder before its early return.
				event := fmt.Sprintf("{\"Action\":\"build-output\",\"ImportPath\":\"bad\",%q:%s}\n", field, value)
				if field == "Action" || field == "ImportPath" {
					event = fmt.Sprintf("{%q:%s}\n", field, value)
				}
				stream := `{"Action":"start","Package":"good"}
{"Action":"pass","Package":"good"}
{"Action":"start","Package":"bad"}
` + event + `{"Action":"pass","Package":"bad"}` + "\n"
				_, _ = o.Write([]byte(stream))
				if got := o.passes(); got != nil {
					t.Fatalf("malformed %s admitted records: %v", field, got)
				}
			})
		}
	}
}

func TestPackageOutcomesAcceptsTypedFields(t *testing.T) {
	var out bytes.Buffer
	o := newPackageOutcomes([]packageKey{{pkg: "good"}, {pkg: "bad"}}, &out)
	_, _ = o.Write([]byte(`{"Time":"2026-10-10T10:00:00.123456789Z","Action":"start","Package":"good"}
{"Action":"output","Package":"good","Output":"ok\n","OutputType":"frame","Key":"owner","Value":"fixture","Path":"package.log"}
{"Action":"pass","Package":"good","Elapsed":1.5,"FailedBuild":""}
{"Action":"start","Package":"bad"}
{"Action":"build-fail","ImportPath":"bad"}
{"Time":"2026-10-10T10:00:01-04:00","Action":"fail","Package":"bad","Elapsed":0,"FailedBuild":"dependency"}
`))
	if got := o.passes(); len(got) != 2 || !got["good"] || got["bad"] || out.String() != "ok\n" {
		t.Fatalf("passes %v, output %q", got, out.String())
	}
}

// GL-V0-002: Go's metadata and benchmark events do not terminate tests or
// packages, and their output remains visible.
func TestPackageOutcomesAcceptsNonTerminalEvents(t *testing.T) {
	var out bytes.Buffer
	o := newPackageOutcomes([]packageKey{{pkg: "good"}}, &out)
	stream := `{"Action":"start","Package":"good"}
{"Action":"attr","Package":"good","Key":"owner","Value":"fixture"}
{"Action":"artifacts","Package":"good","Path":"package.log"}
{"Action":"run","Package":"good","Test":"TestGood"}
{"Action":"attr","Package":"good","Test":"TestGood","Key":"owner","Value":"fixture"}
{"Action":"artifacts","Package":"good","Test":"TestGood","Path":"test.log"}
{"Action":"output","Package":"good","Test":"TestGood","Output":"test output\n"}
{"Action":"pass","Package":"good","Test":"TestGood"}
{"Action":"output","Package":"good","Test":"BenchmarkGood","Output":"benchmark output\n"}
{"Action":"bench","Package":"good","Test":"BenchmarkGood"}
{"Action":"pass","Package":"good"}
`
	_, _ = o.Write([]byte(stream))
	passes := o.passes()
	if len(passes) != 1 || !passes["good"] || out.String() != "test output\nbenchmark output\n" {
		t.Fatalf("passes %v, output %q", passes, out.String())
	}
}

// GL-V0-002: stream refusal also reaches the record-writing boundary.
func TestRunPackagesRefusesPartialOutput(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the go test stand-in is an sh -c command")
	for _, ending := range []string{"", "garbled\n", `{"Action":"start","Package":"bad"}
{"Action":"pass","Package":"bad","Elapsed":"invalid"}
`} {
		t.Run(fmt.Sprintf("ending-%q", ending), func(t *testing.T) {
			fixtureRepository(t)
			var out bytes.Buffer
			l := open(&out, &out)
			keyed := []packageKey{{pkg: "good", key: strings.Repeat("a", 64), bound: "fixture"}, {pkg: "bad", key: strings.Repeat("b", 64), bound: "fixture"}}
			command := []string{"sh", "-c", `printf '%s\n' '{"Action":"start","Package":"good"}' '{"Action":"pass","Package":"good"}'; printf '%s' "$0"`, ending}
			if code := l.runPackages(keyed, command); code != 0 {
				t.Fatalf("status %d", code)
			}
			if records(t) != 0 {
				t.Fatalf("partial output recorded: %s", out.String())
			}
		})
	}
}

// TestRunStepKeysBatchPrecheckScopes replays GL-V0-600 and GL-V0-601: the
// selection-baseline and capability-catalog steps hit after a change to a CEM
// or a build log, the paths a post-seal precheck adds, and run after a change
// to any declared input: a Go file, an embedded file under a Go package
// directory, a module file, a root-level file, the gate tooling, and each
// step's own data (the shard costs and change set; a top-level docs page).
func TestRunStepKeysBatchPrecheckScopes(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"pkg/testdata/deep", ".corvint/changes", "docs/build-log", ".github/cishards", "benchmarks/selection-baseline", "script"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, root, ".gitignore", "COUNT\n")
	write(t, root, "pkg/a.go", "package pkg\n")
	write(t, root, "pkg/testdata/deep/golden.txt", "one\n")
	write(t, root, "go.mod", "module example.invalid/m\n")
	write(t, root, "README.md", "one\n")
	write(t, root, "docs/CAPABILITIES.md", "one\n")
	write(t, root, ".github/cishards/package-costs.json", "{}\n")
	write(t, root, "benchmarks/selection-baseline/changes.json", "{}\n")
	write(t, root, "script/tool.sh", "true\n")
	for _, step := range []string{"selection-baseline-check+s1", "capability-catalog-check+c1"} {
		run := append([]string{"run", step}, counting...)
		if code, out := ledgerRun(t, run...); code != 0 || !strings.Contains(out, "RECORD "+step) {
			t.Fatalf("%s first run: code %d, %q", step, code, out)
		}
		check := func(file, body string, hit bool) {
			t.Helper()
			write(t, root, file, body)
			_, out := ledgerRun(t, run...)
			if hit && !strings.Contains(out, "HIT "+step) {
				t.Fatalf("%s: a change to %s missed: %q", step, file, out)
			}
			if !hit && !strings.Contains(out, "RUN "+step+": no recorded pass") {
				t.Fatalf("%s: a change to %s hit: %q", step, file, out)
			}
		}
		check(".corvint/changes/"+step+".cem.json", "{}\n", true)
		check("docs/build-log/"+step+".md", "entry\n", true)
		check("pkg/a.go", "package pkg\n\n// "+step+"\n", false)
		check("pkg/testdata/deep/golden.txt", step+"\n", false)
		check("go.mod", "module example.invalid/m\n\n// "+step+"\n", false)
		check("README.md", step+"\n", false)
		check("script/tool.sh", "# "+step+"\n", false)
		if step == "selection-baseline-check+s1" {
			check("docs/CAPABILITIES.md", step+"\n", true)
			check(".github/cishards/package-costs.json", "{\"a\": 1}\n", false)
			check("benchmarks/selection-baseline/changes.json", "{\"a\": 1}\n", false)
		} else {
			check(".github/cishards/package-costs.json", "{\"b\": 2}\n", true)
			check("docs/CAPABILITIES.md", step+"\n", false)
		}
	}
}

// TestInScopeBatchPrecheckForms pins the GL-V0-600 scope forms.
func TestInScopeBatchPrecheckForms(t *testing.T) {
	for _, c := range []struct {
		path, scope string
		want        bool
	}{
		{"README.md", rootScope, true},
		{"docs/README.md", rootScope, false},
		{"docs/CAPABILITIES.md", "docs/*.md", true},
		{"docs/build-log/x.md", "docs/*.md", false},
		{"docs/x.txt", "docs/*.md", false},
		{"cmd/corvint/main.go", goScope, false}, // goScope is matched by goInput, never by inScope
	} {
		if got := inScope(c.path, []string{c.scope}); got != c.want {
			t.Errorf("inScope(%q, %q) = %v, want %v", c.path, c.scope, got, c.want)
		}
	}
	dirs := goDirectories([]treeEntry{{path: "cmd/corvint/main.go"}, {path: ".github/cishards/x.go"}})
	for _, c := range []struct {
		path string
		want bool
	}{
		{"cmd/corvint/main.go", true},
		{"cmd/corvint/testdata/a/b.json", true},
		{"internal/new/x.go", true},
		{"interop/m/go.sum", true},
		{".github/cishards/package-costs.json", true},
		{".github/workflows/ci.yml", false},
		{".corvint/changes/x.cem.json", false},
		{"docs/build-log/x.md", false},
		{"cmd/README.md", false},
	} {
		if got := goInput(c.path, dirs); got != c.want {
			t.Errorf("goInput(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
