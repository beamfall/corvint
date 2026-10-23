//go:build darwin || linux

package upgradechanges

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestReadRefsRejectsFIFO(t *testing.T) {
	t.Run("UCL1142-V0-003 nonblocking special file refusal", func(t *testing.T) {
		root, _ := repoTest(t)
		path := filepath.Join(root, "fifo")
		if err := syscall.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
		if file, err := openRefs(path); err == nil {
			file.Close()
			t.Fatal("FIFO accepted")
		}
	})
}

// TestAFPV0361_OpenRefsNeverReadsFilesystemRoot pins that openRefs reaches a
// nested file by opening only its first component by absolute path, never /
// for reading, which a Landlock-confined test cannot be granted (V1-2365), and
// still refuses a symlinked first component and an empty path. The test swaps
// a package hook, so it does not run in parallel.
func TestAFPV0361_OpenRefsNeverReadsFilesystemRoot(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	refs := filepath.Join(dir, "refs")
	if err := os.WriteFile(refs, []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var opened []string
	t.Cleanup(func() { openRefsTop = syscall.Open })
	openRefsTop = func(name string, flags int, perm uint32) (int, error) {
		opened = append(opened, name)
		return syscall.Open(name, flags, perm)
	}
	file, err := openRefs(refs)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	file.Close()
	if err != nil || string(data) != "ok\n" {
		t.Fatalf("read %q, %v", data, err)
	}
	first := "/" + strings.Split(strings.TrimPrefix(refs, "/"), "/")[0]
	if len(opened) != 1 || opened[0] != first {
		t.Fatalf("absolute opens = %q, want only %q", opened, first)
	}
	if file, err := openRefs("/"); err == nil {
		file.Close()
		t.Fatal("empty path accepted")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if file, err := openRefs(filepath.Join(link, "refs")); err == nil {
		file.Close()
		t.Fatal("symlinked component accepted")
	}
}
