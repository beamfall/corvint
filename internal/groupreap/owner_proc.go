//go:build darwin || linux

package groupreap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// Linux signal 0 succeeds while a group holds only zombies, including the
// owner's own unreaped leader, so it cannot show that no live member remains.
// procGroupQuiet makes that pre-reap observation from /proc instead
// (PGO-V0-006). It sends no signal and grants no signal authority.

const (
	// maxProcEntries bounds one directory listing; it is Linux's pid_max limit.
	maxProcEntries = 1 << 22
	// maxProcStat bounds one stat read; a stat line is far shorter.
	maxProcStat = 4096
	// maxProcStatus bounds one status read; a status file is far shorter.
	maxProcStatus = 16384
)

var errProcProof = errors.New("groupreap: /proc quiet proof unavailable")

type procStat struct {
	pid, ppid, pgrp int
	state           byte
}

func procDead(state byte) bool { return state == 'Z' || state == 'X' }

// procGroupQuiet reports ProbeQuiet only when the exited, unreaped leader is
// identified as root's child of parent leading its own group, and every
// process of that group under root is a zombie or dead with no other thread.
// A live member is ProbeLive. Any failure to obtain that proof is an error, so the
// owner HOLDs; it never reports ProbeAbsent.
func procGroupQuiet(root string, leader, parent int) (Probe, error) {
	if err := procLeaderIdentity(root, leader, parent); err != nil {
		return ProbeLive, err
	}
	sawLeader := false
	// One stat buffer serves the whole scan, so its allocation does not grow
	// with the host's process count.
	buf := make([]byte, maxProcStat+1)
	live, err := procEachEntry(root, func(pid int) (bool, error) {
		stat, err := readProcStatInto(filepath.Join(root, strconv.Itoa(pid), "stat"), pid, buf)
		if procVanished(err) {
			return false, nil
		}
		if err != nil || stat.pgrp != leader {
			return false, err
		}
		sawLeader = sawLeader || pid == leader
		if !procDead(stat.state) {
			return true, nil
		}
		return procThreadLive(root, pid)
	})
	switch {
	case err != nil:
		return ProbeLive, err
	case live:
		return ProbeLive, nil
	case !sawLeader:
		return ProbeLive, fmt.Errorf("%w: leader %d absent from the group scan", errProcProof, leader)
	}
	// The leader stays unreaped until the owner reaps it; re-check that the
	// scanned group is still the one it pins.
	if err := procLeaderIdentity(root, leader, parent); err != nil {
		return ProbeLive, err
	}
	return ProbeQuiet, nil
}

func procLeaderIdentity(root string, leader, parent int) error {
	if leader <= 1 || parent <= 0 {
		return fmt.Errorf("%w: invalid leader %d or parent %d", errProcProof, leader, parent)
	}
	stat, err := readProcStat(filepath.Join(root, strconv.Itoa(leader), "stat"), leader)
	if err != nil {
		return fmt.Errorf("%w: leader %d: %w", errProcProof, leader, err)
	}
	if stat.pgrp != leader || stat.ppid != parent || !procDead(stat.state) {
		return fmt.Errorf("%w: leader %d is not the exited unreaped group leader of %d (state %q ppid %d pgrp %d)",
			errProcProof, leader, parent, stat.state, stat.ppid, stat.pgrp)
	}
	return nil
}

// procThreadLive reports whether a zombie-state process still has another
// thread (a thread-group leader that exited before its other threads). It
// reads the status Threads count, which keeps counting a thread until the
// thread is released, rather than walking task/: task/ is enumerated by
// ordinal, so threads exiting between reads can hide a surviving thread.
func procThreadLive(root string, pid int) (bool, error) {
	data, err := readProcFile(filepath.Join(root, strconv.Itoa(pid), "status"), maxProcStatus)
	if procVanished(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		value, ok := bytes.CutPrefix(line, []byte("Threads:"))
		if !ok {
			continue
		}
		threads, err := strconv.Atoi(string(bytes.TrimSpace(value)))
		if err != nil || threads < 0 {
			break
		}
		// Linux prints Threads 0 when the task was released (reaped) between
		// opening status and reading its signal state: it has no thread left,
		// as if status had vanished (PGO-V0-016, V1-1244).
		return threads > 1, nil
	}
	return false, fmt.Errorf("%w: no thread count for %d", errProcProof, pid)
}

// procEachEntry visits the numeric entries of dir, at most maxProcEntries,
// stopping at the first visit that reports true or fails.
func procEachEntry(dir string, visit func(int) (bool, error)) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()
	seen := 0
	for {
		entries, err := f.ReadDir(256)
		for _, entry := range entries {
			id, convErr := strconv.Atoi(entry.Name())
			if convErr != nil || id <= 0 {
				continue
			}
			if seen++; seen > maxProcEntries {
				return false, fmt.Errorf("%w: %s exceeds %d entries", errProcProof, dir, maxProcEntries)
			}
			if stop, err := visit(id); stop || err != nil {
				return stop, err
			}
		}
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
}

func procVanished(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}

func readProcStat(path string, pid int) (procStat, error) {
	return readProcStatInto(path, pid, make([]byte, maxProcStat+1))
}

// readProcStatInto is readProcStat reading into buf, of maxProcStat+1 bytes.
func readProcStatInto(path string, pid int, buf []byte) (procStat, error) {
	data, err := readProcFileInto(path, buf)
	if err != nil {
		return procStat{}, err
	}
	return parseProcStat(data, pid)
}

// readProcFile reads at most limit bytes; a longer file is a proof failure.
func readProcFile(path string, limit int) ([]byte, error) {
	return readProcFileInto(path, make([]byte, limit+1))
}

// readProcFileInto reads at most len(buf)-1 bytes into buf; a longer file is
// a proof failure. The returned slice aliases buf.
func readProcFileInto(path string, buf []byte) ([]byte, error) {
	limit := len(buf) - 1
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	if n > limit {
		return nil, fmt.Errorf("%w: %s exceeds %d bytes", errProcProof, path, limit)
	}
	return buf[:n], nil
}

// parseProcStat reads "pid (comm) state ppid pgrp ..."; comm may itself hold
// parentheses, so the fields follow the last ')'. Only the first three fields
// are split out, and the error is built only on failure, so a scan of every
// process allocates nothing per well-formed entry here.
func parseProcStat(data []byte, pid int) (procStat, error) {
	malformed := func() error { return fmt.Errorf("%w: malformed stat for %d", errProcProof, pid) }
	start, end := bytes.IndexByte(data, '('), bytes.LastIndexByte(data, ')')
	if start < 1 || end < start {
		return procStat{}, malformed()
	}
	if got, err := strconv.Atoi(string(bytes.TrimSpace(data[:start]))); err != nil || got != pid {
		return procStat{}, malformed()
	}
	var fields [3][]byte
	rest := data[end+1:]
	for i := range fields {
		rest = bytes.TrimLeft(rest, procStatSpace)
		cut := bytes.IndexAny(rest, procStatSpace)
		if cut < 0 {
			cut = len(rest)
		}
		fields[i], rest = rest[:cut], rest[cut:]
	}
	if len(fields[2]) == 0 || len(fields[0]) != 1 {
		return procStat{}, malformed()
	}
	ppid, err1 := strconv.Atoi(string(fields[1]))
	pgrp, err2 := strconv.Atoi(string(fields[2]))
	if err1 != nil || err2 != nil {
		return procStat{}, malformed()
	}
	return procStat{pid: pid, ppid: ppid, pgrp: pgrp, state: fields[0][0]}, nil
}

// procStatSpace is the ASCII white space bytes.Fields splits on.
const procStatSpace = " \t\n\v\f\r"

// ProcGroupLive reports, from /proc, whether the group led by leader, this
// process's child, has a live member (PGO-V0-006, PGO-V0-009). A running
// leader is live. An exited leader that this process still holds unreaped
// is quiet only under the procGroupQuiet proof, with its identity checks and
// status Threads counts. It sends no signal. Any failure to obtain the proof
// is reported as live with the error. It is meaningful only where /proc
// exists (Linux).
func ProcGroupLive(leader int) (bool, error) {
	return procGroupLive("/proc", leader, os.Getpid())
}

func procGroupLive(root string, leader, parent int) (bool, error) {
	probe, err := procGroupQuiet(root, leader, parent)
	if err == nil {
		return probe != ProbeQuiet, nil
	}
	if leader > 1 && parent > 0 {
		stat, statErr := readProcStat(filepath.Join(root, strconv.Itoa(leader), "stat"), leader)
		if statErr == nil && stat.pgrp == leader && stat.ppid == parent && !procDead(stat.state) {
			return true, nil
		}
	}
	return true, err
}
