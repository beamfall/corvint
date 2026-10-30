//go:build linux

package groupreap

import (
	"path/filepath"
	"strconv"
)

// processArguments reads pid's argument vector from /proc/<pid>/cmdline.
func processArguments(pid int) ([][]byte, error) {
	raw, err := readProcFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"), maxArguments)
	if err != nil {
		return nil, err
	}
	return splitArguments(raw, -1), nil
}
