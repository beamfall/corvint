//go:build linux

package authority

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
)

const (
	fixtureLinkat   = syscall.SYS_LINKAT
	fixtureUnlinkat = syscall.SYS_UNLINKAT
	fixtureMkdirat  = syscall.SYS_MKDIRAT
)

// fixtureObserveMount observes the mount identity of f and qualifies its
// filesystem. held is the identity this same, continuously open descriptor
// reported when it was qualified, or zero. An open descriptor keeps its
// mount alive, so its mnt_id cannot be reused and an identity equal to held
// is the mount already qualified; only then is qualification not repeated.
func fixtureObserveMount(f *os.File, held fixtureMount) (fixtureMount, error) {
	return observeMountWith(f, held, func(fd uintptr) ([]byte, error) {
		info, err := os.Open(fdinfoPath(fd))
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(io.LimitReader(info, 4097))
		return raw, errors.Join(err, info.Close())
	})
}

// observeMount is fixtureObserveMount for a session's pinned descriptor. It
// keeps that descriptor's fdinfo file open for the session and re-reads it
// from offset 0 on each check. procfs regenerates fdinfo for the
// descriptor's current state on every such read, so each check still
// observes mnt_id from fdinfo (decision 0411) without a path walk, open and
// close per parent (LGL-V0-002, proposed).
func (s *fixtureSession) observeMount(f *os.File, held fixtureMount) (fixtureMount, error) {
	return observeMountWith(f, held, func(fd uintptr) ([]byte, error) {
		info := s.fdinfo[f]
		if info == nil {
			opened, err := os.Open(fdinfoPath(fd))
			if err != nil {
				return nil, err
			}
			if s.fdinfo == nil {
				s.fdinfo = map[*os.File]*os.File{}
			}
			s.fdinfo[f], info = opened, opened
		}
		return readFDInfoAt(info)
	})
}

func fdinfoPath(fd uintptr) string {
	return "/proc/self/fdinfo/" + strconv.FormatUint(uint64(fd), 10)
}

// readFDInfoAt reads info from offset 0 to end of file, at most 4097 bytes
// so linuxMountID refuses an oversized observation as before.
func readFDInfoAt(info *os.File) ([]byte, error) {
	buf := make([]byte, 4097)
	n := 0
	for n < len(buf) {
		m, err := info.ReadAt(buf[n:], int64(n))
		n += m
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if m == 0 {
			break
		}
	}
	return buf[:n], nil
}

func observeMountWith(f *os.File, held fixtureMount, readInfo func(fd uintptr) ([]byte, error)) (result fixtureMount, err error) {
	err = safeopen.Control(f, func(fd uintptr) (err error) {
		var st syscall.Stat_t
		var fs syscall.Statfs_t
		if err = syscall.Fstat(int(fd), &st); err != nil {
			return err
		}
		if err = syscall.Fstatfs(int(fd), &fs); err != nil {
			return err
		}
		// Linux fdinfo mnt_id identifies the mount of THIS pinned descriptor,
		// including bind mounts sharing st_dev. This read is observation only.
		// Missing procfs/mnt_id, duplicate fields, or oversized observations refuse.
		raw, err := readInfo(fd)
		if err != nil {
			return err
		}
		mount, err := linuxMountID(raw)
		if err != nil {
			return err
		}
		observed := fixtureMount{uint64(st.Dev), fmt.Sprintf("%x:%v", uint64(fs.Type), fs.Fsid), mount}
		if observed != held {
			named := mountFilesystem(uint32(fs.Type), mount)
			if !named.Local || !Classify("linux", named.Type) {
				return fixtureUnsupported
			}
		}
		result = observed
		return nil
	})
	return result, err
}
func linuxMountID(raw []byte) (string, error) {
	if len(raw) > 4096 {
		return "", fixtureRefused
	}
	mount := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "mnt_id:") {
			continue
		}
		fields := strings.Fields(line)
		if mount != "" || len(fields) != 2 {
			return "", fixtureRefused
		}
		id, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || id == 0 {
			return "", fixtureRefused
		}
		mount = strconv.FormatUint(id, 10)
	}
	if mount == "" {
		return "", fixtureRefused
	}
	return mount, nil
}

func fixtureRename(a *os.File, from string, b *os.File, to string, source, dest *os.File) error {
	return fixturePair(a, b, source, dest, func(afd, bfd uintptr) error { return syscall.Renameat(int(afd), from, int(bfd), to) })
}
