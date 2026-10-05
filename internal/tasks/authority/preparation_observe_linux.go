//go:build linux

package authority

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// maxProcLocks bounds the /proc/locks read; a larger table is not observed.
const maxProcLocks = 4 << 20

// initialPIDNamespace is the nsfs link of the kernel's initial PID namespace
// (PROC_PID_INIT_INO, fixed since Linux 3.8).
const initialPIDNamespace = "pid:[4026531836]"

// Linux fcntl(F_GETLK) does not report flock(2) owners, so liveness comes
// from one read-only /proc/locks snapshot. The kernel omits every lock whose
// owner is not visible in the procfs instance's PID namespace, so the table
// is complete only when that namespace is the initial one.
type procLocksView struct {
	// locked maps an inode number to the "major:minor" devices whose flock
	// it carries.
	locked map[uint64][]string
}

var (
	procLocksPath = "/proc/locks"
	// procSelfPIDNamespace resolves through the same procfs mount as
	// procLocksPath; it exists only when the reader is visible there.
	procSelfPIDNamespace = "/proc/self/ns/pid"
)

func loadPreparationLockView() (preparationLockView, error) {
	if err := procLocksComplete(); err != nil {
		return nil, err
	}
	raw, err := readProc(procLocksPath, maxProcLocks)
	if err != nil {
		return nil, errors.New("lock table is not observable")
	}
	return parseProcLocks(string(raw))
}

// procLocksComplete establishes that /proc/locks lists every live owner: the
// reader is in the initial PID namespace and visible in this procfs mount,
// whose namespace is therefore the initial one too. Anything else abstains,
// since writers in other namespaces would be omitted and read as absent.
func procLocksComplete() error {
	ns, err := os.Readlink(procSelfPIDNamespace)
	if err != nil || ns != initialPIDNamespace {
		return errors.New("lock table omits owners outside this PID namespace")
	}
	return nil
}

// parseProcLocks keeps granted FLOCK entries only. The preparation protocol
// is flock(2); on local Linux filesystems POSIX and OFD record locks neither
// conflict with it nor mark a registration, and blocked waiters ("->"),
// leases and delegations convey no slot ownership.
func parseProcLocks(table string) (procLocksView, error) {
	v := procLocksView{locked: map[uint64][]string{}}
	for _, line := range strings.Split(table, "\n") {
		f := strings.Fields(line)
		if len(f) < 6 || f[1] != "FLOCK" {
			continue
		}
		parts := strings.Split(f[5], ":")
		if len(parts) != 3 {
			return v, errors.New("lock table entry is malformed")
		}
		major, e1 := strconv.ParseUint(parts[0], 16, 32)
		minor, e2 := strconv.ParseUint(parts[1], 16, 32)
		ino, e3 := strconv.ParseUint(parts[2], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil {
			return v, errors.New("lock table entry is malformed")
		}
		v.locked[ino] = append(v.locked[ino], strconv.FormatUint(major, 10)+":"+strconv.FormatUint(minor, 10))
	}
	return v, nil
}

func (procLocksView) method() string { return "proc-locks" }

// held matches the file's device and inode. An entry for the same inode on a
// different device refuses: the device mapping is then ambiguous (for
// example an overlay or subvolume), and absence would be an invented fact.
func (v procLocksView) held(_ *os.File, st os.FileInfo) (bool, error) {
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return false, errors.New("file identity is not observable")
	}
	dev := uint64(sys.Dev)
	major := (dev>>8)&0xfff | (dev>>32)&^uint64(0xfff)
	minor := dev&0xff | (dev>>12)&^uint64(0xff)
	want := strconv.FormatUint(major, 10) + ":" + strconv.FormatUint(minor, 10)
	devices := v.locked[uint64(sys.Ino)]
	for _, d := range devices {
		if d == want {
			return true, nil
		}
	}
	if len(devices) != 0 {
		return false, errors.New("lock table device mapping is ambiguous")
	}
	return false, nil
}
