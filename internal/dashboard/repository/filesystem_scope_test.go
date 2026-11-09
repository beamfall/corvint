//go:build darwin || linux || windows

// SPDX-License-Identifier: Apache-2.0
package repository

import (
	"os"
	"path/filepath"
	"testing"
)

// AFP-V0-3360: a directory below the root does not require permission to
// read the root. The resulting binding still names the requested directory.
func TestBoundedDirectoryOpen_AFPV03360(t *testing.T) {
	directory := stableTestDirectory(t, "bounded-open-")
	volumeRoot := filepath.VolumeName(directory) + string(filepath.Separator)
	previous := openAbsoluteDirectory
	t.Cleanup(func() { openAbsoluteDirectory = previous })
	var opened []string
	openAbsoluteDirectory = func(name string) (*os.Root, error) {
		opened = append(opened, name)
		if name == volumeRoot {
			return nil, os.ErrPermission
		}
		return os.OpenRoot(name)
	}
	binding, failure := openDirectory(directory)
	if failure != nil {
		t.Fatalf("fixture directory requires a root read: %+v (opened %v)", failure, opened)
	}
	defer closeDirectoryBinding(&binding)
	want, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	got, err := binding.file.Stat()
	if err != nil || !os.SameFile(want, got) || len(opened) != 1 || opened[0] == volumeRoot {
		t.Fatalf("wrong directory binding: %v, opens %v", err, opened)
	}
	// The explicit root request is still a root read and still refuses when
	// the caller lacks that permission; it cannot silently bind a child.
	root, failure := openDirectory(volumeRoot)
	if failure == nil {
		closeDirectoryBinding(&root)
		t.Fatal("explicit root read bypassed the permission boundary")
	}
}

// AFP-V0-3360: skipping the root read must not admit a first-component
// substitution. Inject an opened directory other than the one Lstat saw.
func TestBoundedDirectoryOpenRejectsReplacement_AFPV03360(t *testing.T) {
	directory := stableTestDirectory(t, "bounded-open-target-")
	other := stableTestDirectory(t, "bounded-open-other-")
	baseline, failure := openDirectory(directory)
	if failure != nil {
		t.Fatalf("fixture initialization: %+v", failure)
	}
	closeDirectoryBinding(&baseline)
	previous := openAbsoluteDirectory
	t.Cleanup(func() { openAbsoluteDirectory = previous })
	called := false
	openAbsoluteDirectory = func(string) (*os.Root, error) {
		called = true
		return os.OpenRoot(other)
	}
	binding, failure := openDirectory(directory)
	if !called {
		t.Fatal("replacement boundary not reached")
	}
	if failure == nil {
		closeDirectoryBinding(&binding)
		t.Fatal("replaced first component was admitted")
	}
}
