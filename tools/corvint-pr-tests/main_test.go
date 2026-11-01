//go:build darwin || linux

package main

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/.github/cishards"
	"github.com/Beamfall/corvint/internal/execfile"
)

func fixture(t *testing.T) (options, identity) {
	t.Helper()
	root := t.TempDir()
	out := t.TempDir()
	o := options{root: root, out: out, mode: "run", source: strings.Repeat("a", 40), runtime: filepath.Join(out, "runtime")}
	mustWrite := func(name, value string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("go.mod", "module example.org/fixture\n\ngo 1.27.3\n")
	mustWrite("a/a.go", "package a\nfunc Value() int { return 1 }\n")
	mustWrite("a/a_test.go", "package a\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value()!=1 { t.Fatal(\"relevant failure\") } }\n")
	mustWrite("b/b_test.go", "package b\nimport \"testing\"\nfunc TestOther(t *testing.T) {}\n")
	ctx := context.Background()
	mustGit := func(args ...string) string {
		t.Helper()
		s, err := git(ctx, o, append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	mustGit("init", "-q")
	mustGit("config", "user.email", "fixture@example.invalid")
	mustGit("config", "user.name", "Fixture")
	mustGit("add", ".")
	mustGit("commit", "-qm", "base")
	o.base = mustGit("rev-parse", "HEAD")
	mustWrite("a/a.go", "package a\nfunc Value() int { return 2 }\n")
	mustGit("add", ".")
	mustGit("commit", "-qm", "head")
	o.head = mustGit("rev-parse", "HEAD")
	tree := mustGit("rev-parse", "HEAD^{tree}")
	o.target = mustGit("commit-tree", tree, "-p", o.base, "-p", o.head, "-m", "merge")
	mustGit("checkout", "--detach", o.target)
	_, file, _, _ := runtime.Caller(0)
	sourceRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	for _, tool := range []struct {
		path string
		dest *string
	}{{"./cmd/corvint", &o.planner}, {"./tools/gate-affected-select", &o.selector}} {
		*tool.dest = filepath.Join(t.TempDir(), "tool")
		var b strings.Builder
		code, err := command(ctx, sourceRoot, append(os.Environ(), "GOTOOLCHAIN=local"), &b, &b, 2*time.Minute, "go", "build", "-trimpath", "-buildvcs=false", "-o", *tool.dest, tool.path)
		if err != nil || code != 0 {
			t.Fatalf("build: %d %v %s", code, err, b.String())
		}
	}
	if err := prepareCache(o); err != nil {
		t.Fatal(err)
	}
	o.seedCache = raceCacheSeed(t, ctx, o)
	id, err := toolIdentity(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	return o, id
}

// raceSeed is one race build cache shared by every fixture execution of this
// test binary, built once by raceCacheSeed and removed by TestMain.
var raceSeed struct {
	once sync.Once
	dir  string
	err  error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if raceSeed.dir != "" {
		_ = os.RemoveAll(raceSeed.dir)
	}
	os.Exit(code)
}

// raceCacheSeed returns the fixture's cache seed (V1-2740). Every execution
// empties HOME/TMP/GOCACHE before its race invocation (AFP-V0-015); without a
// seed each one then recompiles the race standard library under GOMAXPROCS=2,
// about 30 s a run. The seed is built once, in the fixture module under the
// driver's own test environment so its cache keys match, and copied into each
// emptied cache. The returned hook first fails unless all three directories are
// empty, so every seeded execution still proves the cold-runtime boundary.
func raceCacheSeed(t *testing.T, ctx context.Context, o options) func(string) error {
	t.Helper()
	raceSeed.once.Do(func() {
		dir, err := os.MkdirTemp("", "corvint-pr-tests-race-cache-")
		if err != nil {
			raceSeed.err = err
			return
		}
		raceSeed.dir = dir
		tmp := t.TempDir()
		var env []string
		for _, v := range testEnv(o) {
			name, _, _ := strings.Cut(v, "=")
			switch name {
			case "GOCACHE":
				v = "GOCACHE=" + dir
			case "TMPDIR", "TMP", "TEMP":
				v = name + "=" + tmp
			case "GOMAXPROCS":
				continue
			}
			env = append(env, v)
		}
		var b strings.Builder
		code, err := command(ctx, o.root, env, &b, &b, 10*time.Minute, "go", "test", "-race", "-count=1", "-run", "^$", "./...")
		if err != nil || code != 0 {
			raceSeed.err = fmt.Errorf("race cache seed: %d %v %s", code, err, b.String())
		}
	})
	if raceSeed.err != nil {
		t.Fatal(raceSeed.err)
	}
	return func(cache string) error {
		if err := requireColdRuntime(cache); err != nil {
			return err
		}
		return copyTree(raceSeed.dir, cache)
	}
}

// requireColdRuntime fails unless HOME/TMP/GOCACHE are empty after the reset
// that precedes the race invocation (AFP-V0-015). Used alone as the hook, it
// leaves the cache empty, so that execution compiles fully cold.
func requireColdRuntime(cache string) error {
	for _, name := range []string{"home", "tmp", "cache"} {
		entries, err := os.ReadDir(filepath.Join(filepath.Dir(cache), name))
		if err != nil || len(entries) != 0 {
			return fmt.Errorf("AFP-V0-015: %s not empty before the race invocation: %d entries, %v", name, len(entries), err)
		}
	}
	return nil
}

// copyTree copies the regular files and directories under src into dst.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("race cache seed: %s is not a regular file", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
}

func TestToolIdentityRequiresCurrentGoVersion(t *testing.T) {
	// Decision 0557: exactly the go1.27.3 pin is admitted; neighbours on
	// both sides, including the former go1.27.2 pin, are refused.
	admitted := map[string]bool{"go1.27.3": true}
	for _, version := range []string{"go1.27.1", "go1.27.2", "go1.27.3", "go1.27.4", "go1.28.2"} {
		t.Run(version, func(t *testing.T) {
			bin := t.TempDir()
			script := "#!/bin/sh\ncase \"$1:$2\" in\nenv:GOVERSION) printf '%s\\n' '" + version + "' ;;\nenv:CC) printf '%s\\n' '/usr/bin/cc' ;;\n*) exit 1 ;;\nesac\n"
			if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			o := options{root: t.TempDir(), out: t.TempDir(), source: strings.Repeat("a", 40), planner: filepath.Join(bin, "planner"), selector: filepath.Join(bin, "selector")}
			o.runtime = filepath.Join(o.out, "runtime")
			for _, path := range []string{o.planner, o.selector} {
				if err := os.WriteFile(path, []byte("identity fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := prepareCache(o); err != nil {
				t.Fatal(err)
			}
			id, err := toolIdentity(context.Background(), o)
			if id.GoVersion != version {
				t.Fatalf("observed version=%q, want %q: %v", id.GoVersion, version, err)
			}
			if admitted[version] {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || err.Error() != "unsupported Go version" {
				t.Fatalf("unsupported version admission: %v", err)
			}
		})
	}
}

// TestSelectedFailureAndFallback executes the actual trusted planner/selector,
// then the exact race argv over a merge fixture, preserving a relevant failure.
func TestSelectedFailureAndFallback(t *testing.T) {
	t.Run("AFP-V0-013 AFP-V0-014 AFP-V0-015", selectedFailureAndFallback)
}

func selectedFailureAndFallback(t *testing.T) {
	o, id := fixture(t)
	ctx := context.Background()
	s := plan(ctx, o, id, true)
	if s.Reason != "" || !equal(s.Packages, []string{"example.org/fixture/a"}) {
		t.Fatalf("selection: %+v", s)
	}
	// AFP-V0-015 is traced here: the selected execution gets no cache seed and
	// runs its race invocation fully cold, as the driver does outside tests.
	seed := o.seedCache
	o.seedCache = requireColdRuntime
	code, err := execute(ctx, o, s)
	o.seedCache = seed
	if err != nil || code != 1 {
		t.Fatalf("selected failure: %d %v", code, err)
	}
	assertRunExports(t, o.out)
	var e execution
	if err = readJSON(filepath.Join(o.out, "execution.json"), &e); err != nil {
		t.Fatal(err)
	}
	fail, err := outcomes(filepath.Join(o.out, "go.json"), s.Packages, e)
	if err != nil || !equal(fail, s.Packages) {
		t.Fatalf("failure witness: %v %v", fail, err)
	}

	original, err := os.ReadFile(o.selector)
	if err != nil {
		t.Fatal(err)
	}
	if err = execfile.WriteFile(o.selector, append(original, 0), 0700); err != nil {
		t.Fatal(err)
	}
	driftCode, driftErr := execute(ctx, o, s)
	if driftErr != nil || driftCode != 1 {
		t.Fatalf("tool drift fallback: %d %v", driftCode, driftErr)
	}
	var drift selection
	if err = readJSON(filepath.Join(o.out, "selection.json"), &drift); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(drift.Reason, "drift") || !equal(drift.Packages, []string{"./..."}) {
		t.Fatalf("tool drift was narrowed: %+v", drift)
	}
	if err = execfile.WriteFile(o.selector, original, 0700); err != nil {
		t.Fatal(err)
	}
	wrong := s
	wrong.Identity.GoBinary = "changed"
	if _, err = execute(ctx, o, wrong); err == nil {
		t.Fatal("Go binary drift executed")
	}
	if id.Git == "" {
		t.Fatal("Git executable missing from frozen identity")
	}
	wrong = s
	wrong.Identity.Git = "changed"
	if _, err = execute(ctx, o, wrong); err == nil || !strings.Contains(err.Error(), "Git") {
		t.Fatalf("Git drift executed: %v", err)
	}
	code, err = runPR(ctx, o)
	if err != nil || code != 1 {
		t.Fatalf("full fallback: %d %v", code, err)
	}
	if err = readJSON(filepath.Join(o.out, "selection.json"), &s); err != nil {
		t.Fatal(err)
	}
	if !equal(s.Packages, []string{"./..."}) || !strings.Contains(s.Reason, "qualification") {
		t.Fatalf("fallback: %+v", s)
	}
	if err = readJSON(filepath.Join(o.out, "execution.json"), &e); err != nil {
		t.Fatal(err)
	}
	if _, err = outcomes(filepath.Join(o.out, "go.json"), []string{"example.org/fixture/a", "example.org/fixture/b"}, e); err != nil {
		t.Fatal(err)
	}
	assertRunExports(t, o.out)
	o.mode = "shadow"
	rowResult := observeRow(ctx, o, id)
	if !rowResult.Valid || len(rowResult.Misses) > 0 {
		t.Fatalf("full historical observation: %+v", rowResult)
	}
	if err = validateRow(o.out, rowResult, pair{o.base, o.target}, id); err != nil {
		t.Fatal(err)
	}
	frozenWithoutGit(t, ctx, o, id, rowResult)

	var observed execution
	executionPath := filepath.Join(o.out, "execution.json")
	if err = readJSON(executionPath, &observed); err != nil {
		t.Fatal(err)
	}
	observed.Selection.Target = o.base
	if err = writeJSON(executionPath, observed); err != nil {
		t.Fatal(err)
	}
	changedRow := rowResult
	changedRow.Files = map[string]string{}
	for k, v := range rowResult.Files {
		changedRow.Files[k] = v
	}
	changedRow.Files["execution.json"], _ = digestFile(executionPath)
	if validateRow(o.out, changedRow, pair{o.base, o.target}, id) == nil {
		t.Fatal("detached execution source accepted")
	}
	observed.Selection.Target = o.target
	if err = writeJSON(executionPath, observed); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(o.out, "go.stderr"), []byte("drift"), 0600); err != nil {
		t.Fatal(err)
	}
	if validateRow(o.out, rowResult, pair{o.base, o.target}, id) == nil {
		t.Fatal("tampered row reused")
	}
	o.head = o.base
	s = plan(ctx, o, id, true)
	if !strings.Contains(s.Reason, "merge parents") {
		t.Fatalf("topology mismatch admitted: %+v", s)
	}
}

// frozenWithoutGit checks that corpora and rows frozen before Git joined the
// identity, or under another Git, are never resumed or reused under the
// current runtime (ticket ci-shadow-historical-compat).
func frozenWithoutGit(t *testing.T, ctx context.Context, o options, id identity, observed row) {
	t.Helper()
	for name, mutate := range map[string]func(*identity){
		"no Git field": func(v *identity) { v.Git = "" },
		"other Git":    func(v *identity) { v.Git += " (other)" },
	} {
		old := id
		mutate(&old)
		c := corpus{Schema: schema, Identity: old}
		for i := 0; i < 200; i++ {
			c.Pairs = append(c.Pairs, pair{fmt.Sprintf("%040x", i+1), fmt.Sprintf("%040x", i+2)})
		}
		resume := o
		resume.out, resume.row = t.TempDir(), 1
		resume.corpus = filepath.Join(resume.out, "corpus.json")
		if err := writeJSON(resume.corpus, c); err != nil {
			t.Fatal(err)
		}
		if err := shadow(ctx, resume); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
			t.Fatalf("%s: corpus resumed under the current runtime: %v", name, err)
		}
		if _, err := os.Stat(shadowCheckout(resume)); !os.IsNotExist(err) {
			t.Fatalf("%s: checkout created before the identity check: %v", name, err)
		}
		stale := observed
		stale.Identity = old
		if validateRow(o.out, stale, pair{o.base, o.target}, id) == nil {
			t.Fatalf("%s: row reused under the current identity", name)
		}
	}
}

func TestQualificationAndTerminalFailures(t *testing.T) {
	t.Run("AFP-V0-014", qualificationAndTerminalFailures)
}

func qualificationAndTerminalFailures(t *testing.T) {
	o := options{root: t.TempDir(), out: t.TempDir()}
	id := identity{Source: strings.Repeat("a", 40)}
	q := qualification{Schema: schema, Identity: id, Verdict: "PASS"}
	for i := 0; i < 200; i++ {
		files := map[string]string{}
		for _, f := range rowFiles {
			files[f] = strings.Repeat("a", 64)
		}
		q.Rows = append(q.Rows, row{Pair: pair{fmt.Sprintf("%040x", i+1), fmt.Sprintf("%040x", i+2)}, Identity: id, Valid: true, Files: files})
	}
	o.qualification = filepath.Join(o.out, "qualification.json")
	if err := writeJSON(o.qualification, q); err != nil {
		t.Fatal(err)
	}
	o.qualificationSHA, _ = digestFile(o.qualification)
	if err := admit(o, id); err != nil {
		t.Fatal(err)
	}
	other := id
	other.Arch = "other"
	if admit(o, other) == nil {
		t.Fatal("mismatched architecture admitted")
	}
	q.Rows[2].Pair = q.Rows[1].Pair
	_ = writeJSON(o.qualification, q)
	o.qualificationSHA, _ = digestFile(o.qualification)
	if admit(o, id) == nil {
		t.Fatal("duplicate corpus row admitted")
	}
	path := filepath.Join(o.out, "go.json")
	for _, data := range []string{"", `{"Action":"pass","Package":"a"}` + "\n" + `{"Action":"build-fail","Package":"b"}`, `{"Action":"pass","Package":"a"}`} {
		_ = os.WriteFile(path, []byte(data), 0600)
		if _, err := outcomes(path, []string{"a", "b"}, execution{}); err == nil {
			t.Fatal("incomplete/build failure admitted")
		}
	}
}

func TestInterruptionLeavesNoLiveDescendant(t *testing.T) {
	t.Run("AFP-V0-013", interruptionLeavesNoLiveDescendant)
}

func interruptionLeavesNoLiveDescendant(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := command(ctx, dir, os.Environ(), os.Stdout, os.Stderr, time.Minute, "sh", "-c", "sleep 60 & echo $! > \"$1\"; wait", "sh", pidPath)
		done <- err
	}()
	var pid int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := os.ReadFile(pidPath)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		if pid > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		cancel()
		<-done
		t.Fatal("no descendant PID")
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("parent survived interruption")
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		b, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		if strings.HasPrefix(strings.TrimSpace(string(b)), "Z") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("live descendant %d survived", pid)
}

func TestClosedEnvironmentAndBoundedOutput(t *testing.T) {
	t.Run("AFP-V0-013", closedEnvironmentAndBoundedOutput)
}

// TestTestEnvironmentSilencesGraftAdvice keeps the graft refusal for tests
// while they see no graft-deprecation advice in combined Git output.
func TestTestEnvironmentSilencesGraftAdvice(t *testing.T) {
	t.Run("AFP-V0-013 graft advice", func(t *testing.T) {
		o := options{root: t.TempDir(), out: t.TempDir()}
		if err := prepareCache(o); err != nil {
			t.Fatal(err)
		}
		driver, tests := closedEnv(o), testEnv(o)
		var want []string
		for _, v := range driver {
			if v == "GIT_CONFIG_COUNT=0" {
				want = append(want, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=advice.graftFileDeprecated", "GIT_CONFIG_VALUE_0=false")
			} else {
				want = append(want, v)
			}
		}
		if !strings.Contains(strings.Join(tests, "\n"), "GIT_GRAFT_FILE="+os.DevNull) || !equal(tests, want) {
			t.Fatalf("test environment is not closedEnv plus the advice setting: driver=%v tests=%v", driver, tests)
		}
		run := func(env []string, args ...string) string {
			t.Helper()
			var b strings.Builder
			code, err := command(context.Background(), o.root, env, &b, &b, time.Minute, "git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
			if err != nil || code != 0 {
				t.Fatalf("git %v: %d %v %s", args, code, err, b.String())
			}
			return b.String()
		}
		run(tests, "init", "-q")
		run(tests, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "one")
		if out := run(tests, "log", "--format=%s"); out != "one\n" {
			t.Fatalf("test Git output carried advice: %q", out)
		}
		// Recorded, not asserted: advice wording depends on the Git version.
		t.Logf("driver environment log output: %q", run(driver, "log", "--format=%s"))
	})
}

// TestExecuteGivesTestsSilencedGraftEnvironment pins the environment execute
// hands to go test: a fixture test fails if grafts are not refused or if
// combined git log output carries graft-deprecation advice.
func TestExecuteGivesTestsSilencedGraftEnvironment(t *testing.T) {
	t.Run("AFP-V0-013 graft advice", func(t *testing.T) {
		root, tools := t.TempDir(), t.TempDir()
		o := options{root: root, out: t.TempDir(), mode: "run", source: strings.Repeat("a", 40), planner: filepath.Join(tools, "planner"), selector: filepath.Join(tools, "selector")}
		o.runtime = filepath.Join(o.out, "runtime")
		for path, body := range map[string]string{
			o.planner:                             "identity fixture",
			o.selector:                            "identity fixture",
			filepath.Join(root, "go.mod"):         "module example.org/graft\n\ngo 1.27.3\n",
			filepath.Join(root, "g", "g_test.go"): "package g\nimport (\"os\"; \"os/exec\"; \"testing\")\nfunc TestGitOutput(t *testing.T) {\n\tif os.Getenv(\"GIT_GRAFT_FILE\") != os.DevNull { t.Fatal(\"grafts not refused\") }\n\tout, err := exec.Command(\"git\", \"log\", \"-1\", \"--format=%s\").CombinedOutput()\n\tif err != nil || string(out) != \"base\\n\" { t.Fatalf(\"git output %q: %v\", out, err) }\n}\n",
		} {
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := prepareCache(o); err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "base"}} {
			if _, err := git(ctx, o, append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...); err != nil {
				t.Fatal(err)
			}
		}
		id, err := toolIdentity(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		code, err := execute(ctx, o, selection{Identity: id, Packages: []string{"./..."}, Reason: "qualification unavailable"})
		if err != nil || code != 0 {
			b, _ := os.ReadFile(filepath.Join(o.out, "go.json"))
			t.Fatalf("go test environment: %d %v\n%s", code, err, b)
		}
	})
}

// TestExecuteRequiresGitIdentity refuses execution when the test Git cannot be
// identified, even without a frozen identity to compare against.
func TestExecuteRequiresGitIdentity(t *testing.T) {
	t.Run("AFP-V0-013", func(t *testing.T) {
		bin := t.TempDir()
		for name, body := range map[string]string{
			"go":  "#!/bin/sh\ncase \"$1:$2\" in\nenv:GOVERSION) printf '%s\\n' go1.27.3 ;;\nenv:CC) printf '%s\\n' /usr/bin/cc ;;\n*) exit 1 ;;\nesac\n",
			"git": "#!/bin/sh\nexit 1\n",
		} {
			if err := execfile.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
		o := options{root: t.TempDir(), out: t.TempDir(), mode: "run", source: strings.Repeat("a", 40), planner: filepath.Join(bin, "planner"), selector: filepath.Join(bin, "selector")}
		o.runtime = filepath.Join(o.out, "runtime")
		if err := prepareCache(o); err != nil {
			t.Fatal(err)
		}
		code, err := execute(context.Background(), o, selection{Packages: []string{"./..."}})
		if code != 2 || err == nil || !strings.Contains(err.Error(), "runtime identity unavailable") {
			t.Fatalf("unidentified Git executed: %d %v", code, err)
		}
		if _, err := os.Stat(filepath.Join(o.out, "go.json")); !os.IsNotExist(err) {
			t.Fatalf("go test launched without a Git identity: %v", err)
		}
	})
}

func closedEnvironmentAndBoundedOutput(t *testing.T) {
	t.Setenv("CORVINT_TEST_POISON", "must-not-reach-tests")
	o := options{root: t.TempDir(), out: t.TempDir()}
	if err := prepareCache(o); err != nil {
		t.Fatal(err)
	}
	for _, v := range closedEnv(o) {
		if strings.Contains(v, "POISON") {
			t.Fatal("ambient environment admitted")
		}
	}
	env, err := capture(context.Background(), o, "env")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(env, "POISON") || !strings.Contains(env, "HOME="+filepath.Join(runtimePath(o), "home")) {
		t.Fatalf("not closed: %s", env)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var output strings.Builder
	bounded := &cappedWriter{dst: &output, remaining: 1024, cancel: cancel}
	pidPath := filepath.Join(o.out, "child")
	code, err := command(ctx, o.root, closedEnv(o), bounded, &output, time.Minute, "sh", "-c", "sleep 60 & echo $! > \"$1\"; while :; do printf 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'; done", "sh", pidPath)
	if err == nil || code == 0 || !bounded.overflow || output.Len() > 1024 {
		t.Fatalf("unbounded output: %d %v %d", code, err, output.Len())
	}
	b, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		b, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		if strings.HasPrefix(strings.TrimSpace(string(b)), "Z") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("output-overflow descendant %d survived", pid)
}

func TestSymlinkParentsNeverMutateRepository(t *testing.T) {
	t.Run("AFP-V0-013", symlinkParentsNeverMutateRepository)
}

func symlinkParentsNeverMutateRepository(t *testing.T) {
	for _, which := range []string{"out", "runtime"} {
		t.Run(which, func(t *testing.T) {
			root := t.TempDir()
			scratch := t.TempDir()
			marker := filepath.Join(root, "marker")
			if err := os.WriteFile(marker, []byte("unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(scratch, "link")
			if err := os.Symlink(root, link); err != nil {
				t.Fatal(err)
			}
			o := options{mode: "invalid", root: root, out: filepath.Join(scratch, "out"), runtime: filepath.Join(scratch, "runtime")}
			if which == "out" {
				o.out = filepath.Join(link, "new", "out")
			} else {
				o.runtime = filepath.Join(link, "new")
			}
			code, err := dispatch(context.Background(), o)
			if code == 0 || err == nil {
				t.Fatal("symlink-parent mutation admitted")
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || string(b) != "unchanged" {
				t.Fatalf("repository changed before refusal: %v %q", entries, b)
			}
		})
	}
}

func TestRuntimeCleanupFailureCannotPass(t *testing.T) {
	t.Run("AFP-V0-013", runtimeCleanupFailureCannotPass)
}

func runtimeCleanupFailureCannotPass(t *testing.T) {
	path := t.TempDir()
	cleanupFailure := fmt.Errorf("injected permission refusal")
	code, err := func() (code int, err error) {
		defer finishCleanup(&code, &err, path, func(string) error { return cleanupFailure })
		return 0, nil
	}()
	if code == 0 || err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("cleanup failure passed: %d %v", code, err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal("injected refusal unexpectedly deleted evidence")
	}
}

func TestDockerCLIInterruption(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		dir := t.TempDir()
		pidPath := filepath.Join(dir, "pid")
		if err := execfile.WriteFile(filepath.Join(dir, "docker"), []byte("#!/bin/sh\nsleep 60 &\necho $! > \"$CORVINT_FIXTURE_PID\"\nwait\n"), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		t.Setenv("CORVINT_FIXTURE_PID", pidPath)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { _, err := dockerCapture(ctx, "fixture", "inspect", "fixture"); done <- err }()
		var pid int
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			b, _ := os.ReadFile(pidPath)
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
			if pid > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
		if err := <-done; err == nil {
			t.Fatal("interruption succeeded")
		}
		if pid == 0 {
			t.Fatal("no fixture descendant")
		}
		defer syscall.Kill(pid, syscall.SIGKILL)
		deadline = time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if syscall.Kill(pid, 0) == syscall.ESRCH {
				return
			}
			b, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
			if strings.HasPrefix(strings.TrimSpace(string(b)), "Z") {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("Docker CLI descendant survived")
	})
}

func TestContainerCleanupRefusal(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		bin := t.TempDir()
		if err := execfile.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
		trusted := t.TempDir()
		for _, name := range trustedNames {
			if err := execfile.WriteFile(filepath.Join(trusted, name), []byte(name), 0700); err != nil {
				t.Fatal(err)
			}
		}
		out := filepath.Join(t.TempDir(), "new")
		code, err := launchContainer(context.Background(), options{root: root, out: out, trusted: trusted, source: strings.Repeat("a", 40), dockerContext: "fixture", containerMode: "freeze"})
		if code == 0 || err == nil || !strings.Contains(err.Error(), "container cleanup") {
			t.Fatalf("cleanup refusal: %d %v", code, err)
		}
		b, e := os.ReadFile(filepath.Join(out, "cleanup.json"))
		if e != nil || !strings.Contains(string(b), `"removed": false`) {
			t.Fatalf("cleanup witness %s %v", b, e)
		}
	})
}

func assertRunExports(t *testing.T, dir string) {
	t.Helper()
	for _, name := range exportFiles("run") {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(err) && (name == "plan.json" || name == "selector.txt") {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		var archive bytes.Buffer
		w := tar.NewWriter(&archive)
		if err = w.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(data)
		_ = w.Close()
		var retained bytes.Buffer
		limit := int64(8 << 20)
		if name == "go.json" {
			limit = maxStdout
		}
		if err = copyRegularTar(&retained, &archive, name, limit); err != nil || !bytes.Equal(retained.Bytes(), data) {
			t.Fatalf("run export %s: %v", name, err)
		}
	}
}
func TestDockerCaptureBoundAndTimeout(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		dir := t.TempDir()
		script := []byte("#!/bin/sh\nif [ \"$3\" = overflow ]; then head -c 9437184 /dev/zero; else sleep 60; fi\n")
		if err := execfile.WriteFile(filepath.Join(dir, "docker"), script, 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		b, err := dockerCaptureFor(context.Background(), "fixture", time.Second, "overflow")
		if err == nil || len(b) > 8<<20 {
			t.Fatalf("capture exceeded: %d %v", len(b), err)
		}
		start := time.Now()
		_, err = dockerCaptureFor(context.Background(), "fixture", 50*time.Millisecond, "stall")
		if err == nil || time.Since(start) > 2*time.Second {
			t.Fatalf("control timeout: %v", err)
		}
	})
}

func TestShardedPRExecution_AFPV0022(t *testing.T) {
	t.Run("AFP-V0-022", func(t *testing.T) {
		o, _ := fixture(t)
		ctx := context.Background()
		o.shards, o.partitionDigest = 4, cishards.ProfileDigest()
		id, err := toolIdentity(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		if id.Shards != 4 || id.PartitionDigest != o.partitionDigest {
			t.Fatal("partition profile not frozen")
		}
		s := plan(ctx, o, id, true)
		if s.Reason != "" {
			t.Fatal(s.Reason)
		}
		parts, err := cishards.Partition([]string{"example.org/fixture/a", "example.org/fixture/b"}, 4)
		if err != nil {
			t.Fatal(err)
		}
		owner, empty := -1, -1
		for i, pkgs := range parts {
			if len(pkgs) == 0 {
				empty = i
			}
			for _, pkg := range pkgs {
				if pkg == "example.org/fixture/a" {
					owner = i
				}
			}
		}
		for _, index := range []int{empty, owner} {
			o.shard = index
			code, err := execute(ctx, o, s)
			want := 0
			if index == owner {
				want = 1
			}
			if err != nil || code != want {
				t.Fatalf("shard=%d code=%d err=%v", index, code, err)
			}
			var e execution
			if err = readJSON(filepath.Join(o.out, "execution.json"), &e); err != nil {
				t.Fatal(err)
			}
			if e.SkippedEmptyShard != (index == empty) || e.Selection.Shard != index || e.Selection.Shards != 4 {
				t.Fatalf("execution audit: %+v", e)
			}
			if !equal(e.Args[len(testArgs):], e.Selection.ShardPackages) {
				t.Fatal("argv differs from audited partition")
			}
		}
		code, err := runPR(ctx, o)
		if err != nil || code != 1 {
			t.Fatalf("missing qualification must fail relevant full shard: %d %v", code, err)
		}
		var fallback selection
		if err = readJSON(filepath.Join(o.out, "selection.json"), &fallback); err != nil {
			t.Fatal(err)
		}
		if !equal(fallback.ShardPackages, parts[owner]) || !equal(fallback.Packages, []string{"./..."}) {
			t.Fatalf("fallback changed partition: %+v", fallback)
		}
		o.partitionDigest = strings.Repeat("0", 64)
		if code, err = dispatch(ctx, o); code != 2 || err == nil || !strings.Contains(err.Error(), "partition profile mismatch") {
			t.Fatalf("mismatch: %d %v", code, err)
		}
		// Qualification rows keep their one complete invocation even for a four-shard PR profile.
		o.mode, o.partitionDigest = "shadow", ""
		row := observeRow(ctx, o, id)
		if !row.Valid {
			t.Fatalf("four-shard profile lost complete shadow outcome: %+v", row)
		}
		if err = validateRow(o.out, row, pair{o.base, o.target}, id); err != nil {
			t.Fatal(err)
		}
	})
}

func TestShardedSourceDrift_AFPV0022(t *testing.T) {
	t.Run("AFP-V0-022 selected and full fallback", func(t *testing.T) {
		o, _ := fixture(t)
		realGo, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		realGo, err = filepath.EvalSymlinks(realGo)
		if err != nil {
			t.Fatal(err)
		}
		bin, marker := t.TempDir(), filepath.Join(t.TempDir(), "enumerated")
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
		proxy := "#!/bin/sh\nif [ \"$1\" = list ]; then\n" + quote(realGo) + " \"$@\"\nresult=$?\nprintf 0 > " + quote(marker) + "\nexit \"$result\"\nfi\nexec " + quote(realGo) + " \"$@\"\n"
		// The second post-list tree read happens after the cold-cache reset, beyond the
		// initial enumeration check. Add a new package there, without changing HEAD.
		gitProxy := "#!/bin/sh\ncase \"$*\" in *'HEAD^{tree}'*)\nif [ -f " + quote(marker) + " ]; then\nn=$(cat " + quote(marker) + ")\nn=$((n+1))\nprintf '%s' \"$n\" > " + quote(marker) + "\nif [ \"$n\" = 2 ]; then\nmkdir -p " + quote(filepath.Join(o.root, "new")) + "\nprintf 'package new\\n' > " + quote(filepath.Join(o.root, "new/new.go")) + "\nfi\nfi;; esac\nexec /usr/bin/git \"$@\"\n"
		for name, body := range map[string]string{"go": proxy, "git": gitProxy} {
			if err := execfile.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
		o.shards, o.partitionDigest = 4, cishards.ProfileDigest()
		ctx := context.Background()
		id, err := toolIdentity(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		s := selection{Schema: schema, Base: o.base, Head: o.head, Target: o.target, Identity: id, Packages: []string{"example.org/fixture/a"}}
		s.Tree, err = git(ctx, o, "rev-parse", "HEAD^{tree}")
		if err != nil {
			t.Fatal(err)
		}
		for _, fallback := range []bool{false, true} {
			if fallback {
				s.Reason, s.Packages = "qualification unavailable", []string{"./..."}
			}
			code, err := execute(ctx, o, s)
			if code != 2 || err == nil || !strings.Contains(err.Error(), "source drift") {
				t.Fatalf("stale universe executed fallback=%v: %d %v", fallback, code, err)
			}
			if _, err := os.Stat(filepath.Join(o.out, "execution.json")); !os.IsNotExist(err) {
				t.Fatal("stale universe recorded successful execution")
			}
			if err := os.RemoveAll(filepath.Join(o.root, "new")); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if code, err := execute(cancelled, o, s); code == 0 || err == nil {
			t.Fatalf("cancelled shard succeeded: %d %v", code, err)
		}
	})
}

// AFP-V0-053 (V1-1521): a non-shim Git is identified by itself, while the Darwin xcrun shim
// also binds the Git it runs, so two hosts sharing the shim but not the Git differ; an
// unresolvable shim refuses identification.
func TestGitIdentityResolvesDarwinShim(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "git")
	if err := execfile.WriteFile(target, []byte("real git"), 0700); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other-git")
	if err := execfile.WriteFile(other, []byte("another git"), 0700); err != nil {
		t.Fatal(err)
	}
	unexpected := func() (string, error) { t.Fatal("resolver called for a non-shim Git"); return "", nil }
	for _, c := range []struct{ goos, path string }{{"linux", darwinGitShim}, {"darwin", "/opt/homebrew/bin/git"}} {
		got, err := gitIdentity(c.goos, c.path, "h", "git version 2.x", unexpected)
		if err != nil || got != c.path+"\nh\ngit version 2.x" {
			t.Fatalf("%s %s: %q %v", c.goos, c.path, got, err)
		}
	}
	shim := func(path string) string {
		got, err := gitIdentity("darwin", darwinGitShim, "shim", "git version 2.x", func() (string, error) { return path, nil })
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	first, second := shim(target), shim(other)
	if !strings.HasPrefix(first, darwinGitShim+"\nshim\ngit version 2.x\nxcrun "+target+"\n") || first == second {
		t.Fatalf("shim identities do not bind the resolved Git:\n%s\n%s", first, second)
	}
	for name, resolve := range map[string]func() (string, error){
		"xcrun fails":   func() (string, error) { return "", errors.New("no developer tools") },
		"relative":      func() (string, error) { return "git", nil },
		"missing":       func() (string, error) { return filepath.Join(dir, "absent"), nil },
		"itself (shim)": func() (string, error) { return darwinGitShim, nil },
	} {
		if got, err := gitIdentity("darwin", darwinGitShim, "shim", "v", resolve); err == nil {
			t.Fatalf("%s: identified as %q", name, got)
		}
	}
}

func TestAFPV0098PlanModeExecutesNothing(t *testing.T) {
	o, _ := fixture(t)
	o.mode = "plan"
	o.shard, o.shards = 1, 8
	code, err := planOnly(context.Background(), o)
	if err != nil || code != 0 {
		t.Fatalf("plan mode: %d %v", code, err)
	}
	var s selection
	if err = readJSON(filepath.Join(o.out, "selection.json"), &s); err != nil {
		t.Fatal(err)
	}
	// No qualification is read: the PR narrowing gate decides from this selection.
	if s.Reason != "" || !equal(s.Packages, []string{"example.org/fixture/a"}) || s.Shard != 1 || s.Shards != 8 {
		t.Fatalf("plan selection: %+v", s)
	}
	for _, name := range []string{"execution.json", "go.json"} {
		if _, err = os.Stat(filepath.Join(o.out, name)); !os.IsNotExist(err) {
			t.Fatalf("plan mode wrote %s: %v", name, err)
		}
	}
	o.planner = filepath.Join(t.TempDir(), "missing")
	if code, err = planOnly(context.Background(), o); err != nil || code != 0 {
		t.Fatalf("plan mode with a missing planner: %d %v", code, err)
	}
	if err = readJSON(filepath.Join(o.out, "selection.json"), &s); err != nil {
		t.Fatal(err)
	}
	if s.Reason == "" || !equal(s.Packages, []string{"./..."}) {
		t.Fatalf("a failed plan must name a reason and the whole universe: %+v", s)
	}
}

// AFP-V0-800/801: the event base may lag main, but only parent 0 is planned.
func TestAFPV0800StaleEventBase(t *testing.T) {
	o, id := fixture(t)
	o.mode = "plan"
	ctx := context.Background()
	g := func(args ...string) string {
		t.Helper()
		v, err := git(ctx, o, args...)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	event := o.base
	g("checkout", "--detach", event)
	if err := os.MkdirAll(filepath.Join(o.root, ".github"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(o.root, ".github", "main-only.yml"), []byte("main only\n"), 0600); err != nil {
		t.Fatal(err)
	}
	g("add", ".")
	g("commit", "-qm", "main moved")
	effective := g("rev-parse", "HEAD")
	g("merge", "--no-ff", "-m", "tested merge", o.head)
	o.target = g("rev-parse", "HEAD")
	s := plan(ctx, o, id, true)
	if s.Reason != "" || s.EventBase != event || s.Base != effective || !equal(s.Packages, []string{"example.org/fixture/a"}) {
		t.Fatalf("stale base selection: %+v", s)
	}
	var r receipt
	if err := readJSON(filepath.Join(o.out, "plan.json"), &r); err != nil {
		t.Fatal(err)
	}
	if r.Range.Base != effective || !equal(r.Plan.Dirty, []string{"a/a.go"}) {
		t.Fatalf("main's .github change attributed to PR: %+v", r)
	}
	valid := o
	tree := g("rev-parse", "HEAD^{tree}")
	unreadable := strings.Repeat("0", 40)
	if _, err := git(ctx, o, "show", "-s", "--format=%P", unreadable); err == nil {
		t.Fatal("unreadable target unexpectedly resolved")
	}
	for _, tc := range []struct{ name, base, head, target, reason string }{
		{"missing-base", "", o.head, o.target, "pr-base-invalid-oid"},
		{"missing-head", event, "", o.target, "pr-base-invalid-oid"},
		{"missing-target", event, o.head, "", "pr-base-invalid-oid"},
		{"invalid-base", "invalid", o.head, o.target, "pr-base-invalid-oid"},
		{"invalid-head", event, "invalid", o.target, "pr-base-invalid-oid"},
		{"invalid-target", event, o.head, "invalid", "pr-base-invalid-oid"},
		{"unreadable-parents", event, o.head, unreadable, "pr-base-parents-unreadable"},
		{"one-parent", event, o.head, o.head, "pr-base-parent-count"},
		{"three-parents", event, o.head, g("commit-tree", tree, "-p", effective, "-p", o.head, "-p", event, "-m", "octopus"), "pr-base-parent-count"},
		{"wrong-head", event, event, o.target, "pr-base-head-mismatch"},
		{"non-ancestor", o.head, o.head, o.target, "pr-base-not-ancestor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o = valid
			o.base, o.head, o.target = tc.base, tc.head, tc.target
			if base, err := effectivePRBase(ctx, o); err == nil || err.Error() != tc.reason || base != "" {
				t.Fatalf("effectivePRBase: base=%q err=%v, want %s", base, err, tc.reason)
			}
			s := plan(ctx, o, id, true)
			if s.Reason != tc.reason || !equal(s.Packages, []string{"./..."}) {
				t.Fatalf("refusal: %+v", s)
			}
		})
	}
}
