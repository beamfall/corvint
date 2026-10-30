//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/perfbudget"
)

// readyWriter collects stderr and cancels the run once a fake build reports its descendant.
type readyWriter struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	pid    int
	cancel context.CancelFunc
}

func (w *readyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for _, line := range strings.Split(w.buf.String(), "\n") {
		if rest, ok := strings.CutPrefix(line, "ready "); ok && w.pid == 0 {
			if pid, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
				w.pid = pid
				w.cancel()
			}
		}
	}
	return len(p), nil
}

// PRB-V0-037: interrupting the harness while it builds retires the build's descendants (a
// compiler or linker stands in as a detached sleep) before run returns and its deferred
// scratch-directory removal runs.
func TestPRBV0037InterruptRetiresBuildDescendants(t *testing.T) {
	bin := t.TempDir()
	fakeGo := "#!/bin/sh\nsleep 30 </dev/null >/dev/null 2>&1 &\necho \"ready $!\" >&2\nwait\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(fakeGo), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stderr := &readyWriter{cancel: cancel}
	var stdout bytes.Buffer
	code := run(ctx, []string{"--budgets", filepath.Join("..", "..", "budgets.json")}, &stdout, stderr)
	stderr.mu.Lock()
	pid, output := stderr.pid, stderr.buf.String()
	stderr.mu.Unlock()
	if pid == 0 {
		t.Fatalf("fake build never reported its descendant: exit %d stderr %q", code, output)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	if code != 2 {
		t.Fatalf("interrupted build: exit %d, want 2; stderr %q", code, output)
	}
	state, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if s := strings.TrimSpace(string(state)); s != "" && !strings.HasPrefix(s, "Z") {
		t.Fatalf("build descendant %d still running (%s) after the harness returned", pid, s)
	}
}

// PRB-V0-029 (review of GitHub #1602): a corpus MCP executable that cannot be started is a
// harness failure, exit 2, not a failed budget report with exit 1.
func TestPRBV0029UnstartableMCPExitsTwo(t *testing.T) {
	dir := t.TempDir()
	corvint := filepath.Join(dir, "corvint")
	if err := os.WriteFile(corvint, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	notExecutable := filepath.Join(dir, "not-executable")
	if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	builder, err := perfbudget.FileSHA256(corvint)
	if err != nil {
		t.Fatal(err)
	}
	// A reusable fixture record built by the fake corvint, so the run reaches the MCP scenarios
	// without generating a fixture; the fake's reads all succeed.
	fixture := filepath.Join(dir, "fixture")
	for _, sub := range []string{"app", "ruby-app"} {
		if err := os.MkdirAll(filepath.Join(fixture, sub), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	record, err := json.Marshal(perfbudget.Fixture{Generator: perfbudget.FixtureGenerator, Scale: perfbudget.Smoke, BuilderSHA256: builder, Previous: "previous.json", Current: "corpus.json", SearchQuery: "q", GetID: "id", RubyPath: "a.rb"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "fixture.json"), record, 0o600); err != nil {
		t.Fatal(err)
	}
	budgets, err := perfbudget.LoadBudgets(filepath.Join("..", "..", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	budgets.Scale = perfbudget.Smoke.Name
	for i := range budgets.Scenarios {
		budgets.Scenarios[i].Samples = 1
	}
	encoded, err := json.Marshal(budgets)
	if err != nil {
		t.Fatal(err)
	}
	budgetsPath := filepath.Join(dir, "budgets.json")
	if err := os.WriteFile(budgetsPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mcp := range []string{filepath.Join(dir, "absent"), notExecutable} {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"--budgets", budgetsPath, "--fixture", fixture, "--corvint", corvint, "--corpus-mcp", mcp}, &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "perf-budget: cannot start ") {
			t.Fatalf("--corpus-mcp %s: exit %d stdout %.200q stderr %.400q", filepath.Base(mcp), code, &stdout, &stderr)
		}
	}
}
