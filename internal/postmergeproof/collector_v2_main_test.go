// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/postmergehost"
)

// TestMain lets this test binary serve as the pinned observer executable of
// the host collector test. The observer role is selected by argv alone, since
// the observer's environment stays inside the invocation allowlist.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--internal-process-observer" {
		if err := postmergehost.RunProcessObserverV2(context.Background(), os.Args[2:], os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
