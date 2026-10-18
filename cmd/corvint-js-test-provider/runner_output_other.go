//go:build !darwin && !linux

package main

import "os"

// openOutputFile refuses an existing non-regular entry, then sets mode 0600
// and truncates (PWP-V0-028). Without O_NOFOLLOW the entry check and the open
// are separate steps.
func openOutputFile(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, errRunnerOutputNotRegular
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = errRunnerOutputNotRegular
	}
	if err == nil {
		err = file.Chmod(0o600)
	}
	if err == nil {
		err = file.Truncate(0)
	}
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
