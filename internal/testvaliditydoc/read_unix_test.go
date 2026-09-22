//go:build darwin || linux

package testvaliditydoc

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// MTV-V0-003/LPCV-V0-051: a FIFO substituted after stat must not wait for a writer.
func TestReceiptRejectsFIFOAfterStat(t *testing.T) {
	directory := t.TempDir()
	name := filepath.Join(directory, "receipt")
	if err := os.WriteFile(name, nil, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	before, err := root.Lstat("receipt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(name, 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := readRegular(root, "receipt", before); err == nil || len(data) != 0 {
		t.Fatalf("data=%q err=%v", data, err)
	}
}
