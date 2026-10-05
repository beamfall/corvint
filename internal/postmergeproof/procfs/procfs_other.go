//go:build !(linux && (amd64 || arm64))

// SPDX-License-Identifier: AGPL-3.0-or-later

package procfs

import "context"

// Supported reports whether this build reads the Linux procfs profile.
const Supported = false

// CaptureBirth is unsupported on this host and reports NOT_OBSERVED.
func CaptureBirth(context.Context, int, int64) (BirthCapture, error) {
	return BirthCapture{}, UnsupportedError{}
}

// SweepProcesses is unsupported on this host and reports NOT_OBSERVED.
func SweepProcesses(context.Context, int) (Sweep, error) {
	return Sweep{}, UnsupportedError{}
}
