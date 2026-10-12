//go:build !darwin && !linux

package receiptbundle

import (
	"os"
	"time"
)

// changeTime is unavailable here, so the final walk refuses every bundle; the
// stable verifier itself runs on Darwin and Linux only.
func changeTime(os.FileInfo) (time.Time, bool) { return time.Time{}, false }
