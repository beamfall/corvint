package main

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnrollmentCopyAnchorsDirectoryFD(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	stage := filepath.Join(root, "stage")
	outside := filepath.Join(root, "outside")
	for _, p := range []string{input, stage, outside} {
		if e := os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"enrollment.json", "objects.json", "cem.json", "ocm.json", "selection.json", "target.json"} {
		os.WriteFile(filepath.Join(input, name), []byte(name), 0600)
	}
	in, e := unix.Open(input, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer unix.Close(in)
	out, e := unix.Open(stage, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer unix.Close(out)
	// Simulate the exact parent rename/symlink substitution identified in review.
	os.Rename(stage, stage+"-held")
	os.Symlink(outside, stage)
	if e = copyEnrollmentFiles(in, out, principal{uint32(os.Getuid()), uint32(os.Getgid())}); e != nil {
		t.Fatal(e)
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("root write redirected outside held directory")
	}
	if _, e = os.Stat(filepath.Join(stage+"-held", "objects.json")); e != nil {
		t.Fatal(e)
	}
}
func TestMutableBundleSpecialFileRefused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "fifo")
	if e := unix.Mkfifo(p, 0600); e != nil {
		t.Fatal(e)
	}
	before := time.Now()
	if _, e := readRegular(p, 100); e == nil {
		t.Fatal("FIFO accepted")
	}
	if time.Since(before) > time.Second {
		t.Fatal("FIFO blocked")
	}
}
