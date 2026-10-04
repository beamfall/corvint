package gitauth

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func TestRevisionFSImmutableAndBounds(t *testing.T) {
	t.Run("DLT-V0-002 immutable revision source", testRevisionFSImmutableAndBounds)
}

func TestRevisionFSReadBoundedBeforeBody_DLT_V0_003(t *testing.T) {
	t.Run("DLT-V0-003 one MiB admission survives session and memo reuse", func(t *testing.T) {
		const limit = 1 << 20
		root, _, _ := makeRepo(t)
		writeFile(t, root, "exact", strings.Repeat("x", limit))
		writeFile(t, root, "larger", strings.Repeat("y", limit+1))
		gitCmd(t, root, "add", ".")
		gitCmd(t, root, "commit", "-qm", "bounded inputs")
		head := gitCmd(t, root, "rev-parse", "HEAD")
		ctx := context.Background()
		view := open(t, root)
		if err := view.LoadObjectFormat(ctx); err != nil {
			t.Fatal(err)
		}
		memo, err := NewRequestReadMemo(view)
		if err != nil {
			t.Fatal(err)
		}
		defer memo.Release()
		r, err := memo.Open(gitrun.NewDefaultBudget())
		if err != nil {
			t.Fatal(err)
		}
		defer r.BeginObjectSession()()
		source, err := r.RevisionFS(ctx, head, 4<<20)
		if err != nil {
			t.Fatal(err)
		}
		bounded := source.(interface {
			OpenBounded(string, int) (fs.File, error)
		})
		entry, ok, err := r.LookupTreeEntry(ctx, head, "larger")
		if err != nil || !ok {
			t.Fatalf("entry: %v %v", ok, err)
		}
		// Populate the actual request memo, and exercise the ordinary 4MiB reader.
		if body, err := r.BlobBytes(ctx, entry.OID); err != nil || len(body) != limit+1 {
			t.Fatalf("memo read: %d %v", len(body), err)
		}
		if body, err := fs.ReadFile(source, "larger"); err != nil || len(body) != limit+1 {
			t.Fatalf("ordinary read: %d %v", len(body), err)
		}
		if file, err := bounded.OpenBounded("larger", limit); err == nil || file != nil {
			t.Fatalf("cached oversized blob admitted: %v", err)
		}
		file, err := bounded.OpenBounded("exact", limit)
		if err != nil {
			t.Fatal(err)
		}
		info, err := file.Stat()
		if err != nil || info.Size() != limit {
			t.Fatalf("exact-bound size: %v %v", info, err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := bounded.OpenBounded("missing", limit); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("missing: %v", err)
		}
		if _, err := bounded.OpenBounded("exact", 0); !errors.Is(err, fs.ErrInvalid) {
			t.Fatalf("invalid bound: %v", err)
		}
		smaller, err := r.RevisionFS(ctx, head, limit-1)
		if err != nil {
			t.Fatal(err)
		}
		if file, err := smaller.(interface {
			OpenBounded(string, int) (fs.File, error)
		}).OpenBounded("exact", 4<<20); err == nil || file != nil {
			t.Fatalf("caller enlarged FS bound: %v", err)
		}
	})
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

func TestRevisionFSListingCostScalesWithDirectories_DLT_V0_003(t *testing.T) {
	t.Run("DLT-V0-003 repeated stat above the default operation ceiling", func(t *testing.T) {
		// Per-path lookups cost two operations per stat (2,400 > 1,024); verified
		// listings cost one per directory plus one blob read per file (606).
		root, _, _ := makeRepo(t)
		const directories, perDirectory = 6, 100
		for d := range directories {
			for f := range perDirectory {
				writeFile(t, root, filepath.Join("d"+strings.Repeat("x", d), "f"+strings.Repeat("y", f)), "z")
			}
		}
		gitCmd(t, root, "add", ".")
		gitCmd(t, root, "commit", "-qm", "wide tree")
		head := gitCmd(t, root, "rev-parse", "HEAD")
		ctx := context.Background()
		r, err := Open(root, gitrun.NewDefaultBudget())
		if err != nil {
			t.Fatal(err)
		}
		defer r.BeginObjectSession()()
		source, err := r.RevisionFS(ctx, head, 4<<20)
		if err != nil {
			t.Fatal(err)
		}
		files := 0
		err = fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			for range 2 {
				if _, err := fs.Stat(source, name); err != nil {
					return err
				}
			}
			if !entry.IsDir() {
				files++
			}
			return nil
		})
		if err != nil || files < directories*perDirectory {
			t.Fatalf("walk %d files: %v", files, err)
		}
		if _, err := fs.Stat(source, "d/absent"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("absent child: %v", err)
		}
		if _, err := fs.Stat(source, "d/fy/below-blob"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("child of blob: %v", err)
		}
	})
}
