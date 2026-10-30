//go:build !darwin && !linux

package journal

import "os"

// sharedLockFree proves nothing off Darwin and Linux (§5.1).
func sharedLockFree(*os.File) bool { return false }
