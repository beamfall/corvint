// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"runtime"
	"testing"
)

// GLS-V0-058: the watchdog is enforced exactly where /proc gives an
// unprivileged group RSS; elsewhere it is reported unenforced, never assumed.
func TestRSSWatchdogEnforcedOnlyOnLinux(t *testing.T) {
	if got, want := RSSWatchdogEnforced(), runtime.GOOS == "linux"; got != want {
		t.Fatalf("RSSWatchdogEnforced() = %t on %s, want %t", got, runtime.GOOS, want)
	}
}
