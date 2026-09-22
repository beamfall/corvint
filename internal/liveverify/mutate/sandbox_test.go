package mutate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// requireSandbox skips a test that must run cited tests on a host that has no
// sandbox: the runner refuses such runs by design.
func requireSandbox(t *testing.T) {
	t.Helper()
	if _, err := HostSandbox(); err != nil {
		t.Skipf("no sandbox on this host: %v", err)
	}
}

// fixtureEscapingTest is a test that passes only when the sandbox holds: it
// fails the moment it can write outside the workspace or into the export,
// reach the network, or signal a process outside the sandbox.
const fixtureEscapingTest = `package calc

import (
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	if err := os.WriteFile(%q, []byte("leak"), 0o644); err == nil {
		t.Fatal("wrote outside the sandbox")
	}
	if err := os.WriteFile("planted.txt", []byte("leak"), 0o644); err == nil {
		t.Fatal("wrote into the export")
	}
	if conn, err := net.DialTimeout("tcp", "1.1.1.1:80", 2*time.Second); err == nil {
		conn.Close()
		t.Fatal("reached the network")
	}
	if err := syscall.Kill(%d, 0); err == nil {
		t.Fatal("signalled a process outside the sandbox")
	}
	if Add(1, 2) != 3 {
		t.Fatal("Add(1, 2) != 3")
	}
}
`

// fixturePlantingTest asserts nothing about calc but plants a file in the
// temporary directory and fails if a previous run's plant is still there.
const fixturePlantingTest = `package calc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdd(t *testing.T) {
	planted := filepath.Join(os.TempDir(), "planted")
	if _, err := os.Stat(planted); err == nil {
		t.Fatal("a previous run's scratch survived")
	}
	if err := os.WriteFile(planted, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = Add(1, 2)
}
`

// fixtureLingeringTest leaves a shell behind that would outlive go test.
const fixtureLingeringTest = `package calc

import (
	"os/exec"
	"testing"
)

func TestAdd(t *testing.T) {
	if err := exec.Command("sh", "-c", "sleep 300; : %s").Start(); err != nil {
		t.Fatal(err)
	}
	if Add(1, 2) != 3 {
		t.Fatal("Add(1, 2) != 3")
	}
}
`

// TestRunConfinesTheCitedTests proves a cited test can neither write outside
// scratch nor into the export, reach the network, nor signal this process,
// and still gets judged normally.
func TestRunConfinesTheCitedTests(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	leak := filepath.Join(t.TempDir(), "leak")
	root, revision := standardFixture(t, git, fmt.Sprintf(fixtureEscapingTest, leak, os.Getpid()))
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Killed {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, Killed)
	}
	if _, err := os.Stat(leak); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("leak file after the run: stat err = %v, want not-exist", err)
	}
}

// TestRunKillsWhatTheCitedTestsLeftRunning proves nothing a cited test starts
// outlives the judgment.
func TestRunKillsWhatTheCitedTestsLeftRunning(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	marker := fmt.Sprintf("corvint-mutate-linger-%d", time.Now().UnixNano())
	root, revision := standardFixture(t, git, fmt.Sprintf(fixtureLingeringTest, marker))
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Killed {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, Killed)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		output, _ := exec.Command("pgrep", "-f", marker).Output()
		if strings.TrimSpace(string(output)) == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("processes still carrying %s: %s", marker, output)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestRunReportsUnsupportedWithoutASandbox pins the fail-closed rule: no
// sandbox, no run, and the report says so.
func TestRunReportsUnsupportedWithoutASandbox(t *testing.T) {
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	previous := findSandbox
	findSandbox = func() (sandbox, error) { return sandbox{}, errors.New("no sandbox here") }
	t.Cleanup(func() { findSandbox = previous })
	report, err := Run(context.Background(), baseRequest(root, git, revision))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Unsupported {
		t.Fatalf("verdict = %s, want %s", report.Verdict, Unsupported)
	}
	if want := "cited tests cannot be sandboxed: no sandbox here"; report.Detail != want {
		t.Fatalf("detail = %q, want %q", report.Detail, want)
	}
}

// TestSeatbeltProfileConfinesWritesAndNetwork pins the profile shape.
func TestSeatbeltProfileConfinesWritesAndNetwork(t *testing.T) {
	directory := t.TempDir()
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := seatbeltProfile([]string{directory})
	if err != nil {
		t.Fatal(err)
	}
	for _, clause := range []string{"(deny network*)", "(deny signal)", "(allow signal (target same-sandbox))", "(deny file-write*)", `(literal "/dev/null")`, `(subpath "` + resolved + `")`} {
		if !strings.Contains(profile, clause) {
			t.Errorf("profile lacks %s: %s", clause, profile)
		}
	}
	if _, err := seatbeltProfile([]string{filepath.Join(directory, "missing")}); err == nil {
		t.Error("an unresolvable path produced a profile")
	}
	quoted := filepath.Join(directory, `a"b`)
	if err := os.Mkdir(quoted, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := seatbeltProfile([]string{quoted}); err == nil {
		t.Error("a path with a quote produced a profile")
	}
}

// TestBubblewrapArgvConfinesWritesAndNetwork pins the bwrap invocation.
func TestBubblewrapArgvConfinesWritesAndNetwork(t *testing.T) {
	directory := t.TempDir()
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	argv, err := bubblewrap("/usr/bin/bwrap")([]string{directory})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/usr/bin/bwrap", "--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc",
		"--bind", resolved, resolved, "--unshare-net", "--unshare-pid", "--die-with-parent", "--"}
	if strings.Join(argv, " ") != strings.Join(want, " ") {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	if _, err := bubblewrap("/usr/bin/bwrap")([]string{filepath.Join(directory, "missing")}); err == nil {
		t.Error("an unresolvable path produced an argv")
	}
}

// TestRunGivesEveryRunAFreshScratch proves a baseline cannot plant state that
// a later mutant run would trip over: the planting test survives every mutant.
func TestRunGivesEveryRunAFreshScratch(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixturePlantingTest)
	request := baseRequest(root, git, revision)
	request.Complete = true
	report, err := Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Verdict != Survived {
		t.Fatalf("verdict = %s (%s), want %s", report.Verdict, report.Detail, Survived)
	}
}

// TestRunRefusesToJudgeARunWithoutAVerdict pins that a run which produced no
// go test failure report is an error, never a kill.
func TestRunRefusesToJudgeARunWithoutAVerdict(t *testing.T) {
	git := gitExecutable(t)
	broken, err := exec.LookPath("false")
	if err != nil {
		t.Skipf("no false on PATH: %v", err)
	}
	root, revision := standardFixture(t, git, fixtureCalcTest)
	previous := findSandbox
	findSandbox = func() (sandbox, error) {
		return sandbox{name: "broken", confine: func([]string) ([]string, error) { return []string{broken}, nil }}, nil
	}
	t.Cleanup(func() { findSandbox = previous })
	_, err = Run(context.Background(), baseRequest(root, git, revision))
	if err == nil || !strings.Contains(err.Error(), "run ended without a verdict") {
		t.Fatalf("err = %v, want a run-without-verdict error", err)
	}
}

// TestRunRejectsARelativeCacheDir keeps the shared cache an absolute path.
func TestRunRejectsARelativeCacheDir(t *testing.T) {
	request := baseRequest("/repo", "git", "HEAD")
	request.CacheDir = "cache"
	if _, err := Run(context.Background(), request); err == nil || !strings.Contains(err.Error(), "CacheDir") {
		t.Fatalf("err = %v, want a CacheDir error", err)
	}
}
