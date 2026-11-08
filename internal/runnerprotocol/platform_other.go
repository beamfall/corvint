//go:build !linux

// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"fmt"
	"os"
)

func openRegular(string) (*os.File, error) { return nil, fmt.Errorf("unsupported-safe-open") }
func lockStore(string) (func(), error)     { return nil, fmt.Errorf("unsupported-store-lock") }
func executableFile(Resolved) (*os.File, error) {
	return nil, fmt.Errorf("unsupported-executable-sealing")
}
func executablePath() string { return "" }
