//go:build darwin || linux

package worklistadapter

import (
	"os"
	"syscall"
)

func producerReadFlags() int { return os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK }
