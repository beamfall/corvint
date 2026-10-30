//go:build !darwin && !linux

package main

import (
	"io/fs"
	"time"
)

const affectedOpenNonblocking = 0

// affectedExecutable names no executable: the graph cache is never used on
// this platform (AFP-V0-392), so no identity is needed.
func affectedExecutable() (string, bool) { return "", false }

func affectedChangeTime(info fs.FileInfo) time.Time { return info.ModTime() }
