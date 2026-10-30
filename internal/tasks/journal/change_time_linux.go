//go:build linux

package journal

import (
	"os"
	"syscall"
)

// changeTime is the inode change time the platform reports for info, in
// nanoseconds (CAL-V0-224). Unlike the modification time it cannot be set
// back by utimes: restoring a stamp is itself a change.
func changeTime(info os.FileInfo) (int64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, false
	}
	return st.Ctim.Nano(), true
}

// haveChangeTime says changeTime reports a change time on this platform.
const haveChangeTime = true
