//go:build linux

package perfbudget

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// PRB-V0-060 (V1-3078): Go starts a child with a vfork-style clone that shares the harness's
// memory until exec, and Linux copies that shared memory's high-water mark into the child's
// maxrss at exec. A tiny child measured by a harness holding 400 MiB must still report its own
// peak resident set, not the harness's.
func TestPRBV0060PeakRSSExcludesHarness(t *testing.T) {
	ballast := make([]byte, 400<<20)
	for i := range ballast {
		ballast[i] = byte(i)
	}
	sample, err := runOnce(context.Background(), t.TempDir(), []string{"/bin/true"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sample.PeakRSS == nil || *sample.PeakRSS <= 0 || *sample.PeakRSS > 64<<20 {
		t.Fatalf("peak RSS of /bin/true under a 400 MiB harness: %v bytes", deref(sample.PeakRSS))
	}
	if sample.WallMS < 0 {
		t.Fatalf("wall %v", sample.WallMS)
	}
	_ = ballast[len(ballast)-1]
}

func deref(v *int64) int64 {
	if v == nil {
		return -1
	}
	return *v
}

// PRB-V0-060 (V1-3079): an MCP server the relay cannot start is a harness start failure naming the
// server, even though the relay itself started and the initialize request is what fails first.
func TestPRBV0060RelayedMCPStartFailure(t *testing.T) {
	const missing = "/definitely/missing/corpus-mcp"
	_, _, err := startMCP(context.Background(), t.TempDir(), []string{missing})
	var start *StartError
	if !errors.As(err, &start) || start.Command != missing {
		t.Fatalf("missing server: %T %v, want *StartError naming %s", err, err, missing)
	}
}

// PRB-V0-060 (V1-3079): the relay's report pipe is not inherited by the measured command, so a
// detached descendant that outlives the group sweep cannot hold the harness's read open.
func TestPRBV0060RelayReportNotInherited(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid is not installed")
	}
	// The tag is built at run time so no other process's command line names it.
	tag := strconv.Itoa(28+1) + ".4179"
	t.Cleanup(func() { killExact(t, "sleep "+tag) })
	done := make(chan error, 1)
	go func() {
		_, err := runOnce(context.Background(), t.TempDir(), []string{"/bin/sh", "-c",
			"setsid sleep " + tag + " </dev/null >/dev/null 2>&1 & sleep 0.3; exit 0"}, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runOnce still waiting on the relay report while a detached descendant lives")
	}
}

// killExact kills every process whose whole command line is cmdline.
func killExact(t *testing.T, cmdline string) {
	dirs, _ := filepath.Glob("/proc/[0-9]*")
	for _, d := range dirs {
		b, _ := os.ReadFile(d + "/cmdline")
		if strings.TrimSuffix(strings.ReplaceAll(string(b), "\x00", " "), " ") != cmdline {
			continue
		}
		if pid, err := strconv.Atoi(filepath.Base(d)); err == nil {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
		}
	}
}
