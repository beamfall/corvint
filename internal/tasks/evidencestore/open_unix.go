//go:build unix

package evidencestore

import "syscall"

// openNonblock keeps a FIFO from blocking a receipt open.
const openNonblock = syscall.O_NONBLOCK
