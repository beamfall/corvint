//go:build darwin || linux

package supervisor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// procRows snapshots the process table from a procfs root: the same pid,
// ppid, pgid and zombie facts `ps -axo pid=,ppid=,pgid=,stat=` reports,
// read from each /proc/<pid>/stat without forking ps (CAL-V0-136). A
// process that exits between the directory read and its stat read is
// skipped, as ps skips it; any other failure is an error, so the caller's
// scan stays uncertain rather than missing a process.
func procRows(root string) ([]processRow, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var rows []processRow
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 || !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name(), "stat"))
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return nil, err
		}
		r, err := procStatRow(raw)
		if err != nil || r.pid != pid {
			return nil, fmt.Errorf("unexpected process row %s", e.Name())
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// procStatRow parses "pid (comm) state ppid pgrp ...". The command name may
// hold spaces and parentheses, so the fields after it start at the last ')'.
func procStatRow(raw []byte) (processRow, error) {
	s := string(raw)
	open, end := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if open < 1 || end < open {
		return processRow{}, errors.New("unexpected stat")
	}
	f := strings.Fields(s[end+1:])
	if len(f) < 3 {
		return processRow{}, errors.New("unexpected stat")
	}
	var r processRow
	var e1, e2, e3 error
	r.pid, e1 = strconv.Atoi(strings.TrimSpace(s[:open]))
	r.ppid, e2 = strconv.Atoi(f[1])
	r.pgid, e3 = strconv.Atoi(f[2])
	if e1 != nil || e2 != nil || e3 != nil || len(f[0]) != 1 {
		return processRow{}, errors.New("unexpected stat")
	}
	r.zombie = f[0] == "Z"
	return r, nil
}
