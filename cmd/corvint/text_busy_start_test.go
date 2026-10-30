// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

// startCopiedBinary starts a fresh command from build, retrying a start that
// fails with ETXTBSY on workTextBusyRetries. Test helpers copy or build a
// binary and then exec it while other parallel tests fork; such a fork can
// hold the new file's write descriptor until its own exec closes it, so the
// first start can fail "text file busy" on Linux although nothing started.
// Every test helper that execs a binary it just wrote starts it here
// (ASD-V0-106, V1-3175), as workStartAdapter does for the product's adapter.
func startCopiedBinary(build func() *exec.Cmd) (*exec.Cmd, error) {
	command := build()
	err := command.Start()
	for _, delay := range workTextBusyRetries {
		if err == nil || !errors.Is(err, syscall.ETXTBSY) {
			break
		}
		time.Sleep(delay)
		command = build()
		err = command.Start()
	}
	return command, err
}
