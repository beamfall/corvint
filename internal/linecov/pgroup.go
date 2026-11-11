// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"os/exec"
	"time"
)

// RunIsolated runs cmd as the collector runs a test binary: in a process
// group of its own, which a cancelled context kills whole, with a bounded
// wait for output pipes a descendant still holds, and with the group's
// remaining members killed once cmd returns. Where Unix process groups do not
// exist only cmd itself is killed, as exec.CommandContext does.
func RunIsolated(cmd *exec.Cmd) error {
	isolate(cmd)
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	cleanup(cmd)
	return err
}
