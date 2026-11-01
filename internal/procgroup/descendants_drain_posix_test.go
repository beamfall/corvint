//go:build darwin || linux

package procgroup

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

// V1-3435: a snapshot whose producer exits while its output pipe is still held
// briefly (here by a background child for 500 ms, standing in for an output
// copy starved on a loaded host) is read in full under the load-safe bound,
// and fails as unavailable under the former 100 ms bound.
func TestDescendantSnapshotSurvivesBrieflyHeldPipe(t *testing.T) {
	row := fmt.Sprintf("%d 1 Wed Oct 14 00:00:00 2026 S\\n", os.Getpid())
	held := func() *exec.Cmd {
		return exec.Command("/bin/sh", "-c", "printf '"+row+"'; (sleep 0.5) &")
	}
	snapshot, err := descendantSnapshotCommand(held())
	if err != nil {
		t.Fatalf("a briefly held pipe failed the snapshot: %v", err)
	}
	if _, ok := snapshot[os.Getpid()]; !ok {
		t.Fatalf("snapshot lost the row: %+v", snapshot)
	}
	saved := snapshotOutputDrain
	snapshotOutputDrain = 100 * time.Millisecond
	t.Cleanup(func() { snapshotOutputDrain = saved })
	if _, err := descendantSnapshotCommand(held()); err == nil {
		t.Fatal("negative control: the 100 ms bound did not expire on the held pipe")
	}
}
