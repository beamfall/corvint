// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path"
	"strconv"
	"testing"
)

// ASD-V0-925: membership, mount roots and every visible ancestor bound workers.
func TestBaselineCgroupMemoryASDV0925(t *testing.T) {
	for _, tc := range []struct {
		name, membership, mount, mountRoot, mountPoint, leaf string
		v1, parentLimit, exhausted, unreadable               bool
		want                                                 int
	}{
		{name: "v2 leaf", membership: "0::/system.slice/job\n", mount: "cgroup2 cgroup rw", mountRoot: "/", mountPoint: "/cg", leaf: "/cg/system.slice/job", want: 2},
		{name: "v2 ancestor", membership: "0::/system.slice/job\n", mount: "cgroup2 cgroup rw", mountRoot: "/", mountPoint: "/cg", leaf: "/cg/system.slice/job", parentLimit: true, want: 2},
		{name: "v1 leaf", membership: "4:cpu:/other\n5:memory,hugetlb:/system.slice/job\n", mount: "cgroup cgroup rw,memory,hugetlb", mountRoot: "/", mountPoint: "/cg", leaf: "/cg/system.slice/job", v1: true, want: 2},
		{name: "v1 ancestor", membership: "5:memory:/system.slice/job\n", mount: "cgroup cgroup rw,memory", mountRoot: "/", mountPoint: "/cg", leaf: "/cg/system.slice/job", v1: true, parentLimit: true, want: 2},
		{name: "subtree mount", membership: "0::/system.slice/job/child\n", mount: "cgroup2 cgroup rw", mountRoot: "/system.slice", mountPoint: "/cg", leaf: "/cg/job/child", parentLimit: true, want: 2},
		{name: "escaped mount", membership: "0::/job/child\n", mount: "cgroup2 cgroup rw", mountRoot: "/", mountPoint: `/cg\040space`, leaf: "/cg space/job/child", parentLimit: true, want: 2},
		{name: "exhausted ancestor", membership: "0::/job/child\n", mount: "cgroup2 cgroup rw", mountRoot: "/", mountPoint: "/cg", leaf: "/cg/job/child", parentLimit: true, exhausted: true, want: 1},
		{name: "unreadable bound", membership: "0::/job/child\n", mount: "cgroup2 cgroup rw", mountRoot: "/", mountPoint: "/cg", leaf: "/cg/job/child", unreadable: true, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{
				"/proc/meminfo":        "MemAvailable: 67108864 kB\n",
				"/proc/self/cgroup":    tc.membership,
				"/proc/self/mountinfo": "30 20 0:1 " + tc.mountRoot + " " + tc.mountPoint + " rw - " + tc.mount + "\n",
			}
			limit, used, unlimited := "memory.max", "memory.current", "max"
			if tc.v1 {
				limit, used, unlimited = "memory.limit_in_bytes", "memory.usage_in_bytes", "9223372036854771712"
			}
			mountPoint := tc.mountPoint
			if tc.name == "escaped mount" {
				mountPoint = "/cg space"
			}
			for dir := tc.leaf; ; dir = path.Dir(dir) {
				files[path.Join(dir, limit)] = unlimited
				files[path.Join(dir, used)] = "0"
				if dir == mountPoint {
					break
				}
			}
			bounded := tc.leaf
			if tc.parentLimit {
				bounded = path.Dir(bounded)
			}
			files[path.Join(bounded, limit)] = strconv.FormatUint(3<<30, 10)
			files[path.Join(bounded, used)] = strconv.FormatUint(1<<30, 10)
			if tc.exhausted {
				files[path.Join(bounded, used)] = strconv.FormatUint(4<<30, 10)
			}
			if tc.unreadable {
				delete(files, path.Join(bounded, limit))
			}
			readFile := func(name string) ([]byte, error) {
				if data, ok := files[name]; ok {
					return []byte(data), nil
				}
				return nil, os.ErrNotExist
			}
			if got := memoryJobsFrom(readFile); got != tc.want {
				t.Fatalf("workers=%d, want %d", got, tc.want)
			}
		})
	}
}
