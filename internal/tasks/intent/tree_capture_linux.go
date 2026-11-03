// SPDX-License-Identifier: Apache-2.0

package intent

import (
	"os"
	"syscall"
	"time"
)

func sameCaptureFile(a, b os.FileInfo, capturedAt time.Time) bool {
	if !sameCaptureKey(a, b) {
		return false
	}
	x := a.Sys().(*syscall.Stat_t)
	return capturedAt.Sub(time.Unix(int64(x.Ctim.Sec), int64(x.Ctim.Nsec))) > 2*time.Second
}

func sameCaptureKey(a, b os.FileInfo) bool {
	if a == nil || b == nil || !a.Mode().IsRegular() || a.Mode() != b.Mode() || a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime()) || !os.SameFile(a, b) {
		return false
	}
	x, ok := a.Sys().(*syscall.Stat_t)
	y, yes := b.Sys().(*syscall.Stat_t)
	// Unlike mtime, ctime cannot be restored by a normal filesystem writer.
	// Require nanosecond change stamps; fail back to content reads otherwise.
	// This is only a planning hint: MAP_SHARED writes may change neither
	// timestamp, so pre-effect binding must hash live content independently.
	return ok && yes && x.Ctim.Nsec != 0 && y.Ctim.Nsec != 0 && x.Ctim == y.Ctim
}
