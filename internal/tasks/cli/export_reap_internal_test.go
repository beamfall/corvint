//go:build darwin || linux

package cli

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// GroupRunning reports whether a live, non-zombie process is in the process
// group. It only observes; an unreadable process table is an error.
func GroupRunning(pgid int) (bool, error) {
	table, err := groupreap.ReadProcessTable()
	if err != nil {
		return false, fmt.Errorf("read the process table: %w", err)
	}
	for _, p := range table {
		if p.PGID == pgid && p.State != groupreap.StateZombie {
			return true, nil
		}
	}
	return false, nil
}
