package flowdocs

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAFPV0361_OpenDirectoryNeverReadsFilesystemRoot pins that a nested
// directory is reached without opening / for reading, which a Landlock-confined
// test cannot be granted (V1-0883); / itself is still the one directory opened. The test swaps a
// package hook, so it does not run in parallel.
func TestAFPV0361_OpenDirectoryNeverReadsFilesystemRoot(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var opened []string
	t.Cleanup(func() { openAbsoluteRoot = os.OpenRoot })
	openAbsoluteRoot = func(name string) (*os.Root, error) {
		opened = append(opened, name)
		return os.OpenRoot(name)
	}
	data, err := ReadFile(filepath.Join(dir, "a.md"))
	if err != nil || string(data) != "ok\n" {
		t.Fatalf("ReadFile = %q, %v", data, err)
	}
	for _, name := range opened {
		if name == string(filepath.Separator) {
			t.Fatalf("nested directory opened the filesystem root: %q", opened)
		}
	}
	if len(opened) != 1 {
		t.Fatalf("absolute opens = %q, want only the first component", opened)
	}
	// Opening / itself may be refused where the test is Landlock-confined (CI);
	// only the attempted open is asserted, not its success.
	opened = nil
	if root, err := openDirectory(string(filepath.Separator)); err == nil {
		root.Close()
	}
	if len(opened) != 1 || opened[0] != string(filepath.Separator) {
		t.Fatalf("opening / itself = %q", opened)
	}
}
