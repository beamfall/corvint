//go:build darwin || linux

package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
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
	mustWrite("go.mod", "module example.org/fixture\n\ngo 1.27.1\n")
	mustWrite("a/a.go", "package a\nfunc Value() int { return 1 }\n")
	mustWrite("a/a_test.go", "package a\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value()!=1 { t.Fatal(\"relevant failure\") } }\n")
	mustWrite("b/b_test.go", "package b\nimport \"testing\"\nfunc TestOther(t *testing.T) {}\n")
	ctx := context.Background()
	mustGit := func(args ...string) string {
		t.Helper()
		s, err := git(ctx, o, args...)
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
	id, err := toolIdentity(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	return o, id
}

func TestToolIdentityRequiresCurrentGoVersion(t *testing.T) {
	for _, version := range []string{"go1.27.0", "go1.27.1", "go1.27.2"} {
		t.Run(version, func(t *testing.T) {
			bin := t.TempDir()
			script := "#!/bin/sh\ncase \"$1:$2\" in\nenv:GOVERSION) printf '%s\\n' '" + version + "' ;;\nenv:CC) printf '%s\\n' '/usr/bin/cc' ;;\n*) exit 1 ;;\nesac\n"
			if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0700); err != nil {
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
			if version == "go1.27.1" {
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
	code, err := execute(ctx, o, s)
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
	if err = os.WriteFile(o.selector, append(original, 0), 0700); err != nil {
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
	if err = os.WriteFile(o.selector, original, 0700); err != nil {
		t.Fatal(err)
	}
	wrong := s
	wrong.Identity.GoBinary = "changed"
	if _, err = execute(ctx, o, wrong); err == nil {
		t.Fatal("Go binary drift executed")
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
		if err := os.WriteFile(filepath.Join(dir, "docker"), []byte("#!/bin/sh\nsleep 60 &\necho $! > \"$CORVINT_FIXTURE_PID\"\nwait\n"), 0700); err != nil {
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
		if err := os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
		trusted := t.TempDir()
		for _, name := range trustedNames {
			if err := os.WriteFile(filepath.Join(trusted, name), []byte(name), 0700); err != nil {
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
		if err := os.WriteFile(filepath.Join(dir, "docker"), script, 0700); err != nil {
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
