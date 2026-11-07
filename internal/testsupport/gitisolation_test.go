package testsupport

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// V1-2633: on the self-hosted runners the account's ~/.gitconfig became
// unopenable inside a confined package (its Landlock rule stayed pinned to the
// inode a job start had renamed away), and every git the tests spawned exited
// 128. IsolateGit must keep git working when the global configuration cannot
// be read. A mode-000 file does not stage that: git skips a global file whose
// access(2) check fails with EACCES. A directory in its place fails at open,
// as the denied inode did, with `unable to access` and `fatal: unknown error
// occurred while reading the configuration files`.
func TestIsolateGitSurvivesAnUnopenableGlobalConfiguration(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".gitconfig"), 0o700); err != nil {
		t.Fatal(err)
	}
	// Git for Windows also derives the home directory from HOME first, so one
	// variable covers every platform; the others keep no other global
	// configuration path reachable.
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))

	// Control: the same environment without IsolateGit's variables reads the
	// broken global configuration and fails as the runners did.
	control := exec.Command(git, "init", "-q", filepath.Join(home, "control"))
	control.Env = withoutGitIsolation(os.Environ())
	output, err := control.CombinedOutput()
	switch {
	case err == nil:
		t.Skip("precondition: this git ignores a directory in place of its global configuration, so an unopenable global configuration cannot be staged")
	case !strings.Contains(string(output), "configuration"):
		t.Fatalf("control failed for another reason: %v: %s", err, output)
	}

	for _, name := range []string{"GIT_AUTHOR_DATE", "GIT_COMMITTER_DATE"} {
		t.Setenv(name, os.Getenv(name))
	}
	for _, variable := range gitIsolation {
		t.Setenv(variable[0], os.Getenv(variable[0])) // restored after the test
	}
	IsolateGit()
	isolated := exec.Command(git, "init", "-q", filepath.Join(home, "isolated"))
	if output, err := isolated.CombinedOutput(); err != nil {
		t.Fatalf("git under IsolateGit still read the global configuration: %v: %s", err, output)
	}
}

// withoutGitIsolation is environ without the variables that hide the global
// configuration, so a control git reads it as an unisolated test would.
func withoutGitIsolation(environ []string) []string {
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if name == "GIT_CONFIG_GLOBAL" || name == "GIT_CONFIG_NOSYSTEM" {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}

// CCF-V1-9800: package-level fixture isolation also removes caller dates.
func TestIsolateGitRemovesCallerDates_CCFV19800(t *testing.T) {
	for _, variable := range gitIsolation {
		t.Setenv(variable[0], "foreign")
	}
	for _, name := range []string{"GIT_AUTHOR_DATE", "GIT_COMMITTER_DATE"} {
		t.Setenv(name, "2033-04-05T06:07:08+0300")
	}
	IsolateGit()
	for _, variable := range gitIsolation {
		if got := os.Getenv(variable[0]); got != variable[1] {
			t.Errorf("%s = %q, want %q", variable[0], got, variable[1])
		}
	}
	for _, name := range []string{"GIT_AUTHOR_DATE", "GIT_COMMITTER_DATE"} {
		if value, exists := os.LookupEnv(name); exists {
			t.Errorf("%s still inherited: %q", name, value)
		}
	}
}
