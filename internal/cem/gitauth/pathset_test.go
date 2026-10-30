package gitauth

import (
	"context"
	"errors"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func TestDCPV1155ResolvePathsMatchesSingleLookupsWithinOneOpPerLevel(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"sha1", "sha256"} {
		t.Run("DCP-V1-155 DCP-V1-156 "+format, func(t *testing.T) {
			root, commit := pathsRepo(t, format)
			ctx := context.Background()
			paths := []string{"pkg/d-x", "pkg/d/inner.txt", "pkg/d/deeper/x.go", "pkg/d0", "pkg/qé.txt", "pkg/sp ace.txt", "pkg/blob.txt", "other.txt", "pkg/link", "pkg/sub", "pkg/d", "pkg/absent", "pkg/blob.txt/below", "missing/dir/file"}
			single, err := Open(root, gitrun.NewDefaultBudget())
			if err != nil {
				t.Fatal(err)
			}
			// Three directory levels below the root: one pinning op plus one op per level.
			batched, err := Open(root, gitrun.NewBudget(4, gitrun.DefaultTotalBudget))
			if err != nil {
				t.Fatal(err)
			}
			resolved, err := batched.ResolvePaths(ctx, commit, paths)
			if err != nil {
				t.Fatalf("ResolvePaths within four Git operations: %v", err)
			}
			for _, p := range paths {
				want, exists, err := single.LookupTreeEntry(ctx, commit, p)
				if err != nil {
					t.Fatalf("LookupTreeEntry %s: %v", p, err)
				}
				got, ok := resolved[p]
				if ok != exists || got != want {
					t.Fatalf("%s: batched %v %v, single %v %v", p, got, ok, want, exists)
				}
			}
			oids := []string{resolved["pkg/blob.txt"].OID, resolved["pkg/d/deeper/x.go"].OID}
			blobs, err := Open(root, gitrun.NewBudget(2, gitrun.DefaultTotalBudget))
			if err != nil {
				t.Fatal(err)
			}
			sizes, err := blobs.BlobHeaders(ctx, oids)
			if err != nil || sizes[oids[0]] != 2 || sizes[oids[1]] != 2 {
				t.Fatalf("BlobHeaders: %v %v", sizes, err)
			}
			bodies, err := blobs.BlobBatch(ctx, oids, sizes)
			if err != nil || string(bodies[oids[0]]) != "7\n" || string(bodies[oids[1]]) != "3\n" {
				t.Fatalf("BlobBatch: %q %v", bodies, err)
			}
			wrong := map[string]int{oids[0]: 1}
			if _, err := single.BlobBatch(ctx, oids[:1], wrong); err == nil {
				t.Fatal("BlobBatch accepted a body whose size differs from its admitted header")
			}
			var coded *cemcode.Error
			if _, err := blobs.BlobHeaders(ctx, oids); !errors.As(err, &coded) || coded.Code != cemcode.GitBudgetExceeded {
				t.Fatalf("third operation on a two-operation budget: %v", err)
			}
		})
	}
}
