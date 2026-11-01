package fixture

import (
	"io/fs"
	"os"
	"syscall"
)

// WriteExecutable is os.WriteFile for a file the test process will execute.
// It holds syscall.ForkLock for reading while the write descriptor is open, so
// a parallel test that forks cannot inherit it and fail the later exec with
// ETXTBSY (V1-2587). It is the Tasks-side twin of internal/execfile.WriteFile,
// which decision 0397 keeps out of Tasks imports.
func WriteExecutable(name string, data []byte, perm fs.FileMode) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return os.WriteFile(name, data, perm)
}
