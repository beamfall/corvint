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
)

var errProcProof = errors.New("groupreap: /proc quiet proof unavailable")

type procStat struct {
	pid, ppid, pgrp int
	state           byte
}

func procDead(state byte) bool { return state == 'Z' || state == 'X' }

// procGroupQuiet reports ProbeQuiet only when the exited, unreaped leader is
// identified as root's child of parent leading its own group, and every
// process and thread of that group under root is a zombie or dead. A live
// member is ProbeLive. Any failure to obtain that proof is an error, so the
// owner HOLDs; it never reports ProbeAbsent.
func procGroupQuiet(root string, leader, parent int) (Probe, error) {
	if err := procLeaderIdentity(root, leader, parent); err != nil {
		return ProbeLive, err
	}
	sawLeader := false
	live, err := procEachEntry(root, func(pid int) (bool, error) {
		stat, err := readProcStat(filepath.Join(root, strconv.Itoa(pid), "stat"), pid)
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

// procThreadLive reports whether any thread of a zombie-state process still
// runs (a thread-group leader that exited before its other threads).
func procThreadLive(root string, pid int) (bool, error) {
	dir := filepath.Join(root, strconv.Itoa(pid), "task")
	live, err := procEachEntry(dir, func(tid int) (bool, error) {
		stat, err := readProcStat(filepath.Join(dir, strconv.Itoa(tid), "stat"), tid)
		if procVanished(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return !procDead(stat.state), nil
	})
	if procVanished(err) {
		return false, nil
	}
	return live, err
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
	f, err := os.Open(path)
	if err != nil {
		return procStat{}, err
	}
	defer f.Close()
	var buf [maxProcStat + 1]byte
	n, err := io.ReadFull(f, buf[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return procStat{}, err
	}
	if n > maxProcStat {
		return procStat{}, fmt.Errorf("%w: %s exceeds %d bytes", errProcProof, path, maxProcStat)
	}
	return parseProcStat(buf[:n], pid)
}

// parseProcStat reads "pid (comm) state ppid pgrp ..."; comm may itself hold
// parentheses, so the fields follow the last ')'.
func parseProcStat(data []byte, pid int) (procStat, error) {
	malformed := fmt.Errorf("%w: malformed stat for %d", errProcProof, pid)
	start, end := bytes.IndexByte(data, '('), bytes.LastIndexByte(data, ')')
	if start < 1 || end < start {
		return procStat{}, malformed
	}
	if got, err := strconv.Atoi(string(bytes.TrimSpace(data[:start]))); err != nil || got != pid {
		return procStat{}, malformed
	}
	fields := bytes.Fields(data[end+1:])
	if len(fields) < 3 || len(fields[0]) != 1 {
		return procStat{}, malformed
	}
	ppid, err1 := strconv.Atoi(string(fields[1]))
	pgrp, err2 := strconv.Atoi(string(fields[2]))
	if err1 != nil || err2 != nil {
		return procStat{}, malformed
	}
	return procStat{pid: pid, ppid: ppid, pgrp: pgrp, state: fields[0][0]}, nil
}
