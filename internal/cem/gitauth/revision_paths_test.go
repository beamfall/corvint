package gitauth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// lsTreeFiles is `ls-tree -r`'s non-tree path listing, sorted.
func lsTreeFiles(t *testing.T, root, commit string) []string {
	t.Helper()
	out := strings.TrimSuffix(string(integrityGit(t, root, "ls-tree", "-rz", "--name-only", "--full-tree", commit)), "\x00")
	paths := strings.Split(out, "\x00")
	slices.Sort(paths)
	return paths
}

// TestDCXV1520RevisionPathsMatchesLsTreeWithinOneOpPerLevel pins DCX-V1-520:
// the verified walk lists exactly ls-tree -r's non-tree paths (symlinks and
// gitlinks included) in one pinning operation plus one per directory level.
func TestDCXV1520RevisionPathsMatchesLsTreeWithinOneOpPerLevel(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"sha1", "sha256"} {
		t.Run("DCX-V1-520 "+format, func(t *testing.T) {
			root, commit := pathsRepo(t, format)
			// pkg, pkg/d and pkg/d/deeper are three levels below the root.
			repo, err := Open(root, gitrun.NewBudget(4, gitrun.DefaultTotalBudget))
			if err != nil {
				t.Fatal(err)
			}
			paths, err := repo.RevisionPaths(context.Background(), commit, 1000, nil)
			if err != nil {
				t.Fatalf("RevisionPaths within four Git operations: %v", err)
			}
			if want := lsTreeFiles(t, root, commit); !slices.Equal(paths, want) {
				t.Fatalf("paths = %q, want %q", paths, want)
			}
		})
	}
}

// TestDCXV1520RevisionPathsOpsIndependentOfDirectoryCount pins DCX-V1-520 for
// a tree with more directories than the default 1,024-operation budget: two
// levels below the root list in three Git operations.
func TestDCXV1520RevisionPathsOpsIndependentOfDirectoryCount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	integrityGit(t, root, "init", "-q")
	integrityGit(t, root, "config", "user.email", "fixture@example.invalid")
	integrityGit(t, root, "config", "user.name", "Fixture")
	const dirs = 1500
	for i := 0; i < dirs; i++ {
		writeFile(t, root, fmt.Sprintf("a/d%04d/f.txt", i), fmt.Sprintf("%d\n", i))
	}
	integrityGit(t, root, "add", "-A")
	integrityGit(t, root, "commit", "-qm", "seed")
	commit := integrityOID(t, root, "HEAD")
	repo, err := Open(root, gitrun.NewBudget(3, gitrun.DefaultTotalBudget))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := repo.RevisionPaths(context.Background(), commit, 2*dirs+1, nil)
	if err != nil {
		t.Fatalf("RevisionPaths over %d directories within three Git operations: %v", dirs, err)
	}
	if want := lsTreeFiles(t, root, commit); len(paths) != dirs || !slices.Equal(paths, want) {
		t.Fatalf("listed %d paths, want the %d ls-tree paths", len(paths), len(want))
	}
}

// TestDCXV1521RevisionPathsPrunesRejectedSubtrees pins DCX-V1-521: a subtree
// descend rejects is never read, so pruning pkg leaves one Git operation.
func TestDCXV1521RevisionPathsPrunesRejectedSubtrees(t *testing.T) {
	t.Parallel()
	root, commit := pathsRepo(t, "sha1")
	repo, err := Open(root, gitrun.NewBudget(1, gitrun.DefaultTotalBudget))
	if err != nil {
		t.Fatal(err)
	}
	asked := []string{}
	paths, err := repo.RevisionPaths(context.Background(), commit, 1000, func(dir string) bool {
		asked = append(asked, dir)
		return false
	})
	if err != nil {
		t.Fatalf("pruned RevisionPaths within one Git operation: %v", err)
	}
	if !slices.Equal(paths, []string{"other.txt"}) || !slices.Equal(asked, []string{"pkg"}) {
		t.Fatalf("paths = %q, asked = %q", paths, asked)
	}
	repo, err = Open(root, gitrun.NewBudget(3, gitrun.DefaultTotalBudget))
	if err != nil {
		t.Fatal(err)
	}
	paths, err = repo.RevisionPaths(context.Background(), commit, 1000, func(dir string) bool { return dir != "pkg/d/deeper" })
	if err != nil {
		t.Fatalf("RevisionPaths pruning pkg/d/deeper within three Git operations: %v", err)
	}
	if slices.Contains(paths, "pkg/d/deeper/x.go") || !slices.Contains(paths, "pkg/d/inner.txt") {
		t.Fatalf("paths = %q", paths)
	}
}

// TestDCXV1522RevisionPathsRefusesPastItsBounds pins DCX-V1-522: the entry
// bound refuses before the next level is read (so within the operations already
// spent), and an unpinned revision is refused before any Git operation.
func TestDCXV1522RevisionPathsRefusesPastItsBounds(t *testing.T) {
	t.Parallel()
	root, commit := pathsRepo(t, "sha1")
	// The root lists two entries and pkg nine; a bound of two refuses on pkg's
	// level, before pkg/d is read, so two operations suffice.
	repo, err := Open(root, gitrun.NewBudget(2, gitrun.DefaultTotalBudget))
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.RevisionPaths(context.Background(), commit, 2, nil)
	var coded *cemcode.Error
	if !errors.As(err, &coded) || coded.Code != cemcode.RepositoryObjectUnavailable || !strings.Contains(err.Error(), "more than 2 tree entries") {
		t.Fatalf("entry bound: %v", err)
	}
	repo, err = Open(root, gitrun.NewBudget(0, gitrun.DefaultTotalBudget))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RevisionPaths(context.Background(), "HEAD", 1000, nil); !errors.As(err, &coded) || coded.Code != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("unpinned revision: %v", err)
	}
}

// TestDCXV1520RevisionPathsListsGitSafeNamesUnfiltered pins DCX-V1-520: the
// walk lists every name Git's tree grammar admits, as the per-tree walk it
// replaces did, leaving wire-path policy to the caller's selected paths, so an
// unrelated backslash-containing name does not refuse the listing.
func TestDCXV1520RevisionPathsListsGitSafeNamesUnfiltered(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	integrityGit(t, root, "init", "-q")
	integrityGit(t, root, "config", "user.email", "fixture@example.invalid")
	integrityGit(t, root, "config", "user.name", "Fixture")
	writeFile(t, root, "app/a.rb", "def a\nend\n")
	integrityGit(t, root, "add", "-A")
	blob := strings.TrimSpace(string(integrityGit(t, root, "rev-parse", ":app/a.rb")))
	integrityGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+blob+`,notes\old.txt`)
	integrityGit(t, root, "commit", "-qm", "seed")
	commit := integrityOID(t, root, "HEAD")
	repo, err := Open(root, gitrun.NewBudget(2, gitrun.DefaultTotalBudget))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := repo.RevisionPaths(context.Background(), commit, 1000, nil)
	if err != nil {
		t.Fatalf("RevisionPaths with a backslash-containing name: %v", err)
	}
	if want := lsTreeFiles(t, root, commit); !slices.Equal(paths, want) || !slices.Contains(paths, `notes\old.txt`) {
		t.Fatalf("paths = %q, want %q", paths, want)
	}
}
