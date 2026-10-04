package gitstatus

import (
	"errors"
	"os"
	"sync"
)

// scratch tracks the private metadata directories in use, so a process that exits while a status
// read is still running (an adapter whose watchdog fired) can still remove them (CloseScratch).
var scratch struct {
	sync.Mutex
	live   map[string]struct{}
	closed bool
}

var errScratchClosed = errors.New("scratch closed")

func createScratch(parent string) (string, error) {
	scratch.Lock()
	defer scratch.Unlock()
	if scratch.closed {
		return "", errScratchClosed
	}
	temp, err := os.MkdirTemp(parent, "corvint-git-status-")
	if err != nil {
		return "", err
	}
	if scratch.live == nil {
		scratch.live = map[string]struct{}{}
	}
	scratch.live[temp] = struct{}{}
	return temp, nil
}

// releaseScratch removes the directory before it stops tracking it, both under the lock, so an exit
// racing the removal finds it either gone or still listed for CloseScratch.
func releaseScratch(temp string) {
	scratch.Lock()
	defer scratch.Unlock()
	_ = os.RemoveAll(temp)
	delete(scratch.live, temp)
}

// CloseScratch removes every scratch directory still in use and refuses new ones. A process calls
// it once, just before it exits, since a status read it abandoned would otherwise leave its
// directory behind: os.Exit runs no deferred cleanup.
func CloseScratch() {
	scratch.Lock()
	defer scratch.Unlock()
	scratch.closed = true
	for temp := range scratch.live {
		// The abandoned read may still be writing inside, which can fail one removal; nothing
		// recreates the directory once it is gone, so a few attempts suffice.
		for attempt := 0; attempt < 3; attempt++ {
			if os.RemoveAll(temp) == nil {
				break
			}
		}
		delete(scratch.live, temp)
	}
}
