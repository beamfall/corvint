//go:build darwin || linux

package cli

// GroupPresent reports whether a non-zombie process is in the process group.
func GroupPresent(pgid int) bool { return groupPresent(pgid) }
