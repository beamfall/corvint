//go:build linux

package authority

import (
	"os"
	"syscall"
)

const platform = "linux"

// observeFilesystem reports the mounted filesystem of an open directory
// from fstatfs(2) `f_type`. Shared ext2/ext3/ext4 magic is ambiguous and
// refused; no mount-name or path guess upgrades it to ext4. Unknown magic
// is reported verbatim and refused. `f_type` is int64
// on 64-bit and int32 on 32-bit targets; the magics fit in 32 bits.
func observeFilesystem(dir *os.File) (Filesystem, error) {
	var st syscall.Statfs_t
	if err := withFD(dir, func(fd int) error { return syscall.Fstatfs(fd, &st) }); err != nil {
		return Filesystem{Platform: platform}, err
	}
	return filesystemFromMagic(uint32(st.Type)), nil
}

// fullSync is fsync(2), which on the allowed Linux filesystems flushes the
// file's data and metadata to stable storage.
func fullSync(f *os.File) error {
	return f.Sync()
}
