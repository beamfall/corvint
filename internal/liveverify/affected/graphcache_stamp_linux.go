package affected

import (
	"io/fs"
	"syscall"
)

// statStamp reads the identity a graph cache entry pins for one observed path.
func statStamp(info fs.FileInfo) (pathStamp, bool) {
	raw, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return pathStamp{}, false
	}
	return pathStamp{
		Present: true, Mode: uint32(info.Mode()), Size: info.Size(),
		Mtime: raw.Mtim.Nano(), Ctime: raw.Ctim.Nano(),
		Inode: uint64(raw.Ino), Device: uint64(raw.Dev),
	}, true
}
