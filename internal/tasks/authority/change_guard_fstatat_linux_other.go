//go:build linux && !amd64 && !arm64

package authority

import (
	"bytes"
	"strconv"
	"syscall"
)

// fstatat has no raw form here; it reads the name through /proc/self/fd so
// the directory descriptor still names the directory (V1-2530).
func fstatat(dirfd int, name []byte, st *syscall.Stat_t) error {
	return syscall.Lstat("/proc/self/fd/"+strconv.Itoa(dirfd)+"/"+string(bytes.TrimSuffix(name, []byte{0})), st)
}
