package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// V1-3179: materialize joins every owned snapshot entry through
// archivepath.Join, so a member never lands outside the scratch root even if
// the snapshot decoder admitted it (on Windows, a reserved device name).
func TestV13179MaterializeConfinesEntriesToRoot(t *testing.T) {
	ok, err := canonicalSnapshot([]fileEntry{
		{Path: "dir", Mode: 0o755, Directory: true},
		{Path: "dir/file", Mode: 0o644, Data: []byte("x")},
	})
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "root")
	if err := materialize(ok, root); err != nil {
		t.Fatalf("local snapshot refused: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "dir", "file")); err != nil || string(got) != "x" {
		t.Fatalf("materialized entry = %q, %v", got, err)
	}
	escapes := []string{"../outside", "dir/../../outside"}
	if runtime.GOOS == "windows" {
		escapes = append(escapes, "NUL", "dir/CON")
	}
	for _, p := range escapes {
		t.Run(p, func(t *testing.T) {
			base := t.TempDir()
			data, err := canonicalSnapshot([]fileEntry{{Path: p, Mode: 0o644, Data: []byte("x")}})
			if err != nil {
				t.Fatal(err)
			}
			if err := materialize(data, filepath.Join(base, "root")); err == nil {
				t.Fatalf("entry %q materialized", p)
			}
			if _, err := os.Stat(filepath.Join(base, "outside")); !os.IsNotExist(err) {
				t.Fatalf("entry %q wrote outside the root: %v", p, err)
			}
		})
	}
}
