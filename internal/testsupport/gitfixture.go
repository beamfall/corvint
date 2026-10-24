package testsupport

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testplatform"
)

// gitFixtureTimeout bounds one fixture git command. A fixture command touches
// a small temporary repository; one that runs this long has hung.
var gitFixtureTimeout = 2 * time.Minute

// gitFixturePassthrough names the only host variables a fixture git command
// inherits: enough to find git and a temporary directory, and nothing that can
// redirect it (GIT_DIR, GIT_INDEX_FILE, GIT_WORK_TREE and the rest of a hook's
// environment) or read the host's configuration.
var gitFixturePassthrough = []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "HOME", "USERPROFILE"}

// RunGit runs git in the fixture repository root and returns its
// standard output, failing t on error (V1-0730). It is the shared hardened helper for
// test fixtures: the host's Git environment and configuration are hidden,
// automatic maintenance is off so no detached `git maintenance` outlives the
// command and races the TempDir cleanup (V1-0662), the identity is fixed, and
// the command's whole process group is stopped after gitFixtureTimeout. On
// success it returns standard output only.
func RunGit(t testing.TB, git, root string, arguments ...string) string {
	t.Helper()
	output, err := runGitFixture(gitFixtureTimeout, git, root, arguments...)
	if err != nil {
		// procgroup deliberately refuses platforms without owned process
		// groups; on Windows that refusal skips the test (V1-2486).
		testplatform.SkipWindowsRefusal(t, err)
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return output
}

func runGitFixture(timeout time.Duration, git, root string, arguments ...string) (string, error) {
	// procgroup runs git as the leader of its own process group and stops the
	// whole group at the timeout, so a hook or other descendant cannot outlive
	// the command and touch the fixture after cleanup. procgroup needs an
	// absolute clean executable, and callers may pass the bare name "git".
	git, err := exec.LookPath(git)
	if err == nil {
		git, err = filepath.Abs(git)
	}
	if err != nil {
		return "", err
	}
	run := procgroup.Run(context.Background(), procgroup.Spec{
		Argv:        append([]string{git, "-C", root, "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, arguments...),
		Dir:         root,
		Env:         gitFixtureEnvironment(),
		Timeout:     timeout,
		OutputLimit: 16 << 20,
	})
	switch {
	case run.TimedOut:
		return string(run.Stdout) + string(run.Stderr), fmt.Errorf("timed out after %s", timeout)
	case run.Err != nil:
		return string(run.Stdout) + string(run.Stderr), run.Err
	case run.ExitStatus != 0:
		return string(run.Stdout) + string(run.Stderr), fmt.Errorf("exit status %d", run.ExitStatus)
	}
	return string(run.Stdout), nil
}

func gitFixtureEnvironment() []string {
	environment := make([]string, 0, len(gitFixturePassthrough)+12)
	for _, name := range gitFixturePassthrough {
		if value, ok := os.LookupEnv(name); ok {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test",
	)
}
