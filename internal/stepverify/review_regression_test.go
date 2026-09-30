package stepverify

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestStepReviewRegressions(t *testing.T) {
	t.Run("ASS-V0-004 missing marker retains completed writes", func(t *testing.T) {
		f := newFixture(t)
		s := snapshot(t, f)
		put(t, f.root+"/outside", "changed")
		if err := os.Rename(f.root+"/.git", f.root+"/.git-hidden"); err != nil {
			t.Fatal(err)
		}
		r, code := verify(t, f, s)
		if code != 2 || !has(r, "OUT_OF_SCOPE_WRITE", "outside") || r.AfterDigest != nil || len(r.After) != 1 || !r.After[0].ContentComplete {
			t.Fatalf("%d %+v", code, r)
		}
	})
	for _, directory := range []string{"git", "common"} {
		for _, readonly := range []bool{false, true} {
			t.Run("ASS-V0-003 linked "+directory+" root mode readonly "+strconv.FormatBool(readonly), func(t *testing.T) {
				f := newFixture(t)
				holder, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				linked := holder + "/linked"
				gitTest(t, f.root, "worktree", "add", "--detach", linked, "HEAD")
				if readonly {
					f.d.ReadOnly = []Checkout{{"readonly", linked}}
				} else {
					f.d.Author.Root = linked
				}
				f.h.DeclarationDigest = DeclarationDigest(f.d)
				s := snapshot(t, f)
				c := s.Checkouts[0]
				if readonly {
					c = s.Checkouts[1]
				}
				path := c.GitDir
				if directory == "common" {
					path = c.CommonDir
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				mode := info.Mode().Perm() ^ 0040
				if err := os.Chmod(path, mode); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(path, info.Mode().Perm())
				r, code := verify(t, f, s)
				if code != 1 || !has(r, "ADMIN_CHANGED", directory) || readonly && !has(r, "READ_ONLY_CHANGED", directory) {
					t.Fatalf("%d %+v", code, r)
				}
			})
		}
	}
	t.Run("ASS-V0-006 linked pointer symlink ancestor refused before metadata", func(t *testing.T) {
		f := newFixture(t)
		holder, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		linked := holder + "/linked"
		gitTest(t, f.root, "worktree", "add", "--detach", linked, "HEAD")
		f.d.Author.Root = linked
		f.h.DeclarationDigest = DeclarationDigest(f.d)
		gitdir, _, err := worktreeDirectories(linked)
		if err != nil {
			t.Fatal(err)
		}
		alias := holder + "/alias"
		if err := os.Symlink(filepath.Dir(gitdir), alias); err != nil {
			t.Fatal(err)
		}
		put(t, linked+"/.git", "gitdir: "+alias+"/"+filepath.Base(gitdir)+"\n")
		if _, _, err := worktreeDirectories(linked); err != ErrUnsupported {
			t.Fatal("symlink pointer ancestor admitted")
		}
		s, err := Snapshot(context.Background(), f.d, f.h)
		if err != ErrUnsupported || s.Complete || len(s.Checkouts) != 1 || !s.Checkouts[0].ContentComplete {
			t.Fatalf("%v %+v", err, s)
		}
	})
	t.Run("ASS-V0-004 two bounded states cannot emit an oversized receipt", func(t *testing.T) {
		rows := make([]Entry, 16000)
		prefix := strings.Repeat("p", 900)
		for i := range rows {
			rows[i] = Entry{Path: prefix + strconv.Itoa(i), Kind: "FILE", Mode: 0600, Size: 1, Digest: hash(nil), Identity: hash(nil)}
		}
		state := State{Checkouts: []CheckoutState{{Entries: rows, AdminEntries: []Entry{}, Unknowns: []string{}}}}
		if _, err := BoundedEncode(state); err != nil {
			t.Fatalf("one component unexpectedly exceeds bound: %v", err)
		}
		r := Receipt{Before: state.Checkouts, After: state.Checkouts, Findings: []Finding{}, Unknowns: []string{}}
		if err := sealReceipt(&r); err != ErrUnsupported {
			t.Fatal("oversized receipt admitted")
		}
		if _, err := BoundedEncode(r); err != ErrUnsupported {
			t.Fatal("output writer bound missing")
		}
	})
}
