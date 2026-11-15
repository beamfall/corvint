//go:build linux

// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"bytes"
	"os"
	"strconv"
	"syscall"
)

const groupRSSSupported = true

// groupRSS sums the resident set of every process whose process group is
// pgid, read from /proc/PID/stat (pgrp is field 5, rss in pages field 24).
// It needs no privileges: stat is world-readable for every process. A process
// that exits between listing and reading is skipped.
func groupRSS(pgid int) int64 {
	d, err := os.Open("/proc")
	if err != nil {
		return 0
	}
	names, _ := d.Readdirnames(-1)
	d.Close()
	want := strconv.Itoa(pgid)
	page := int64(os.Getpagesize())
	var total int64
	for _, name := range names {
		if name[0] < '0' || name[0] > '9' {
			continue
		}
		data, err := os.ReadFile("/proc/" + name + "/stat")
		if err != nil {
			continue
		}
		// comm, field 2, may hold spaces and parentheses: fields resume
		// after its last closing parenthesis, at field 3.
		i := bytes.LastIndexByte(data, ')')
		if i < 0 {
			continue
		}
		f := bytes.Fields(data[i+1:])
		if len(f) < 22 || string(f[2]) != want {
			continue
		}
		if pages, err := strconv.ParseInt(string(f[21]), 10, 64); err == nil {
			total += pages * page
		}
	}
	return total
}

func killGroup(pgid int) { _ = syscall.Kill(-pgid, syscall.SIGKILL) }
