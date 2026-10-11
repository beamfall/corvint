//go:build darwin || linux

package cli_test

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTOLV0043_LegacyCasesFIFORefused: a --legacy-cases path naming a FIFO
// is refused promptly as not a regular file instead of blocking on the open
// or the read.
func TestTOLV0043_LegacyCasesFIFORefused(t *testing.T) {
	r, id, _ := obligationRepo(t)
	seedObligations(t, r.Root, id)
	setSources(t, r.Root, id, "set-sources",
		`[{"id":"AC-1","state":null,"core":null,"reason":"reviewed","sources":[{"kind":"legacy-case","ref":"C7","digest":"`+digestA+`"}]}]`)
	fifo := filepath.Join(t.TempDir(), "cases.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan string, 1)
	go func() { // the test goroutine must stay free to report a hang
		x := atm(t, r.Root, nil, "ticket", "obligations", "show", "--target", id, "--legacy-cases", fifo)
		if x.res.Outcome == wire.OutcomeOK {
			done <- "accepted: " + string(x.stdout)
			return
		}
		done <- string(x.stdout)
	}()
	select {
	case out := <-done:
		if !strings.Contains(out, "not a regular file") {
			t.Fatalf("FIFO not refused as nonregular: %s", out)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("--legacy-cases blocked on a FIFO")
	}
}
