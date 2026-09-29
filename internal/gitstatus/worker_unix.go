//go:build darwin || linux

// SPDX-License-Identifier: AGPL-3.0-or-later
package gitstatus

import (
	"errors"
	"os"
	"syscall"
)

func verifyWorkerGroup() error {
	if syscall.Getpgrp() != os.Getpid() {
		return errors.New("worker must own its process group")
	}
	return nil
}
