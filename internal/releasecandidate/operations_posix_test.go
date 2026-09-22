//go:build darwin || linux

package releasecandidate

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/processidentity"
)

func TestPUBV0026InterruptedInstallReapsDescendant(t *testing.T) {
	t.Run("PUB-V0-026 cancelled installation joins descendants and removes staging", func(t *testing.T) {
		candidate, verified := installFixture(t)
		root := canonicalTemp(t)
		store := filepath.Join(root, "store")
		pidFile := filepath.Join(root, "child-pid")
		program := "#!/bin/sh\ntrap 'trap - EXIT INT TERM; kill \"$child\" 2>/dev/null; wait \"$child\"; exit 1' EXIT INT TERM\n/bin/sleep 60 &\nchild=$!\nprintf '%s\\n' \"$child\" > '" + pidFile + "'\nwait \"$child\"\n"
		setFixtureProgram(t, verified, program)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		finished := false
		var pid int
		var start string
		gone := false
		t.Cleanup(func() {
			cancel()
			if !finished {
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("install did not join")
				}
			}
			if gone || pid <= 0 || start == "" {
				return
			}
			cleanupContext, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			current, err := processidentity.Start(cleanupContext, pid)
			if err == nil && current == start {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		})
		go func() { _, err := InstallCore(ctx, candidate, store); done <- err }()
		deadline := time.Now().Add(10 * time.Second)
		for pid == 0 && time.Now().Before(deadline) {
			raw, _ := os.ReadFile(pidFile)
			pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			if pid == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}
		if pid <= 0 {
			t.Fatal("fixture descendant did not start")
		}
		identityContext, stop := context.WithTimeout(t.Context(), time.Second)
		var err error
		start, err = processidentity.Start(identityContext, pid)
		stop()
		if err != nil || start == "" {
			t.Fatalf("fixture identity unavailable: %v", err)
		}
		cancel()
		select {
		case err := <-done:
			finished = true
			if err == nil {
				t.Fatal("cancelled install accepted")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("cancellation exceeded cleanup bound")
		}
		assertFixtureProcessGone(t, strconv.Itoa(pid))
		gone = true
		entries, err := os.ReadDir(filepath.Join(store, "corvint", verified.Manifest.Version))
		if err != nil || len(entries) != 0 {
			t.Fatalf("interrupted stage retained: %v %v", entries, err)
		}
	})
}
