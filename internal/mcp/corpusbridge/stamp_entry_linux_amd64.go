// SPDX-License-Identifier: Apache-2.0
//go:build linux && amd64

package corpusbridge

import (
	"io/fs"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// stampEntryReader keeps a directory descriptor and one metadata buffer per
// chunk (DCR-V1-8000). The returned info is borrowed until the next call. A
// descriptor-relative no-follow stat avoids allocating an absolute path, a
// NUL-terminated copy and FileInfo for every entry. Every call still stats the
// entry: directory times cannot establish freshness of an in-place file edit.
func stampEntryReader(abs string) (func(os.DirEntry) (fs.FileInfo, error), func(), error) {
	dir, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	fd := dir.Fd()
	var info stampFileInfo
	var name [256]byte // Linux NAME_MAX plus the terminating NUL.
	read := func(e os.DirEntry) (fs.FileInfo, error) {
		n := e.Name()
		if len(n) >= len(name) {
			return nil, errStampUnavailable
		}
		copy(name[:], n)
		name[len(n)] = 0
		_, _, errno := syscall.Syscall6(syscall.SYS_NEWFSTATAT, fd,
			uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&info.stat)),
			0x100 /* AT_SYMLINK_NOFOLLOW */, 0, 0)
		runtime.KeepAlive(dir)
		if errno != 0 {
			return nil, errno
		}
		info.name = n
		return &info, nil
	}
	return read, func() { dir.Close() }, nil
}

type stampFileInfo struct {
	name string
	stat syscall.Stat_t
}

func (i *stampFileInfo) Name() string       { return i.name }
func (i *stampFileInfo) Size() int64        { return i.stat.Size }
func (i *stampFileInfo) ModTime() time.Time { return time.Unix(i.stat.Mtim.Unix()) }
func (i *stampFileInfo) Sys() any           { return &i.stat }
func (i *stampFileInfo) IsDir() bool        { return i.stat.Mode&syscall.S_IFMT == syscall.S_IFDIR }
func (i *stampFileInfo) Mode() fs.FileMode {
	mode := fs.FileMode(i.stat.Mode & 0777)
	switch i.stat.Mode & syscall.S_IFMT {
	case syscall.S_IFDIR:
		mode |= fs.ModeDir
	case syscall.S_IFLNK:
		mode |= fs.ModeSymlink
	case syscall.S_IFBLK:
		mode |= fs.ModeDevice
	case syscall.S_IFCHR:
		mode |= fs.ModeDevice | fs.ModeCharDevice
	case syscall.S_IFIFO:
		mode |= fs.ModeNamedPipe
	case syscall.S_IFSOCK:
		mode |= fs.ModeSocket
	}
	for _, bit := range []struct {
		unix   uint32
		goMode fs.FileMode
	}{
		{syscall.S_ISUID, fs.ModeSetuid}, {syscall.S_ISGID, fs.ModeSetgid}, {syscall.S_ISVTX, fs.ModeSticky},
	} {
		if i.stat.Mode&bit.unix != 0 {
			mode |= bit.goMode
		}
	}
	return mode
}
