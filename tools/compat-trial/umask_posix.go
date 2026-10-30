//go:build darwin || linux

package main

import "syscall"

func setProcessUmask() {
	syscall.Umask(0o022)
}
