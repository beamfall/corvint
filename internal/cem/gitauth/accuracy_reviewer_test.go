package gitauth

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func TestAccuracyWholeTreePublicReadersAgree(t *testing.T) {
	t.Run("CEM-CB-023 complete tree validation refuses before memoization", func(t *testing.T) {
		testAccuracyWholeTreePublicReadersAgree(t)
	})
}

func testAccuracyWholeTreePublicReadersAgree(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			gitCmd(t, root, "init", "-q", "--object-format="+format, "-b", "main")
			gitCmd(t, root, "config", "user.name", "fixture")
			gitCmd(t, root, "config", "user.email", "fixture@example.invalid")
			writeFile(t, root, "a", "payload\n")
			gitCmd(t, root, "add", "a")
			gitCmd(t, root, "commit", "-qm", "seed")
			blobOID := gitCmd(t, root, "rev-parse", "HEAD:a")
			blob, err := hex.DecodeString(blobOID)
			if err != nil {
				t.Fatal(err)
			}
			entry := func(name string) []byte { return append([]byte("100644 "+name+"\x00"), blob...) }
			storeTree := func(raw []byte) string {
				t.Helper()
				object := append([]byte(fmt.Sprintf("tree %d\x00", len(raw))), raw...)
				oid := hex.EncodeToString(integrityHash(len(blobOID), object))
				corruptLoose(t, root, oid, "tree", raw)
				return oid
			}
			for _, test := range []struct {
				name   string
				suffix []byte
				target string
				valid  bool
			}{
				{"valid", nil, "a", true},
				{"duplicate-target", entry("a"), "a", false},
				{"duplicate-other", append(entry("b"), entry("b")...), "a", false},
				{"malformed-tail", []byte("100644 truncated\x00\x00"), "a", false},
				{"missing-target-malformed-tail", []byte("100644 truncated\x00\x00"), "z", false},
			} {
				t.Run(test.name, func(t *testing.T) {
					raw := append(entry("a"), test.suffix...)
					child := storeTree(raw)
					childBytes, err := hex.DecodeString(child)
					if err != nil {
						t.Fatal(err)
					}
					tree := storeTree(append([]byte("40000 docs\x00"), childBytes...))
					immutable := open(t, root)
					if err := immutable.LoadObjectFormat(context.Background()); err != nil {
						t.Fatal(err)
					}
					memo, err := NewRequestReadMemo(immutable)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(memo.Release)
					repo, err := memo.Open(gitrun.NewDefaultBudget())
					if err != nil {
						t.Fatal(err)
					}
					got, exists, lookupErr := repo.LookupTreeEntry(context.Background(), tree, "docs/"+test.target)
					paths, walkErr := repo.TreePaths(context.Background(), tree, "docs")
					t.Logf("tree=%s lookupExists=%t lookupCode=%s lookupOID=%s walkCode=%s walkPaths=%v", tree, exists, cemcode.CodeOf(lookupErr), got.OID, cemcode.CodeOf(walkErr), paths)
					if test.valid {
						if lookupErr != nil || !exists || got.OID != blobOID || walkErr != nil || len(paths) != 1 || paths[0] != "docs/a" {
							t.Fatalf("valid control failed: lookup=%v walk=%v", lookupErr, walkErr)
						}
					} else if lookupErr == nil || exists || cemcode.CodeOf(lookupErr) != cemcode.RepositoryObjectUnavailable || cemcode.CodeOf(walkErr) != cemcode.RepositoryObjectUnavailable {
						t.Fatalf("malformed whole tree must be refused by both public readers: lookup=%v walk=%v", lookupErr, walkErr)
					} else if got != (TreeEntry{}) || memo.count != 0 {
						t.Fatalf("malformed tree returned entry or populated memo: entry=%+v memoEntries=%d", got, memo.count)
					}
				})
			}
		})
	}
}
