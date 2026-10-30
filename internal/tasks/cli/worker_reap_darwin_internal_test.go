//go:build darwin

package cli

// openPin has no Darwin implementation: Darwin has no process handle that a
// non-parent can signal through without a window in which the pid may be
// reaped and reused, so the reaper proves no ownership here and signals
// nothing (ATR-V0-310).
func openPin(int) (pin, bool, error) { return nil, false, errPinUnsupported }
