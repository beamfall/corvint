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
