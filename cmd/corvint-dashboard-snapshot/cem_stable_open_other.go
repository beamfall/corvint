//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package main

// cemStableOpenFlags is empty on hosts without Unix FIFO files.
const cemStableOpenFlags = 0
