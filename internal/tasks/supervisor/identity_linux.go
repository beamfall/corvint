//go:build linux

package supervisor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
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
	if n, e := strconv.ParseUint(f[19], 10, 64); e != nil || n == 0 {
		return "", fmt.Errorf("invalid process start identity")
	}
	boot, e := bootIdentity()
	if e != nil {
		return "", e
	}
	return "linux:" + boot + ":" + f[19], nil
}

// readBootStat reads /proc/stat; tests replace it to make btime jitter.
var readBootStat = os.ReadFile

// bootTime is the first valid btime this process read (V1-1091). The kernel
// derives btime on every read as the boot instant in realtime seconds,
// truncated, so a realtime adjustment that straddles a second boundary makes
// reads differ by one (1,076 and 1,924 of 3,000 reads in one VM). The boot
// itself cannot change while this process runs, so every identity it
// computes reuses that first value and two reads of one live process always
// match. An identity another process recorded can still differ by that one
// second; see https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-v1-1091-btime-jitter.md.
var bootTime struct {
	sync.Mutex
	value string
}

func bootIdentity() (string, error) {
	bootTime.Lock()
	defer bootTime.Unlock()
	if bootTime.value != "" {
		return bootTime.value, nil
	}
	stat, e := readBootStat("/proc/stat")
	if e != nil {
		return "", e
	}
	boot := ""
	for _, line := range strings.Split(string(stat), "\n") {
		if strings.HasPrefix(line, "btime ") {
			boot = strings.TrimPrefix(line, "btime ")
		}
	}
	if n, e := strconv.ParseUint(boot, 10, 64); e != nil || n == 0 {
		return "", fmt.Errorf("boot identity unavailable")
	}
	bootTime.value = boot
	return boot, nil
}

// processGone reports a /proc read failure that proves the process ended.
func processGone(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}
