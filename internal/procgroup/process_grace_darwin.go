//go:build darwin

package procgroup

import "sync/atomic"

// graceHeldQuiet is nil on Darwin: its grace keeps the signal-0 probe alone
// (PGO-V0-022 changes Linux only).
func graceHeldQuiet(*atomic.Bool, func(int) (bool, error)) func(int) bool { return nil }
