//go:build darwin || linux

package intent

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type rootReadError struct{}

func (rootReadError) Read(p []byte) (int, error) {
	copy(p, "part")
	return 4, errors.New("injected read failure")
}

func TestTMV0008_AS07_ReadFileFromRootParity(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := safeopen.Root(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	write := func(name, stringValue string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(stringValue), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []string{"", "abc", "12345"} {
		write("file", v)
		for _, max := range []int{0, 3, 8} {
			a, ae := ReadFile(filepath.Join(dir, "file"), max)
			b, be := ReadFileFromRoot(root, "label", "file", max)
			if string(a) != string(b) || wire.CodeOf(ae) != wire.CodeOf(be) {
				t.Fatalf("parity %q/%d: %q %v / %q %v", v, max, a, ae, b, be)
			}
		}
	}
	for _, name := range []string{"", ".", "..", "a/b", `a\b`, "/file"} {
		if _, err := ReadFileFromRoot(root, "label", name, 8); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
			t.Fatalf("basename %q: %v", name, err)
		}
	}
	if _, err := ReadFileFromRoot(nil, "label", "file", 8); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatal(err)
	}
	if _, err := ReadFileFromRoot(root, "label", "absent", 8); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Symlink("file", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFileFromRoot(root, "label", "link", 8); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFileFromRoot(root, "label", "fifo", 8); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatal(err)
	}
	original := openRootReadFile
	defer func() { openRootReadFile = original }()
	for _, kind := range []string{"replacement", "symlink", "fifo", "truncation", "growth", "parent-enoent"} {
		t.Run(kind, func(t *testing.T) {
			os.Remove(filepath.Join(dir, "race"))
			write("race", "abc")
			openRootReadFile = func(r *os.Root, name string) (*os.File, error) {
				switch kind {
				case "parent-enoent":
					return nil, &os.PathError{Op: "openat", Path: ".", Err: os.ErrNotExist}
				case "replacement", "symlink", "fifo":
					if err := os.Rename(filepath.Join(dir, name), filepath.Join(dir, name+"-"+kind+"-old")); err != nil {
						t.Fatal(err)
					}
					if kind == "replacement" {
						write(name, "xyz")
					} else if kind == "symlink" {
						if err := os.Symlink("file", filepath.Join(dir, name)); err != nil {
							t.Fatal(err)
						}
					} else if err := syscall.Mkfifo(filepath.Join(dir, name), 0600); err != nil {
						t.Fatal(err)
					}
				case "truncation":
					write(name, "a")
				case "growth":
					write(name, "abcde")
				}
				return original(r, name)
			}
			got, err := ReadFileFromRoot(root, "label", "race", 3)
			switch kind {
			case "parent-enoent":
				if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || os.IsNotExist(err) {
					t.Fatalf("parent absence erased: %v", err)
				}
			case "truncation":
				if err != nil || string(got) != "a" {
					t.Fatalf("ordinary EOF parity: %q %v", got, err)
				}
			case "growth":
				if wire.CodeOf(err) != wire.CodeLimitExceeded {
					t.Fatal(err)
				}
			default:
				if err == nil {
					t.Fatalf("replacement accepted %q", got)
				}
			}
			openRootReadFile = original
		})
	}
	legacy := openReadFile
	defer func() { openReadFile = legacy }()
	write("legacy", "abc")
	openReadFile = func(path string) (*os.File, error) { write("legacy", "a"); return legacy(path) }
	if got, err := ReadFile(filepath.Join(dir, "legacy"), 3); err != nil || string(got) != "a" {
		t.Fatalf("legacy EOF changed: %q %v", got, err)
	}
	if got, err := readAll(rootReadError{}, "label", 8, 0); got != nil || wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("partial non-EOF: %q %v", got, err)
	}
	if got, err := readAll(io.LimitReader(strings.NewReader("abc"), 1), "label", 8, 3); err != nil || string(got) != "a" {
		t.Fatalf("EOF: %q %v", got, err)
	}
}
