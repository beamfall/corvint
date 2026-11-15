//go:build !linux

// SPDX-License-Identifier: Apache-2.0

package linecov

// Outside Linux no portable, unprivileged reading of a process group's
// resident memory is available, so the watchdog is off and the run's
// deadline is its only bound; qualify records the limit as unenforced
// (GLS-V0-058).
const groupRSSSupported = false

func groupRSS(pgid int) int64 { return 0 }

func killGroup(pgid int) {}
