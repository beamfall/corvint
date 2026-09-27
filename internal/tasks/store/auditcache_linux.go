//go:build linux

package store

import (
	"os"
	"syscall"
)

// fileStamp is the metadata the audit cache compares. Any content or
// metadata change moves a file's change time, which user space cannot set.
func fileStamp(info os.FileInfo) (stamp, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return stamp{}, false
	}
	return stamp{Mode: uint32(info.Mode()), Size: info.Size(), Mtime: info.ModTime().UnixNano(), Ctime: st.Ctim.Nano(), Ino: st.Ino, Dev: uint64(st.Dev)}, true
}
