// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// publishCheckReport preserves atomic publication across split Git/worktree mounts
// (DWC-V0-7319). The rename argument keeps failure-path tests independent of the host layout.
func publishCheckReport(staged, report string, data []byte, rename func(string, string) error) error {
	defer os.Remove(staged)
	if err := os.WriteFile(staged, data, 0o666); err != nil {
		return err
	}
	if err := rename(staged, report); !errors.Is(err, syscall.EXDEV) {
		return err
	}
	// A sibling is on the destination mount. Never copy over the old report:
	// a partial write or failed final rename must leave its bytes unchanged.
	f, err := os.CreateTemp(filepath.Dir(report), ".dogfood-report.*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return rename(f.Name(), report)
}
