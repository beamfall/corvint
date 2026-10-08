//go:build darwin || linux

package gitauth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestPromisorObjectNeverFetchedByAGitThatDropsTheLazyFetchGuard: V1-0349. The cem, ocm and
// frontier reads go through Open, whose default Git may ignore GIT_NO_LAZY_FETCH (Git before 2.46
// does for a diff's blob prefetch). The empty GIT_ALLOW_PROTOCOL must still keep a missing
// promisor blob from being fetched, so the read is refused as repository-object-unavailable, and a
// diff that needs the blob is refused with the same code rather than git-diff-failed. The
// promisor source stays present, so a fetch would succeed; the remote's upload-pack touches a
// sentinel first, so an attempt leaves it. It sets PATH, so it is not parallel.
func TestPromisorObjectNeverFetchedByAGitThatDropsTheLazyFetchGuard(t *testing.T) {
	root, base, target := makeRepo(t)
	partial, sentinel := promisorClone(t, root)
	shim := guardDroppingGit(t)
	for _, dropsGuard := range []bool{false, true} {
		t.Run(map[bool]string{false: "guarded", true: "drops-guard"}[dropsGuard], func(t *testing.T) {
			if dropsGuard {
				t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			ctx := context.Background()
			// The diff needs the base f.go blob, which the clone never fetched. A failed diff
			// read is a missing object (CEM-CB-019), not a patch-derivation failure.
			_, diffErr := open(t, partial).CanonicalDiff(ctx, base, target)
			_, _, createErr := open(t, partial).CanonicalDiffWithCreateDestinations(ctx, base, target)
			repository := open(t, partial)
			promised, exists, err := repository.LookupTreeEntry(ctx, base, "f.go")
			if err != nil || !exists {
				t.Fatalf("base tree entry: exists=%v err=%v", exists, err)
			}
			_, blobErr := repository.BlobBytes(ctx, promised.OID)
			if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
				t.Fatalf("a gitauth read reached the promisor remote (sentinel stat: %v)", statErr)
			}
			for name, err := range map[string]error{"CanonicalDiff": diffErr, "CanonicalDiffWithCreateDestinations": createErr, "BlobBytes": blobErr} {
				if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
					t.Errorf("%s over a promised-missing blob: got %v, want repository-object-unavailable without fetch", name, err)
				}
			}
		})
	}
}

// promisorClone makes a blob:none clone of root whose upload-pack touches the returned sentinel
// before serving, so any fetch attempt leaves it. The source stays present, so a fetch would
// succeed.
func promisorClone(t *testing.T, root string) (string, string) {
	t.Helper()
	gitCmd(t, root, "config", "uploadpack.allowfilter", "true")
	partial := filepath.Join(t.TempDir(), "partial")
	gitCmd(t, filepath.Dir(partial), "clone", "-q", "--no-local", "--filter=blob:none", "file://"+root, partial)
	partial, err := filepath.EvalSymlinks(partial)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(t.TempDir(), "fetch-attempted")
	gitCmd(t, partial, "config", "remote.origin.uploadpack", "touch '"+sentinel+"' && git-upload-pack")
	return partial, sentinel
}

// guardDroppingGit returns a directory holding a git that unsets GIT_NO_LAZY_FETCH, as Git before
// 2.46 ignores it.
func guardDroppingGit(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	script := "#!/bin/sh\nunset GIT_NO_LAZY_FETCH\nexec '" + strings.ReplaceAll(realGit, "'", `'\''`) + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return shim
}

// TestCanonicalDiffNamesAMissingPromisedBlobAmongManySharedOnes: V1-0349. When the batch read
// exits, each distinct blob is probed once, so many paths sharing one present blob cannot exhaust
// the operation budget before the missing base blob is named. It sets PATH, so it is not parallel.
func TestCanonicalDiffNamesAMissingPromisedBlobAmongManySharedOnes(t *testing.T) {
	root, base, _ := makeRepo(t)
	for i := range 1100 {
		writeFile(t, root, filepath.Join("a", fmt.Sprintf("%04d.txt", i)), "shared content\n")
	}
	writeFile(t, root, "f.go", "package f\n\n// changed again\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "many shared blobs")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	partial, sentinel := promisorClone(t, root)
	t.Setenv("PATH", guardDroppingGit(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := open(t, partial).CanonicalDiff(context.Background(), base, target)
	if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
		t.Fatalf("the diff reached the promisor remote (sentinel stat: %v)", statErr)
	}
	if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("got %v, want repository-object-unavailable naming the missing base blob", err)
	}
}

// TestDiffExitClassificationKeepsCancellation: V1-0349. A cancellation during the missing-object
// walk is returned as such, not hidden behind the diff's git-diff-failed.
func TestDiffExitClassificationKeepsCancellation(t *testing.T) {
	root, base, target := makeRepo(t)
	repository := open(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := repository.diffExitError(ctx, cemcode.New(cemcode.GitExitFailure, "diff exited"), base, target)
	if cemcode.CodeOf(err) != cemcode.GitCancelled {
		t.Fatalf("got %v, want git-cancelled", err)
	}
}
