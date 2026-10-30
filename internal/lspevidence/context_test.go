package lspevidence

import (
	"os"
	"path/filepath"
	"testing"
)

// TCP-V0-051: relative PATH entries must not authorize an executable.
func TestContextLSPExecutableRefusesRelativePATH(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gopls"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("PATH", ".")
	if got := Executable(); got != "" {
		t.Fatalf("relative executable %q", got)
	}
	t.Setenv("PATH", dir)
	if got := Executable(); got != filepath.Join(dir, "gopls") {
		t.Fatalf("absolute executable %q", got)
	}
}
