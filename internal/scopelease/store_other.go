//go:build !darwin && !linux

package scopelease

// lock is the deferred-platform O_EXCL lock (decision 0061, SCL-V0-900).
func lock(directory string) (func(), error) { return exclusiveLock(directory) }

// syncDirectory is a no-op where a directory handle cannot be synced.
func syncDirectory(string) error { return nil }
