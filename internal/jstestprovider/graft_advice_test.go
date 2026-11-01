package jstestprovider

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitprobe"
)

// V1-3588: gitObservationEnvironment pins GIT_GRAFT_FILE to the null device;
// under a Git that prints graft-file deprecation advice the observation's
// stderr carries none and its stdout stays exact.
func TestGitObservationSuppressesGraftAdvice(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("procgroup.Run refuses execution outside Darwin and Linux")
	}
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
	if code != "" || len(strings.TrimSpace(string(out))) != 40 {
		t.Fatalf("rev-list HEAD = %q, %q; want one object name", out, code)
	}
	obs := runGitObservation(context.Background(), dir, "rev-list", "HEAD")
	if !obs.ExitObserved || obs.ExitStatus != 0 {
		t.Fatalf("rev-list HEAD observation = %+v", obs)
	}
	if stderr := string(obs.Stderr); strings.Contains(stderr, "grafts is deprecated") || strings.Contains(stderr, "hint:") {
		t.Fatalf("graft advice reached stderr: %q", stderr)
	}
}
