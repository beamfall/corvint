package contextindex

import (
	"os"
	"syscall"
)

// fileIndexStat reads the fields Git's index records from an lstat or fstat
// result, truncated as the index stores them.
func fileIndexStat(info os.FileInfo) (gitIndexStat, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return gitIndexStat{}, false
	}
	return gitIndexStat{
		ctimeSec: uint32(st.Ctim.Sec), ctimeNsec: uint32(st.Ctim.Nsec),
		mtimeSec: uint32(st.Mtim.Sec), mtimeNsec: uint32(st.Mtim.Nsec),
		ino: uint32(st.Ino), uid: st.Uid, gid: st.Gid, size: uint32(st.Size),
	}, true
}
