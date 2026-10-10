//go:build linux

package supervisor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// readProcStat reads a /proc stat file; tests replace it to reap the
// process between the open and the read.
var readProcStat = os.ReadFile

// ProcessIdentity returns pid's start identity, or "" when no such process
// runs. A process reaped after /proc/<pid>/stat was opened fails the read
// with ESRCH; it has ended exactly as if the open had found nothing, so it is
// not an unreadable identity (CAL-V0-056).
func ProcessIdentity(pid int) (string, error) {
	b, e := readProcStat("/proc/" + strconv.Itoa(pid) + "/stat")
	if processGone(e) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	end := strings.LastIndex(string(b), ") ")
	if end < 0 {
		return "", fmt.Errorf("invalid process stat")
	}
	f := strings.Fields(string(b[end+2:]))
	if len(f) < 20 {
		return "", fmt.Errorf("short process stat")
	}
	stat, e := os.ReadFile("/proc/stat")
	if e != nil {
		return "", e
	}
	boot := ""
	for _, line := range strings.Split(string(stat), "\n") {
		if strings.HasPrefix(line, "btime ") {
			boot = strings.TrimPrefix(line, "btime ")
		}
	}
	if n, e := strconv.ParseUint(f[19], 10, 64); e != nil || n == 0 {
		return "", fmt.Errorf("invalid process start identity")
	}
	if n, e := strconv.ParseUint(boot, 10, 64); e != nil || n == 0 {
		return "", fmt.Errorf("boot identity unavailable")
	}
	return "linux:" + boot + ":" + f[19], nil
}

// processGone reports a /proc read failure that proves the process ended.
func processGone(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}
