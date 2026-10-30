//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const dashboardRepositoryUnavailable = `{"code":"DASHBOARD_REPOSITORY_UNAVAILABLE","profile":"corvint-dashboard-error/0"}` + "\n"

// TestCEMV1292DashboardCEMStableInputStaysUnderRoot (CEM-V1-292): a map
// reached through a symlinked parent or final component, or a FIFO, is
// DASHBOARD_REPOSITORY_UNAVAILABLE with no stdout and without blocking,
// although the same bytes at a real path are admitted.
func TestCEMV1292DashboardCEMStableInputStaysUnderRoot(t *testing.T) {
	f := newDashboardStableFixture(t, false)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "change.cem.json"), f.stable.Map, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(f.root, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.root, filepath.FromSlash(wire.ExcludedCEMPath)), filepath.Join(f.root, "final.cem.json")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(f.root, "fifo.cem.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	with := func(flag, value string) []string {
		arguments := f.arguments()
		for index := range arguments {
			if arguments[index] == flag {
				arguments[index+1] = value
			}
		}
		return arguments
	}
	// The real path is admitted, so each refusal below is the path form.
	if exit, _, stderr := runCEMStable(t, f.arguments()); exit != 0 {
		t.Fatalf("real path: exit=%d stderr=%s", exit, stderr)
	}
	for name, arguments := range map[string][]string{
		"symlinked-parent":     with("--cem", "linked/change.cem.json"),
		"symlinked-parent-ocm": with("--ocm", "linked/change.cem.json"),
		"symlinked-final":      with("--cem", "final.cem.json"),
		"fifo":                 with("--cem", "fifo.cem.json"),
	} {
		done := make(chan [3]any, 1)
		go func() {
			exit, stdout, stderr := runCEMStable(t, arguments)
			done <- [3]any{exit, stdout, stderr}
		}()
		select {
		case got := <-done:
			if got[0].(int) != 2 || len(got[1].([]byte)) != 0 || string(got[2].([]byte)) != dashboardRepositoryUnavailable {
				t.Fatalf("%s: exit=%v stdout=%s stderr=%s", name, got[0], got[1], got[2])
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: read blocked", name)
		}
	}
}

// TestCEMV1292DashboardCEMStableRefusesSwappedFIFO (CEM-V1-292): when the
// regular file Lstat saw is replaced by a FIFO before the open, the open
// returns at once and the descriptor check refuses it.
func TestCEMV1292DashboardCEMStableRefusesSwappedFIFO(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "seen.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(directory, "swapped.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	before, err := root.Lstat("seen.json")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := readCEMStableRegular(root, "swapped.json", before)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("swapped FIFO admitted")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("open of a swapped-in FIFO blocked")
	}
}
