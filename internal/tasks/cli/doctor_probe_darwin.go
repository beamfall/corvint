//go:build darwin

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// doctorLockHolder reports whether another process holds path's flock
// (TQD-V0-021). Darwin's F_GETLK sees a flock as a whole-file write lock
// whose holder pid is not recorded, so pid is 0 unless the kernel names it.
// An absent lock file is not held. The file is opened read-only, never
// created, and no lock is taken.
var doctorLockHolder = func(path string) (bool, int, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	defer f.Close()
	lk := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0}
	if err := syscall.FcntlFlock(f.Fd(), syscall.F_GETLK, &lk); err != nil {
		return false, 0, err
	}
	if lk.Type == syscall.F_UNLCK {
		return false, 0, nil
	}
	pid := 0
	if lk.Pid > 0 {
		pid = int(lk.Pid)
	}
	return true, pid, nil
}

// doctorLockOpeners maps each lock path to the candidate pids that have it
// open, from one bounded lsof call restricted to those pids and files.
var doctorLockOpeners = func(paths []string, pids []int) (map[string][]int, error) {
	inodes := map[uint64]string{}
	var files []string
	for _, p := range paths {
		var st syscall.Stat_t
		if err := syscall.Lstat(p, &st); err == nil {
			inodes[st.Ino] = p
			files = append(files, p)
		}
	}
	out := map[string][]int{}
	if len(files) == 0 || len(pids) == 0 {
		return out, nil
	}
	ids := make([]string, len(pids))
	for i, p := range pids {
		ids[i] = strconv.Itoa(p)
	}
	ctx, cancel := context.WithTimeout(context.Background(), doctorPsTimeout)
	defer cancel()
	args := append([]string{"-n", "-P", "-w", "-a", "-p", strings.Join(ids, ","), "-F", "pi", "--"}, files...)
	c := exec.CommandContext(ctx, "/usr/sbin/lsof", args...)
	c.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin:/usr/sbin"}
	buf := &doctorLimited{max: doctorPsBytes}
	c.Stdout = buf
	err := c.Run()
	// lsof exits 1 when some listed pid or file has no match.
	var ee *exec.ExitError
	if err != nil && !(errors.As(err, &ee) && ee.ExitCode() == 1 && ctx.Err() == nil) {
		return nil, err
	}
	if buf.over {
		return nil, errors.New("lsof output larger than the walk bound")
	}
	for ino, ps := range parseDoctorLsof(buf.buf.String()) {
		if p, ok := inodes[ino]; ok {
			out[p] = ps
		}
	}
	return out, nil
}

// doctorFileTable reads the system file table's use and size.
var doctorFileTable = func() (uint64, uint64, error) {
	used, err := syscall.SysctlUint32("kern.num_files")
	if err != nil {
		return 0, 0, err
	}
	max, err := syscall.SysctlUint32("kern.maxfiles")
	if err != nil {
		return 0, 0, err
	}
	return uint64(used), uint64(max), nil
}
