//go:build darwin || linux

package upgradechanges

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadRefsRejectsFIFO(t *testing.T) {
	t.Run("UCL1142-V0-003 nonblocking special file refusal", func(t *testing.T) {
		root, _ := repoTest(t)
		path := filepath.Join(root, "fifo")
		if err := syscall.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
		if file, err := openRefs(path); err == nil {
			file.Close()
			t.Fatal("FIFO accepted")
		}
	})
}
