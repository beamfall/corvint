//go:build !darwin && !linux

package cli_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTS-V0-025: off darwin and linux, a command is refused UNSUPPORTED before
// any store or worktree access; version still answers.
func TestCTSV0025_UnsupportedPlatformRefusesUpFront(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: dir, Args: []string{"ticket", "list"}, Stdout: &stdout}); code == 0 {
		t.Fatalf("ticket list exited 0: %s", stdout.String())
	}
	if out := stdout.String(); !strings.Contains(out, wire.CodeUnsupported) || !strings.Contains(out, wire.PlatformRefusal().Error()) {
		t.Fatalf("want the platform refusal, got %s", out)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("refusal touched the worktree: %v %v", entries, err)
	}
	// A --help that is a flag's value is no help request (CAL-V0-047).
	for _, args := range [][]string{{"ticket", "create", "--payload", "--help"}, {"criterion-binding", "verify"}} {
		stdout.Reset()
		if code := cli.Run(cli.Env{Cwd: dir, Args: args, Stdout: &stdout}); code == 0 || !strings.Contains(stdout.String(), wire.CodeUnsupported) {
			t.Fatalf("%v: want the platform refusal, got %s", args, stdout.String())
		}
	}
	stdout.Reset()
	if code := cli.Run(cli.Env{Cwd: dir, Args: []string{"version"}, Stdout: &stdout}); code != 0 {
		t.Fatalf("version exited %d: %s", code, stdout.String())
	}
}
