//go:build linux

// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"syscall"
)

const groupRSSSupported = true

// procRoot is where procfs is read; tests point it at an unreadable
// directory to prove the watchdog fails closed.
var procRoot = "/proc"

// groupRSSAvailable reports whether procfs is mounted and readable here: a
// chroot or container without it cannot measure a group, so the limit is
// recorded unenforced rather than silently read as zero (GLS-V0-058).
func groupRSSAvailable() bool {
	_, err := os.Stat(procRoot + "/self/stat")
	return err == nil
}

// groupRSS sums the resident set of every process whose process group is
// pgid, read from /proc/PID/stat (pgrp is field 5, rss in pages field 24).
// It needs no privileges: stat is world-readable for every process. A process
// that exits between listing and reading is skipped. A /proc that cannot be
// opened or listed is an error, never a zero reading.
func groupRSS(pgid int) (int64, error) {
	d, err := os.Open(procRoot)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", procRoot, err)
	}
	names, err := d.Readdirnames(-1)
	d.Close()
	if err != nil {
		return 0, fmt.Errorf("list %s: %w", procRoot, err)
	}
	want := strconv.Itoa(pgid)
	page := int64(os.Getpagesize())
	var total int64
	for _, name := range names {
		if name[0] < '0' || name[0] > '9' {
			continue
		}
		data, err := os.ReadFile(procRoot + "/" + name + "/stat")
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
	return total, nil
}

func killGroup(pgid int) { _ = syscall.Kill(-pgid, syscall.SIGKILL) }
