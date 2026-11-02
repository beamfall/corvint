package main

import (
	"io/fs"
	"os"
	"syscall"
)

// writeExecutable is os.WriteFile for a file the test process will execute. It
// holds syscall.ForkLock for reading while the write descriptor is open, so a
// parallel test that forks cannot inherit it and fail the later exec with
// ETXTBSY. This module cannot import internal/execfile, the shared writer
// (V1-2587, ASD-V0-929).
func writeExecutable(name string, data []byte, perm fs.FileMode) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return os.WriteFile(name, data, perm)
}
