//go:build darwin || linux

package worktreeimpact

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadStableTargetRejectsSymlinkHardlinkAndSpecialFile(t *testing.T) {
	for _, test := range []struct {
		name  string
		build func(*testing.T, string) string
	}{
		{"leaf symlink", func(t *testing.T, root string) string {
			if err := os.Symlink("added.go", filepath.Join(root, "internal/new/link.go")); err != nil {
				t.Fatal(err)
			}
			return "internal/new/link.go"
		}},
		{"parent symlink", func(t *testing.T, root string) string {
			if err := os.Symlink("new", filepath.Join(root, "internal/linked")); err != nil {
				t.Fatal(err)
			}
			return "internal/linked/added.go"
		}},
		{"hard link", func(t *testing.T, root string) string {
			if err := os.Link(filepath.Join(root, "internal/new/added.go"), filepath.Join(root, "internal/new/hard.go")); err != nil {
				t.Fatal(err)
			}
			return "internal/new/hard.go"
		}},
		{"fifo", func(t *testing.T, root string) string {
			name := filepath.Join(root, "internal/new/pipe.go")
			if err := syscall.Mkfifo(name, 0o600); err != nil {
				t.Fatal(err)
			}
			return "internal/new/pipe.go"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rootName, _, _ := fixture(t)
			value := test.build(t, rootName)
			root, err := os.OpenRoot(rootName)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			_, err = readStableTarget(root, value, nil)
			requireCode(t, err, "unsafe-working-tree-impact-file")
		})
	}
}
