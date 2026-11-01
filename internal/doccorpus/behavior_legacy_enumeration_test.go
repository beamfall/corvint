package doccorpus

import (
	"context"
	"fmt"
	"io/fs"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
)

const legacyEnumerationSpec = "describe('suite', () => {\n  it('works', () => {\n    expect(1).toBe(1);\n  });\n});\n"

// TestDCXV1810LegacyDiscoveryEnumeratesMoreThan1024Directories pins DCX-V1-810 (V1-2560): legacy
// discovery over a revision with more directories below its root than the default 1,024-operation
// Git budget succeeds under that budget and under the small phase bound, as extraction does
// (DCX-V1-520), because it lists the revision one verified batch per directory level.
func TestDCXV1810LegacyDiscoveryEnumeratesMoreThan1024Directories(t *testing.T) {
	t.Parallel()
	const dirs = 1100
	files := map[string]string{}
	for i := 0; i < dirs; i++ {
		files[fmt.Sprintf("e2e/d%04d/helper.js", i)] = "module.exports = 1;\n"
		files[fmt.Sprintf("vendor/v%04d/lib.spec.js", i)] = legacyEnumerationSpec
	}
	files["e2e/d0007/login.spec.js"] = legacyEnumerationSpec
	files["e2e/d1099/nested/deep/cart.test.js"] = legacyEnumerationSpec
	files["e2e/node_modules/pkg/skip.spec.js"] = legacyEnumerationSpec
	root := t.TempDir()
	revision := writeBulkFixture(t, root, files)
	for name, ctx := range map[string]context.Context{
		"default budget": context.Background(),
		"phase bound":    boundCorpusGitOps(context.Background(), phaseGitOps),
	} {
		inventory, _, err := legacyInventoryContext(ctx, t, root, "jasmine", revision, "e2e", nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var paths []string
		for _, record := range inventory.Legacy {
			if !slices.Contains(paths, record.Evidence.Path) {
				paths = append(paths, record.Evidence.Path)
			}
		}
		if want := []string{"e2e/d0007/login.spec.js", "e2e/d1099/nested/deep/cart.test.js"}; !slices.Equal(paths, want) {
			t.Fatalf("%s: inventoried %v, want %v", name, paths, want)
		}
	}
}

// TestDCXV1811LegacyPathsMatchPerTreeWalk pins DCX-V1-811: the level-batched listing returns the
// test files of the per-tree fs.WalkDir it replaces, in the same order, for roots whose names sort
// differently by segment than as whole strings (`a.spec.js` against `a/b.spec.js`), with
// node_modules skipped and a non-regular test-named entry kept for the read to refuse.
func TestDCXV1811LegacyPathsMatchPerTreeWalk(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"t/a.spec.js": legacyEnumerationSpec, "t/a/b.spec.js": legacyEnumerationSpec, "t/a-b.test.ts": legacyEnumerationSpec,
		"t/a/c/d_test.js": legacyEnumerationSpec, "t/ab.cy.js": legacyEnumerationSpec, "t/node_modules/x.spec.js": legacyEnumerationSpec,
		"t/z/node_modules/y.spec.js": legacyEnumerationSpec, "t/readme.md": "x\n", "other/o.spec.js": legacyEnumerationSpec,
		"top.spec.js": legacyEnumerationSpec, "t.spec.js": legacyEnumerationSpec,
	}
	root := t.TempDir()
	revision := writeBulkFixture(t, root, files)
	ctx := context.Background()
	auth, err := gitauth.Open(root, newGitBudget(ctx))
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{".", "t", "t/a", "t/a/c"} {
		got, err := legacyPaths(ctx, auth, revision, scope)
		if err != nil {
			t.Fatalf("%s: %v", scope, err)
		}
		tree, err := auth.RevisionFS(ctx, revision, MaxBytes)
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		if err := fs.WalkDir(tree, scope, func(current string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "node_modules" {
					return fs.SkipDir
				}
				return nil
			}
			if legacyTestPath(current) {
				want = append(want, current)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("%s: legacyPaths %v, per-tree walk %v", scope, got, want)
		}
	}
}
