//go:build !darwin && !linux

package gitauth

import "os"

// This native authority seam requires a change-time witness; platforms without
// one implemented here remain unavailable rather than accepting weaker reads.
func authorityChangeTime(os.FileInfo) (int64, int64, bool) { return 0, 0, false }
