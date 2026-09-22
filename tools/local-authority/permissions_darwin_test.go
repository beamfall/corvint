//go:build darwin

package main

import (
	"golang.org/x/sys/unix"
	"testing"
)

func TestRootReadStatIgnoresOnlyAccessTime(t *testing.T) {
	a := unix.Stat_t{Uid: 0, Mode: 0444, Size: 5}
	b := a
	b.Atim.Sec = 100
	if !sameRootStat(a, b) {
		t.Fatal("read access time treated as mutation")
	}
	b.Mtim.Sec = 1
	if sameRootStat(a, b) {
		t.Fatal("write mutation ignored")
	}
}
