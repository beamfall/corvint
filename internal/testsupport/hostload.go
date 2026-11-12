// Package testsupport holds small load-detection helpers shared by timing-
// sensitive tests across packages, so a wall-clock ceiling can be skipped
// under a loaded host instead of flaking (decision 0036).
package testsupport

import (
	"os"
)

// forceTimingTestsRequested reports whether CORVINT_FORCE_TIMING_TESTS asks to
// force timing-sensitive tests. Only the documented "1" forces; unset and "0"
// both mean off, so a shell exporting CORVINT_FORCE_TIMING_TESTS=0 to disable
// the override does not accidentally force every timing test on.
func forceTimingTestsRequested() bool {
	return os.Getenv("CORVINT_FORCE_TIMING_TESTS") == "1"
}
