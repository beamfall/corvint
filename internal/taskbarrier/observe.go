// SPDX-License-Identifier: Apache-2.0

// Package taskbarrier consumes only the Tasks read wire at the host boundary.
package taskbarrier

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Observe asks the optional companion for its advisory warnings. It has no Tasks
// implementation imports, no writer argv, bounded output and a short deadline.
func Observe(root string) string {
	// No Tasks projection means no Tasks hook integration in this checkout.
	if _, err := os.Stat(filepath.Join(root, ".taskman", "queue.json")); err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		return "Tasks barrier warning NOT_OBSERVED: projection unreadable"
	}
	exe, err := exec.LookPath("corvint-tasks")
	if err != nil {
		return "Tasks barrier warning NOT_OBSERVED: companion unavailable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "queue", "status", "--fields", "barrier")
	cmd.Dir = root

	cmd.WaitDelay = 50 * time.Millisecond
	var out boundedOutput
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil || out.overflow {
		return "Tasks barrier warning NOT_OBSERVED: companion read failed or exceeded budget"
	}
	raw := out.buf.Bytes()
	res, err := wire.DecodeResult(raw)
	if err != nil || res.Outcome != wire.OutcomeOK || strings.Join(res.Command, " ") != "queue status" {
		return "Tasks barrier warning NOT_OBSERVED: invalid companion response"
	}
	for _, warning := range res.Warnings {
		if strings.HasPrefix(warning, "STALE_BARRIER:") {
			return warning
		}
	}

	if len(res.Items) != 1 {
		return "Tasks barrier warning NOT_OBSERVED: missing barrier observation"
	}
	barrier, ok := res.Items[0].Obj.Get("barrier")
	if !ok {
		return "Tasks barrier warning NOT_OBSERVED: missing barrier observation"
	}
	if barrier.Kind != wire.KindNull {
		if barrier.Kind != wire.KindObject {
			return "Tasks barrier warning NOT_OBSERVED: invalid barrier observation"
		}
		limit, ok := barrier.Obj.Get("warningPolicySeconds")
		if !ok {
			return "Tasks barrier warning NOT_OBSERVED: companion predates age warnings"
		}
		n, err := wire.ParseCount("warningPolicySeconds", limit.Str)
		if limit.Kind != wire.KindString || err != nil || n.Int() < 1 || n.Int() > 604800 {
			return "Tasks barrier warning NOT_OBSERVED: invalid age policy observation"
		}
	}
	return ""
}

type boundedOutput struct {
	buf      bytes.Buffer
	overflow bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > 65536 {
		b.overflow = true
		return 0, io.ErrShortBuffer
	}
	return b.buf.Write(p)
}
