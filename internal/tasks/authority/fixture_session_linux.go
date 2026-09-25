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

func fixtureObserveMount(f *os.File) (result fixtureMount, err error) {
	err = safeopen.Control(f, func(fd uintptr) (err error) {
		var st syscall.Stat_t
		var fs syscall.Statfs_t
		if err = syscall.Fstat(int(fd), &st); err != nil {
			return err
		}
		if err = syscall.Fstatfs(int(fd), &fs); err != nil {
			return err
		}
		observed := filesystemFromMagic(uint32(fs.Type))
		if !observed.Local || !Classify("linux", observed.Type) {
			return fixtureUnsupported
		}
		// Linux fdinfo mnt_id identifies the mount of THIS pinned descriptor,
		// including bind mounts sharing st_dev. This read is observation only.
		// Missing procfs/mnt_id, duplicate fields, or oversized observations refuse.
		info, err := os.Open("/proc/self/fdinfo/" + strconv.FormatUint(uint64(fd), 10))
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, info.Close()) }()
		raw, err := io.ReadAll(io.LimitReader(info, 4097))
		if err != nil {
			return err
		}
		mount, err := fixtureLinuxMountID(raw)
		if err != nil {
			return err
		}
		result = fixtureMount{uint64(st.Dev), fmt.Sprintf("%x:%v", uint64(fs.Type), fs.Fsid), mount}
		return nil
	})
	return result, err
}
func fixtureLinuxMountID(raw []byte) (string, error) {
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
