package gitstatus

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExecutableFollowsPathChanges(t *testing.T) {
	fake := t.TempDir()
	script := filepath.Join(fake, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake)
	if got := Executable(); got != script {
		t.Fatalf("PATH fixture: got %q want %q", got, script)
	}
	t.Setenv("PATH", filepath.Join(fake, "missing"))
	if got := Executable(); got != "git" {
		t.Fatalf("unresolvable git: got %q want literal name", got)
	}
}

func TestExecutableResolvesAppleShimToRealGit(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Apple Git shim only exists on darwin")
	}
	if _, err := os.Stat(appleGitShim); err != nil {
		t.Skip("no /usr/bin/git on this host")
	}
	t.Setenv("PATH", "/usr/bin:/bin")
	got := Executable()
	if got == appleGitShim {
		t.Skip("xcrun could not resolve a developer Git on this host")
	}
	info, err := os.Stat(got)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("resolved %q is not a regular file: %v", got, err)
	}
}
