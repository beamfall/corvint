//go:build darwin || linux

package analyzerswift

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadDescriptorAcceptsStableRegularFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "request.json")
	want := []byte("{}\n")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readDescriptor(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestReadDescriptorRejectsUnsafeKindsAndIdentityRaces(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "request.json")
	write := func(path string, value []byte) {
		t.Helper()
		if err := os.WriteFile(path, value, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(path, []byte("{}\n"))
	link := filepath.Join(directory, "request-link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readDescriptor(link); err == nil {
		t.Fatal("accepted symlink")
	}
	if _, err := readDescriptor(directory); err == nil {
		t.Fatal("accepted directory")
	}
	if _, err := readDescriptor("/dev/null"); err == nil {
		t.Fatal("accepted device")
	}
	fifo := filepath.Join(directory, "request.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := readDescriptor(fifo); err == nil {
		t.Fatal("accepted FIFO")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("FIFO read blocked for %s", elapsed)
	}
	over := filepath.Join(directory, "over.json")
	write(over, []byte("[]\n"))
	if _, err := readDescriptorWithHooks(path, descriptorHooks{afterBefore: func() {
		if err := os.Rename(over, path); err != nil {
			t.Fatal(err)
		}
	}}); err == nil {
		t.Fatal("accepted before-open replacement")
	}
	write(path, []byte("{}\n"))
	fifoReplacement := filepath.Join(directory, "replacement.fifo")
	if err := syscall.Mkfifo(fifoReplacement, 0o600); err != nil {
		t.Fatal(err)
	}
	started = time.Now()
	if _, err := readDescriptorWithHooks(path, descriptorHooks{afterBefore: func() {
		if err := os.Rename(fifoReplacement, path); err != nil {
			t.Fatal(err)
		}
	}}); err == nil {
		t.Fatal("accepted FIFO path replacement")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("FIFO replacement blocked for %s", elapsed)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	write(path, []byte("{}\n"))
	over = filepath.Join(directory, "after-open.json")
	write(over, []byte("[]\n"))
	if _, err := readDescriptorWithHooks(path, descriptorHooks{afterOpen: func() {
		if err := os.Rename(over, path); err != nil {
			t.Fatal(err)
		}
	}}); err == nil {
		t.Fatal("accepted after-open path replacement")
	}
	write(path, []byte("{}\n"))
	if _, err := readDescriptorWithHooks(path, descriptorHooks{afterOpen: func() {
		stamp := time.Unix(1, 0)
		write(path, []byte("[]\n"))
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}}); err == nil {
		t.Fatal("accepted in-place mutation")
	}
	large := filepath.Join(directory, "too-large.json")
	write(large, bytes.Repeat([]byte{'x'}, maxWire+1))
	if _, err := readDescriptor(large); err == nil {
		t.Fatal("accepted oversized descriptor")
	}
}
