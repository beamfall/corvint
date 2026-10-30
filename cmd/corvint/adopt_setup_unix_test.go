//go:build unix && !solaris

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// ADS-V0-007: a FIFO at an instruction or hook file is rejected by Lstat
// without being opened, so the report neither blocks nor reads it.
func TestAdoptSetupRejectsFIFOWithoutOpening(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := setupFixture(t)
	for _, name := range []string{"AGENTS.md", ".claude/settings.json"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(path, 0o644); err != nil {
			t.Skipf("mkfifo: %v", err)
		}
	}
	checks := runSetup(t, root)
	if checks["agent-instructions"]["status"] != "gap" || checks["hooks"]["status"] != "gap" {
		t.Fatalf("FIFO read as setup: %v", checks)
	}
}

// ADS-V0-006: Git exits 0 while warning about an ignore file it cannot read;
// the check reports that as unobserved with the warning and a fix.
func TestAdoptSetupUnreadableIgnoreFileIsUnobserved(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads mode-000 files")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := setupFixture(t)
	ignore := filepath.Join(root, ".gitignore")
	if err := os.Chmod(ignore, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ignore, 0o644) })
	check := runSetup(t, root)["ignored-paths"]
	if check["status"] != "unobserved" || !strings.Contains(fmt.Sprint(check["evidence"]), "git check-ignore: ") {
		t.Fatalf("unreadable ignore file: %v", check)
	}
}
