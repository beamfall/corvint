//go:build !unix

package store

import "time"

// processCPU is not observed on this platform.
func processCPU() (time.Duration, bool) { return 0, false }
