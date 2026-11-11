package contextindex

import (
	"testing"
)

const (
	recencyOldDate = "2025-01-01T00:00:00Z"
	recencyNewDate = "2026-06-01T00:00:00Z"
)

// recencyCommit commits everything staged at one author and committer date.
func recencyCommit(t *testing.T, root, date, message string) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_DATE", date)
	t.Setenv("GIT_COMMITTER_DATE", date)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", message)
}

func recencyRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "go.mod", "module example.test/fixture\n\ngo 1.27.0\n")
	return root
}

func rowReason(t *testing.T, packet map[string]any, id string) string {
	t.Helper()
	for _, item := range mapsFromAny(packet["results"]) {
		if item["id"] == id {
			return mapsFromAny(item["evidence"])[0]["reason"].(string)
		}
	}
	t.Fatalf("no row %s in %v", id, contextRowsByKind(t, packet))
	return ""
}
