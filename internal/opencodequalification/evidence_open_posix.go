//go:build darwin || linux

package opencodequalification

import "syscall"

// evidenceOpenFlags refuses a symlink swapped in after lstat and never blocks opening a FIFO.
const evidenceOpenFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK
