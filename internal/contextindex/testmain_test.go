package contextindex

import (
	"os"
	"testing"
)

// gitIdentityEnvironment names the Git variables that override a fixture
// repository's configured identity and dates. Fixtures pin identity through
// `git config` and dates through t.Setenv, so an exported value from the
// caller's shell would otherwise change blame authors and golden bytes
// (V1-2287).
var gitIdentityEnvironment = []string{
	"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_AUTHOR_DATE",
	"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "GIT_COMMITTER_DATE",
}

func TestMain(m *testing.M) {
	for _, name := range gitIdentityEnvironment {
		os.Unsetenv(name)
	}
	os.Exit(m.Run())
}

// TestGitIdentityEnvironmentIsScrubbed keeps the package hermetic when the
// caller exports a Git identity, as lane and CI shells do (V1-2287).
func TestGitIdentityEnvironmentIsScrubbed(t *testing.T) {
	for _, name := range gitIdentityEnvironment {
		if value, ok := os.LookupEnv(name); ok {
			t.Errorf("%s=%q reached the package tests", name, value)
		}
	}
}
