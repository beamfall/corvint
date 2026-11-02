package main

import (
	"os"
	"strconv"
	"strings"
)

// hostLoadAverage reads the 1-minute load average from /proc/loadavg (AHI-800).
func hostLoadAverage() (float64, bool) {
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, false
	}
	field, _, _ := strings.Cut(string(raw), " ")
	load, err := strconv.ParseFloat(field, 64)
	return load, err == nil
}
