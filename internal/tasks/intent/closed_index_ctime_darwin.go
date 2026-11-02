// SPDX-License-Identifier: Apache-2.0

package intent

import (
	"os"
	"syscall"
)

// changeTime is info's change time in nanoseconds.
func changeTime(info os.FileInfo) (int64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return st.Ctimespec.Nano(), true
}
