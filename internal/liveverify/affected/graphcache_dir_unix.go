//go:build darwin || linux

package affected

import (
	"io/fs"
	"os"
	"syscall"
)

// openNonblocking opens a cache entry without waiting on a FIFO swapped in
// for it; the descriptor's type is checked before any read (AFP-V0-392).
const openNonblocking = syscall.O_NONBLOCK

// privateDirectory reports a directory this user owns that no other user may
// write.
func privateDirectory(info fs.FileInfo) bool {
	raw, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.IsDir() && int(raw.Uid) == os.Geteuid() && info.Mode().Perm()&0o022 == 0
}
