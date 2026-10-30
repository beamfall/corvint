//go:build darwin || linux

package procgroup

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	syscall.Umask(0o022)
	os.Exit(m.Run())
}

func TestRunProcessInheritsCallerUmask(t *testing.T) {
	previous := syscall.Umask(0o077)
	defer syscall.Umask(previous)

	fixture := t.TempDir()
	created := filepath.Join(fixture, "created")
	observation := Run(context.Background(), Spec{
		Argv:    []string{"/bin/sh", "-c", ": > created"},
		Dir:     fixture,
		Timeout: time.Second,
	})
	if observation.Err != nil {
		t.Fatal(observation.Err)
	}
	info, err := os.Stat(created)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("caller umask: got mode %04o, want 0600", got)
	}
}
