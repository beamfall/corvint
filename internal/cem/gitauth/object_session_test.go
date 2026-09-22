package gitauth

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// An object session only changes how many children run: inside a scope every
// resolve, tree, blob and diff read returns the one-shot bytes and error and
// charges the same budget operations, for present, absent, wrong-type and
// mislabeled objects.
func TestObjectSessionMatchesOneShotReads(t *testing.T) {
	outcomes := func(scoped bool) []string {
		root, base, target := makeRepo(t)
		budget := gitrun.NewDefaultBudget()
		repo, err := Open(root, budget)
		if err != nil {
			t.Fatal(err)
		}
		if scoped {
			defer repo.BeginObjectSession()()
		}
		ctx := context.Background()
		var got []string
		record := func(value any, err error) {
			got = append(got, fmt.Sprintf("%v|%s|%v", value, cemcode.CodeOf(err), err))
		}
		for _, path := range []string{"docs/rule.txt", "docs/none", "f.go/x"} {
			entry, exists, err := repo.LookupTreeEntry(ctx, target, path)
			record(fmt.Sprint(entry, exists), err)
		}
		file := integrityOID(t, root, target+":f.go")
		for _, revision := range []string{target, "HEAD", integrityOID(t, root, target+"^{tree}"), file, strings.Repeat("0", len(file))} {
			oid, err := repo.Resolve(ctx, revision)
			record(oid, err)
		}
		for _, oid := range []string{file, target, strings.Repeat("0", len(file))} {
			raw, err := repo.BlobBytes(ctx, oid)
			record(string(raw), err)
		}
		patch, err := repo.CanonicalDiff(ctx, base, target)
		record(string(patch), err)
		corruptLoose(t, root, file, "blob", []byte(strings.Replace(bodyV2, "h := 9", "h := 0", 1)))
		patch, err = repo.CanonicalDiff(ctx, base, target)
		record(string(patch), err)
		corruptLoose(t, root, integrityOID(t, root, target+":docs"), "tree", []byte("forged"))
		entry, exists, err := repo.LookupTreeEntry(ctx, target, "docs/rule.txt")
		record(fmt.Sprint(entry, exists), err)
		corruptLoose(t, root, target, "commit", append(integrityGit(t, root, "cat-file", "commit", target), "forged\n"...))
		oid, err := repo.Resolve(ctx, target)
		record(oid, err)
		remaining := 0
		for ; ; remaining++ {
			if _, err := budget.ReserveOperation(0); err != nil {
				break
			}
		}
		return append(got, fmt.Sprint("remaining operations ", remaining))
	}
	oneShot, scoped := outcomes(false), outcomes(true)
	if !slices.Equal(oneShot, scoped) {
		t.Fatalf("scoped reads differ\none-shot: %q\nscoped:   %q", oneShot, scoped)
	}
}
