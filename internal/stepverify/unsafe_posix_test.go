//go:build darwin || linux

package stepverify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestStepNativeRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, fixture)
	}{
		{"FIFO never opened", func(t *testing.T, f fixture) {
			if err := syscall.Mkfifo(f.root+"/fifo", 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"file bound", func(t *testing.T, f fixture) {
			file, err := os.Create(f.root + "/large")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if err := file.Truncate(MaxFileBytes + 1); err != nil {
				t.Fatal(err)
			}
		}},
		{"depth bound", func(t *testing.T, f fixture) {
			if err := os.MkdirAll(filepath.Join(f.root, strings.Repeat("d/", MaxDepth+1)), 0700); err != nil {
				t.Fatal(err)
			}
		}},
		{"index checksum", func(t *testing.T, f fixture) {
			data, err := os.ReadFile(f.root + "/.git/index")
			if err != nil {
				t.Fatal(err)
			}
			data[len(data)-1] ^= 1
			if err := os.WriteFile(f.root+"/.git/index", data, 0600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run("ASS-V0-006 "+tc.name, func(t *testing.T) {
			f := newFixture(t)
			tc.change(t, f)
			s, err := Snapshot(context.Background(), f.d, f.h)
			if err != ErrUnsupported || s.Complete {
				t.Fatalf("%v %+v", err, s)
			}
		})
	}
	t.Run("ASS-V0-002 existing staged state is preserved", func(t *testing.T) {
		f := newFixture(t)
		put(t, f.root+"/outside", "initial staged dirt")
		gitTest(t, f.root, "add", "outside")
		s := snapshot(t, f)
		r, code := verify(t, f, s)
		if code != 0 || len(r.Findings) != 0 {
			t.Fatalf("%d %+v", code, r)
		}
	})
	t.Run("ASS-V0-006 cancelled observation refuses", func(t *testing.T) {
		f := newFixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		s, err := Snapshot(ctx, f.d, f.h)
		if err != ErrUnsupported || s.Complete {
			t.Fatalf("%v %+v", err, s)
		}
	})
	t.Run("ASS-V0-003 read only admin changes", func(t *testing.T) {
		f := newFixture(t)
		ro := newFixture(t)
		f.d.ReadOnly = []Checkout{{"readonly", ro.root}}
		f.h.DeclarationDigest = DeclarationDigest(f.d)
		s := snapshot(t, f)
		gitTest(t, ro.root, "config", "user.name", "changed")
		r, code := verify(t, f, s)
		if code != 1 || !has(r, "READ_ONLY_CHANGED", "git/config") {
			t.Fatalf("%d %+v", code, r)
		}
	})
	t.Run("ASS-V0-006 input outside scope and no follow", func(t *testing.T) {
		f := newFixture(t)
		outside, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		put(t, outside+"/input", "safe")
		for _, path := range []string{outside + "/input", f.root + "/outside"} {
			_, err := ReadInput(context.Background(), path, &f.d)
			if strings.HasPrefix(path, f.root) {
				if err != ErrUnsupported {
					t.Fatal("in-scope host input accepted")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		}
		os.Symlink(outside+"/input", outside+"/link")
		if _, err := ReadInput(context.Background(), outside+"/link", &f.d); err != ErrUnsupported {
			t.Fatal("symlink input followed")
		}
		os.Link(outside+"/input", outside+"/hardlink")
		if _, err := ReadInput(context.Background(), outside+"/input", &f.d); err != ErrUnsupported {
			t.Fatal("hardlinked input accepted")
		}
	})
}
