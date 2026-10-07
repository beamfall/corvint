//go:build !(darwin || linux)

package cli_test

import (
	"errors"
	"os"
	"time"
)

// The CAL-V0-130 upgrade test skips where the owned process group API is
// unavailable; these stand-ins only keep it compiling there.
func commandGroup(int) (int, error) { return 0, errors.ErrUnsupported }

func retireSurvivor(runner *os.Process, _ int, done chan error, bound time.Duration) error {
	_ = runner.Kill()
	select {
	case err := <-done:
		done <- err
		return nil
	case <-time.After(bound):
		return errors.New("survivor runner not reaped within bound")
	}
}
