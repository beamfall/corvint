package gitauth

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestRevisionFSImmutableAndBounds(t *testing.T) {
	t.Run("DLT-V0-002 immutable revision source", testRevisionFSImmutableAndBounds)
}
func testRevisionFSImmutableAndBounds(t *testing.T) {
	root, base, head := makeRepo(t)
	r := open(t, root)
	ctx := context.Background()
	f, err := r.RevisionFS(ctx, base, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "f.go", "dirty")
	body, err := fs.ReadFile(f, "f.go")
	if err != nil || string(body) != bodyV1 {
		t.Fatalf("base: %q %v", body, err)
	}
	h, err := r.RevisionFS(ctx, head, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	body, err = fs.ReadFile(h, "f.go")
	if err != nil || string(body) != bodyV2 {
		t.Fatalf("head: %q %v", body, err)
	}
	if err := fs.WalkDir(f, ".", func(p string, d fs.DirEntry, e error) error { return e }); err != nil {
		t.Fatal(err)
	}
	if _, err = fs.ReadFile(f, "../f.go"); err == nil {
		t.Fatal("traversal admitted")
	}
	before := gitCmd(t, root, "status", "--porcelain=v1")
	small, err := r.RevisionFS(ctx, head, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fs.ReadFile(small, "f.go"); err == nil {
		t.Fatal("bound bypassed")
	}
	if after := gitCmd(t, root, "status", "--porcelain=v1"); before != after {
		t.Fatal("read mutated worktree")
	}
}
func TestBoundedBlobIdentityAndNoMemoBypass(t *testing.T) {
	t.Run("DLT-V0-003 identity without memo bypass", testBoundedBlobIdentityAndNoMemoBypass)
}
func testBoundedBlobIdentityAndNoMemoBypass(t *testing.T) {
	root, _, head := makeRepo(t)
	ctx := context.Background()
	r := open(t, root)
	entry, ok, err := r.LookupTreeEntry(ctx, head, "f.go")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if _, err = r.BlobBytes(ctx, entry.OID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.BlobBytesBounded(ctx, entry.OID, 1); err == nil {
		t.Fatal("memo bypassed consumer bound")
	}
	if _, err = r.BlobBytesBounded(ctx, head, 4<<20); err == nil {
		t.Fatal("commit admitted as blob")
	}
	corruptLoose(t, root, entry.OID, "blob", []byte("wrong"))
	if _, err = r.BlobBytesBounded(ctx, entry.OID, 4<<20); err == nil {
		t.Fatal("unverified bytes admitted")
	}
}
func TestBoundedBlobFourMiBBoundary(t *testing.T) {
	t.Run("DLT-V0-003 four MiB source bound", testBoundedBlobFourMiBBoundary)
}
func testBoundedBlobFourMiBBoundary(t *testing.T) {
	root, _, _ := makeRepo(t)
	for _, n := range []int{4 << 20, (4 << 20) + 1, 64 << 20} {
		if err := os.WriteFile(filepath.Join(root, "big"), bytes.Repeat([]byte("x"), n), 0644); err != nil {
			t.Fatal(err)
		}
		oid := gitCmd(t, root, "hash-object", "-w", "big")
		r := open(t, root)
		body, err := r.BlobBytesBounded(context.Background(), oid, 4<<20)
		if n == 4<<20 {
			if err != nil || len(body) != n {
				t.Fatalf("exact bound %v", err)
			}
		} else if err == nil || body != nil {
			t.Fatalf("oversize %d admitted", n)
		}
	}
}
