//go:build darwin || linux

package groupreap

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
)

// readProcTable reads a /proc-format process table under root. It is the
// Linux process table and is built on Darwin only for fixture tests.
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
		if p.State != StateRunning {
			// stat reports the leader thread only: a stopped or zombie
			// leader proves nothing about the other threads.
			state, err := procTaskState(root, pid)
			if procVanished(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			p.State = state
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
	// A tracing stop ('t') is the tracer's to resume, so it is not a stop.
	switch base.state {
	case 'T':
		p.State = StateStopped
	case 'Z', 'X':
		p.State = StateZombie
	}
	return p, nil
}

// procTaskState classifies a whole thread group from every task's state: it
// is stopped only when every live task is in a group stop, a zombie only
// when no task is live, and running otherwise.
func procTaskState(root string, pid int) (ProcState, error) {
	live, running := 0, false
	_, err := procEachEntry(filepath.Join(root, strconv.Itoa(pid), "task"), func(tid int) (bool, error) {
		data, err := readProcFile(filepath.Join(root, strconv.Itoa(pid), "task", strconv.Itoa(tid), "stat"), maxProcStat)
		if procVanished(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		st, err := parseProcStat(data, tid)
		if err != nil {
			return false, err
		}
		switch {
		case procDead(st.state):
		case st.state == 'T':
			live++
		default:
			running = true
			return true, nil
		}
		return false, nil
	})
	switch {
	case err != nil:
		return StateRunning, err
	case running:
		return StateRunning, nil
	case live == 0:
		return StateZombie, nil
	}
	return StateStopped, nil
}
