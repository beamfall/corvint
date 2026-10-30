//go:build unix && !solaris

package playwrightruns

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"
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

// PWR-V0-041: a FIFO named as the baseline is refused without blocking, as a report is.
func TestPWRV0041_ReadBaselineRefusesFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "baseline.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := ReadBaseline(fifo)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrUnreadable) {
			t.Fatalf("fifo: err = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ReadBaseline blocked on a FIFO")
	}
}
