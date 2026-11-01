package gitprobe

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeGit writes a POSIX shell stand-in for git that answers `version` with
// the given version and exits init with code.
func fakeGit(t *testing.T, version string, code int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake git is a POSIX shell script")
	}
	path := filepath.Join(t.TempDir(), "git")
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then echo 'git version " + version + "'; exit 0; fi\n"
	if code != 0 {
		script += "echo \"error: unknown option \\`ref-format=reftable'\" >&2\nexit 129\n"
	} else {
		script += "exit 0\n"
	}
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReftableNamesTheOldGitVersion(t *testing.T) {
	err := Reftable(fakeGit(t, "2.43.0", 129))
	if err == nil || !strings.Contains(err.Error(), "git 2.43.0 does not support --ref-format=reftable (needs Git 2.45 or later)") || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("old git probe = %v", err)
	}
	if err := Reftable(fakeGit(t, "2.45.1", 0)); err != nil {
		t.Fatalf("supporting git refused: %v", err)
	}
}
