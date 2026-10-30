//go:build linux

// SPDX-License-Identifier: Apache-2.0
// Derived from internal/tasks/safeopen/traversal_linux_test.go.
// Kept private to the Core observer to preserve decision 0397 component separation.
package safeopen

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestAFPV0361_TraversalOnlyAncestors proves the walk from / takes only
// traversal handles: an execute-only ancestor is crossed, so a Landlock-confined
// test needs no read access to / or any other ancestor, while the final
// directory and file keep their ordinary read permission (V1-0883).
func TestAFPV0361_TraversalOnlyAncestors(t *testing.T) {
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

	held, err := Root(leaf)
	must(t, err)
	must(t, held.Close())
	f, err := InRoot(root, "ancestor/leaf/record", false)
	must(t, err)
	data, err := io.ReadAll(f)
	must(t, err)
	must(t, f.Close())
	if string(data) != "retained" {
		t.Fatalf("walk read %q", data)
	}

	// O_PATH must not turn an unreadable final directory into an admitted one.
	must(t, os.Chmod(leaf, 0111))
	t.Cleanup(func() { must(t, os.Chmod(leaf, 0755)) })
	if held, err := Root(leaf); !errors.Is(err, os.ErrPermission) {
		if held != nil {
			held.Close()
		}
		t.Fatalf("unreadable final Root admitted: %v", err)
	}
	if f, err := InRoot(root, "ancestor/leaf", true); !errors.Is(err, os.ErrPermission) {
		if f != nil {
			f.Close()
		}
		t.Fatalf("unreadable final directory admitted: %v", err)
	}
}
