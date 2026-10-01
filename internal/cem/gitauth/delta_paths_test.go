package gitauth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeltaPathsIncludeCEMAndTypeChanges(t *testing.T) {
	root, _, base := makeRepo(t)
	writeFile(t, root, ".corvint/change.cem.json", "{}\n")
	os.Remove(filepath.Join(root, "f.go"))
	writeFile(t, root, "f.go/child", "new")
	os.Chmod(filepath.Join(root, "docs/rule.txt"), 0755)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "all types")
	head := gitCmd(t, root, "rev-parse", "HEAD")
	r := open(t, root)
	changes, err := r.DeltaPaths(context.Background(), base, head)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range changes {
		seen[c.Path] = true
	}
	for _, p := range []string{".corvint/change.cem.json", "f.go", "f.go/child", "docs/rule.txt"} {
		if !seen[p] {
			t.Fatalf("missing %s: %+v", p, changes)
		}
	}
}
func TestDeltaMetadataAndBoundedSHA256(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root, _, _, head := objectViewFixture(t, format)
			r := open(t, root)
			ctx := context.Background()
			entry, ok, err := r.LookupTreeEntry(ctx, head, "file")
			if err != nil || !ok {
				t.Fatal(err)
			}
			body, err := r.BlobBytesBounded(ctx, entry.OID, 1024)
			if err != nil || string(body) != "new\n" {
				t.Fatalf("blob %q %v", body, err)
			}
			message, err := r.DeltaCommitMessage(ctx, head)
			if err != nil || string(message) != "target\n" {
				t.Fatalf("metadata %q %v", message, err)
			}
			if r.blobBytes <= int64(len(body)) {
				t.Fatal("commit bytes not charged")
			}
			before := r.blobBytes
			r.BlobBytesBounded(ctx, entry.OID, 1024)
			if before != r.blobBytes {
				t.Fatal("distinct object charged twice")
			}
		})
	}
}
