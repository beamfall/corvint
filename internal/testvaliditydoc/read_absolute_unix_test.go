//go:build darwin || linux

package testvaliditydoc

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAFPV0361_ReadAbsoluteFileBoundedSkipsFilesystemRoot pins that an
// absolute read below a directory never opens / for reading (V1-0883), while
// the no-follow, identity and bound checks still apply. The test swaps a
// package hook, so it does not run in parallel.
func TestAFPV0361_ReadAbsoluteFileBoundedSkipsFilesystemRoot(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(directory, "nested", "connection.json")
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { openFilesystemRoot = os.OpenRoot })
	openFilesystemRoot = func(name string) (*os.Root, error) {
		t.Fatalf("opened the filesystem root %q", name)
		return nil, nil
	}
	data, err := ReadAbsoluteFileBounded(name, 16)
	if err != nil || string(data) != "{}\n" {
		t.Fatalf("read = %q, %v", data, err)
	}
	if _, err := ReadAbsoluteFileBounded(name, 2); err != ErrInputTooLarge {
		t.Fatalf("bound = %v", err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(filepath.Join(directory, "nested"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAbsoluteFileBounded(filepath.Join(link, "connection.json"), 16); err == nil {
		t.Fatal("symlinked component admitted")
	}
	for _, bad := range []string{"relative/connection.json", directory + "/nested/../nested/connection.json"} {
		if _, err := ReadAbsoluteFileBounded(bad, 16); err == nil {
			t.Fatalf("unclean path %q admitted", bad)
		}
	}
}
