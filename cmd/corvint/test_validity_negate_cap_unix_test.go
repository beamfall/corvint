// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/stepnegation"
)

// TestNegateCapsProviderOutputCopiedByExec_GPKV09930 runs a real provider
// process whose stdout os/exec copies into negateOutput with io.Copy. A
// provider that exits 2 with more than 4 MiB of output must be refused for
// its size by the core's own cap (LPCV-V0-941, GPK-V0-9930, V1-5122): an
// embedded buffer's ReadFrom would have absorbed every byte, left the
// overrun unset, and reported a missing refusal document instead.
func TestNegateCapsProviderOutputCopiedByExec_GPKV09930(t *testing.T) {
	worktree, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := negateProvider(t)
	negateFixture(t, bytes.Repeat([]byte("x"), stepnegation.MaxDocumentBytes+1), "2")
	status, stdout, stderr := runNegateCommand(worktree, "--provider="+provider, "--spec=cart.spec.cjs", "--test=cart > total", "--project=chromium", "--step=total")
	if status != 2 || stdout != "" || !strings.Contains(stderr, `"negate-provider-failed"`) || !strings.Contains(stderr, "provider output exceeds 4 MiB") {
		t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
	}
}
