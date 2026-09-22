package gitauth

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func objectViewFixture(t *testing.T, format string) (string, string, string, string) {
	t.Helper()
	root := t.TempDir()
	gitCmd(t, root, "init", "-q", "--object-format="+format)
	gitCmd(t, root, "config", "user.email", "fixture@example.invalid")
	gitCmd(t, root, "config", "user.name", "Fixture")
	if err := os.Mkdir(filepath.Join(root, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs/rule.txt"), []byte("stable rule\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	view := filepath.Join(t.TempDir(), "view")
	gitCmd(t, root, "clone", "-q", "--no-hardlinks", root, view)
	return root, view, base, target
}
func TestObjectViewIsolatesLiveCorruptionAndFinalRawObservation(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, view, base, target := objectViewFixture(t, format)
			repo, immutable := open(t, root), open(t, view)
			ctx := context.Background()
			if err := repo.UseObjectView(ctx, immutable); err != nil {
				t.Fatal(err)
			}
			patch, err := immutable.CanonicalDiff(ctx, base, target)
			if err != nil {
				t.Fatal(err)
			}
			nested := integrityOID(t, root, target+":docs")
			nestedBody := integrityGit(t, root, "cat-file", "tree", nested)
			original := integrityOID(t, root, target+":docs/rule.txt")
			replacement := integrityOID(t, root, target+":file")
			from, _ := hex.DecodeString(original)
			to, _ := hex.DecodeString(replacement)
			corruptLoose(t, root, nested, "tree", bytes.Replace(nestedBody, from, to, 1))
			for _, revision := range []string{base, target} {
				oid := integrityOID(t, view, revision+":file")
				corruptLoose(t, root, oid, "blob", []byte("forged\n"))
			}
			actual, err := immutable.CanonicalDiff(ctx, base, target)
			if err != nil || !bytes.Equal(actual, patch) {
				t.Fatalf("live corruption influenced view: %v", err)
			}
			entry, ok, err := immutable.LookupTreeEntry(ctx, target, "docs/rule.txt")
			if err != nil || !ok || entry.OID != original {
				t.Fatalf("live tree influenced view: %+v %v", entry, err)
			}
			if err := repo.RequireCleanTarget(ctx, target); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "file"), []byte("dirty at entry\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := repo.RequireTargetState(ctx, target); err != nil {
				t.Fatal("entry raw bytes influenced metadata guard", err)
			}
			if err := repo.RequireCleanTarget(ctx, target); err == nil {
				t.Fatal("dirty final raw bytes accepted")
			}
			if err := os.WriteFile(filepath.Join(root, "file"), []byte("new\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := repo.RequireCleanTarget(ctx, target); err != nil {
				t.Fatal("repaired final target refused", err)
			}
		})
	}
}
func TestObjectViewMissingObjectsNeverFallBackToLive(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		for _, kind := range []string{"commit", "nested-tree", "new-blob", "old-diff-blob"} {
			t.Run(format+"/"+kind, func(t *testing.T) {
				root, view, base, target := objectViewFixture(t, format)
				repo, immutable := open(t, root), open(t, view)
				ctx := context.Background()
				if err := repo.UseObjectView(ctx, immutable); err != nil {
					t.Fatal(err)
				}
				revision := target
				switch kind {
				case "nested-tree":
					revision = target + ":docs"
				case "new-blob":
					revision = target + ":file"
				case "old-diff-blob":
					revision = base + ":file"
				}
				oid := integrityOID(t, root, revision)
				p := filepath.Join(view, ".git/objects", oid[:2], oid[2:])
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				var err error
				switch kind {
				case "commit":
					_, err = repo.Resolve(ctx, "HEAD")
				case "nested-tree":
					err = repo.RequireCleanTarget(ctx, target)
				case "new-blob":
					_, err = immutable.BlobBytes(ctx, oid)
				case "old-diff-blob":
					_, err = immutable.CanonicalDiff(ctx, base, target)
				}
				if err == nil {
					t.Fatal("missing protected object fell back to live")
				}
			})
		}
	}
}
func TestObjectViewKeepsIndexUntrackedAndIgnoreLive(t *testing.T) {
	root, view, _, target := objectViewFixture(t, "sha1")
	repo, immutable := open(t, root), open(t, view)
	ctx := context.Background()
	if err := repo.UseObjectView(ctx, immutable); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := repo.RequireTargetState(ctx, target); err == nil {
		t.Fatal("untracked hidden")
	}
	if err := os.MkdirAll(filepath.Join(root, ".git/info"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git/info/exclude"), []byte("untracked\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := repo.RequireTargetState(ctx, target); err != nil {
		t.Fatal("live ignore ignored", err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("staged\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", "file")
	if err := repo.RequireTargetState(ctx, target); err == nil {
		t.Fatal("live index hidden")
	}
}
func TestAuthorityParentEvictionPreservesAllEdges(t *testing.T) {
	root := t.TempDir()
	paths := []string{}
	for i := 0; i < 90; i++ {
		p := fmt.Sprintf("d%03d/nested", i)
		if err := os.MkdirAll(filepath.Join(root, p), 0700); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	deep := strings.Repeat("deep/", 45) + "last"
	if err := os.MkdirAll(filepath.Join(root, deep), 0700); err != nil {
		t.Fatal(err)
	}
	paths = append(paths, deep)
	held, err := openAuthorityRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	info, err := held.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	parents := newAuthorityParents(context.Background(), root, held, info)
	defer parents.close()
	for _, p := range paths {
		n, err := parents.get(p)
		if err != nil {
			t.Fatal(err)
		}
		parents.release(n)
		if len(parents.cache) > authorityParentHandles {
			t.Fatal("descriptor ceiling")
		}
	}
	if err := parents.check(); err != nil {
		t.Fatal(err)
	}
	// Replace an evicted early directory with identical names and metadata shape.
	early := filepath.Join(root, paths[0])
	if err := os.Rename(early, early+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(early, 0700); err != nil {
		t.Fatal(err)
	}
	if err := parents.check(); err == nil {
		t.Fatal("eviction hid namespace replacement")
	}
}

func TestObjectViewScrubsCallerAlternatesAndObjectDirectory(t *testing.T) {
	root, view, _, target := objectViewFixture(t, "sha1")
	repo, immutable := open(t, root), open(t, view)
	ctx := context.Background()
	if err := repo.UseObjectView(ctx, immutable); err != nil {
		t.Fatal(err)
	}
	oid := integrityOID(t, root, target+":file")
	if err := os.Remove(filepath.Join(view, ".git/objects", oid[:2], oid[2:])); err != nil {
		t.Fatal(err)
	}
	// Set hostile ambient redirects after construction so this exercises the
	// operation environment as well as Open's independent alternates refusal.
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", filepath.Join(root, ".git/objects"))
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(root, ".git/objects"))
	t.Setenv("GIT_COMMON_DIR", filepath.Join(root, ".git"))
	t.Setenv("GIT_DIR", filepath.Join(root, ".git"))
	if _, err := immutable.BlobBytes(ctx, oid); err == nil {
		t.Fatal("ambient object fallback")
	}
	if err := repo.RequireTargetState(ctx, target); err != nil {
		t.Fatal("caller redirection changed valid live state", err)
	}
}

func TestObjectViewFormatMismatchRefuses(t *testing.T) {
	root, _, _, _ := objectViewFixture(t, "sha1")
	_, view, _, _ := objectViewFixture(t, "sha256")
	if err := open(t, root).UseObjectView(context.Background(), open(t, view)); err == nil {
		t.Fatal("mixed object formats")
	}
}

func TestAuthorityParentWitnessesCanExceedLeafCount(t *testing.T) {
	root := t.TempDir()
	const count = 4097
	for i := 0; i < count; i++ {
		if err := os.MkdirAll(filepath.Join(root, fmt.Sprintf("p%04d/nested", i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	held, err := openAuthorityRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	info, err := held.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	parents := newAuthorityParents(context.Background(), root, held, info)
	defer parents.close()
	for i := 0; i < count; i++ {
		n, err := parents.get(fmt.Sprintf("p%04d/nested", i))
		if err != nil {
			t.Fatal(err)
		}
		parents.release(n)
	}
	if len(parents.observed) <= 8192 {
		t.Fatal("fixture missed expanded-parent boundary")
	}
	if err := parents.check(); err != nil {
		t.Fatal(err)
	}
}
