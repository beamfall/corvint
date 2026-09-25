//go:build linux

package fixture

import (
	"os"
	"syscall"
)

// Linux ext2/ext3/ext4 share one statfs magic, which the qualification
// refuses as ambiguous. When the default temp dir carries that magic,
// relocate to /dev/shm if it is tmpfs.
const (
	magicExt4  = 0xEF53
	magicTmpfs = 0x01021994
	shm        = "/dev/shm"
)

func qualifiedTempDir() string {
	var st syscall.Statfs_t
	if syscall.Statfs(os.TempDir(), &st) != nil || uint32(st.Type) != magicExt4 {
		return ""
	}
	if syscall.Statfs(shm, &st) != nil || uint32(st.Type) != magicTmpfs {
		return ""
	}
	return shm
}
