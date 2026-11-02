//go:build unix

package testsupport

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// V1-0730: a hung fixture git is stopped at the timeout together with the
// descendants it started, instead of holding the package until the go test
// deadline or leaving a child that touches the fixture after cleanup.
func TestRunGitFixtureTimesOutAndStopsDescendants(t *testing.T) {
	// Argv is: stand-in -C ROOT -c ... status, so $2 is the fixture root.
	hang := filepath.Join(t.TempDir(), "git")
	script := "#!/bin/sh\nsleep 30 &\necho $! > \"$2/child\"\necho $$ > \"$2/leader\"\nwait\n"
	if err := execfile.WriteFile(hang, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	started := time.Now()
	_, err := runGitFixture(time.Second, hang, root, "status")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("hung git returned err=%v", err)
	}
	if elapsed := time.Since(started); elapsed > 15*time.Second {
		t.Fatalf("timeout took %s", elapsed)
	}
	for _, name := range []string{"leader", "child"} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("the stand-in never recorded its %s: %v", name, err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		// The orphaned child is reaped by init, so it may take a moment to vanish.
		deadline := time.Now().Add(5 * time.Second)
		for processAlive(pid) {
			if time.Now().After(deadline) {
				_ = syscall.Kill(pid, syscall.SIGKILL)
				t.Fatalf("the %s (pid %d) outlived the timed-out fixture command", name, pid)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
