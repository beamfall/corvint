//go:build darwin || linux

package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAFPV0361_OpenStableArtifactNeverReadsFilesystemRoot pins that a stable
// artifact under a nested root is reached by opening only the root's first
// component by absolute path, never / for reading, which a Landlock-confined
// test cannot be granted (V1-2365); a symlinked first component is still
// refused. The test swaps a package hook, so it does not run in parallel.
func TestAFPV0361_OpenStableArtifactNeverReadsFilesystemRoot(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence", "a.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	var opened []string
	t.Cleanup(func() { openStableRoot = os.OpenRoot })
	openStableRoot = func(name string) (*os.Root, error) {
		opened = append(opened, name)
		return os.OpenRoot(name)
	}
	f, err := openStableArtifact(dir, "evidence/a.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(data) != "{}" {
		t.Fatalf("read %q, %v", data, err)
	}
	first := "/" + strings.Split(strings.TrimPrefix(dir, "/"), "/")[0]
	if len(opened) != 1 || opened[0] != first {
		t.Fatalf("absolute opens = %q, want only %q", opened, first)
	}
	if top, err := openStableTop("."); err == nil {
		top.Close()
		t.Fatal("dot first component accepted")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(dir, "evidence"), link); err != nil {
		t.Fatal(err)
	}
	if f, err := openStableArtifact(dir, "link/a.json"); err == nil {
		f.Close()
		t.Fatal("symlinked component accepted")
	}
}
