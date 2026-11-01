package jstestprovider

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitprobe"
)

// V1-3588: gitObservationEnvironment pins GIT_GRAFT_FILE to the null device;
// under a Git that prints graft-file deprecation advice the observed stdout
// stays exact.
func TestGitObservationReadsStdoutOnlyUnderGraftAdvice(t *testing.T) {
	if !gitprobe.GraftAdvice("git") {
		t.Skip("this git prints no graft-file advice, so there is nothing to suppress")
	}
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "--allow-empty", "-m", "one"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	out, code := gitObservation(context.Background(), dir, "rev-list", "HEAD")
	if code != "" || len(strings.TrimSpace(string(out))) != 40 || strings.Contains(string(out), "hint:") {
		t.Fatalf("rev-list HEAD = %q, %q; want one object name", out, code)
	}
}
