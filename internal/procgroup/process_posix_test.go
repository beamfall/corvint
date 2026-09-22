//go:build darwin || linux

package procgroup

import "syscall"

// killTestProcessGroup sends SIGKILL straight to the OS process group led by
// pgid, bypassing any grace period. It exists only so process_test.go's
// group-directed kill assertions (which are inherently POSIX process-group
// semantics) still compile on platforms where process_other.go's contract
// has no process-group equivalent (see TestRunProcessNonPOSIXContract).
func killTestProcessGroup(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGKILL)
}
