//go:build darwin || linux

package criterionexperiment

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/groupreap"
	"os/exec"
	"sync"
	"time"
)

type tasksBuffer struct {
	mu       sync.Mutex
	b        bytes.Buffer
	limit    int
	overflow bool
	cancel   context.CancelFunc
}

func (b *tasksBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	left := b.limit - b.b.Len()
	if len(p) > left {
		b.overflow = true
		b.cancel()
		p = p[:left]
	}
	_, _ = b.b.Write(p)
	return n, nil
}
func runTasks(ctx context.Context, dir, exe string, args []string, input []byte) ([]byte, error) {
	limited, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c := exec.CommandContext(limited, exe, args...)
	c.Dir = dir
	c.Env = []string{"PATH=/usr/bin:/bin", "LC_ALL=C"}
	c.Stdin = bytes.NewReader(input)
	ownGroup(c)
	stdout := tasksBuffer{limit: 4 << 20, cancel: cancel}
	stderr := tasksBuffer{limit: 64 << 10, cancel: cancel}
	c.Stdout = &stdout
	c.Stderr = &stderr
	e := groupreap.Run(c)
	if limited.Err() != nil || stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("Tasks verifier interrupted or output bound exceeded")
	}
	if e != nil {
		return nil, fmt.Errorf("Tasks criterion-binding unavailable or refused: %w", e)
	}
	return stdout.b.Bytes(), nil
}
