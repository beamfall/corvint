//go:build !darwin && !linux

package journal

import "os"

// changeTime reports no change time here, so a writer audit's second
// capture never reuses its first listing (CAL-V0-224).
func changeTime(os.FileInfo) (int64, bool) { return 0, false }

// haveChangeTime says changeTime reports no change time on this platform.
const haveChangeTime = false
