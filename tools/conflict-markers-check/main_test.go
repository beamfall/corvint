// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SRG-V0-8202: every marker, governed class, fixture exception and diagnostic.
func TestConflictMarkersGate(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"docs/spec.md", "pkg/main.go", "data.json", "script/check", "test.sh"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
			t.Fatal(err)
		}
		for _, marker := range []string{"<<<<<<< HEAD", "=======", ">>>>>>> origin/main"} {
			if err := os.WriteFile(filepath.Join(root, path), []byte("ok\n"+marker+"\r\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := check(root, []string{path}, nil); err == nil || !strings.Contains(err.Error(), path+":2:") {
				t.Fatalf("%s %s: %v", path, marker, err)
			}
		}
	}
	path := "testdata/conflict.go"
	if err := os.MkdirAll(filepath.Join(root, "testdata"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte("<<<<<<< fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := check(root, []string{path}, map[string]string{path: "intentional merge fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := check(root, []string{path}, nil); err == nil {
		t.Fatal("unlisted fixture passed")
	}
	for _, exception := range []string{"docs/spec.md", "testdata/../source.go", "testdata/*", "fixtures/"} {
		if err := check(root, nil, map[string]string{exception: "hide conflict"}); err == nil {
			t.Fatalf("invalid exception admitted: %s", exception)
		}
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte("// <<<<<<< quoted\n========\n>>>>>>>x\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := check(root, []string{path}, nil); err != nil {
		t.Fatal(err)
	}
}
