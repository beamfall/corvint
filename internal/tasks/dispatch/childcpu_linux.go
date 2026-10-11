package dispatch

import (
	"os"
	"strconv"
)

// preciseCPU replaces the listed processes' ps CPU times, which Linux prints
// in whole seconds, with utime+stime from /proc/<pid>/stat, so a CAL-V0-218
// share is measurable within a short idle timeout. A process whose stat
// cannot be read or parsed gets an unknown CPU time.
func preciseCPU(procs map[int]proc, pids []int) {
	for _, pid := range pids {
		p, ok := procs[pid]
		if !ok {
			continue
		}
		p.cpu, p.cpuKnown = 0, false
		if raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
			p.cpu, p.cpuKnown = parseProcStatCPU(string(raw))
		}
		procs[pid] = p
	}
}
