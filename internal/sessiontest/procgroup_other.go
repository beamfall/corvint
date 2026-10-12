//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package sessiontest

import "os/exec"

// groupsSupported is false where the run cannot own and kill a process
// group; execution there is INCOMPLETE rather than unkillable (STR-V0-014).
const groupsSupported = false

func ownGroup(*exec.Cmd) {}

func killGroup(*exec.Cmd) {}
