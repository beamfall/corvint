// Package gitprobe reports whether the host Git supports an optional feature a
// test needs, so the test can skip with a named reason instead of failing on an
// older Git (V1-3582, V1-3568).
package gitprobe

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ReftableMinimum is the first Git release whose `git init` accepts
// --ref-format=reftable.
const ReftableMinimum = "2.45"

// Reftable returns nil when git (a path or a name on PATH) can create a
// reftable repository, and otherwise an error naming the Git version and the
// required minimum. It creates one throwaway repository under the system
// temporary directory and removes it.
func Reftable(git string) error {
	dir, err := os.MkdirTemp("", "corvint-gitprobe-")
	if err != nil {
		return fmt.Errorf("reftable probe: %w", err)
	}
	defer os.RemoveAll(dir)
	command := exec.Command(git, "init", "-q", "--ref-format=reftable", dir)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	return fmt.Errorf("git %s does not support --ref-format=reftable (needs Git %s or later): %v: %s", version(git), ReftableMinimum, err, strings.TrimSpace(string(output)))
}

func version(git string) string {
	output, err := exec.Command(git, "version").Output()
	if err != nil {
		return "(version unknown)"
	}
	return strings.TrimPrefix(strings.TrimSpace(string(output)), "git version ")
}

// GraftAdvice reports whether git prints its graft-file deprecation advice on
// stderr when GIT_GRAFT_FILE names the null device, the precondition of a test
// that proves a caller suppresses it (V1-3588).
func GraftAdvice(git string) bool {
	dir, err := os.MkdirTemp("", "corvint-gitprobe-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	environment := append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	// The advice is printed when Git parses a commit, so the probe needs one.
	for _, args := range [][]string{{"init", "-q", dir}, {"-C", dir, "-c", "user.name=probe", "-c", "user.email=probe@example.invalid", "commit", "-q", "--allow-empty", "-m", "probe"}} {
		step := exec.Command(git, args...)
		step.Env = environment
		if step.Run() != nil {
			return false
		}
	}
	command := exec.Command(git, "-C", dir, "rev-list", "HEAD")
	command.Env = append(environment, "GIT_GRAFT_FILE="+os.DevNull)
	var stderr strings.Builder
	command.Stderr = &stderr
	_ = command.Run()
	return strings.Contains(stderr.String(), "grafts is deprecated")
}
