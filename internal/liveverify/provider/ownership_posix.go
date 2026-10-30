//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package provider

import (
	"os"
	"syscall"
)

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

func supportedPlatform() bool { return true }
