//go:build unix

package playwrightruns

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadFileRefusesFIFO(t *testing.T) {
	t.Run("PWR-V0-001 a FIFO is refused without blocking", func(t *testing.T) {
		fifo := filepath.Join(t.TempDir(), "fifo.json")
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadFile(fifo); !errors.Is(err, ErrUnreadable) {
			t.Fatalf("fifo: err = %v", err)
		}
	})
}
