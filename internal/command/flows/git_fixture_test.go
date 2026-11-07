// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"os"
	"os/exec"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// fixtureGitEnvironment and fixtureGitCommand keep fixture commits independent of an exported
// Git author or committer (V1-4719, CCF-V1-9800); they mirror cmd/corvint/git_fixture_test.go
// for the flows tests that moved here (V1-4756).
func fixtureGitEnvironment() []string {
	return testsupport.WithoutGitIdentity(os.Environ())
}

func fixtureGitCommand(arguments ...string) *exec.Cmd {
	command := exec.Command("git", arguments...)
	command.Env = fixtureGitEnvironment()
	return command
}
