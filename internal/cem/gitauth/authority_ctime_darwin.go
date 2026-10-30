package gitauth

import (
	"os"
	"syscall"
)

func authorityChangeTime(info os.FileInfo) (int64, int64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return int64(stat.Ctimespec.Sec), int64(stat.Ctimespec.Nsec), true
}
