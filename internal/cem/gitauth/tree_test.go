package gitauth

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// pathsRepo commits one directory holding a gitlink, a symlink, nested trees,
// non-ASCII and space names, and the sibling prefixes d-x, d, d0 that ls-tree
// orders differently from a bytewise path sort, plus one path outside it.
func pathsRepo(t *testing.T, format string) (string, string) {
	t.Helper()
	root := t.TempDir()
	integrityGit(t, root, "init", "-q", "--object-format="+format)
	integrityGit(t, root, "config", "user.email", "fixture@example.invalid")
	integrityGit(t, root, "config", "user.name", "Fixture")
	for path, content := range map[string]string{
		"pkg/d-x": "1\n", "pkg/d/inner.txt": "2\n", "pkg/d/deeper/x.go": "3\n", "pkg/d0": "4\n",
		"pkg/qé.txt": "5\n", "pkg/sp ace.txt": "6\n", "pkg/blob.txt": "7\n", "other.txt": "8\n",
	} {
		writeFile(t, root, path, content)
	}
	if err := os.Symlink("d-x", filepath.Join(root, "pkg", "link")); err != nil {
		t.Fatal(err)
	}
	integrityGit(t, root, "add", "-A")
	integrityGit(t, root, "commit", "-qm", "seed")
	seed := integrityOID(t, root, "HEAD")
	integrityGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+seed+",pkg/sub")
	integrityGit(t, root, "commit", "-qm", "paths")
	return root, integrityOID(t, root, "HEAD")
}

// rawTreePaths is the ls-tree read TreePaths used before the verified walk.
func rawTreePaths(t *testing.T, root, commit, dir string) []string {
	t.Helper()
	out := string(integrityGit(t, root, "ls-tree", "-rzt", "--name-only", "--full-tree", commit, "--", ":(literal)"+dir))
	paths := []string{}
	for _, path := range strings.Split(strings.TrimSuffix(out, "\x00"), "\x00") {
		if strings.HasPrefix(path, dir+"/") {
			paths = append(paths, path)
		}
	}
	return paths
}

// The verified listing must equal ls-tree's, in ls-tree's order, for every
// directory shape and both object formats.
func TestTreePathsMatchesLsTreeOrder(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, commit := pathsRepo(t, format)
			repo := open(t, root)
			for _, dir := range []string{"pkg", "pkg/d", "pkg/blob.txt", "pkg/missing"} {
				got, err := repo.TreePaths(context.Background(), commit, dir)
				if want := rawTreePaths(t, root, commit, dir); err != nil || !slices.Equal(got, want) {
					t.Fatalf("%s: %v\n got %q\nwant %q", dir, err, got, want)
				}
			}
			got, _ := repo.TreePaths(context.Background(), commit, "pkg")
			want := []string{"pkg/blob.txt", "pkg/d-x", "pkg/d", "pkg/d/deeper", "pkg/d/deeper/x.go", "pkg/d/inner.txt",
				"pkg/d0", "pkg/link", "pkg/qé.txt", "pkg/sp ace.txt", "pkg/sub"}
			if !slices.Equal(got, want) {
				t.Fatalf("fixture lost a shape:\n got %q\nwant %q", got, want)
			}
		})
	}
}

// Git lists a tree body that does not hash to its name; the verified walk
// refuses it instead of listing a forged path or hiding a dropped one.
func TestTreePathsRefusesMislabeledSubtree(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, commit := pathsRepo(t, format)
			blob, _ := hex.DecodeString(integrityOID(t, root, commit+":pkg/d/inner.txt"))
			corruptLoose(t, root, integrityOID(t, root, commit+":pkg/d"), "tree", append([]byte("100644 forged.txt\x00"), blob...))
			if raw := rawTreePaths(t, root, commit, "pkg"); !slices.Equal(raw, []string{"pkg/blob.txt", "pkg/d-x", "pkg/d",
				"pkg/d/forged.txt", "pkg/d0", "pkg/link", "pkg/qé.txt", "pkg/sp ace.txt", "pkg/sub"}) {
				t.Fatalf("fixture Git did not list the forged body: %q", raw)
			}
			got, err := open(t, root).TreePaths(context.Background(), commit, "pkg")
			if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || got != nil {
				t.Fatalf("forged subtree: %q %v", got, err)
			}
		})
	}
}

// The walk lists exactly MaxTreeDepth levels beneath the directory and
// refuses one more, and refuses tree bodies totalling more than MaxTreeBytes
// even when every level fits its own batch.
func TestTreePathsBoundsDepthAndBytes(t *testing.T) {
	root, base, _ := makeRepo(t)
	repo := open(t, root)
	within := deepTarget(t, root, base, MaxTreeDepth-1)
	got, err := repo.TreePaths(context.Background(), within, "deep")
	if last := "deep/" + strings.Repeat("d/", MaxTreeDepth-1) + "f.go"; err != nil || len(got) != MaxTreeDepth || got[MaxTreeDepth-1] != last {
		t.Fatalf("depth %d: %d paths %v", MaxTreeDepth, len(got), err)
	}
	beyond := deepTarget(t, root, base, MaxTreeDepth)
	if got, err = repo.TreePaths(context.Background(), beyond, "deep"); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || got != nil {
		t.Fatalf("depth %d: %q %v", MaxTreeDepth+1, got, err)
	}
	if got, err = repo.TreePaths(context.Background(), wideTarget(t, root, base), "big"); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || got != nil {
		t.Fatalf("bytes: %d paths %v", len(got), err)
	}
}

// wideTarget commits big/, two nested trees that each fit MaxTreeBytes but
// together exceed it.
func wideTarget(t *testing.T, root, base string) string {
	raw, _ := hex.DecodeString(integrityOID(t, root, base+":f.go"))
	var body bytes.Buffer
	for body.Len() <= MaxTreeBytes/2 {
		fmt.Fprintf(&body, "100644 f%07d\x00%s", body.Len(), raw)
	}
	inner, _ := hex.DecodeString(writeLooseTree(t, root, body.Bytes()))
	outer := writeLooseTree(t, root, append(append(body.Bytes(), "40000 sub\x00"...), inner...))
	entries := string(integrityGit(t, root, "ls-tree", base)) + "040000 tree " + outer + "\tbig\n"
	tree := gitStdin(t, root, entries, "mktree")
	return gitStdin(t, root, "", "commit-tree", tree, "-p", base, "-m", "wide")
}
