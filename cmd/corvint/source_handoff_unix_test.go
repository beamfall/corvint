//go:build darwin || linux

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sourceGitScript writes an executable stand-in for /usr/bin/git. A child
// process writes it, so this test process never holds a write descriptor a
// concurrent fork could inherit, which on Linux makes exec of the stand-in
// fail "text file busy" before any Git step runs (V1-2433).
func sourceGitScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "git")
	write := exec.Command("/bin/sh", "-c", `umask 077 && /bin/cat > "$1" && /bin/chmod 700 "$1"`, "sh", path)
	write.Stdin = strings.NewReader("#!/bin/sh\n" + body)
	if output, err := write.CombinedOutput(); err != nil {
		t.Fatalf("write git stand-in: %v %s", err, output)
	}
	return path
}
