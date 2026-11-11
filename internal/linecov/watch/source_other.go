//go:build !linux && !darwin

// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"context"
	"fmt"
	"runtime"
)

// native refuses: no in-tree event source exists for this system, and a stale
// answer is worse than none (LPCV-V0-1087).
func native(context.Context, string) (*Source, error) {
	return nil, fmt.Errorf("native save notifications unavailable on %s: use -events-stdin or -once", runtime.GOOS)
}
