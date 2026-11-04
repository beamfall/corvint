// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// DWC-V0-7501: a Git directory can live on a different mount from the worktree.
// Keep atomic publication, with the staging file's effective permissions on both paths.
func publishReport(staged, destination string, rename func(string, string) error) error {
	defer os.Remove(staged)
	if err := rename(staged, destination); err == nil || !errors.Is(err, syscall.EXDEV) {
		return err
	}
	info, err := os.Stat(staged)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(staged)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".dogfood-report.*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(info.Mode().Perm())
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return rename(tmp.Name(), destination)
}
