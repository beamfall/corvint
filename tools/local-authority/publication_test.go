package main

import (
	"github.com/Beamfall/corvint/internal/localauthority"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationCompleteBeforeActivePointer(t *testing.T) {
	root := t.TempDir()
	handle := strings.Repeat("a", 64)
	files := map[string][]byte{"receipt.json": []byte("fixture receipt"), "terminal.json": []byte("fixture terminal")}
	if e := publishAt(root, handle, files); e != nil {
		t.Fatal(e)
	}
	for name, want := range files {
		got, e := os.ReadFile(filepath.Join(root, handle, name))
		if e != nil || string(got) != string(want) {
			t.Fatal("incomplete publication")
		}
		info, _ := os.Stat(filepath.Join(root, handle, name))
		if info.Mode().Perm() != 0444 {
			t.Fatal("mutable published file")
		}
	}
	raw, e := os.ReadFile(filepath.Join(root, "active-enrollment.json"))
	if e != nil {
		t.Fatal(e)
	}
	var active struct {
		Profile          string `json:"profile"`
		EnrollmentHandle string `json:"enrollmentHandle"`
	}
	if e = localauthority.Decode(raw, &active); e != nil || active.EnrollmentHandle != handle {
		t.Fatal("pointer mismatch")
	}
	if publishAt(root, handle, files) == nil {
		t.Fatal("replaced immutable publication")
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			t.Fatal("staging residue")
		}
	}
}
