//go:build linux

package safeopen

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCTSV0004_TraversalOnlyAncestors(t *testing.T) {
	base := realTemp(t)
	ancestor := filepath.Join(base, "ancestor")
	leaf := filepath.Join(ancestor, "leaf")
	must(t, os.MkdirAll(leaf, 0755))
	must(t, os.WriteFile(filepath.Join(leaf, "record"), []byte("retained"), 0600))
	root, err := Root(base)
	must(t, err)
	defer root.Close()
	must(t, os.Chmod(ancestor, 0111))
	t.Cleanup(func() { must(t, os.Chmod(ancestor, 0755)) })
	if f, err := os.Open(ancestor); err == nil {
		f.Close()
		t.Skip("runner can read an execute-only directory; permission boundary not observable")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Fatal(err)
	}

	// Absolute and relative walks must traverse the unreadable ancestor while
	// still opening the final directory/file with its ordinary read permissions.
	held, err := Root(leaf)
	must(t, err)
	data, err := held.ReadFile("record")
	must(t, err)
	must(t, held.Close())
	if string(data) != "retained" {
		t.Fatalf("absolute walk read %q", data)
	}
	f, err := InRoot(root, "ancestor/leaf/record", os.O_RDONLY, 0, false)
	must(t, err)
	must(t, f.Close())

	// The final directory remains a readable descriptor; O_PATH must not turn
	// this denial into a successful Root or directory InRoot operation.
	must(t, os.Chmod(leaf, 0111))
	t.Cleanup(func() { must(t, os.Chmod(leaf, 0755)) })
	if held, err := Root(leaf); !errors.Is(err, os.ErrPermission) {
		if held != nil {
			held.Close()
		}
		t.Fatalf("unreadable final Root admitted: %v", err)
	}
	if f, err := InRoot(root, "ancestor/leaf", os.O_RDONLY, 0, true); !errors.Is(err, os.ErrPermission) {
		if f != nil {
			f.Close()
		}
		t.Fatalf("unreadable final directory admitted: %v", err)
	}
}
