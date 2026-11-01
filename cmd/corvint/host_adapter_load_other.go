//go:build !linux && !darwin

package main

// hostLoadAverage reports no load average where the OS exposes none cheaply (AHI-800).
func hostLoadAverage() (float64, bool) {
	return 0, false
}
