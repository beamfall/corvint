// Package testsupport holds small load-detection helpers shared by timing-
// sensitive tests across packages, so a wall-clock ceiling can be skipped
// under a loaded host instead of flaking (decision 0036).
package testsupport

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// HostAppearsLoaded reports whether the current 1-minute load average exceeds
// the machine's core count, using /proc/loadavg on Linux and `sysctl -n
// vm.loadavg` on Darwin. It returns false (not loaded) whenever load cannot
// be determined, so the caller runs the test by default.
func HostAppearsLoaded(t *testing.T) bool {
	t.Helper()
	load, ok := currentLoadAverage()
	if !ok {
		return false
	}
	return load > float64(runtime.NumCPU())
}

// SkipTimingUnderLoad skips t when the host appears loaded, unless
// CORVINT_FORCE_TIMING_TESTS=1 forces timing-sensitive tests to run anyway.
func SkipTimingUnderLoad(t *testing.T, reason string) {
	t.Helper()
	if HostAppearsLoaded(t) && !forceTimingTestsRequested() {
		t.Skipf("%s (set CORVINT_FORCE_TIMING_TESTS=1 to force)", reason)
	}
}

// forceTimingTestsRequested reports whether CORVINT_FORCE_TIMING_TESTS asks to
// force timing-sensitive tests. Only the documented "1" forces; unset and "0"
// both mean off, so a shell exporting CORVINT_FORCE_TIMING_TESTS=0 to disable
// the override does not accidentally force every timing test on.
func forceTimingTestsRequested() bool {
	return os.Getenv("CORVINT_FORCE_TIMING_TESTS") == "1"
}

func currentLoadAverage() (float64, bool) {
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		if fields := strings.Fields(string(data)); len(fields) > 0 {
			if value, err := strconv.ParseFloat(fields[0], 64); err == nil {
				return value, true
			}
		}
	}
	output, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(strings.Trim(strings.TrimSpace(string(output)), "{}"))
	if len(fields) == 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
