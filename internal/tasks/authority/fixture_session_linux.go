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
func fixtureObserveMount(f *os.File, held fixtureMount) (result fixtureMount, err error) {
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
		info, err := os.Open("/proc/self/fdinfo/" + strconv.FormatUint(uint64(fd), 10))
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, info.Close()) }()
		raw, err := io.ReadAll(io.LimitReader(info, 4097))
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
