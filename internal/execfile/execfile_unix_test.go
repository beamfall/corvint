//go:build unix

package execfile

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// forkRace writes and immediately executes fresh scripts from several
// goroutines while other goroutines fork continuously, and counts the starts
// that failed with ETXTBSY. write is the writer under test.
func forkRace(t *testing.T, duration time.Duration, write func(string, []byte, os.FileMode) error) (busy, runs int64) {
	t.Helper()
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("no true executable:", err)
	}
	dir := t.TempDir()
	stop := make(chan struct{})
	var forkers sync.WaitGroup
	for i := 0; i < 4; i++ {
		forkers.Add(1)
		go func() {
			defer forkers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = exec.Command(truePath).Run()
			}
		}()
	}
	deadline := time.Now().Add(duration)
	var writers sync.WaitGroup
	for w := 0; w < 4; w++ {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			path := filepath.Join(dir, "script-"+string(rune('a'+w)))
			for time.Now().Before(deadline) {
				if err := write(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
					t.Error(err)
					return
				}
				atomic.AddInt64(&runs, 1)
				err := exec.Command(path).Run()
				switch {
				case err == nil:
				case errors.Is(err, syscall.ETXTBSY):
					atomic.AddInt64(&busy, 1)
				default:
					t.Errorf("start %s: %v", path, err)
					return
				}
			}
		}(w)
	}
	writers.Wait()
	close(stop)
	forkers.Wait()
	return atomic.LoadInt64(&busy), atomic.LoadInt64(&runs)
}

// WQO-V0-093: an executable written under the fork lock never fails to start
// with ETXTBSY, however many forks race the write. The unguarded control only
// reports how often this host reproduces the race (Linux: hundreds per second
// under load; Darwin: rarely), so the assertion cannot depend on it.
func TestWriteFileExcludesForkInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("fork race stress skipped in -short")
	}
	t.Run("control-os.WriteFile", func(t *testing.T) {
		busy, runs := forkRace(t, time.Second, os.WriteFile)
		t.Logf("unguarded control: runs=%d etxtbsy=%d", runs, busy)
	})
	t.Run("WQO-V0-093", func(t *testing.T) {
		busy, runs := forkRace(t, 2*time.Second, WriteFile)
		t.Logf("guarded: runs=%d etxtbsy=%d", runs, busy)
		if runs == 0 {
			t.Fatal("no guarded write ran")
		}
		if busy != 0 {
			t.Fatalf("guarded executable start failed ETXTBSY %d times in %d runs", busy, runs)
		}
	})
}
