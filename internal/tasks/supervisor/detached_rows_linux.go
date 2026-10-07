//go:build linux

package supervisor

// processRows reads /proc instead of forking ps every scan (CAL-V0-136).
func processRows() ([]processRow, error) { return procRows("/proc") }
