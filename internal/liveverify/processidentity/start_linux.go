//go:build linux

package processidentity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

const Source = "PROC_STAT_STARTTIME"

func Start(_ context.Context, pid int) (string, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	end := strings.LastIndexByte(string(raw), ')')
	if end < 0 {
		return "", errors.New("malformed proc stat comm")
	}
	fields := strings.Fields(string(raw[end+1:]))
	if len(fields) < 20 {
		return "", errors.New("short proc stat")
	}
	return fields[19], nil
}
