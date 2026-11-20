// SPDX-License-Identifier: Apache-2.0
//go:build !darwin

package flows

import "os"

// openTestTerminal returns a pseudo-terminal file that terminalInput accepts where
// the platform supports one.
func openTestTerminal() (*os.File, func(), error) {
	terminal, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	return terminal, func() { terminal.Close() }, nil
}
