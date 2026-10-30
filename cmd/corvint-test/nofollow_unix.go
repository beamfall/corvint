//go:build darwin || linux

package main

import "syscall"

// openNoFollow makes writeFile refuse a final-component symlink at open time.
const openNoFollow = syscall.O_NOFOLLOW
