// SPDX-License-Identifier: Apache-2.0

package appmap

import (
	"os"
	"path/filepath"
	"testing"
)

// AMAP-V0-9501: fixture reuse preserves the committed tree while every mutable
// file, including the index and loose objects, belongs to its test alone.
func TestAMAPV09501FixtureCopiesAreIndependent(t *testing.T) {
	t.Parallel()
	seed, revision := fixtureSeed(t, "testdata/fixture")
	for range 3 {
		t.Run("copy", func(t *testing.T) {
			t.Parallel()
			root, rev := fixtureRepo(t)
			if rev != revision || gitTest(t, root, "status", "--porcelain") != "" {
				t.Fatal("copy changed fixture revision or checkout")
			}
			for _, path := range []string{"appmap.json", ".git/index", filepath.Join(".git/objects", rev[:2], rev[2:])} {
				a, err := os.Stat(filepath.Join(seed, path))
				if err != nil {
					t.Fatal(err)
				}
				b, err := os.Stat(filepath.Join(root, path))
				if err != nil {
					t.Fatal(err)
				}
				if os.SameFile(a, b) {
					t.Fatalf("shared mutable file: %s", path)
				}
			}
			if _, err := os.Lstat(filepath.Join(root, ".git/objects/info/alternates")); !os.IsNotExist(err) {
				t.Fatalf("copy borrows objects: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, "appmap.json"), []byte("changed\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if commitAll(t, root, "private edit") == revision {
				t.Fatal("private edit did not change revision")
			}
			other, rev := fixtureRepo(t)
			if rev != revision || gitTest(t, other, "rev-parse", "HEAD") != revision || gitTest(t, other, "status", "--porcelain") != "" {
				t.Fatal("private edit contaminated another copy or the seed")
			}
		})
	}
}
