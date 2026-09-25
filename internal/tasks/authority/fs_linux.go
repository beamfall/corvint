//go:build linux

package authority

import (
	"errors"
	"io"
	"os"
	"strconv"
	"syscall"
)

const platform = "linux"

// maxMountinfo bounds the /proc/self/mountinfo read; a larger table leaves
// the ext magic ambiguous.
const maxMountinfo = 4 << 20

// observeFilesystem reports the mounted filesystem of an open directory
// from fstatfs(2) `f_type`. Unknown magic is reported verbatim and refused.
// `f_type` is int64 on 64-bit and int32 on 32-bit targets; the magics fit
// in 32 bits.
func observeFilesystem(dir *os.File) (Filesystem, error) {
	var fs Filesystem
	err := withFD(dir, func(fd int) error {
		var st syscall.Statfs_t
		if err := syscall.Fstatfs(fd, &st); err != nil {
			return err
		}
		fs = linuxFilesystem(fd, uint32(st.Type))
		return nil
	})
	if err != nil {
		return Filesystem{Platform: platform}, err
	}
	return fs, nil
}

// linuxFilesystem names the filesystem of fd. The shared ext2/ext3/ext4
// magic is resolved only from the kernel's record of this descriptor's own
// mount: fdinfo `mnt_id`, then that mount's fstype in mountinfo. No mount
// name or path guess upgrades it; any failed observation stays ambiguous.
func linuxFilesystem(fd int, magic uint32) Filesystem {
	if magic != magicExt4 {
		return filesystemFromMagic(magic)
	}
	fdinfo, err := readProc("/proc/self/fdinfo/"+strconv.Itoa(fd), 4096)
	if err != nil {
		return filesystemFromMagic(magic)
	}
	id, err := linuxMountID(fdinfo)
	if err != nil {
		return filesystemFromMagic(magic)
	}
	mountinfo, err := readProc("/proc/self/mountinfo", maxMountinfo)
	if err != nil {
		return filesystemFromMagic(magic)
	}
	return extFromMountinfo(mountinfo, id)
}

// readProc reads at most limit bytes of a procfs file; a longer file refuses.
func readProc(path string, limit int64) (raw []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	raw, err = io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(raw)) > limit {
		err = errors.New("procfs observation exceeds its bound")
	}
	return raw, err
}

// fullSync is fsync(2), which on the allowed Linux filesystems flushes the
// file's data and metadata to stable storage.
func fullSync(f *os.File) error {
	return f.Sync()
}
