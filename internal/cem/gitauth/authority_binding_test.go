//go:build darwin || linux

package gitauth

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func TestAuthorityDirectoryFIFORefusal(t *testing.T) {
	if path := os.Getenv("CORVINT_AUTHORITY_FIFO_PROBE"); path != "" {
		var root *os.Root
		var err error
		if os.Getenv("CORVINT_AUTHORITY_FIFO_CHILD") == "1" {
			parent, e := os.OpenRoot(filepath.Dir(path))
			if e != nil {
				t.Fatal(e)
			}
			defer parent.Close()
			root, err = openAuthorityDirectory(parent, filepath.Base(path))
		} else {
			root, err = openAuthorityRoot(path)
		}
		if err == nil {
			root.Close()
			t.Fatal("FIFO accepted as directory")
		}
		return
	}
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	for _, child := range []string{"0", "1"} {
		t.Run(child, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			options := gitrun.Options{Binary: os.Args[0], Env: []string{"CORVINT_AUTHORITY_FIFO_PROBE=" + path, "CORVINT_AUTHORITY_FIFO_CHILD=" + child}, StdoutLimit: 4096}
			if out, err := gitrun.Run(ctx, gitrun.NewDefaultBudget(), options, "-test.run=^TestAuthorityDirectoryFIFORefusal$"); err != nil {
				t.Fatalf("directory open blocked or failed regression: %v %s", err, out)
			}
		})
	}
}

type authorityModeContext struct {
	context.Context
	calls  int
	mutate func()
}

func (c *authorityModeContext) Err() error {
	c.calls++
	if c.calls == 2 {
		c.mutate()
	}
	return nil
}

func TestAuthorityRawReadRejectsExecuteModeChange(t *testing.T) {
	path := t.TempDir()
	contents := []byte("same immutable bytes")
	name := filepath.Join(path, "file")
	if err := os.WriteFile(name, contents, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	digest := sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(contents))), contents...))
	ctx := &authorityModeContext{Context: context.Background(), mutate: func() {
		if err := os.Chmod(name, 0700); err != nil {
			t.Fatal(err)
		}
	}}
	remaining := int64(128 << 20)
	if err := rawFileMatches(ctx, root, "file", "100644", fmt.Sprintf("%x", digest), &remaining); err == nil {
		t.Fatal("mode change during read accepted")
	}
}

func authorityBindingFixture(t *testing.T) (string, *os.Root) {
	t.Helper()
	path := t.TempDir()
	if err := os.Mkdir(filepath.Join(path, "dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "dir", "file"), []byte("same bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return path, root
}

func TestAuthorityBindingRejectsTerminalAndParentSymlinks(t *testing.T) {
	for _, parent := range []bool{false, true} {
		t.Run(map[bool]string{false: "terminal", true: "parent"}[parent], func(t *testing.T) {
			path, root := authorityBindingFixture(t)
			name, target, lookup := "link", "dir/file", "link"
			if parent {
				target, lookup = "dir", "link/file"
			}
			if err := os.Symlink(target, filepath.Join(path, name)); err != nil {
				t.Fatal(err)
			}
			file, err := openAuthorityFile(root, lookup)
			if err == nil {
				file.Close()
				t.Fatal("symlink accepted as raw authority")
			}
		})
	}
}

func TestAuthorityBindingRejectsObservedNamespaceAndModeDrift(t *testing.T) {
	cases := map[string]func(*testing.T, string){
		"terminal replacement": func(t *testing.T, path string) {
			if err := os.Rename(filepath.Join(path, "dir", "file"), filepath.Join(path, "saved")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../saved", filepath.Join(path, "dir", "file")); err != nil {
				t.Fatal(err)
			}
		},
		"detached parent": func(t *testing.T, path string) {
			if err := os.Rename(filepath.Join(path, "dir"), filepath.Join(path, "saved")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(path, "dir"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "dir", "file"), []byte("same bytes"), 0644); err != nil {
				t.Fatal(err)
			}
		},
		"parent replaced by symlink": func(t *testing.T, path string) {
			if err := os.Rename(filepath.Join(path, "dir"), filepath.Join(path, "saved")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("saved", filepath.Join(path, "dir")); err != nil {
				t.Fatal(err)
			}
		},
		"executable mode": func(t *testing.T, path string) {
			if err := os.Chmod(filepath.Join(path, "dir", "file"), 0755); err != nil {
				t.Fatal(err)
			}
		},
		"restored modification time": func(t *testing.T, path string) {
			file := filepath.Join(path, "dir", "file")
			before, err := os.Stat(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("same bytes"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(file, before.ModTime(), before.ModTime()); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			path, root := authorityBindingFixture(t)
			file, err := openAuthorityFile(root, "dir/file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			mutate(t, path)
			if err := file.validate(context.Background()); err == nil {
				t.Fatal("observed drift accepted")
			}
		})
	}
}

func TestAuthorityBindingCancellationAndDescriptorCleanup(t *testing.T) {
	_, root := authorityBindingFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if file, err := openAuthorityFileContext(ctx, root, "dir/file"); err == nil {
		file.Close()
		t.Fatal("cancelled open accepted")
	}
	file, err := openAuthorityFile(root, "dir/file")
	if err != nil {
		t.Fatal(err)
	}
	child := file.edges[0].child
	if err := file.validate(ctx); err == nil {
		t.Fatal("cancelled validation accepted")
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Stat(); err == nil {
		t.Fatal("leaf descriptor survived close")
	}
	if _, err := child.Stat("."); err == nil {
		t.Fatal("parent descriptor survived close")
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stat("."); err != nil {
		t.Fatal("caller root was closed")
	}
}
