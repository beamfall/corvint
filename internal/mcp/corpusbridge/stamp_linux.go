// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"io/fs"
	"syscall"
)

// fileTimes reads the inode and the change and modification times, in
// nanoseconds, from an lstat result.
func fileTimes(info fs.FileInfo) (ino uint64, ctime, mtime int64, ok bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, 0, false
	}
	return uint64(st.Ino), st.Ctim.Nano(), st.Mtim.Nano(), true
}
