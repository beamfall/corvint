// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path"
	"strconv"
	"strings"
)

// Reserve one GiB per replay worker (each runs one subprocess at a time).
// MemAvailable avoids using occupied host memory; cgroup v1/v2 bounds, when
// exposed, further restrict it. Unobservable memory keeps the old two-worker
// operational setting. An explicit SELECTION_BASELINE_JOBS remains an override.
func memoryJobs() int {
	return memoryJobsFrom(os.ReadFile)
}

func memoryJobsFrom(readFile func(string) ([]byte, error)) int {
	data, err := readFile("/proc/meminfo")
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
	for _, pair := range cgroupMemoryFiles(readFile) {
		limit, e1 := readFile(pair[0])
		if e1 == nil && strings.TrimSpace(string(limit)) == "max" {
			continue
		}
		used, e2 := readFile(pair[1])
		n, e3 := strconv.ParseUint(strings.TrimSpace(string(limit)), 10, 64)
		u, e4 := strconv.ParseUint(strings.TrimSpace(string(used)), 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			// An identified but unreadable bound must not admit sixteen workers.
			available = min(available, 2<<30)
			continue
		}
		if u >= n {
			return 1
		}
		available = min(available, n-u)
	}
	return jobsForMemory(available)
}

// Map this process's membership through each memory-controller mount, then
// include every visible ancestor: a parent's remaining budget is shared with
// siblings and can be tighter than the leaf's limit. Mount roots can be subtrees
// (bind mounts); only ancestors visible in that mount can be observed.
func cgroupMemoryFiles(readFile func(string) ([]byte, error)) [][2]string {
	membership, e1 := readFile("/proc/self/cgroup")
	mounts, e2 := readFile("/proc/self/mountinfo")
	if e1 != nil || e2 != nil {
		return nil
	}
	unescape := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	var pairs [][2]string
	for _, line := range strings.Split(string(mounts), "\n") {
		left, right, ok := strings.Cut(line, " - ")
		fields, fs := strings.Fields(left), strings.Fields(right)
		if !ok || len(fields) < 6 || len(fs) < 3 {
			continue
		}
		v2 := fs[0] == "cgroup2"
		if !v2 && (fs[0] != "cgroup" || !strings.Contains(","+fs[2]+",", ",memory,")) {
			continue
		}
		root, mount := unescape.Replace(fields[3]), unescape.Replace(fields[4])
		if !strings.HasPrefix(root, "/") || !strings.HasPrefix(mount, "/") || path.Clean(root) != root || path.Clean(mount) != mount {
			continue
		}
		for _, member := range strings.Split(string(membership), "\n") {
			parts := strings.SplitN(member, ":", 3)
			if len(parts) != 3 || (v2 && parts[1] != "") || (!v2 && !strings.Contains(","+parts[1]+",", ",memory,")) {
				continue
			}
			group := parts[2]
			if !strings.HasPrefix(group, "/") || path.Clean(group) != group {
				continue
			}
			rel := strings.TrimPrefix(group, root)
			if root != "/" && group != root && !strings.HasPrefix(group, root+"/") {
				continue
			}
			limit, used := "memory.max", "memory.current"
			if !v2 {
				limit, used = "memory.limit_in_bytes", "memory.usage_in_bytes"
			}
			for dir := path.Join(mount, rel); ; dir = path.Dir(dir) {
				pairs = append(pairs, [2]string{path.Join(dir, limit), path.Join(dir, used)})
				if dir == mount {
					break
				}
			}
		}
	}
	return pairs
}

func jobsForMemory(available uint64) int {
	return int(max(1, min(16, available/(1<<30))))
}
