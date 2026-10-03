//go:build linux

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/traversal_linux_test.go at
// 29a6db884ed795f7694c316433896d190e1ab508; kept private for decision 0397.
package postmergeworkflow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeOpenTraversalOnlyAncestors(t *testing.T) {
	base := nativeOpenTemp(t)
	ancestor := filepath.Join(base, "ancestor")
	leaf := filepath.Join(ancestor, "leaf")
	nativeOpenMust(t, os.MkdirAll(leaf, 0755))
	nativeOpenMust(t, os.WriteFile(filepath.Join(leaf, "record"), []byte("retained"), 0600))
	root, err := nativeOpenRoot(base)
	nativeOpenMust(t, err)
	defer root.Close()
	nativeOpenMust(t, os.Chmod(ancestor, 0111))
	t.Cleanup(func() { nativeOpenMust(t, os.Chmod(ancestor, 0755)) })
	if f, err := os.Open(ancestor); err == nil {
		f.Close()
		t.Skip("runner can read an execute-only directory; permission boundary not observable")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Fatal(err)
	}

	// Absolute and relative walks nativeOpenMust traverse the unreadable ancestor while
	// still opening the final directory/file with its ordinary read permissions.
	held, err := nativeOpenRoot(leaf)
	nativeOpenMust(t, err)
	data, err := held.ReadFile("record")
	nativeOpenMust(t, err)
	nativeOpenMust(t, held.Close())
	if string(data) != "retained" {
		t.Fatalf("absolute walk read %q", data)
	}
	f, err := nativeOpenInRoot(root, "ancestor/leaf/record", os.O_RDONLY, 0, false)
	nativeOpenMust(t, err)
	nativeOpenMust(t, f.Close())

	// The final directory remains a readable descriptor; O_PATH nativeOpenMust not turn
	// this denial into a successful nativeOpenRoot or directory nativeOpenInRoot operation.
	nativeOpenMust(t, os.Chmod(leaf, 0111))
	t.Cleanup(func() { nativeOpenMust(t, os.Chmod(leaf, 0755)) })
	if held, err := nativeOpenRoot(leaf); !errors.Is(err, os.ErrPermission) {
		if held != nil {
			held.Close()
		}
		t.Fatalf("unreadable final nativeOpenRoot admitted: %v", err)
	}
	if f, err := nativeOpenInRoot(root, "ancestor/leaf", os.O_RDONLY, 0, true); !errors.Is(err, os.ErrPermission) {
		if f != nil {
			f.Close()
		}
		t.Fatalf("unreadable final directory admitted: %v", err)
	}
}
