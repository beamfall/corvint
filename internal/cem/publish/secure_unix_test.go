//go:build darwin || linux

package publish

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func TestPublishSecureLockWaitIsContextBounded(t *testing.T) {
	root := openRoot(t)
	holder, err := os.Open(root.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(holder.Fd()), syscall.LOCK_UN)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = PublishSecure(ctx, Output{Root: root, Relative: "report.md", Data: []byte("report")}, false)
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("got %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("lock wait exceeded cancellation bound: %v", elapsed)
	}
	if _, err := os.Lstat(root.Path() + "/report.md"); !os.IsNotExist(err) {
		t.Fatal("contended publication wrote output")
	}
}
