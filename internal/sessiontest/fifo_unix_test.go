//go:build unix && !solaris

package sessiontest

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestReadersRefuseFIFOWithoutWriter pins V1-1635: the manifest, receipt and
// declaration readers refuse a FIFO with no writer inside a deadline instead
// of blocking in the open.
func TestReadersRefuseFIFOWithoutWriter(t *testing.T) {
	readers := map[string]func(string) error{
		"ReadManifest": func(name string) error { _, _, err := ReadManifest(name); return err },
		"ReadReceipt":  func(name string) error { _, err := ReadReceipt(name); return err },
		"readDeclaration": func(name string) error {
			_, err := readDeclaration(name)
			return err
		},
	}
	for label, read := range readers {
		t.Run(label, func(t *testing.T) {
			fifo := filepath.Join(t.TempDir(), "input.json")
			if err := syscall.Mkfifo(fifo, 0o600); err != nil {
				t.Skipf("mkfifo: %v", err)
			}
			done := make(chan error, 1)
			go func() { done <- read(fifo) }()
			select {
			case err := <-done:
				if err == nil {
					t.Fatalf("%s(FIFO) succeeded, want a refusal", label)
				}
			case <-time.After(5 * time.Second):
				// Unblock the reader so the test binary can exit.
				if w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
					w.Close()
				}
				t.Fatalf("%s blocked on a FIFO with no writer", label)
			}
		})
	}
}
