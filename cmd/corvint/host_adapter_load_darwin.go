package main

import (
	"encoding/binary"
	"syscall"
)

// hostLoadAverage reads the 1-minute load average from the vm.loadavg sysctl in process, without
// spawning sysctl(8) (AHI-800). The value is struct loadavg { fixpt_t ldavg[3]; long fscale; };
// syscall.Sysctl drops one trailing NUL byte, so fscale is read from the bytes that remain.
func hostLoadAverage() (float64, bool) {
	value, err := syscall.Sysctl("vm.loadavg")
	if err != nil || len(value) < 17 {
		return 0, false
	}
	raw := make([]byte, 24)
	copy(raw, value)
	scale := binary.LittleEndian.Uint64(raw[16:24])
	if scale == 0 {
		return 0, false
	}
	return float64(binary.LittleEndian.Uint32(raw[0:4])) / float64(scale), true
}
