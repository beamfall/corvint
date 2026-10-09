//go:build darwin || linux

package gitstatus

import (
	"os/exec"
	"testing"
)

// AHI-048: a lookup child leads its own recorded group, except in an owned
// worker, whose children stay in the worker's group for the enclosing runner.
func TestAHI048LookupKeepsOwnedWorkerGroup(t *testing.T) {
	previous := ownedWorker.Load()
	t.Cleanup(func() { ownedWorker.Store(previous) })
	for _, owned := range []bool{false, true} {
		ownedWorker.Store(owned)
		command := exec.Command("/bin/sh", "-c", "exit 0")
		if err := runLookup(command); err != nil {
			t.Fatal(err)
		}
		ownGroup := command.SysProcAttr != nil && command.SysProcAttr.Setpgid
		if ownGroup == owned {
			t.Fatalf("owned worker %t: lookup led its own group %t", owned, ownGroup)
		}
	}
}
