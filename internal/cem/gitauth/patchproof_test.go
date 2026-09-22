package gitauth

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// shapeRepo commits every section shape the canonical patch can carry:
// create, delete, modify, mode-only, executable, symlink and gitlink
// typechanges, gitlink change, binary (with a space in its name), empty
// file, no final newline, nested and deleted directory, directory-to-blob
// typechange and its blob-to-directory mirror, gitlink create, a quoted (tab)
// path, non-ASCII and space paths, and a change under the excluded sidecar
// path.
func shapeRepo(t *testing.T, format string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	integrityGit(t, root, "init", "-q", "--object-format="+format)
	integrityGit(t, root, "config", "user.email", "fixture@example.invalid")
	integrityGit(t, root, "config", "user.name", "Fixture")
	files := map[string]string{
		"f.txt": "one\ntwo\n", "keep.txt": "keep\n", "d/inner.txt": "x\n", "nonl.txt": "nonl",
		"bi n.dat": "a\x00b", "m.sh": "mode\n", "empty.txt": "", "sp ace.txt": "v1\n",
		"qé.txt": "v1\n", "t\tab.txt": "tab\n", ".corvint/change.cem.json": "{}\n", "b2t.txt": "file\n",
	}
	for path, content := range files {
		writeFile(t, root, path, content)
	}
	if err := os.Symlink("f.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	integrityGit(t, root, "add", "-A")
	integrityGit(t, root, "commit", "-qm", "seed")
	seed := integrityOID(t, root, "HEAD")
	integrityGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+seed+",sub")
	integrityGit(t, root, "commit", "-qm", "base")
	base := integrityOID(t, root, "HEAD")
	files = map[string]string{
		"f.txt": "one\nthree\n", "nonl.txt": "nonl2", "bi n.dat": "a\x00c", "empty.txt": "e\n",
		"empty2.txt": "", "sp ace.txt": "v2\n", "qé.txt": "v2\n", "t\tab.txt": "tab2\n",
		"link": "now file\n", "d": "blob\n", ".corvint/change.cem.json": "{\"x\":1}\n", "b2t.txt/inner.txt": "dir\n",
	}
	for _, path := range []string{"keep.txt", "d", "link", "b2t.txt"} {
		if err := os.RemoveAll(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range files {
		writeFile(t, root, path, content)
	}
	if err := os.Chmod(filepath.Join(root, "m.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	integrityGit(t, root, "add", "-A")
	integrityGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+base+",sub", "--cacheinfo", "160000,"+seed+",sub2")
	integrityGit(t, root, "commit", "-qm", "target")
	return root, base, integrityOID(t, root, "HEAD")
}

func rawCanonicalDiff(t *testing.T, root, base, target string) []byte {
	t.Helper()
	return integrityGit(t, root, "-c", "core.quotePath=false", "diff", "--full-index", "--no-color",
		"--no-ext-diff", "--no-textconv", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
		"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/", "--ignore-submodules=none",
		base, target, "--", ".", ":(exclude).corvint/change.cem.json")
}

// Every honest section shape must pass the proof byte for byte, in both
// object formats.
func TestCanonicalDiffProvesEveryHonestShape(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, base, target := shapeRepo(t, format)
			want := rawCanonicalDiff(t, root, base, target)
			got, err := open(t, root).CanonicalDiff(context.Background(), base, target)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("canonical diff %v\n--- got ---\n%s\n--- want ---\n%s", err, got, want)
			}
			for _, shape := range []string{"new file mode 100644\n", "deleted file mode 120000\n", "old mode 100644\nnew mode 100755\n",
				"Binary files a/bi n.dat and b/bi n.dat differ\n", " 160000\n", "\\ No newline at end of file\n", "--- a/sp ace.txt\t\n",
				"diff --git \"a/t\\tab.txt\" \"b/t\\tab.txt\"\n", "diff --git a/qé.txt b/qé.txt\n", "diff --git a/d b/d\n", "diff --git a/d/inner.txt b/d/inner.txt\n",
				"diff --git a/b2t.txt b/b2t.txt\n", "diff --git a/b2t.txt/inner.txt b/b2t.txt/inner.txt\n", "new file mode 160000\n"} {
				if !bytes.Contains(got, []byte(shape)) {
					t.Errorf("fixture lost shape %q", shape)
				}
			}
			if bytes.Contains(got, []byte("change.cem.json")) {
				t.Error("excluded path leaked into the patch")
			}
		})
	}
}

// A patch that is not the derivation of the verified objects refuses, whether
// a hunk byte, a header, a marker, or a whole section disagrees.
func TestRequirePatchProvenanceRefusesDoctoredPatch(t *testing.T) {
	root, base, target := shapeRepo(t, "sha1")
	repo := open(t, root)
	honest := rawCanonicalDiff(t, root, base, target)
	changed, err := repo.verifiedChangeSet(context.Background(), base, target)
	if err != nil || repo.requirePatchProvenance(context.Background(), honest, changed) != nil {
		t.Fatalf("honest patch refused: %v", err)
	}
	doctored := map[string][]byte{
		"added line":     bytes.Replace(honest, []byte("+three\n"), []byte("+threx\n"), 1),
		"context line":   bytes.Replace(honest, []byte(" one\n"), []byte(" onE\n"), 1),
		"mode line":      bytes.Replace(honest, []byte("new mode 100755\n"), []byte("new mode 100644\n"), 1),
		"newline marker": bytes.Replace(honest, []byte("\\ No newline at end of file\n"), nil, 1),
		"hunk header":    bytes.Replace(honest, []byte("@@ -1,2 +1,2 @@"), []byte("@@ -1,2 +2,2 @@"), 1),
		"truncated":      honest[:len(honest)-1],
		"extra section":  append(bytes.Clone(honest), honest[:bytes.Index(honest, []byte("\ndiff --git"))+1]...),
		"binary as text": bytes.Replace(honest, []byte("Binary files a/bi n.dat and b/bi n.dat differ\n"), []byte("--- a/bi n.dat\t\n+++ b/bi n.dat\t\n@@ -1 +1 @@\n-a\x00b\n+a\x00c\n"), 1),
	}
	for name, patch := range doctored {
		if bytes.Equal(patch, honest) {
			t.Fatalf("%s: doctoring did not apply", name)
		}
		err := repo.requirePatchProvenance(context.Background(), patch, changed)
		t.Logf("%s: %v", name, err)
		if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
			t.Errorf("%s: doctored patch accepted: %v", name, err)
		}
	}
}

// An intermediate target tree replaced under its OID, so that Git's diff no
// longer sees one changed entry, must refuse rather than yield the narrower
// patch.
func TestCanonicalDiffRefusesMislabeledIntermediateTree(t *testing.T) {
	root, base, target := makeRepo(t)
	writeFile(t, root, "docs/rule.txt", "the rule changed\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "docs change")
	target = gitCmd(t, root, "rev-parse", "HEAD")
	honest, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if err != nil || !bytes.Contains(honest, []byte("+the rule changed\n")) {
		t.Fatalf("honest diff %q: %v", honest, err)
	}
	tree := integrityOID(t, root, target+":docs")
	raw := integrityGit(t, root, "cat-file", "tree", tree)
	from, _ := hex.DecodeString(integrityOID(t, root, target+":docs/rule.txt"))
	to, _ := hex.DecodeString(integrityOID(t, root, base+":docs/rule.txt"))
	corruptLoose(t, root, tree, "tree", bytes.Replace(raw, from, to, 1))
	narrowed := string(integrityGit(t, root, "diff", "--stat", base, target))
	if strings.Contains(narrowed, "rule.txt") {
		t.Fatalf("fixture: Git still sees the dropped entry: %s", narrowed)
	}
	got, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || got != nil {
		t.Fatalf("mislabeled intermediate tree: %q %v", got, err)
	}
}

// deepTarget commits the base tree plus a chain of depth nested directories
// ending in one blob, written as loose objects so no Git process runs per
// level.
func deepTarget(t *testing.T, root, base string, depth int) string {
	t.Helper()
	raw, _ := hex.DecodeString(integrityOID(t, root, base+":f.go"))
	child := writeLooseTree(t, root, append([]byte("100644 f.go\x00"), raw...))
	for i := 0; i < depth; i++ {
		raw, _ = hex.DecodeString(child)
		child = writeLooseTree(t, root, append([]byte("40000 d\x00"), raw...))
	}
	entries := string(integrityGit(t, root, "ls-tree", base)) + "040000 tree " + child + "\tdeep\n"
	tree := gitStdin(t, root, entries, "mktree")
	return gitStdin(t, root, "", "commit-tree", tree, "-p", base, "-m", "deep")
}

func gitStdin(t *testing.T, root, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("fixture Git %v: %v", args, err)
	}
	return string(bytes.TrimSpace(out))
}

func writeLooseTree(t *testing.T, root string, body []byte) string {
	t.Helper()
	oid := hex.EncodeToString(integrityHash(40, append([]byte(fmt.Sprintf("tree %d\x00", len(body))), body...)))
	corruptLoose(t, root, oid, "tree", body)
	return oid
}

// The walk descends exactly MaxTreeDepth levels and refuses one more, so the
// per-level batch spawn count is bounded.
func TestCanonicalDiffBoundsChangeSetDepth(t *testing.T) {
	root, base, _ := makeRepo(t)
	repo := open(t, root)
	within := deepTarget(t, root, base, MaxTreeDepth-1)
	got, err := repo.CanonicalDiff(context.Background(), base, within)
	if err != nil || !bytes.Contains(got, []byte("+++ b/deep/"+strings.Repeat("d/", MaxTreeDepth-1)+"f.go\n")) {
		t.Fatalf("depth %d: %v", MaxTreeDepth, err)
	}
	beyond := deepTarget(t, root, base, MaxTreeDepth)
	got, err = repo.CanonicalDiff(context.Background(), base, beyond)
	if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || got != nil {
		t.Fatalf("depth %d: %q %v", MaxTreeDepth+1, got, err)
	}
}
