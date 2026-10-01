//go:build !darwin && !linux

package intake

import "os"

// Native reader preflight qualification is limited to Darwin/Linux.
func singleLink(os.FileInfo) bool { return false }
