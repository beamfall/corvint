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

// Preserve the existing loose gate on hosts without qualified archive containment.
// The archive path still refuses those hosts through configureContainedCommand.
func configureLooseCommand(command *exec.Cmd) error { return nil }
func cleanupLooseCommand(command *exec.Cmd) error   { return nil }
