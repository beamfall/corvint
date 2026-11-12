package contextindex

import (
	"testing"
)

// evalVersionTokenRepository is a Git repository whose only symbols are two
// copies of one function, at an unversioned path and, when versioned is set,
// at a `v2` path as well.
func evalVersionTokenRepository(t *testing.T, versioned bool) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "go.mod", "module example.test/versions\n\ngo 1.27.0\n")
	writeTestFile(t, root, "internal/auth/session.go", "package auth\n\nfunc EnforceSessionRevocation() bool { return true }\n")
	if versioned {
		writeTestFile(t, root, "internal/v2/session.go", "package v2\n\nfunc EnforceSessionRevocation() bool { return true }\n")
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "session revocation")
	return root
}
