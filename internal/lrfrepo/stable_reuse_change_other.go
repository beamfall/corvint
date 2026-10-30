//go:build !darwin

package lrfrepo

import (
	"os"
	"time"
)

// fileChangedBefore reports whether the file's contents last changed before
// start (OCM-V0-027). Linux reads the running image itself and never calls it.
func fileChangedBefore(info os.FileInfo, start time.Time) bool {
	return info.ModTime().Before(start)
}
