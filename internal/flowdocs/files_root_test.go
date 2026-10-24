package flowdocs

import (
	"os"
	"path/filepath"
	"strings"
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
	// openDirectory is asserted directly: ReadFile's no-follow regular-file
	// reader refuses on platforms without O_NOFOLLOW (Windows), which is a
	// separate deliberate refusal.
	root, err := openDirectory(dir)
	if err != nil {
		t.Fatalf("openDirectory(%q) = %v", dir, err)
	}
	if _, err := root.Stat("a.md"); err != nil {
		t.Fatalf("confined root misses a.md: %v", err)
	}
	root.Close()
	volumeRoot := filepath.VolumeName(dir) + string(filepath.Separator)
	for _, name := range opened {
		if name == volumeRoot {
			t.Fatalf("nested directory opened the filesystem root: %q", opened)
		}
	}
	if want := volumeRoot + strings.SplitN(dir[len(volumeRoot):], string(filepath.Separator), 2)[0]; len(opened) == 1 && opened[0] != want {
		t.Fatalf("first absolute open = %q, want %q", opened[0], want)
	}
	if len(opened) != 1 {
		t.Fatalf("absolute opens = %q, want only the first component", opened)
	}
	// Opening / itself may be refused where the test is Landlock-confined (CI);
	// only the attempted open is asserted, not its success.
	opened = nil
	if root, err := openDirectory(volumeRoot); err == nil {
		root.Close()
	}
	if len(opened) != 1 || opened[0] != volumeRoot {
		t.Fatalf("opening / itself = %q", opened)
	}
}
