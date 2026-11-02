// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
)

// TCD-V0-773: observe actual opens, including an intent hint whose contents
// Resolve would ignore. No reader process or concurrent watcher is needed.
func TestTCDV0773_UnauditedNeverOpensQueue(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	tcdCommit(t, root, "init")
	dir := filepath.Join(root, ".taskman")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	queue := filepath.Join(dir, "queue.json")
	if err := os.WriteFile(queue, []byte("not an intent hint"), 0o600); err != nil {
		t.Fatal(err)
	}
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	if _, err := syscall.InotifyAddWatch(fd, queue, syscall.IN_OPEN); err != nil {
		t.Fatal(err)
	}
	x := tdsRun(t, root, "ticket", "delivery-screen", "V1-9001", "--unaudited", "--prs", tdsExport(t, "prs.json", "[]"))
	if got := tdsVerdicts(t, x); x.code != 0 || got["V1-9001"] != "CLEAR" {
		t.Fatalf("screen: %v %s", got, x.stdout)
	}
	buf := make([]byte, 4096)
	if n, err := syscall.Read(fd, buf); n > 0 || !errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("unaudited screen opened queue.json: events=%d err=%v", n, err)
	}
	t.Setenv(cli.ReadOnlyEnv, "1")
	x = tdsRun(t, root, "ticket", "delivery-screen", "V1-9001", "--unaudited", "--prs", tdsExport(t, "prs.json", "[]"))
	if got := tdsVerdicts(t, x); x.code != 0 || got["V1-9001"] != "CLEAR" {
		t.Fatalf("read-only screen: %v %s", got, x.stdout)
	}
	if n, err := syscall.Read(fd, buf); n > 0 || !errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("read-only screen opened queue.json: events=%d err=%v", n, err)
	}
	// Positive control: the same watch detects a real read.
	if _, err := os.ReadFile(queue); err != nil {
		t.Fatal(err)
	}
	if n, err := syscall.Read(fd, buf); n <= 0 || err != nil {
		t.Fatalf("queue read control: events=%d err=%v", n, err)
	}
}
