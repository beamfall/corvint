//go:build linux

package receiptbundle

import (
	"os"
	"syscall"
	"time"
)

// changeTime returns the inode change time, which a writer cannot set back.
func changeTime(info os.FileInfo) (time.Time, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(stat.Ctim.Unix()), true
}
