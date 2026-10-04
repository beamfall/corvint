package gitstatus

import (
	"errors"
	"os"
	"sync"
	"testing"
)

// withOpenScratch restores an open, empty registry after a test closes it. The test must not run in
// parallel: a closed registry refuses every status read in the process.
func withOpenScratch(t *testing.T) {
	t.Cleanup(func() {
		scratch.Lock()
		defer scratch.Unlock()
		scratch.closed, scratch.live = false, nil
	})
}

func TestAHI044ScratchRemovedAtClose(t *testing.T) {
	withOpenScratch(t)
	parent := t.TempDir()
	released, err := createScratch(parent)
	if err != nil {
		t.Fatal(err)
	}
	abandoned, err := createScratch(parent)
	if err != nil {
		t.Fatal(err)
	}
	releaseScratch(released)
	if _, err := os.Stat(released); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("released scratch survives: %v", err)
	}
	if err := os.WriteFile(abandoned+"/HEAD", []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	CloseScratch()
	if _, err := os.Stat(abandoned); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("abandoned scratch survives close: %v", err)
	}
	if _, err := createScratch(parent); !errors.Is(err, errScratchClosed) {
		t.Fatalf("closed registry created scratch: %v", err)
	}
	if entries, _ := os.ReadDir(parent); len(entries) != 0 {
		t.Fatalf("scratch left behind: %v", entries)
	}
}

// Whatever order reads and the exit interleave in, no scratch directory survives them.
func TestAHI044ScratchCloseRacesReads(t *testing.T) {
	withOpenScratch(t)
	parent := t.TempDir()
	var group sync.WaitGroup
	start := make(chan struct{})
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			for {
				temp, err := createScratch(parent)
				if err != nil {
					return
				}
				releaseScratch(temp)
			}
		}()
	}
	close(start)
	CloseScratch()
	group.Wait()
	if entries, _ := os.ReadDir(parent); len(entries) != 0 {
		t.Fatalf("scratch survived a racing close: %d entries", len(entries))
	}
}
