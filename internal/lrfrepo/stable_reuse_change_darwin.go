package lrfrepo

import (
	"os"
	"syscall"
	"time"
)

// fileChangedBefore reports whether the file's status, including a rename
// onto its path, last changed before start (OCM-V0-027).
func fileChangedBefore(info os.FileInfo, start time.Time) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return time.Unix(stat.Ctimespec.Unix()).Before(start) && info.ModTime().Before(start)
}
