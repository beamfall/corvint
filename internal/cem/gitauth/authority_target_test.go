package gitauth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorityRequiresActualCleanHead(t *testing.T) {
	root, base, target := makeRepo(t)
	repo := open(t, root)
	if err := repo.RequireCleanTarget(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	if err := repo.RequireCleanTarget(context.Background(), base); err == nil {
		t.Fatal("stale target accepted")
	}
	writeFile(t, root, "untracked.txt", "untracked")
	if err := repo.RequireCleanTarget(context.Background(), target); err == nil {
		t.Fatal("untracked state accepted")
	}
}

func TestAuthoritySnapshotNeverRunsRepositoryCleanFilter(t *testing.T) {
	root, _, _ := makeRepo(t)
	writeFile(t, root, ".gitattributes", "*.go filter=evil\n")
	gitCmd(t, root, "add", ".gitattributes")
	gitCmd(t, root, "commit", "-qm", "attribute target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	sentinel := filepath.Join(root, "FILTER_EXECUTED")
	gitCmd(t, root, "config", "filter.evil.clean", "touch "+sentinel+"; cat")
	writeFile(t, root, "f.go", bodyV1)
	if err := open(t, root).RequireCleanTarget(context.Background(), target); err == nil {
		t.Fatal("dirty raw bytes hidden")
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("repository filter executed")
	}
}
