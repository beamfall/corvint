package testsupport

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// gitFixtureTimeout bounds one fixture git command. A fixture command touches
// a small temporary repository; one that runs this long has hung.
var gitFixtureTimeout = 2 * time.Minute

// gitFixturePassthrough names the only host variables a fixture git command
// inherits: enough to find git and a temporary directory, and nothing that can
// redirect it (GIT_DIR, GIT_INDEX_FILE, GIT_WORK_TREE and the rest of a hook's
// environment) or read the host's configuration.
var gitFixturePassthrough = []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "HOME", "USERPROFILE"}

// RunGit runs git in the fixture repository root and returns its combined
// output, failing t on error (V1-0730). It is the shared hardened helper for
// test fixtures: the host's Git environment and configuration are hidden,
// automatic maintenance is off so no detached `git maintenance` outlives the
// command and races the TempDir cleanup (V1-0662), the identity is fixed, and
// the command is killed after gitFixtureTimeout with its pipes released.
func RunGit(t testing.TB, git, root string, arguments ...string) string {
	t.Helper()
	output, err := runGitFixture(gitFixtureTimeout, git, root, arguments...)
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return output
}

func runGitFixture(timeout time.Duration, git, root string, arguments ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, git, append([]string{"-C", root, "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, arguments...)...)
	command.Env = gitFixtureEnvironment()
	command.WaitDelay = 5 * time.Second
	output, err := command.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		err = fmt.Errorf("timed out after %s: %w", timeout, ctx.Err())
	}
	return string(output), err
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
