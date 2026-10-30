package testacceptance

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Both live packages share one explicitly reserved loopback port. The kernel
// releases this advisory fixture lock on interruption as well as normal exit.
func ptfLivePort(t *testing.T) {
	t.Helper()
	f, e := os.OpenFile(filepath.Join(os.TempDir(), "corvint-ptf-live-4394.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(3 * time.Minute)
	for {
		e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if e == nil {
			break
		}
		if e != syscall.EWOULDBLOCK || time.Now().After(deadline) {
			f.Close()
			t.Fatalf("fixture port lock: %v", e)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() })
}
