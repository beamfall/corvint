package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// maximumCommandOutput bounds each contained subprocess's stdout and stderr. The largest
// expected stdout is the raw export's `git ls-tree -r -z --full-tree` listing, which grows
// with the tracked path list (1,044,510 bytes at 575939667 on 2026-10-18), so the bound
// leaves headroom above it while keeping every capture finite.
const maximumCommandOutput = 8 << 20

type boundedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

func (b *boundedBuffer) Write(content []byte) (int, error) {
	original := len(content)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(content) > remaining {
			content = content[:remaining]
		}
		_, _ = b.buffer.Write(content)
	}
	if original > remaining {
		b.overflow = true
	}
	return original, nil
}

func (b *boundedBuffer) String() string { return b.buffer.String() }

func runContained(ctx context.Context, name string, arguments, environment []string, directory string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = directory
	command.Env = environment
	if err := configureContainedCommand(command); err != nil {
		return nil, nil, err
	}
	stdout := &boundedBuffer{limit: maximumCommandOutput}
	stderr := &boundedBuffer{limit: maximumCommandOutput}
	command.Stdout, command.Stderr = stdout, stderr
	err := command.Run()
	cleanupErr := cleanupContainedCommand(command, time.Second)
	if stdout.overflow || stderr.overflow {
		return []byte(stdout.String()), []byte(stderr.String()), fmt.Errorf("subprocess-output-limit")
	}
	if cleanupErr != nil {
		return []byte(stdout.String()), []byte(stderr.String()), cleanupErr
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		return []byte(stdout.String()), []byte(stderr.String()), err
	}
	return []byte(stdout.String()), []byte(stderr.String()), err
}
