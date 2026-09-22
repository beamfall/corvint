//go:build unix

package roadmap

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestLoadReceiptNotObservedForNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "TCP-V0-001.json"), 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan TestReceipt, 1)
	go func() { done <- LoadReceipt(dir, "TCP-V0-001", "tree-abc", false) }()
	select {
	case receipt := <-done:
		if receipt.State != notObserved || receipt.Reason == "" {
			t.Fatalf("receipt = %+v", receipt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("LoadReceipt blocked on a FIFO receipt")
	}
}

func TestLoadDocStateNotObservedForNonRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs-state.json")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan DocState, 1)
	go func() { done <- LoadDocState(path, "IPR-10") }()
	select {
	case state := <-done:
		if state.State != notObserved || state.Reason == "" {
			t.Fatalf("doc state = %+v", state)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("LoadDocState blocked on a FIFO docs-state file")
	}
}

func TestLoadRequirementsRefusesNonRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "REQUIREMENTS.tsv")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := LoadRequirements(path)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want error for a FIFO requirements table")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("LoadRequirements blocked on a FIFO requirements table")
	}
}
