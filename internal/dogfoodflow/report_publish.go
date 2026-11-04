// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// publishDogfoodReport preserves atomic publication when Git metadata and
// the worktree are separate mounts (DWC-V0-048). Only EXDEV takes the
// same-directory fallback; other failures retain their original refusal.
func publishDogfoodReport(staged, target string, data []byte, rename func(string, string) error) error {
	defer os.Remove(staged)
	if err := os.WriteFile(staged, data, 0o666); err != nil {
		return err
	}
	err := rename(staged, target)
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".dogfood-report.*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return rename(tmp.Name(), target)
}
