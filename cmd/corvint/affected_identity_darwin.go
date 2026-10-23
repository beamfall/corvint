package main

import (
	"io/fs"
	"os"
	"syscall"
	"time"
)

// affectedOpenNonblocking keeps an open from waiting on a FIFO; the
// descriptor's type is checked before any read.
const affectedOpenNonblocking = syscall.O_NONBLOCK

// affectedExecutable names the executable to hash. Darwin has no handle on the
// running image, so the pathname is checked against the process start time.
func affectedExecutable() (string, bool) {
	name, err := os.Executable()
	if err != nil {
		return "", false
	}
	return name, false
}

func affectedChangeTime(info fs.FileInfo) time.Time {
	if raw, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(raw.Ctimespec.Unix())
	}
	return time.Now()
}
