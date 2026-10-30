//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import "syscall"

// cemStableOpenFlags makes opening a FIFO or device return at once, so the
// descriptor's own type check refuses it instead of blocking.
const cemStableOpenFlags = syscall.O_NONBLOCK
