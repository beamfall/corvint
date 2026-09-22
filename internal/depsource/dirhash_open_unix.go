//go:build darwin || linux

package depsource

import (
	"os"
	"syscall"
)

// cacheFileOpenFlags refuses a final symlink and keeps a FIFO open from blocking.
const cacheFileOpenFlags = os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
