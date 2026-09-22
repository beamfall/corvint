//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package parentverify

import (
	"os"
	"syscall"
)

func fileLinkCount(info os.FileInfo) uint64 {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return uint64(stat.Nlink)
}
