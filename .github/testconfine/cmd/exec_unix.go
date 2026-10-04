//go:build unix

package main

import (
	"os"
	"syscall"
)

func execUnconfined(binary string, args []string) error {
	return syscall.Exec(binary, args, os.Environ())
}
