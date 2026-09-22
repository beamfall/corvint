package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

const maximumCommandOutput = 1 << 20

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
