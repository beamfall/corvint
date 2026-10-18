//go:build darwin || linux

package obligation

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTOLV0062_AllowListReadRefusesSymlinkAndFIFO: the allow-list is opened
// no-follow and nonblocking and the descriptor must be a regular file, so a
// symlink or a FIFO in its place refuses MALFORMED without blocking and is
// never read as an empty list.
func TestTOLV0062_AllowListReadRefusesSymlinkAndFIFO(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, wire.EncodeFile((&AllowList{Entries: map[string]Qualification{}}).Value()), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAllowList(target); err != nil {
		t.Fatalf("regular file: %v", err)
	}
	link := AllowListPath(filepath.Join(dir, "link"))
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	fifo := AllowListPath(filepath.Join(dir, "fifo"))
	if err := os.MkdirAll(filepath.Dir(fifo), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{"symlink": link, "fifo": fifo} {
		done := make(chan error, 1)
		go func() { _, err := ReadAllowList(p); done <- err }()
		select {
		case err := <-done:
			if wire.CodeOf(err) != wire.CodeMalformed {
				t.Fatalf("%s: %v", name, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: the read blocked", name)
		}
	}
}
