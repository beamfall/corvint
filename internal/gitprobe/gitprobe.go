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
