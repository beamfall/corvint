package publish

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// lockWait is a hang detector, not a performance budget (decision 0082): a
// live holder finishes one bounded update well inside its Git budget, and the
// kernel releases an flock when its holder dies, so only a hung holder can
// exhaust it.
const lockWait = 2 * time.Minute

const lockRetry = 10 * time.Millisecond

// Lock serializes one root-relative output's read-modify-write under an
// exclusive advisory lock on the sibling file "<output>.lock". A non-empty file
// at that path is refused, never adopted. The returned release removes the lock
// file before unlocking, but only while the path still names the held inode; a
// waiter that then acquires the unlinked inode sees it no longer names the path
// and retries. Expiry of
// the wait refuses with map-locked rather than risking a lost update.
func (r *Root) Lock(relative string) (func(), error) {
	final, err := r.resolveOutput(relative)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return nil, cemcode.New(cemcode.PublishFailed, "lock directory for %q cannot be created", relative)
	}
	lockPath := final + ".lock"
	deadline := time.Now().Add(lockWait)
	for {
		file, err := openLockFile(lockPath)
		if err != nil {
			return nil, cemcode.New(cemcode.PublishFailed, "update lock for %q cannot be opened", relative)
		}
		held, err := waitExclusive(file, deadline)
		if err != nil {
			_ = file.Close()
			return nil, cemcode.New(cemcode.PublishFailed, "update lock for %q cannot be taken: %v", relative, err)
		}
		if !held {
			_ = file.Close()
			return nil, cemcode.New(cemcode.MapLocked,
				"%q is held by another CEM update; the lock wait expired after %s", relative, lockWait)
		}
		if !emptyLock(file) {
			unlockFile(file)
			_ = file.Close()
			return nil, cemcode.New(cemcode.PublishFailed,
				"update lock path for %q names an existing non-empty file, not a lock file", relative)
		}
		if namesPath(file, lockPath) {
			return func() {
				if namesPath(file, lockPath) {
					_ = os.Remove(lockPath)
				}
				unlockFile(file)
				_ = file.Close()
			}, nil
		}
		unlockFile(file)
		_ = file.Close()
	}
}

func waitExclusive(file *os.File, deadline time.Time) (bool, error) {
	for {
		held, err := tryLockFile(file)
		if err != nil || held {
			return held, err
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		time.Sleep(lockRetry)
	}
}

// emptyLock reports whether the held file can be a lock file: Lock creates it
// empty and never writes it, so a non-empty file is someone's data (an input
// map or cache that shares the name) and must not be removed on release.
func emptyLock(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode().IsRegular() && info.Size() == 0
}

func namesPath(file *os.File, path string) bool {
	opened, openedErr := file.Stat()
	current, currentErr := os.Lstat(path)
	return openedErr == nil && currentErr == nil && os.SameFile(opened, current)
}

// RecoverPairBackup repairs the one crash window of PublishPair that loses the
// first output: its prior file was set aside as "<output>.bak-<hex>" and the
// process died before the new file landed. With the output missing, exactly
// one such backup is restored by atomic rename; several are refused because
// the intended prior file is ambiguous. A present output is left untouched.
// Callers must hold the output's Lock so an in-flight pair is never mistaken
// for an interrupted one.
func (r *Root) RecoverPairBackup(relative string) error {
	final, err := r.resolveOutput(relative)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(final); !os.IsNotExist(err) {
		return nil
	}
	backups, err := pairBackups(final)
	if err != nil {
		return cemcode.New(cemcode.MapUnavailable, "directory of %q cannot be listed for interrupted-publication backups", relative)
	}
	if len(backups) == 0 {
		return nil
	}
	if len(backups) > 1 {
		return cemcode.New(cemcode.MapUnavailable,
			"%q is missing but %d interrupted-publication backups remain; restore the intended one by hand", relative, len(backups))
	}
	if err := os.Rename(backups[0], final); err != nil {
		return cemcode.New(cemcode.PublishFailed, "interrupted-publication backup of %q could not be restored", relative)
	}
	if err := syncOutputDirectory(filepath.Dir(final)); err != nil {
		return cemcode.New(cemcode.PublishFailed, "restored backup of %q could not be synced", relative)
	}
	return nil
}

// RemoveValidatedPairBackup removes the one recognized backup left when a
// pair publication crashed after its new first output landed. The caller must
// already have validated the present final output while holding its Lock.
// Multiple backups remain an ambiguous, fail-closed condition.
func (r *Root) RemoveValidatedPairBackup(relative string) error {
	final, err := r.resolveOutput(relative)
	if err != nil {
		return err
	}
	info, err := os.Lstat(final)
	if err != nil || !info.Mode().IsRegular() {
		return cemcode.New(cemcode.MapUnavailable, "validated output %q is no longer a regular file", relative)
	}
	backups, err := pairBackups(final)
	if err != nil {
		return cemcode.New(cemcode.MapUnavailable, "directory of %q cannot be listed for interrupted-publication backups", relative)
	}
	if len(backups) == 0 {
		return nil
	}
	if len(backups) > 1 {
		return cemcode.New(cemcode.MapUnavailable,
			"%q has %d interrupted-publication backups; remove the obsolete files by hand", relative, len(backups))
	}
	if err := os.Remove(backups[0]); err != nil {
		return cemcode.New(cemcode.PublishFailed, "validated interrupted-publication backup of %q could not be removed", relative)
	}
	if err := syncOutputDirectory(filepath.Dir(final)); err != nil {
		return cemcode.New(cemcode.PublishFailed, "removal of validated interrupted-publication backup of %q could not be synced", relative)
	}
	return nil
}

func pairBackups(final string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(final))
	if err != nil {
		return nil, err
	}
	prefix := filepath.Base(final) + ".bak-"
	backups := []string{}
	for _, entry := range entries {
		if isPairBackup(entry, prefix) {
			backups = append(backups, filepath.Join(filepath.Dir(final), entry.Name()))
		}
	}
	return backups, nil
}

func isPairBackup(entry os.DirEntry, prefix string) bool {
	suffix, found := strings.CutPrefix(entry.Name(), prefix)
	if !found || len(suffix) != 16 || !entry.Type().IsRegular() {
		return false
	}
	_, err := hex.DecodeString(suffix)
	return err == nil && strings.ToLower(suffix) == suffix
}
