package main

import (
	"io/fs"
	"syscall"
	"time"
)

// affectedOpenNonblocking keeps an open from waiting on a FIFO; the
// descriptor's type is checked before any read.
const affectedOpenNonblocking = syscall.O_NONBLOCK

// affectedExecutable names the running image itself: /proc/self/exe opens the
// file this process was started from even after its pathname is replaced.
func affectedExecutable() (string, bool) { return "/proc/self/exe", true }

func affectedChangeTime(info fs.FileInfo) time.Time {
	if raw, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(raw.Ctim.Unix())
	}
	return time.Now()
}
