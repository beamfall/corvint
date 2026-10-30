//go:build darwin || linux

package authority

import "os"

// SetDirectorySyncTestHook is a seam for tests in other packages (LGL-V0-016
// proposed): until restore runs, every directory sync calls hook with the
// directory's FileInfo (for os.SameFile) and a next that performs the sync,
// so a test can observe when each sync starts and returns, or fail it
// without calling next. hook may be called from several goroutines at once.
// Production code never calls it.
func SetDirectorySyncTestHook(hook func(dir os.FileInfo, next func() error) error) (restore func()) {
	prev := syncDirectory
	syncDirectory = func(f *os.File) error {
		info, err := f.Stat()
		if err != nil {
			return err
		}
		return hook(info, func() error { return prev(f) })
	}
	return func() { syncDirectory = prev }
}
