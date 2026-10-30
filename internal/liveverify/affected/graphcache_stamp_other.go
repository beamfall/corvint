//go:build !darwin && !linux

package affected

import "io/fs"

// statStamp has no change-time source here, so no graph cache entry is written
// or trusted on this platform (AFP-V0-392).
func statStamp(fs.FileInfo) (pathStamp, bool) { return pathStamp{}, false }
