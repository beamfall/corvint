//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
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
