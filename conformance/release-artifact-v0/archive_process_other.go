//go:build !unix

package main

import (
	"errors"
	"os/exec"
	"time"
)

func configureContainedCommand(command *exec.Cmd) error {
	return errors.New("archive gate requires qualified descendant containment")
}

func cleanupContainedCommand(command *exec.Cmd, deadline time.Duration) error { return nil }
