package contextindex

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// ERI-V0-060: a repository whose remote is a promisor only through
// remote.<name>.promisor or remote.<name>.partialclonefilter may lack the base
// blobs the exact diff reads. The over-bound shortcut must not replace the
// repository-object-unavailable refusal and its recovery with the bound refusal.
func TestRangeImpactOverBoundKeepsMissingObjectRefusalForPromisorRemotes(t *testing.T) {
	for _, key := range []string{"remote.origin.promisor", "remote.origin.partialclonefilter"} {
		t.Run(key, func(t *testing.T) {
			source := impactRepository(t, "")
			for item := 0; item < 101; item++ {
				writeTestFile(t, source, fmt.Sprintf("before/%03d.txt", item), fmt.Sprintf("base member %d\n%s", item, strings.Repeat("shared body line\n", 8)))
			}
			testGit(t, source, "add", ".")
			testGit(t, source, "commit", "-qm", "base members")
			base := testGit(t, source, "rev-parse", "HEAD")
			testGit(t, source, "rm", "-rq", "before")
			for item := 0; item < 101; item++ {
				writeTestFile(t, source, fmt.Sprintf("after/%03d.txt", item), fmt.Sprintf("head member %d\n%s", item, strings.Repeat("shared body line\n", 8)))
			}
			testGit(t, source, "add", ".")
			testGit(t, source, "commit", "-qm", "move members")
			testGit(t, source, "config", "uploadpack.allowFilter", "true")
			root := filepath.Join(t.TempDir(), "clone")
			testGit(t, t.TempDir(), "clone", "-q", "--filter=blob:none", "file://"+source, root)
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			// Keep only the named promisor key and make lazy fetches fail, so
			// the base blobs stay missing.
			testGitOptional(t, root, "config", "--unset", "extensions.partialclone")
			if key == "remote.origin.promisor" {
				testGit(t, root, "config", "--unset", "remote.origin.partialclonefilter")
			} else {
				testGit(t, root, "config", "--unset", "remote.origin.promisor")
			}
			testGit(t, root, "config", "remote.origin.url", "file://"+filepath.Join(t.TempDir(), "gone"))
			calls := 0
			readRangeChangesFor = func(ctx context.Context, index *Index, baseTree string) ([]rangeChange, error) {
				calls++
				return readRangeChanges(ctx, index, baseTree)
			}
			t.Cleanup(func() { readRangeChangesFor = readRangeChanges })
			_, err = RangeImpact(context.Background(), index, base, 20)
			var refusal *Error
			if calls != 1 || !errors.As(err, &refusal) || refusal.Code != "repository-object-unavailable" {
				t.Fatalf("promisor remote via %s: error %v, exact diff calls %d; want repository-object-unavailable from the exact diff", key, err, calls)
			}
		})
	}
}
