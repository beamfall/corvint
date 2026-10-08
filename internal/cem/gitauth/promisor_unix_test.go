//go:build darwin || linux

package gitauth

import (
	"context"
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
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root, base, target := makeRepo(t)
	gitCmd(t, root, "config", "uploadpack.allowfilter", "true")
	partial := filepath.Join(t.TempDir(), "partial")
	gitCmd(t, filepath.Dir(partial), "clone", "-q", "--no-local", "--filter=blob:none", "file://"+root, partial)
	partial, err = filepath.EvalSymlinks(partial)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(t.TempDir(), "fetch-attempted")
	gitCmd(t, partial, "config", "remote.origin.uploadpack", "touch '"+sentinel+"' && git-upload-pack")
	shim := t.TempDir()
	script := "#!/bin/sh\nunset GIT_NO_LAZY_FETCH\nexec '" + strings.ReplaceAll(realGit, "'", `'\''`) + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
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
