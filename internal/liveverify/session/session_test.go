//go:build darwin || linux

package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

func goExecutable(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go executable not found")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Skip("go executable is not resolvable")
	}
	return resolved
}

func testEnvironment(tmp string) []gorunner.EnvironmentVariable {
	home := os.Getenv("HOME")
	if home == "" {
		home = tmp
	}
	return []gorunner.EnvironmentVariable{
		{Name: "GOCACHE", Value: filepath.Join(tmp, "gocache")},
		{Name: "GOENV", Value: "off"},
		{Name: "GOPATH", Value: filepath.Join(tmp, "gopath")},
		{Name: "GOTOOLCHAIN", Value: "local"},
		{Name: "HOME", Value: home},
	}
}

func writeModule(t *testing.T, dir, testBody string) (modPath, testPath string) {
	t.Helper()
	modPath = filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module fixture\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testPath = filepath.Join(dir, "fixture_test.go")
	if err := os.WriteFile(testPath, []byte(testBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return modPath, testPath
}

// saveFile replaces path by rename, as an editor's atomic save does, so the
// polling watcher never settles on a truncated file.
func saveFile(t *testing.T, path, body string) {
	t.Helper()
	tmp := path + ".save"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

type eventLog struct {
	mu     sync.Mutex
	events []Event
}

func (l *eventLog) publish(e Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *eventLog) snapshot() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Event(nil), l.events...)
}

func waitFor(t *testing.T, log *eventLog, deadline time.Duration, match func(Event) bool) Event {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		for _, e := range log.snapshot() {
			if match(e) {
				return e
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for matching event; got %+v", log.snapshot())
	return Event{}
}

func waitForSequenceTerminal(t *testing.T, log *eventLog, sequence uint64, want State, deadline time.Duration) Event {
	t.Helper()
	event := waitFor(t, log, deadline, func(event Event) bool {
		return event.Sequence == sequence && event.State != StateRunning
	})
	if event.State != want {
		t.Fatalf("sequence %d terminal event = %+v, want state %s; all events: %+v", sequence, event, want, log.snapshot())
	}
	return event
}

func baseConfig(t *testing.T, dir, modPath, testPath string, log *eventLog) Config {
	t.Helper()
	return Config{
		Files:            []string{modPath, testPath},
		TestFiles:        map[string]bool{testPath: true},
		Scope:            []string{"./..."},
		ModulePath:       "fixture",
		ToolchainVersion: "test-toolchain",
		GoExecutable:     goExecutable(t),
		WorkingDirectory: dir,
		Environment:      testEnvironment(dir),
		Interval:         30 * time.Millisecond,
		Debounce:         60 * time.Millisecond,
		Timeout:          20 * time.Second,
		OutputLimitBytes: gorunner.MaxOutputBytes,
		Publish:          log.publish,
	}
}

// TestRunningFailedPassed covers the core acceptance path: editing a real
// assertion in a watched test file automatically drives the session from a
// passing baseline to failed, then back to passed on a second edit.
func TestRunningFailedPassed(t *testing.T) {
	// Every wait below is a hang detector, not a performance bound (decision
	// 0082): under whole-suite load the running event alone has taken >5s.
	const runDeadline = 5 * time.Minute

	dir := t.TempDir()
	passing := "package fixture\n\nimport \"testing\"\n\nfunc TestAssertion(t *testing.T) {\n\tif false {\n\t\tt.Fatal(\"boom\")\n\t}\n}\n"
	failing := "package fixture\n\nimport \"testing\"\n\nfunc TestAssertion(t *testing.T) {\n\tif true {\n\t\tt.Fatal(\"boom\")\n\t}\n}\n"
	modPath, testPath := writeModule(t, dir, passing)

	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	cfg.Timeout = runDeadline
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			cancel()
			if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("Run returned unexpected error during shutdown: %v; events: %+v", err, log.snapshot())
			}
		})
	}
	t.Cleanup(shutdown)

	waitFor(t, log, runDeadline, func(e Event) bool { return e.Sequence == 1 && e.State == StateRunning })
	passed := waitForSequenceTerminal(t, log, 1, StatePassed, runDeadline)
	assertProjection(t, passed, testvalidity.ExecutionPassed, "", testvalidity.FreshnessCurrent)

	saveFile(t, testPath, failing)
	waitFor(t, log, runDeadline, func(e Event) bool { return e.Sequence == 2 && e.State == StateRunning })
	failed := waitForSequenceTerminal(t, log, 2, StateFailed, runDeadline)
	assertProjection(t, failed, testvalidity.ExecutionFailed, "ASSERTION_OR_TEST", testvalidity.FreshnessCurrent)
	if len(failed.Tests) != 1 || failed.Tests[0].Name != "TestAssertion" || failed.Tests[0].Package != "fixture" ||
		failed.Tests[0].Projection.Execution.State != testvalidity.ExecutionFailed {
		t.Fatalf("failed event per-test projections = %+v, want one FAILED fixture.TestAssertion", failed.Tests)
	}
	if anchors := failed.Tests[0].Projection.Execution.Anchors; len(anchors) != 2 || anchors[0] != "fixture_test.go:5" {
		t.Fatalf("failed per-test execution anchors = %v, want the declaration fixture_test.go:5 first", anchors)
	}

	saveFile(t, testPath, passing)
	waitFor(t, log, runDeadline, func(e Event) bool { return e.Sequence == 3 && e.State == StateRunning })
	waitForSequenceTerminal(t, log, 3, StatePassed, runDeadline)
	shutdown()
}

// TestPassWithoutRunAbstains drives the session entry point with a malformed
// stream that names a test only at its terminal. Without an observed run, the
// per-test execution axis has no evidence and must not publish a ghost pass.
func TestPassWithoutRunAbstains(t *testing.T) {
	dir := t.TempDir()
	modPath, testPath := writeModule(t, dir, "package fixture\n\nimport \"testing\"\n\nfunc TestReal(t *testing.T) {}\n")
	goStub := filepath.Join(dir, "go-stub")
	stub := "#!/bin/sh\nprintf '%s\\n' '{\"Action\":\"pass\",\"Package\":\"fixture\",\"Test\":\"TestGhost\",\"Elapsed\":0}' '{\"Action\":\"pass\",\"Package\":\"fixture\",\"Elapsed\":0}'\n"
	if err := os.WriteFile(goStub, []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	cfg.GoExecutable = goStub
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	passed := waitFor(t, log, 5*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State == StatePassed })
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if len(passed.Tests) != 1 || passed.Tests[0].Name != "TestGhost" || passed.Tests[0].Action != "none" {
		t.Fatalf("pass without run projected %+v, want TestGhost action none", passed.Tests)
	}
	assertProjection(t, Event{Identity: passed.Identity, Projection: passed.Tests[0].Projection}, testvalidity.StateUnsupported, "no-execution-input", testvalidity.FreshnessCurrent)
}

// TestExitZeroWithFailedTestIsInfrastructure pins GLTP-V0-048: a TestMain
// that exits 0 after a failed test makes go test exit 0, and that exit status
// disagrees with the stream, so the run publishes infrastructure, not passed.
func TestExitZeroWithFailedTestIsInfrastructure(t *testing.T) {
	dir := t.TempDir()
	body := "package fixture\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestBroken(t *testing.T) { t.Fatal(\"broken\") }\n\nfunc TestMain(m *testing.M) { m.Run(); os.Exit(0) }\n"
	modPath, testPath := writeModule(t, dir, body)
	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	terminal := waitFor(t, log, 20*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State != StateRunning })
	if terminal.State != StateInfrastructure {
		t.Fatalf("exit 0 with a failed test published %s, want infrastructure", terminal.State)
	}
	cancel()
	<-done
}

// TestFramedPassCannotMaskFailedTest pins GLTP-V0-051: a test that prints
// `go test` framing lines makes test2json report a pass for a test that
// already failed, so a test with two different terminal actions abstains as
// none rather than publishing the later PASSED.
func TestFramedPassCannotMaskFailedTest(t *testing.T) {
	dir := t.TempDir()
	body := "package fixture\n\nimport (\n\t\"fmt\"\n\t\"testing\"\n)\n\nfunc TestBroken(t *testing.T) { t.Fatal(\"broken\") }\n\nfunc TestForger(t *testing.T) {\n\tfmt.Print(\"\\x16=== RUN   TestBroken\\n\\x16--- PASS: TestBroken (0.00s)\\n\")\n}\n"
	modPath, testPath := writeModule(t, dir, body)
	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	terminal := waitFor(t, log, 20*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State != StateRunning })
	cancel()
	<-done
	if terminal.State != StateFailed {
		t.Fatalf("run published %s, want failed", terminal.State)
	}
	found := false
	for _, test := range terminal.Tests {
		if test.Name != "TestBroken" {
			continue
		}
		found = true
		if test.Action != "none" || test.Projection.Execution.State != testvalidity.StateUnsupported {
			t.Fatalf("TestBroken projected %s/%s, want none/%s", test.Action, test.Projection.Execution.State, testvalidity.StateUnsupported)
		}
	}
	if !found {
		t.Fatalf("per-test projections = %+v, want a TestBroken entry", terminal.Tests)
	}
}

// TestStaleOnLateArrival covers the second acceptance requirement: a run
// started against an older identity that keeps executing past a second edit
// is reported Stale, not Passed/Failed, when it finally completes.
func TestStaleOnLateArrival(t *testing.T) {
	dir := t.TempDir()
	slow := "package fixture\n\nimport (\n\t\"testing\"\n\t\"time\"\n)\n\nfunc TestAssertion(t *testing.T) {\n\ttime.Sleep(1200 * time.Millisecond)\n}\n"
	fast := "package fixture\n\nimport \"testing\"\n\nfunc TestAssertion(t *testing.T) {\n}\n"
	modPath, testPath := writeModule(t, dir, slow)

	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	waitFor(t, log, 5*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State == StateRunning })

	// Edit while run 1 is still sleeping: this changes the identity but must
	// not disturb the in-flight go test process, whose compiled behavior is
	// fixed at launch.
	time.Sleep(300 * time.Millisecond)
	if err := os.WriteFile(testPath, []byte(fast), 0o644); err != nil {
		t.Fatal(err)
	}

	stale := waitFor(t, log, 20*time.Second, func(e Event) bool { return e.Sequence == 1 && (e.State == StateStale || e.State == StatePassed) })
	if stale.State != StateStale {
		t.Fatalf("expected run 1 to be reported STALE after a later edit, got %s", stale.State)
	}
	assertProjection(t, stale, testvalidity.ExecutionInfrastructure, "STALE", testvalidity.FreshnessStale)

	waitFor(t, log, 5*time.Second, func(e Event) bool { return e.Sequence == 2 && e.State == StateRunning })
	waitFor(t, log, 20*time.Second, func(e Event) bool { return e.Sequence == 2 && e.State == StatePassed })

	cancel()
	<-done
}

// TestDebounceDuringActiveRunRace exercises the same overlap as
// TestStaleOnLateArrival but drives several debounced edits while run 1 is
// still active, so the debounce branch's currentIdentity write and the
// run-completion goroutine's currentIdentity read race under `-race`.
func TestDebounceDuringActiveRunRace(t *testing.T) {
	dir := t.TempDir()
	slow := "package fixture\n\nimport (\n\t\"testing\"\n\t\"time\"\n)\n\nfunc TestAssertion(t *testing.T) {\n\ttime.Sleep(1200 * time.Millisecond)\n}\n"
	modPath, testPath := writeModule(t, dir, slow)

	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	waitFor(t, log, 5*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State == StateRunning })

	for i := 0; i < 3; i++ {
		time.Sleep(150 * time.Millisecond)
		body := fmt.Sprintf("package fixture\n\nimport \"testing\"\n\nfunc TestAssertion(t *testing.T) {\n\t_ = %d\n}\n", i)
		if err := os.WriteFile(testPath, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	waitFor(t, log, 20*time.Second, func(e Event) bool { return e.Sequence == 1 && (e.State == StateStale || e.State == StatePassed) })
	waitFor(t, log, 5*time.Second, func(e Event) bool { return e.Sequence == 2 && e.State == StateRunning })
	waitFor(t, log, 20*time.Second, func(e Event) bool { return e.Sequence == 2 && e.State == StatePassed })

	cancel()
	<-done
}

// TestCancellationLeavesNoDescendants mirrors the console's
// lifecycle_test.go ownedFixture/assertChildGone pattern: a fixture test
// spawns and detaches a long-lived shell child, and session shutdown
// (context cancellation, standing in for SIGINT) must leave no descendant
// running.
func TestCancellationLeavesNoDescendants(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "spawn.sh")
	body := fmt.Sprintf("#!/bin/sh\nsleep 60 &\nchild=$!\necho $child > '%s'\nwait \"$child\"\n", pidfile)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	testBody := fmt.Sprintf("package fixture\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestSpawn(t *testing.T) {\n\tcmd := exec.Command(%q)\n\t_ = cmd.Run()\n}\n", script)
	modPath, testPath := writeModule(t, dir, testBody)

	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	// The run bound must exceed waitChildPID's hang bound, or a cold compile
	// under load is killed first and reported as a descendant that never started.
	cfg.Timeout = 5 * time.Minute
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	waitFor(t, log, 5*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State == StateRunning })

	pid := waitChildPID(t, log, pidfile)

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("session did not shut down after cancellation")
	}
	waitFor(t, log, 5*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State == StateCancelled })
	assertGone(t, pid)
}

// waitChildPID shares the cold `go test -json` compile every waitFor call in
// this file waits on (each test points GOCACHE at a fresh t.TempDir(), so the
// compile is cold every time). The siblings bound that same compile at 20s;
// on a loaded host that 20s itself measured within a second of firing (see
// docs/agent-memory/tests.md history), and 60s fired at load ~60 in the v0-6
// gate at 26f12d41, so this is bounded at 4 minutes — a hang detector for a
// stuck fork or compile, not a budget for either (decision 0082).
func waitChildPID(t *testing.T, log *eventLog, path string) int {
	t.Helper()
	end := time.Now().Add(4 * time.Minute)
	var lastErr error
	for time.Now().Before(end) {
		if ended := runEnded(log, 1); ended != nil {
			t.Fatalf("run 1 ended before its descendant started: %+v", *ended)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
		} else if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid > 0 {
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant did not start within 4m: last pidfile read error=%v", lastErr)
	return 0
}

func runEnded(log *eventLog, sequence uint64) *Event {
	for _, e := range log.snapshot() {
		if e.Sequence == sequence && e.State != StateRunning {
			return &e
		}
	}
	return nil
}

func assertGone(t *testing.T, pid int) {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owned descendant %d survived cancellation", pid)
}

func TestAffectedScopeFallsBackWhenSelectionCannotBeJustified(t *testing.T) {
	testFile := "/repo/pkg/a/a_test.go"
	sourceFile := "/repo/pkg/a/a.go"
	s := &Session{cfg: Config{
		Scope:            []string{"./..."},
		ModulePath:       "example.com/repo",
		WorkingDirectory: "/repo",
		TestFiles:        map[string]bool{testFile: true},
	}}

	if got := s.affectedScope(map[string]bool{testFile: true}); len(got) != 1 || got[0] != "example.com/repo/pkg/a" {
		t.Fatalf("expected narrowed scope for a test-only change, got %v", got)
	}
	if got := s.affectedScope(map[string]bool{sourceFile: true}); len(got) != 1 || got[0] != "./..." {
		t.Fatalf("expected fallback to full scope for a non-test change, got %v", got)
	}
	if got := s.affectedScope(map[string]bool{testFile: true, sourceFile: true}); len(got) != 1 || got[0] != "./..." {
		t.Fatalf("expected fallback to full scope when any changed file is not a known test file, got %v", got)
	}
	if got := s.affectedScope(map[string]bool{}); len(got) != 1 || got[0] != "./..." {
		t.Fatalf("expected full scope with no changed files, got %v", got)
	}

	noModulePath := &Session{cfg: Config{Scope: []string{"./..."}, WorkingDirectory: "/repo", TestFiles: map[string]bool{testFile: true}}}
	if got := noModulePath.affectedScope(map[string]bool{testFile: true}); len(got) != 1 || got[0] != "./..." {
		t.Fatalf("expected fallback to full scope with no ModulePath configured, got %v", got)
	}
}

// sourceModule adds a watched non-test source file to a writeModule fixture.
func sourceModule(t *testing.T, dir string, cfg *Config) string {
	t.Helper()
	sourcePath := filepath.Join(dir, "source.go")
	if err := os.WriteFile(sourcePath, []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Files = append(cfg.Files, sourcePath)
	return sourcePath
}

// TestAffectedScopeFallsBackForNestedModuleTest pins that a test file inside
// a nested module (a go.mod below the working directory) is not narrowed to
// an import path of the root module, which go test cannot resolve.
func TestAffectedScopeFallsBackForNestedModuleTest(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "inner")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "go.mod"), []byte("module other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(nested, "x_test.go")
	s := &Session{cfg: Config{
		Scope:            []string{"./..."},
		ModulePath:       "fixture",
		WorkingDirectory: dir,
		TestFiles:        map[string]bool{testFile: true},
	}}
	if got := s.affectedScope(map[string]bool{testFile: true}); len(got) != 1 || got[0] != "./..." {
		t.Fatalf("affectedScope(nested-module test) = %v, want fallback to ./...", got)
	}
}

// TestQueuedNarrowSettleKeepsEarlierFallback covers GLTP-V0-047 across
// coalescing: a non-test edit queued behind an active run must not be
// dropped when a later test-only edit replaces the queued run.
func TestQueuedNarrowSettleKeepsEarlierFallback(t *testing.T) {
	dir := t.TempDir()
	slow := "package fixture\n\nimport (\n\t\"testing\"\n\t\"time\"\n)\n\nfunc TestAssertion(t *testing.T) {\n\ttime.Sleep(3 * time.Second)\n}\n"
	fast := "package fixture\n\nimport \"testing\"\n\nfunc TestAssertion(t *testing.T) {\n}\n"
	modPath, testPath := writeModule(t, dir, slow)
	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	sourcePath := sourceModule(t, dir, &cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	waitFor(t, log, 5*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State == StateRunning })
	if err := os.WriteFile(sourcePath, []byte("package fixture\n\nconst edited = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	if err := os.WriteFile(testPath, []byte(fast), 0o644); err != nil {
		t.Fatal(err)
	}
	if ended := runEnded(log, 1); ended != nil {
		t.Fatalf("run 1 ended before both edits settled: %+v", *ended)
	}
	second := waitFor(t, log, 60*time.Second, func(e Event) bool { return e.Sequence == 2 && e.State == StateRunning })
	if len(second.Scope) != 1 || second.Scope[0] != "./..." {
		t.Fatalf("queued run scope = %v, want full scope for the earlier non-test edit", second.Scope)
	}
	cancel()
	<-done
}

// TestFailedDigestSettleKeepsFallback covers GLTP-V0-047 when a settle's
// identity cannot be computed: its non-test change still forces the next
// run's full scope.
func TestFailedDigestSettleKeepsFallback(t *testing.T) {
	dir := t.TempDir()
	passing := "package fixture\n\nimport \"testing\"\n\nfunc TestAssertion(t *testing.T) {\n}\n"
	modPath, testPath := writeModule(t, dir, passing)
	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	sourcePath := sourceModule(t, dir, &cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	waitFor(t, log, 60*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State == StatePassed })
	if err := os.WriteFile(sourcePath, []byte("package fixture\n\nconst edited = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(testPath); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	if err := os.WriteFile(testPath, []byte(passing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := waitFor(t, log, 5*time.Second, func(e Event) bool { return e.Sequence == 2 && e.State == StateRunning })
	if len(second.Scope) != 1 || second.Scope[0] != "./..." {
		t.Fatalf("run scope after a failed settle = %v, want full scope for the non-test edit", second.Scope)
	}
	cancel()
	<-done
}

// TestEditBeforeBaselineSnapshotIsDetected covers GLTP-V0-045: an edit that
// lands after the initial identity is computed must still be detected, or
// the session labels results with an identity the files no longer have.
func TestEditBeforeBaselineSnapshotIsDetected(t *testing.T) {
	dir := t.TempDir()
	passing := "package fixture\n\nimport \"testing\"\n\nfunc TestAssertion(t *testing.T) {\n}\n"
	modPath, testPath := writeModule(t, dir, passing)
	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	cfg.Publish = func(e Event) {
		if e.State == StateIdle {
			if err := os.WriteFile(testPath, []byte(passing+"\n// edited\n"), 0o644); err != nil {
				t.Error(err)
			}
		}
		log.publish(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	idle := waitFor(t, log, 5*time.Second, func(e Event) bool { return e.State == StateIdle })
	second := waitFor(t, log, 60*time.Second, func(e Event) bool { return e.Sequence == 2 && e.State == StateRunning })
	if second.Identity == idle.Identity {
		t.Fatalf("second run identity = baseline identity %s despite the edit", idle.Identity)
	}
	cancel()
	<-done
}

// TestUnsettledEditAtCompletionIsStale pins that a run completing while an
// edit is detected but not yet settled is stale, not current, and that
// reverting that edit to the run's own content still reruns.
func TestUnsettledEditAtCompletionIsStale(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "edit-once")
	if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	original := fmt.Sprintf("package fixture\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestSelfEdit(t *testing.T) {\n\tif os.Remove(%q) != nil {\n\t\treturn\n\t}\n\tf, err := os.OpenFile(\"fixture_test.go\", os.O_APPEND|os.O_WRONLY, 0)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\t_, _ = f.WriteString(\"// edited\\n\")\n\t_ = f.Close()\n}\n", sentinel)
	modPath, testPath := writeModule(t, dir, original)
	log := &eventLog{}
	cfg := baseConfig(t, dir, modPath, testPath, log)
	cfg.Debounce = 3 * time.Second
	cfg.Publish = func(e Event) {
		if e.Sequence == 1 && e.State == StateStale {
			if err := os.WriteFile(testPath, []byte(original), 0o644); err != nil {
				t.Error(err)
			}
		}
		log.publish(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	first := waitFor(t, log, 60*time.Second, func(e Event) bool { return e.Sequence == 1 && e.State != StateRunning })
	if first.State != StateStale {
		t.Fatalf("run 1 completed with an unsettled edit on disk as %s, want stale", first.State)
	}
	second := waitFor(t, log, 60*time.Second, func(e Event) bool { return e.Sequence == 2 && e.State == StatePassed })
	if second.Identity != first.Identity {
		t.Fatalf("rerun identity = %s, want the reverted identity %s", second.Identity, first.Identity)
	}
	cancel()
	<-done
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		result gorunner.Result
		err    error
		want   State
	}{
		{"cancelled", gorunner.Result{Cancelled: true}, nil, StateCancelled},
		{"runner error", gorunner.Result{Started: true, ProcessCleanupDone: true}, errors.New("boom"), StateInfrastructure},
		{"not started", gorunner.Result{}, nil, StateInfrastructure},
		{"timed out", gorunner.Result{Started: true, TimedOut: true, ProcessCleanupDone: true}, nil, StateInfrastructure},
		{"output limit", gorunner.Result{Started: true, OutputLimitExceeded: true, ProcessCleanupDone: true}, nil, StateInfrastructure},
		{"cleanup incomplete", gorunner.Result{Started: true, Exited: true, ExitCode: 0}, nil, StateInfrastructure},
		{"passed", gorunner.Result{Started: true, Exited: true, ExitCode: 0, ProcessCleanupDone: true}, nil, StatePassed},
		{"failed", gorunner.Result{Started: true, Exited: true, ExitCode: 1, ProcessCleanupDone: true}, nil, StateFailed},
		{"terminated by signal", gorunner.Result{Started: true, ExitCode: -1, ProcessCleanupDone: true}, nil, StateInfrastructure},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := classify(c.result, c.err)
			if got != c.want {
				t.Fatalf("classify() = %s, want %s", got, c.want)
			}
		})
	}
}

func TestFailureDetail(t *testing.T) {
	stdout := strings.Join([]string{
		`{"Action":"run","Package":"fixture","Test":"TestOne"}`,
		`{"Action":"output","Package":"fixture","Test":"TestOne","Output":"--- FAIL: TestOne (0.00s)\n"}`,
		`{"Action":"fail","Package":"fixture","Test":"TestOne"}`,
		`{"Action":"output","Package":"fixture","Test":"TestTwo","Output":"--- FAIL: TestTwo (0.01s)\n"}`,
		`{"Action":"fail","Package":"fixture","Test":"TestTwo"}`,
		`{"Action":"output","Package":"fixture","Output":"FAIL\n"}`,
		`{"Action":"output","Package":"fixture","Output":"FAIL\tfixture\t0.02s\n"}`,
		`{"Action":"fail","Package":"fixture"}`,
	}, "\n")

	got := failureDetail([]byte(stdout))
	want := "TestOne; TestTwo; FAIL; FAIL\tfixture\t0.02s"
	if got != want {
		t.Fatalf("failureDetail() = %q, want %q", got, want)
	}

	if got := failureDetail(nil); got != "output-not-retained" {
		t.Fatalf("failureDetail(nil) = %q, want output-not-retained", got)
	}
	if got := failureDetail([]byte{}); got != "output-not-retained" {
		t.Fatalf("failureDetail([]byte{}) = %q, want output-not-retained", got)
	}

	long := `{"Action":"output","Package":"fixture","Output":"FAIL\t` + strings.Repeat("é", 400) + `\n"}`
	if got := failureDetail([]byte(long)); len(got) > maxFailureDetailBytes || !utf8.ValidString(got) ||
		!strings.HasSuffix(got, failureDetailTruncatedMarker) {
		t.Fatalf("failureDetail(multi-byte overflow) = %q (%d bytes), want valid UTF-8 within %d bytes ending in the marker", got, len(got), maxFailureDetailBytes)
	}
}

// TestOutputParsersReadPastAnOversizedLine pins that one go test -json line
// over the former 1 MiB scanner bound (a whole build-output event) does not
// silently end parsing: later failures and tests are still reported.
func TestOutputParsersReadPastAnOversizedLine(t *testing.T) {
	stdout := strings.Join([]string{
		`{"ImportPath":"fixture/big","Action":"build-output","Output":"` + strings.Repeat("x", 2<<20) + `"}`,
		`{"Action":"run","Package":"fixture","Test":"TestLate"}`,
		`{"Action":"output","Package":"fixture","Test":"TestLate","Output":"--- FAIL: TestLate (0.00s)\n"}`,
		`{"Action":"fail","Package":"fixture","Test":"TestLate"}`,
		`{"Action":"output","Package":"fixture","Output":"FAIL\tfixture\t0.01s\n"}`,
	}, "\n")
	if got, want := failureDetail([]byte(stdout)), "TestLate; FAIL\tfixture\t0.01s"; got != want {
		t.Fatalf("failureDetail() = %q, want %q", got, want)
	}
	tests, omitted := testProjections([]byte(stdout), "abc", false, nil)
	if len(tests) != 1 || omitted != 0 || tests[0].Name != "TestLate" || tests[0].Action != "fail" {
		t.Fatalf("testProjections() = %+v (%d omitted), want one failing TestLate", tests, omitted)
	}
}

// assertProjection checks that a published event carries the shared
// testvalidity projection its own state implies, and that the two axes no
// session evidence can ever support stay unstated: a session run has no TCQ
// claim behind it, and it never runs a mutation, so passing must not imply
// strength (LPCV-V0-048).
func assertProjection(t *testing.T, event Event, execution, cause, freshness string) {
	t.Helper()
	got := event.Projection
	if got.Execution.State != execution || got.Execution.Reason != cause {
		t.Fatalf("execution axis = %s/%s, want %s/%s", got.Execution.State, got.Execution.Reason, execution, cause)
	}
	if got.Freshness.State != freshness {
		t.Fatalf("freshness axis = %s, want %s", got.Freshness.State, freshness)
	}
	if got.Strength.State != testvalidity.StrengthNotMeasured || got.Strength.Reason != "no-mutation-run" {
		t.Fatalf("strength axis = %s/%s, want NOT_MEASURED/no-mutation-run", got.Strength.State, got.Strength.Reason)
	}
	if got.Association.State != testvalidity.StateUnsupported || got.Hygiene.State != testvalidity.StateUnsupported {
		t.Fatalf("association/hygiene = %s/%s, want both UNSUPPORTED", got.Association.State, got.Hygiene.State)
	}
	// A state with no execution report has no anchor to ground: Project
	// returns the UNSUPPORTED execution axis bare rather than anchoring an
	// absent report.
	if got.Execution.State == testvalidity.StateUnsupported {
		return
	}
	want := "input-identity:" + event.Identity
	if len(got.Execution.Anchors) != 1 || got.Execution.Anchors[0] != want {
		t.Fatalf("execution anchors = %v, want [%s]", got.Execution.Anchors, want)
	}
}

// TestProjectionForState pins the full state-to-projection mapping the wire
// carries, including the two non-terminal states that have no execution
// report at all and the Infrastructure/Cancelled states that produced no
// test result and so cannot claim currency.
func TestProjectionForState(t *testing.T) {
	cases := []struct {
		state     State
		execution string
		cause     string
		freshness string
	}{
		{StateIdle, testvalidity.StateUnsupported, "no-execution-input", testvalidity.FreshnessUnknown},
		{StateRunning, testvalidity.StateUnsupported, "no-execution-input", testvalidity.FreshnessUnknown},
		{StatePassed, testvalidity.ExecutionPassed, "", testvalidity.FreshnessCurrent},
		{StateFailed, testvalidity.ExecutionFailed, "ASSERTION_OR_TEST", testvalidity.FreshnessCurrent},
		{StateStale, testvalidity.ExecutionInfrastructure, "STALE", testvalidity.FreshnessStale},
		{StateInfrastructure, testvalidity.ExecutionInfrastructure, "INFRASTRUCTURE", testvalidity.FreshnessUnknown},
		{StateCancelled, testvalidity.ExecutionCancelled, "CANCELLATION", testvalidity.FreshnessUnknown},
	}
	for _, c := range cases {
		t.Run(string(c.state), func(t *testing.T) {
			assertProjection(t, Event{State: c.state, Identity: "abc", Projection: projectionFor(c.state, "abc")}, c.execution, c.cause, c.freshness)
		})
	}
}

// TestTestProjections pins the per-test projection each go test -json
// terminal action yields (GLTP-V0-051, GLTP-V0-052): pass, fail, and skip
// are stated with currency, a test with no terminal action abstains on execution,
// a superseded run marks every row STALE, package-level events add no row,
// and failures come first.
func TestTestProjections(t *testing.T) {
	stdout := strings.Join([]string{
		`{"Action":"run","Package":"fixture","Test":"TestPass"}`,
		`{"Action":"pass","Package":"fixture","Test":"TestPass"}`,
		`{"Action":"run","Package":"fixture","Test":"TestSkip"}`,
		`{"Action":"skip","Package":"fixture","Test":"TestSkip"}`,
		`{"Action":"run","Package":"fixture","Test":"TestNever"}`,
		`not json`,
		`{"Action":"run","Package":"fixture","Test":"TestFail"}`,
		`{"Action":"fail","Package":"fixture","Test":"TestFail"}`,
		`{"Action":"fail","Package":"fixture"}`,
	}, "\n")
	cases := []struct {
		name, action, execution, reason string
	}{
		{"TestFail", "fail", testvalidity.ExecutionFailed, "ASSERTION_OR_TEST"},
		{"TestPass", "pass", testvalidity.ExecutionPassed, ""},
		{"TestSkip", "skip", testvalidity.ExecutionSkipped, "test-skipped"},
		{"TestNever", "none", testvalidity.StateUnsupported, "no-execution-input"},
	}
	for _, stale := range []bool{false, true} {
		tests, omitted := testProjections([]byte(stdout), "abc", stale, nil)
		if len(tests) != len(cases) || omitted != 0 {
			t.Fatalf("stale=%v: got %d tests (%d omitted), want %d", stale, len(tests), omitted, len(cases))
		}
		freshness := testvalidity.FreshnessCurrent
		if stale {
			freshness = testvalidity.FreshnessStale
		}
		for i, c := range cases {
			got := tests[i]
			if got.Package != "fixture" || got.Name != c.name || got.Action != c.action {
				t.Fatalf("stale=%v row %d = %s/%s/%s, want fixture/%s/%s", stale, i, got.Package, got.Name, got.Action, c.name, c.action)
			}
			assertProjection(t, Event{Identity: "abc", Projection: got.Projection}, c.execution, c.reason, freshness)
		}
	}
}

// TestTestProjectionsCap keeps failures inside the MaxTestProjections bound
// and reports how many rows were dropped rather than silently omitting them.
func TestTestProjectionsCap(t *testing.T) {
	var lines []string
	for i := 0; i < MaxTestProjections+2; i++ {
		lines = append(lines, fmt.Sprintf(`{"Action":"run","Package":"fixture","Test":"TestPass%d"}`, i))
		lines = append(lines, fmt.Sprintf(`{"Action":"pass","Package":"fixture","Test":"TestPass%d"}`, i))
	}
	lines = append(lines, `{"Action":"run","Package":"fixture","Test":"TestLateFail"}`)
	lines = append(lines, `{"Action":"fail","Package":"fixture","Test":"TestLateFail"}`)
	tests, omitted := testProjections([]byte(strings.Join(lines, "\n")), "abc", false, nil)
	if len(tests) != MaxTestProjections || omitted != 3 {
		t.Fatalf("got %d tests, %d omitted; want %d and 3", len(tests), omitted, MaxTestProjections)
	}
	if tests[0].Name != "TestLateFail" {
		t.Fatalf("first row = %s, want the failing test first", tests[0].Name)
	}
}

// TestSourceLocator pins GLTP-V0-052's anchor rule: exactly one top-level
// declaration in a watched, unchanged _test.go file of the package directory
// anchors the test; a duplicate, a subtest, a package outside the module, a
// file changed since its digest, and an unwatched test file all abstain.
func TestSourceLocator(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"a_test.go":     "package fixture\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n\nfunc TestDup(t *testing.T) {}\n",
		"b_test.go":     "//go:build linux\n\npackage fixture\n\nimport \"testing\"\n\nfunc TestDup(t *testing.T) {}\n",
		"sub/c_test.go": "package sub_test\n\nimport \"testing\"\n\nfunc TestSub(t *testing.T) {}\n",
	}
	var paths []string
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	s := &Session{cfg: Config{WorkingDirectory: root, ModulePath: "fixture"}}
	_, digests, err := s.digest(paths)
	if err != nil {
		t.Fatal(err)
	}
	locate := s.sourceLocator(digests)
	cases := []struct{ pkg, name, want string }{
		{"fixture", "TestOne", "a_test.go:5"},
		{"fixture/sub", "TestSub", "sub/c_test.go:5"},
		{"fixture", "TestDup", ""},
		{"fixture", "TestOne/case", ""},
		{"elsewhere", "TestOne", ""},
		{"fixture", "TestMissing", ""},
	}
	for _, c := range cases {
		if got := locate(c.pkg, c.name); got != c.want {
			t.Errorf("locate(%s, %s) = %q, want %q", c.pkg, c.name, got, c.want)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "c_test.go"), []byte(files["sub/c_test.go"]+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.sourceLocator(digests)("fixture/sub", "TestSub"); got != "" {
		t.Errorf("changed file: locate = %q, want unanchored", got)
	}
	if err := os.WriteFile(filepath.Join(root, "z_test.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.sourceLocator(digests)("fixture", "TestOne"); got != "" {
		t.Errorf("unwatched test file: locate = %q, want unanchored", got)
	}
	tests, _ := testProjections([]byte("{\"Action\":\"run\",\"Package\":\"fixture\",\"Test\":\"TestOne\"}\n"+
		`{"Action":"skip","Package":"fixture","Test":"TestOne"}`), "abc", false, locate)
	if anchors := tests[0].Projection.Execution.Anchors; len(anchors) != 2 || anchors[0] != "a_test.go:5" || anchors[1] != "input-identity:abc" {
		t.Fatalf("anchored skip execution anchors = %v, want [a_test.go:5 input-identity:abc]", anchors)
	}
}
