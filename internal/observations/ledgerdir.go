package observations

import (
	"errors"
	"io/fs"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
)

// ledgerDirectoryName is the worktree directory both private ledgers live in.
const ledgerDirectoryName = ".corvint"

// LedgerDirectory is one validated handle on a worktree's `.corvint`
// directory, shared by the self-observation and unplanned-read ledgers
// (SOL-V0-014). Every ledger read, temporary sweep, temporary creation,
// replacement and marker change goes through it, relative to the directory
// that was a real directory when it was opened, and every mutation first
// re-verifies that `.corvint` still names that directory. A `.corvint`
// swapped for a symlink after validation therefore cannot move a write, a
// removal or a read outside the worktree: the operation is refused instead.
// os.Root supplies the descriptor-relative operations on every platform; a
// platform without them returns an error, so the writer fails closed.
type LedgerDirectory struct {
	repository *os.Root
	directory  *os.Root
	handle     *os.File
	identity   os.FileInfo
	refuse     error
}

// OpenLedgerDirectory opens root/.corvint, creating it first when create is
// set, and returns refuse when it is not a real directory. A missing
// directory returns its not-exist error.
func OpenLedgerDirectory(root string, create bool, refuse error) (*LedgerDirectory, error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	ledger := &LedgerDirectory{repository: repository, refuse: refuse}
	if err := ledger.open(create); err != nil {
		ledger.Close()
		return nil, err
	}
	return ledger, nil
}

func (d *LedgerDirectory) open(create bool) error {
	if create {
		if err := d.repository.Mkdir(ledgerDirectoryName, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	linkInfo, err := d.repository.Lstat(ledgerDirectoryName)
	if err != nil {
		return err
	}
	if !linkInfo.IsDir() {
		return d.refuse
	}
	d.directory, err = d.repository.OpenRoot(ledgerDirectoryName)
	if err != nil {
		return d.refuse
	}
	d.handle, err = d.directory.Open(".")
	if err != nil {
		return err
	}
	d.identity, err = d.handle.Stat()
	if err != nil {
		return err
	}
	if !d.identity.IsDir() || !os.SameFile(linkInfo, d.identity) {
		return d.refuse
	}
	return nil
}

// Close releases the directory handle.
func (d *LedgerDirectory) Close() error {
	var errs []error
	if d.handle != nil {
		errs = append(errs, d.handle.Close())
	}
	if d.directory != nil {
		errs = append(errs, d.directory.Close())
	}
	errs = append(errs, d.repository.Close())
	return errors.Join(errs...)
}

// File is the opened directory itself, for the cross-process writer lock.
func (d *LedgerDirectory) File() *os.File { return d.handle }

// verify refuses once `.corvint` no longer names the validated directory.
func (d *LedgerDirectory) verify() error {
	info, err := d.repository.Lstat(ledgerDirectoryName)
	if err != nil {
		return d.refuse
	}
	if !info.IsDir() || !os.SameFile(info, d.identity) {
		return d.refuse
	}
	return nil
}

// OpenRegular opens name for reading only when it is a regular file, checked
// against the opened file. A missing file returns its not-exist error.
func (d *LedgerDirectory) OpenRegular(name string) (*os.File, error) {
	if err := d.verify(); err != nil {
		return nil, err
	}
	linkInfo, err := d.directory.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !linkInfo.Mode().IsRegular() {
		return nil, d.refuse
	}
	file, err := d.directory.Open(name)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err == nil && os.SameFile(linkInfo, opened) {
		return file, nil
	}
	file.Close()
	return nil, d.refuse
}

// Replace atomically replaces name with data. It first removes every entry
// named with temporaryPrefix: the caller holds the directory lock, so each is
// left by a writer that died between write and rename and would keep the tree
// dirty.
func (d *LedgerDirectory) Replace(name, temporaryPrefix string, data []byte) error {
	if err := d.verify(); err != nil {
		return err
	}
	d.removeTemporaries(temporaryPrefix)
	temporary, temporaryName, err := d.createTemporary(temporaryPrefix)
	if err != nil {
		return err
	}
	defer d.directory.Remove(temporaryName)
	defer temporary.Close()
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := d.verify(); err != nil {
		return err
	}
	if err := d.directory.Rename(temporaryName, name); err != nil {
		return err
	}
	return d.handle.Sync()
}

func (d *LedgerDirectory) removeTemporaries(prefix string) {
	listing, err := d.directory.Open(".")
	if err != nil {
		return
	}
	entries, _ := listing.ReadDir(-1)
	listing.Close()
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			d.directory.Remove(entry.Name())
		}
	}
}

// createTemporary is os.CreateTemp relative to the directory handle: an
// exclusive create never follows or reuses an existing entry.
func (d *LedgerDirectory) createTemporary(prefix string) (*os.File, string, error) {
	for range 10000 {
		name := prefix + strconv.FormatUint(rand.Uint64(), 36)
		file, err := d.directory.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return file, name, err
	}
	return nil, "", errors.New("ledger temporary name space exhausted")
}

// WriteEmpty creates or truncates name as an empty regular file.
func (d *LedgerDirectory) WriteEmpty(name string) error {
	if err := d.verify(); err != nil {
		return err
	}
	if info, err := d.directory.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return d.refuse
	}
	return d.directory.WriteFile(name, nil, 0o600)
}

// Remove removes name; a missing name is not an error.
func (d *LedgerDirectory) Remove(name string) error {
	if err := d.verify(); err != nil {
		return err
	}
	if err := d.directory.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// OpenLedgerFile opens root/.corvint/name for reading through a validated
// directory handle, under OpenRegular's rules.
func OpenLedgerFile(root, name string, refuse error) (*os.File, error) {
	ledger, err := OpenLedgerDirectory(root, false, refuse)
	if err != nil {
		return nil, err
	}
	defer ledger.Close()
	return ledger.OpenRegular(name)
}
