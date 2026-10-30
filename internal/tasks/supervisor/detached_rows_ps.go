//go:build darwin && !arm64 && !amd64

package supervisor

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// processRows snapshots the process table through ps where no native
// reader exists (CAL-V0-077).
func processRows() ([]processRow, error) {
	raw, e := exec.Command("/bin/ps", "-axo", "pid=,ppid=,pgid=,stat=").Output()
	if e != nil {
		return nil, e
	}
	var rows []processRow
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 4 {
			return nil, fmt.Errorf("unexpected process row")
		}
		var r processRow
		var e1, e2, e3 error
		r.pid, e1 = strconv.Atoi(f[0])
		r.ppid, e2 = strconv.Atoi(f[1])
		r.pgid, e3 = strconv.Atoi(f[2])
		if e1 != nil || e2 != nil || e3 != nil {
			return nil, fmt.Errorf("unexpected process row")
		}
		r.zombie = strings.HasPrefix(f[3], "Z")
		rows = append(rows, r)
	}
	return rows, nil
}
