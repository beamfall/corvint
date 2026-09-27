//go:build !darwin && !linux

package store

import "os"

// fileStamp has no change time to compare here, so the audit cache never
// validates and every write takes the full audit.
func fileStamp(os.FileInfo) (stamp, bool) { return stamp{}, false }
