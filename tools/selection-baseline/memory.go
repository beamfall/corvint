// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"strconv"
	"strings"
)

// Reserve one GiB per replay worker (each runs one subprocess at a time).
// MemAvailable avoids using occupied host memory; cgroup v1/v2 bounds, when
// exposed, further restrict it. Unobservable memory keeps the old two-worker
// operational setting. An explicit SELECTION_BASELINE_JOBS remains an override.
func memoryJobs() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 2
	}
	var available uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "MemAvailable:" && fields[2] == "kB" {
			n, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil && n <= ^uint64(0)/1024 {
				available = n * 1024
			}
		}
	}
	if available == 0 {
		return 2
	}
	for _, pair := range [][2]string{
		{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory.current"},
		{"/sys/fs/cgroup/memory/memory.limit_in_bytes", "/sys/fs/cgroup/memory/memory.usage_in_bytes"},
	} {
		limit, e1 := os.ReadFile(pair[0])
		used, e2 := os.ReadFile(pair[1])
		if e1 != nil || e2 != nil {
			continue
		}
		n, e1 := strconv.ParseUint(strings.TrimSpace(string(limit)), 10, 64)
		u, e2 := strconv.ParseUint(strings.TrimSpace(string(used)), 10, 64)
		if e1 == nil && e2 == nil {
			if u >= n {
				return 1
			}
			available = min(available, n-u)
		}
	}
	return jobsForMemory(available)
}

func jobsForMemory(available uint64) int {
	return int(max(1, min(16, available/(1<<30))))
}
