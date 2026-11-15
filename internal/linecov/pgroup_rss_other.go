//go:build !linux

// SPDX-License-Identifier: Apache-2.0

package linecov

// Outside Linux no portable, unprivileged reading of a process group's
// resident memory is available, so the watchdog is off and the run's
// deadline is its only bound; qualify records the limit as unenforced
// (GLS-V0-058).
const groupRSSSupported = false

func groupRSSAvailable() bool { return false }

func groupRSS(pgid int) (int64, error) { return 0, nil }

func killGroup(pgid int) {}
