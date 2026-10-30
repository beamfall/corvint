package jstestprovider

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// PTF-V0-003/007: after three full files plus a 31 MiB file, a 2 MiB
// file exceeds the remaining 1 MiB budget. Later inputs must not be read.
func TestPTFV0DependencyTotalBudgetStops(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		if f, err := freshOpenDependency("unused"); f != nil || err == nil {
			t.Fatal("unsupported platform admitted dependency reader")
		}
		return
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := &FreshDependencyManifest{Roots: []string{root}, Files: map[string]string{}}
	sizes := []int64{freshDependencyFileBytes, freshDependencyFileBytes, freshDependencyFileBytes, freshDependencyFileBytes - (1 << 20), 2 << 20, 0}
	for i, size := range sizes {
		path := filepath.Join(root, string(rune('a'+i))+".cjs")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = f.Truncate(size)
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		m.Files[path] = sha256Hex(nil)
	}
	o := ObserveFreshDependencies(m)
	if len(o.Files) != 4 || len(o.Failures) != 1 || o.Failures[0] != "dependency-input-bound" {
		t.Fatalf("reader continued after aggregate overflow: files=%d failures=%v", len(o.Files), o.Failures)
	}
}

// Exact exhaustion also stops inventory and retains uncertainty about any
// remaining entries; it must not certify an incomplete root walk as complete.
func TestPTFV0DependencyExactBudgetStopsInventory(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := &FreshDependencyManifest{Roots: []string{root}, Files: map[string]string{}}
	for i := range 5 {
		path := filepath.Join(root, string(rune('a'+i))+".cjs")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		size := int64(freshDependencyFileBytes)
		if i == 4 {
			size = 0
		}
		err = f.Truncate(size)
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		m.Files[path] = sha256Hex(nil)
	}
	o := ObserveFreshDependencies(m)
	if len(o.Files) != 4 || len(o.Failures) != 1 || FreshDependenciesMatch(m, o) {
		t.Fatalf("inventory continued at exact exhaustion: files=%d failures=%v", len(o.Files), o.Failures)
	}
}
