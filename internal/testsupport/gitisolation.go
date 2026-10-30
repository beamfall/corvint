package testsupport

import "os"

// gitIsolation is the environment IsolateGit sets. Packages that cannot import
// this one (internal/tasks/store, internal/analyzernativebridge) carry the same
// list inline.
var gitIsolation = [][2]string{
	{"GIT_CONFIG_GLOBAL", os.DevNull},
	{"GIT_CONFIG_NOSYSTEM", "1"},
	{"GIT_CONFIG_PARAMETERS", "'maintenance.auto'='false' 'gc.auto'='0'"},
	{"GIT_AUTHOR_NAME", "t"}, {"GIT_AUTHOR_EMAIL", "t@example.test"},
	{"GIT_COMMITTER_NAME", "t"}, {"GIT_COMMITTER_EMAIL", "t@example.test"},
}

// IsolateGit detaches every git a test binary spawns with the inherited
// environment from the account's global and system configuration, and gives
// it the fixture identity RunGit uses. Call it from TestMain before m.Run.
//
// The self-hosted CI runners share one account whose ~/.gitconfig is replaced
// at every job start; a package confined by test-confine holds a Landlock rule
// on the old inode, so git then exits 128 reading the global configuration
// (V1-2633, V1-2635). Hiding that configuration also hides CI's global
// maintenance.auto=false and gc.auto=0, so they are set here as command-line
// configuration instead: a detached `git maintenance` would outlive the test
// (V1-1030). A test that exercises ambient configuration or identity sets its
// own values with t.Setenv, which override these.
func IsolateGit() {
	for _, variable := range gitIsolation {
		if err := os.Setenv(variable[0], variable[1]); err != nil {
			panic("testsupport.IsolateGit: " + err.Error())
		}
	}
}
