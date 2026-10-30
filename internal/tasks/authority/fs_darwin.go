//go:build darwin

package authority

import (
	"os"
	"syscall"
)

const platform = "darwin"

// mntLocal is MNT_LOCAL from <sys/mount.h>: the filesystem is stored
// locally. It is a stable Darwin ABI constant not exported by syscall.
const mntLocal = 0x00001000

// observeFilesystem reports the mounted filesystem of an open directory
// from fstatfs(2): `f_fstypename` (apfs, hfs, nfs, smbfs, exfat, ...) and
// the MNT_LOCAL flag. Nothing is assumed from the path.
func observeFilesystem(dir *os.File) (Filesystem, error) {
	var st syscall.Statfs_t
	if err := withFD(dir, func(fd int) error { return syscall.Fstatfs(fd, &st) }); err != nil {
		return Filesystem{Platform: platform}, err
	}
	return Filesystem{
		Platform: platform,
		Type:     cString(st.Fstypename[:]),
		Local:    st.Flags&mntLocal != 0,
	}, nil
}

// fullSync deliberately bypasses File.Sync: pinned Go 1.27 falls back to
// fsync on ENOTSUP. No weaker durability operation is allowed here.
var fullSyncCall = func(fd int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_FULLFSYNC, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func fullSync(f *os.File) error {
	return withFD(f, func(fd int) error {
		for {
			err := fullSyncCall(fd)
			if err != syscall.EINTR {
				return err
			}
		}
	})
}
