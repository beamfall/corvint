//go:build !linux && !darwin

// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"context"
	"strings"
	"testing"
)

// LPCV-V0-1087: a system without a native source refuses before the first
// save, naming the alternatives, instead of returning a source that never fires.
func TestNativeRefusesExplicitly(t *testing.T) {
	s, err := Native(context.Background(), t.TempDir())
	if s != nil || err == nil || !strings.Contains(err.Error(), "-events-stdin or -once") {
		t.Fatalf("source %v, err %v", s, err)
	}
}
