package cli_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTSV0820_InitInFreshRepositoryNamesMissingQueueAndPolicy: init in a
// fresh `git init` repository refuses PREREQUISITE_UNSATISFIED, names both
// missing files and the template fix, and writes no state (V1-3904).
func TestCTSV0820_InitInFreshRepositoryNamesMissingQueueAndPolicy(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for _, withQueue := range []bool{false, true} {
		if withQueue {
			fixture.Write(t, filepath.Join(root, ".taskman", "queue.json"), fixture.QueueBytes())
		}
		var out bytes.Buffer
		cli.Run(cli.Env{Cwd: root, Args: []string{"init"}, Stdout: &out, Stderr: io.Discard})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil || res.Outcome != wire.OutcomeRefused || len(res.Codes) != 1 || res.Codes[0] != wire.CodePrerequisiteUnsatisfied {
			t.Fatalf("init (queue=%v): %s", withQueue, out.Bytes())
		}
		text := strings.Join(res.Warnings, "\n")
		queueNamed := strings.Contains(text, filepath.Join(root, ".taskman", "queue.json"))
		if queueNamed == withQueue || !strings.Contains(text, filepath.Join(root, ".taskman", "policy.json")) || !strings.Contains(text, "docs/TASKS-EXTERNAL-AGENTS.md") {
			t.Fatalf("init (queue=%v) warnings do not name the missing files and fix: %s", withQueue, text)
		}
		if _, err := os.Lstat(filepath.Join(root, ".git", "taskman")); !os.IsNotExist(err) {
			t.Fatalf("refused init created the state dir: %v", err)
		}
	}
}
