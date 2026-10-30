package analyzernativebridge

import (
	"os"
	"testing"
)

// TestMain detaches every git the tests spawn from the account's global and
// system configuration, as testsupport.IsolateGit does (V1-2635). It sets the
// variables itself: TestCandidateRootMatchesResolvedWalkPaths reruns this
// package from a copy of its directory alone, where no other package resolves.
func TestMain(m *testing.M) {
	for _, variable := range [][2]string{
		{"GIT_CONFIG_GLOBAL", os.DevNull},
		{"GIT_CONFIG_NOSYSTEM", "1"},
		{"GIT_CONFIG_PARAMETERS", "'maintenance.auto'='false' 'gc.auto'='0'"},
		{"GIT_AUTHOR_NAME", "t"}, {"GIT_AUTHOR_EMAIL", "t@example.test"},
		{"GIT_COMMITTER_NAME", "t"}, {"GIT_COMMITTER_EMAIL", "t@example.test"},
	} {
		if err := os.Setenv(variable[0], variable[1]); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
