//go:build linux

package groupreap

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
)

// readProcessTable reads every /proc/<pid>/stat under the fixed entry and
// size bounds. It sends no signal and grants no signal authority.
func readProcessTable() (map[int]Process, error) {
	return readProcTable("/proc")
}

func readProcTable(root string) (map[int]Process, error) {
	table := map[int]Process{}
	_, err := procEachEntry(root, func(pid int) (bool, error) {
		data, err := readProcFile(filepath.Join(root, strconv.Itoa(pid), "stat"), maxProcStat)
		if procVanished(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		p, err := parseProcIdentity(data, pid)
		if err != nil {
			return false, err
		}
		table[pid] = p
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return table, nil
}

// parseProcIdentity reads state, ppid, pgrp and starttime (stat fields 3, 4,
// 5 and 22) after the last ')' of the comm field.
func parseProcIdentity(data []byte, pid int) (Process, error) {
	base, err := parseProcStat(data, pid)
	if err != nil {
		return Process{}, err
	}
	fields := bytes.Fields(data[bytes.LastIndexByte(data, ')')+1:])
	if len(fields) < 20 {
		return Process{}, fmt.Errorf("%w: short stat for %d", errProcessTable, pid)
	}
	start, err := strconv.ParseInt(string(fields[19]), 10, 64)
	if err != nil || start < 0 {
		return Process{}, fmt.Errorf("%w: malformed start time for %d", errProcessTable, pid)
	}
	p := Process{PID: pid, PPID: base.ppid, PGID: base.pgrp, Start: start, State: StateRunning}
	switch base.state {
	case 'T', 't':
		p.State = StateStopped
	case 'Z', 'X':
		p.State = StateZombie
	}
	return p, nil
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
