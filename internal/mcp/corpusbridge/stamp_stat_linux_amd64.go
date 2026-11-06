// SPDX-License-Identifier: Apache-2.0

//go:build linux && amd64

package corpusbridge

import (
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// describeAt uses the opened directory and one worker-local stat buffer. The
// ordinary DirEntry.Info path constructs a full path and FileInfo per entry.
// Relative no-follow stats retain the same metadata and abstentions, without
// those allocations on the 17k-input warm path (DCR-V1-7802).
func (s *stamper) describeAt(dir *os.File, entries []os.DirEntry) stampChunk {
	capacity, longest := 48*len(entries), 0
	for _, e := range entries {
		capacity += len(e.Name())
		longest = max(longest, len(e.Name()))
	}
	c := stampChunk{b: make([]byte, 0, capacity), dirs: make([]bool, len(entries)), coarse: true}
	name := make([]byte, longest+1)
	info := &stampStatInfo{}
	for i, e := range entries {
		if i%stampReadBatch == stampReadBatch-1 && s.ctx.Err() != nil {
			c.err = s.ctx.Err()
			return c
		}
		info.name = e.Name()
		copy(name, info.name)
		name[len(info.name)] = 0
		// AT_SYMLINK_NOFOLLOW is 0x100 on Linux. SYS_NEWFSTATAT and
		// syscall.Stat_t are matched by this file's linux/amd64 build tag.
		_, _, errno := syscall.Syscall6(syscall.SYS_NEWFSTATAT, dir.Fd(), uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&info.stat)), 0x100, 0, 0)
		runtime.KeepAlive(dir)
		if errno != 0 {
			c.err = errStampUnavailable
			return c
		}
		var whole bool
		c.b, whole, c.err = s.line(c.b, info.name, info)
		if c.err != nil {
			return c
		}
		c.coarse = c.coarse && whole
		c.dirs[i] = info.IsDir()
	}
	return c
}

// stampStatInfo exposes exactly os.Lstat's Linux metadata to the shared
// timestamp reader, including the test seam for coarse/skewed filesystems.
type stampStatInfo struct {
	name string
	stat syscall.Stat_t
}

func (i *stampStatInfo) Name() string       { return i.name }
func (i *stampStatInfo) Size() int64        { return i.stat.Size }
func (i *stampStatInfo) ModTime() time.Time { return time.Unix(i.stat.Mtim.Unix()) }
func (i *stampStatInfo) IsDir() bool        { return i.stat.Mode&syscall.S_IFMT == syscall.S_IFDIR }
func (i *stampStatInfo) Sys() any           { return &i.stat }
func (i *stampStatInfo) Mode() os.FileMode {
	m := os.FileMode(i.stat.Mode & 0777)
	switch i.stat.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		m |= os.ModeDevice
	case syscall.S_IFCHR:
		m |= os.ModeDevice | os.ModeCharDevice
	case syscall.S_IFDIR:
		m |= os.ModeDir
	case syscall.S_IFIFO:
		m |= os.ModeNamedPipe
	case syscall.S_IFLNK:
		m |= os.ModeSymlink
	case syscall.S_IFSOCK:
		m |= os.ModeSocket
	}
	if i.stat.Mode&syscall.S_ISGID != 0 {
		m |= os.ModeSetgid
	}
	if i.stat.Mode&syscall.S_ISUID != 0 {
		m |= os.ModeSetuid
	}
	if i.stat.Mode&syscall.S_ISVTX != 0 {
		m |= os.ModeSticky
	}
	return m
}
