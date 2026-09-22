//go:build linux

package main

import (
	"os"
	"syscall"
)

func main() {
	if err := syscall.Exec("/proc/self/fd/3", []string{"corvint-shadow-external-artifact"}, []string{}); err != nil {
		os.Exit(127)
	}
}
