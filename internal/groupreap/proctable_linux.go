//go:build linux

package groupreap

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"syscall"
)

// readProcessTable reads every /proc/<pid>/stat under the fixed entry and
// size bounds. It sends no signal and grants no signal authority.
func readProcessTable() (map[int]Process, error) {
	return readProcTable("/proc")
}

func processSession(pid int) (int, error) {
	data, err := readProcFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"), maxProcStat)
	if err != nil {
		return 0, err
	}
	if _, err := parseProcStat(data, pid); err != nil {
		return 0, err
	}
	fields := bytes.Fields(data[bytes.LastIndexByte(data, ')')+1:])
	if len(fields) < 4 {
		return 0, fmt.Errorf("%w: short stat for %d", errProcessTable, pid)
	}
	return strconv.Atoi(string(fields[3]))
}

// pidfd system call numbers, the same on every Linux architecture Go supports.
const (
	sysPidfdSendSignal = 424
	sysPidfdOpen       = 434
)

// signalPinned signals p through a pidfd opened before p's start time is
// rechecked, so a PID released and reused after the check cannot receive the
// signal. A group signal is accepted only when the pinned leader still
// exists after it: its PID, and so its group ID, was never released between
// the check and the signal. A kernel without pidfds fails closed.
func signalPinned(p Process, group bool, sig syscall.Signal) error {
	r, _, errno := syscall.Syscall(sysPidfdOpen, uintptr(p.PID), 0, 0)
	if errno == syscall.ESRCH {
		return syscall.ESRCH
	}
	if errno != 0 {
		return fmt.Errorf("groupreap: pidfd_open %d: %w", p.PID, errno)
	}
	fd := int(r)
	defer syscall.Close(fd)
	data, err := readProcFile(filepath.Join("/proc", strconv.Itoa(p.PID), "stat"), maxProcStat)
	if procVanished(err) {
		return syscall.ESRCH
	}
	if err != nil {
		return err
	}
	q, err := parseProcIdentity(data, p.PID)
	if err != nil {
		return err
	}
	if !q.same(p) {
		return errIdentityChanged
	}
	if !group {
		return pidfdSignal(fd, sig)
	}
	if err := syscall.Kill(-p.PID, sig); err != nil {
		return err
	}
	if err := pidfdSignal(fd, 0); err != nil {
		return fmt.Errorf("groupreap: session leader %s was released during its group signal: %w", p, err)
	}
	return nil
}

func pidfdSignal(fd int, sig syscall.Signal) error {
	if _, _, errno := syscall.Syscall6(sysPidfdSendSignal, uintptr(fd), uintptr(sig), 0, 0, 0, 0); errno != 0 {
		return errno
	}
	return nil
}
