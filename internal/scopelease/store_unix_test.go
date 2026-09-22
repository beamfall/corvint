//go:build darwin || linux

package scopelease

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// SCL-V0-001
func TestAcquireNeverBlocksOnANonRegularRef(t *testing.T) {
	root := newRoot(t)
	if err := os.MkdirAll(filepath.Join(root, ".git", "refs", "heads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, ".git", "refs", "heads", "main"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan Lease, 1)
	go func() {
		lease, _, _ := Acquire(root, Request{Holder: "agent-a", Paths: []string{"a.go"}, TTL: time.Minute})
		done <- lease
	}()
	select {
	case lease := <-done:
		if lease.Revision != "" {
			t.Fatalf("revision = %q, want empty", lease.Revision)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Acquire blocked reading a FIFO ref")
	}
}

// SCL-V0-011
func TestListReportsASymlinkedLeaseDocumentUnreadableWithoutReadingIt(t *testing.T) {
	root := newRoot(t)
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	const linkedID = "0123456789abcdef"
	if err := os.MkdirAll(Directory(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fifo, filepath.Join(Directory(root), linkedID+".json")); err != nil {
		t.Fatal(err)
	}
	done := make(chan []Report, 1)
	go func() {
		reports, _ := List(root)
		done <- reports
	}()
	select {
	case reports := <-done:
		if len(reports) != 1 || reports[0].Lease.ID != linkedID || reports[0].State != "unreadable" {
			t.Fatalf("List = %+v, want one unreadable %s report", reports, linkedID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("List blocked reading a symlinked lease document")
	}
}
