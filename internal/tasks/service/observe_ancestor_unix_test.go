//go:build unix

package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SERVICE500-002 (V1-3686): a group-writable ancestor, as a 0002 umask leaves
// t.TempDir on WSL and Unraid, is refused naming that directory and its mode,
// and the same tree passes once it is private.
func TestSERVICE500_UnsafeAncestorRefusalNamesTheDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(root, "shared")
	if err := os.Mkdir(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0o775); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(shared, "dispatch.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := Host{UID: os.Getuid()}
	_, _, err = h.observeFile(file, 0)
	if err == nil || !strings.Contains(err.Error(), "UNSUPPORTED_FILESYSTEM") || !strings.Contains(err.Error(), "ancestor directory "+shared+" has mode 0775, so another user can write it") {
		t.Fatalf("group-writable ancestor: %v", err)
	}
	if err := os.Chmod(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	if facts, _, err := h.observeFile(file, 0); err != nil || !facts.SafeAncestors {
		t.Fatalf("private ancestors refused: %+v %v", facts, err)
	}
}
