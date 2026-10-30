//go:build darwin || linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// PGO-V0-009: the version probe's group is killed with its leader, before
// the reap, so a descendant that detaches from the output pipes does not
// outlive the probe.
func TestPGOV0009WorkVersionProbeKillsItsGroupBeforeTheReap(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "member")
	script := filepath.Join(dir, "corvint")
	body := "#!/bin/sh\n/bin/sleep 300 >/dev/null 2>&1 &\necho $! > '" + pidFile + "'\necho 'Corvint test (build 7, commit unknown)'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	version, err := workBoundExecutableVersion(context.Background(), &workExecutable{executionPath: script}, os.Environ(), dir)
	if err != nil || version != "Corvint test (build 7, commit unknown)" {
		t.Fatalf("version = %q, %v", version, err)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	member, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || member <= 1 {
		t.Fatalf("member pid %q", raw)
	}
	t.Cleanup(func() { _ = syscall.Kill(member, syscall.SIGKILL) })
	deadline := time.Now().Add(3 * time.Second)
	for syscall.Kill(member, 0) != syscall.ESRCH {
		if time.Now().After(deadline) {
			t.Fatalf("version probe group member %d survived the probe", member)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
