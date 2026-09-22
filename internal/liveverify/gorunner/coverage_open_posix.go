//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package gorunner

import (
	"os"
	"syscall"
)

func openCoverageNoFollow(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
}

func coverageOwnedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && uint64(stat.Uid) == uint64(os.Geteuid())
}

func coverageSingleLink(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && uint64(stat.Nlink) == 1
}
