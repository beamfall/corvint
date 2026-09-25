//go:build darwin || linux

package appflows

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// AFU-V1-036: inputs are Lstat-checked before open and never block on a FIFO.
func TestAFUV1InputRegularBeforeOpen(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "evidence.json")
	fifo := filepath.Join(dir, "fifo")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(regular, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{fifo, link} {
		done := make(chan error, 1)
		go func() { _, err := ReadFile(name); done <- err }()
		select {
		case err := <-done:
			if err == nil {
				t.Fatalf("accepted non-regular input %s", name)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("blocked on %s", name)
		}
	}
	if _, err := ReadFile(regular); err != nil {
		t.Fatal(err)
	}
}

// AFU-V1-036: a FIFO swapped in between Lstat and open is refused without blocking.
func TestAFUV1InputSwapAfterLstatRefused(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	fifo := filepath.Join(dir, "fifo")
	if err := os.WriteFile(victim, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	previous := openInputFile
	t.Cleanup(func() { openInputFile = previous })
	openInputFile = func(name string) (*os.File, error) {
		if err := os.Rename(fifo, victim); err != nil {
			return nil, err
		}
		return previous(name)
	}
	done := make(chan error, 1)
	go func() { _, err := ReadFile(victim); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("read a swapped FIFO")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("blocked on a swapped FIFO")
	}
}

// AFU-V1-036: record writes resolve under the root and follow no symlink.
func TestAFUV1RecordConfinedToRoot(t *testing.T) {
	root, in := fixture(t)
	raw, _ := json.Marshal(observed(in))
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "real"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(root, "inner")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "target"), filepath.Join(root, "dangling.json")); err != nil {
		t.Fatal(err)
	}
	refused := []string{
		filepath.Join(outside, "record.json"),
		filepath.Join(root, "out", "record.json"),
		filepath.Join(root, "inner", "record.json"),
		filepath.Join(root, "dangling.json"),
		filepath.Join(root, "..", "record.json"),
	}
	for _, name := range refused {
		if Record(in, raw, name) == nil {
			t.Fatalf("wrote outside confinement: %s", name)
		}
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("a refused write left a file outside the root")
	}
	if err := Record(in, raw, filepath.Join(root, "real", "record.json")); err != nil {
		t.Fatal(err)
	}
}

// AFU-V1-036: a manifest FIFO or symlink under the root is refused before open, without blocking.
func TestAFUV1ManifestRegularBeforeOpen(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "manifest.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "real.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.json", filepath.Join(root, "link.json")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"manifest.json", "link.json"} {
		done := make(chan error, 1)
		go func() { _, err := Capture(context.Background(), root, name); done <- err }()
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "source must be regular") {
				t.Fatalf("non-regular manifest %s: %v", name, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("blocked on manifest %s", name)
		}
	}
}
