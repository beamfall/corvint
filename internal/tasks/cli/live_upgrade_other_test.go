//go:build !(darwin || linux)

package cli_test

import (
	"errors"
	"os/exec"
	"time"
)

// The CAL-V0-130 upgrade test skips where the owned process group API is
// unavailable; this stand-in only keeps it compiling there.
type survivorRunner struct {
	done    chan struct{}
	waitErr error
}

func watchSurvivor(cmd *exec.Cmd) *survivorRunner {
	s := &survivorRunner{done: make(chan struct{})}
	go func() { s.waitErr = cmd.Wait(); close(s.done) }()
	return s
}

func (s *survivorRunner) retire(_, _ time.Duration) error {
	return errors.New("survivor cleanup is unavailable on this platform")
}
