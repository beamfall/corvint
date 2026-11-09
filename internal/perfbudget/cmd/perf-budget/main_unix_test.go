//go:build unix

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
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
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(fakeGo), 0o700); err != nil {
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
