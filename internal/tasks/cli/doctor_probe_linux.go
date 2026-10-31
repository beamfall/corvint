//go:build linux

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const doctorProcBytes = 4 << 20

// doctorLockHolder reports whether a process holds path's flock and names
// it from /proc/locks (TQD-V0-021). An absent lock file is not held.
var doctorLockHolder = func(path string) (bool, int, error) {
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		if errors.Is(err, syscall.ENOENT) {
			return false, 0, nil
		}
		return false, 0, err
	}
	raw, err := readBounded("/proc/locks", doctorProcBytes)
	if err != nil {
		return false, 0, err
	}
	dev := uint64(st.Dev)
	major := ((dev >> 8) & 0xfff) | ((dev >> 32) &^ 0xfff)
	minor := (dev & 0xff) | ((dev >> 12) &^ 0xff)
	pids := parseDoctorProcLocks(string(raw), major, minor, uint64(st.Ino))
	if len(pids) == 0 {
		// TQD-V0-821: the stat device can differ from the one /proc/locks
		// prints (btrfs subvolumes); accept a same-inode holder only through a
		// descriptor on this very file that owns the lock.
		for _, pid := range parseDoctorProcLocksByInode(string(raw), uint64(st.Ino)) {
			if doctorHoldsFlock(pid, uint64(st.Dev), uint64(st.Ino)) {
				return true, pid, nil
			}
		}
		return false, 0, nil
	}
	return true, pids[0], nil
}

// doctorHoldsFlock reads pid's descriptors and asks doctorOwnsFlock whether
// one of them owns a FLOCK on the file with this stat device and inode
// (TQD-V0-821).
func doctorHoldsFlock(pid int, dev, ino uint64) bool {
	dir := fmt.Sprintf("/proc/%d/fd", pid)
	d, err := os.Open(dir)
	if err != nil {
		return false
	}
	names, _ := d.Readdirnames(doctorPsProcs)
	d.Close()
	var fds []doctorFd
	for _, n := range names {
		var st syscall.Stat_t
		if syscall.Stat(filepath.Join(dir, n), &st) != nil {
			continue
		}
		fd := doctorFd{dev: uint64(st.Dev), ino: uint64(st.Ino)}
		if fd.dev == dev && fd.ino == ino {
			if info, err := readBounded(fmt.Sprintf("/proc/%d/fdinfo/%s", pid, n), 64<<10); err == nil {
				fd.fdinfo = string(info)
			}
		}
		fds = append(fds, fd)
	}
	return doctorOwnsFlock(fds, dev, ino)
}

// doctorLockOpeners maps each lock path to the candidate pids with an open
// descriptor on it, from each candidate's /proc/<pid>/fd.
var doctorLockOpeners = func(paths []string, pids []int) (map[string][]int, error) {
	type key struct{ dev, ino uint64 }
	want := map[key]string{}
	for _, p := range paths {
		var st syscall.Stat_t
		if err := syscall.Lstat(p, &st); err == nil {
			want[key{uint64(st.Dev), uint64(st.Ino)}] = p
		}
	}
	out := map[string][]int{}
	for _, pid := range pids {
		dir := fmt.Sprintf("/proc/%d/fd", pid)
		d, err := os.Open(dir)
		if err != nil {
			continue // exited, or not readable as this user
		}
		names, err := d.Readdirnames(doctorPsProcs)
		d.Close()
		if err != nil && len(names) == 0 {
			continue
		}
		for _, n := range names {
			var st syscall.Stat_t
			if syscall.Stat(filepath.Join(dir, n), &st) != nil {
				continue
			}
			if p, ok := want[key{uint64(st.Dev), uint64(st.Ino)}]; ok && !containsInt(out[p], pid) {
				out[p] = append(out[p], pid)
			}
		}
	}
	return out, nil
}

// doctorFileTable reads /proc/sys/fs/file-nr: allocated, unused, maximum.
var doctorFileTable = func() (uint64, uint64, error) {
	raw, err := readBounded("/proc/sys/fs/file-nr", 4096)
	if err != nil {
		return 0, 0, err
	}
	return parseDoctorFileNr(string(raw))
}
