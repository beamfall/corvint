// SPDX-License-Identifier: Apache-2.0

package intent

import (
	"os"
	"syscall"
	"time"
)

func sameCaptureFile(a, b os.FileInfo, capturedAt time.Time) bool {
	if a == nil || b == nil || !a.Mode().IsRegular() || a.Mode() != b.Mode() || a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime()) || !os.SameFile(a, b) {
		return false
	}
	x, ok := a.Sys().(*syscall.Stat_t)
	y, yes := b.Sys().(*syscall.Stat_t)
	// Unlike mtime, ctime cannot be restored by a normal filesystem writer.
	// Require nanosecond change stamps; fail back to content reads otherwise.
	return ok && yes && x.Ctim.Nsec != 0 && y.Ctim.Nsec != 0 && x.Ctim == y.Ctim && capturedAt.Sub(time.Unix(int64(x.Ctim.Sec), int64(x.Ctim.Nsec))) > 2*time.Second
}
