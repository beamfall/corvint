// SPDX-License-Identifier: Apache-2.0

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package dogfoodflow

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestProposeCitationsDCWV0442RefreshRefusesFIFO pins the review finding on
// V1-3240: a --refresh FIFO with no writer is refused as non-regular before it
// is opened, rather than blocking the draft forever.
func TestProposeCitationsDCWV0442RefreshRefusesFIFO(t *testing.T) {
	root, base := preflightRepository(t, nil)
	fifo := filepath.Join(t.TempDir(), "plan.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code, _ := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, Refresh: fifo}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "REFUSE refresh-plan-unavailable") || !strings.Contains(stderr.String(), "fix: ") {
		t.Fatalf("FIFO refresh = %d; want a refresh-plan-unavailable refusal with a fix\n%s", code, stderr.String())
	}
}
