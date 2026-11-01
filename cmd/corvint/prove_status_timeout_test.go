package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// FPK-V0-920 (V1-3042): a worktree status read that outlasts its fixed Git deadline refuses
// with the typed time bound, and the caller's own Git environment never reaches the read.
func TestProveStatusDeadlineRefusesWithTheTimeoutCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the slow-status shim is a POSIX shell script")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("a staged entry in the caller's Git environment", func(t *testing.T) {
		root := impactCLIRepository(t)
		base := coreFreezeParent(t, root)
		caller := cliRepository(t)
		if err := os.WriteFile(filepath.Join(caller, "staged.txt"), []byte("staged\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		affectedGit(t, caller, "add", "staged.txt")
		// A caller inside a Git hook or another checkout exports these; the read is hermetic.
		t.Setenv("GIT_DIR", filepath.Join(caller, ".git"))
		t.Setenv("GIT_INDEX_FILE", filepath.Join(caller, ".git", "index"))
		t.Setenv("GIT_WORK_TREE", caller)
		code, _, stderr := runCLI(t, "--root", root, "prove", "--base", base)
		if code != 0 {
			t.Fatalf("prove read the caller's Git environment: exit %d %s", code, stderr)
		}
	})
	t.Run("a status read past its deadline", func(t *testing.T) {
		root := impactCLIRepository(t)
		base := coreFreezeParent(t, root)
		shim := t.TempDir()
		script := "#!/bin/sh\nfor argument in \"$@\"; do\n  if [ \"$argument\" = status ]; then exec /bin/sleep 30; fi\ndone\nexec '" + git + "' \"$@\"\n"
		if err := execfile.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
		code, stdout, stderr := runCLI(t, "--root", root, "prove", "--base", base)
		if code != 2 || stdout != "" || !strings.Contains(stderr, `"repository-probe-timeout"`) || strings.Contains(stderr, "unsupported-prove-history") {
			t.Fatalf("status deadline was not the typed time bound: exit %d stdout=%q stderr=%s", code, stdout, stderr)
		}
	})
}
