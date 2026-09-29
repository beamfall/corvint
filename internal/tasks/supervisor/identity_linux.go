//go:build linux

package supervisor

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func ProcessIdentity(pid int) (string, error) {
	b, e := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if os.IsNotExist(e) {
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
