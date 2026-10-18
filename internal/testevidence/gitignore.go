package testevidence

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// retainedIgnore ignores everything in its directory, itself included, so
// retained evidence never shows in `git status` (LPCV-V0-084).
const retainedIgnore = "*\n"

// corvintDirectory is the top component of the retained-evidence location.
var corvintDirectory = strings.Split(EvidenceDirectory, "/")[0]

// IgnoreRetained keeps a retention out of `git status` (LPCV-V0-084). When the
// retention created the worktree's .corvint directory, the whole directory is
// Corvint-owned state and .corvint/.gitignore ignores it; a .corvint that
// already existed may hold tracked project files, so only the evidence
// directory itself is ignored. A non-empty regular file is never overwritten,
// an empty one is repaired, and a non-regular entry fails the retention. A provider
// with its own retention path calls this after opening its evidence directory.
func IgnoreRetained(worktree, evidence *os.Root, createdCorvint bool) error {
	if !createdCorvint {
		return writeIgnore(evidence)
	}
	corvint, err := openDirectory(worktree, corvintDirectory)
	if err != nil {
		return err
	}
	defer corvint.Close()
	return writeIgnore(corvint)
}

// corvintMissing reports whether the worktree has no .corvint entry yet.
func corvintMissing(worktree *os.Root) bool {
	_, err := worktree.Lstat(corvintDirectory)
	return errors.Is(err, fs.ErrNotExist)
}

// writeIgnore creates the directory's .gitignore exclusively. A file it
// created but could not write is removed, so a later run never mistakes it
// for protection; an existing entry goes to keepOrRepairIgnore.
func writeIgnore(directory *os.Root) error {
	file, err := directory.OpenFile(".gitignore", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return keepOrRepairIgnore(directory)
	}
	if err != nil {
		return fmt.Errorf("retained-evidence .gitignore cannot be created: %w", err)
	}
	if err := writeIgnoreFile(file); err != nil {
		_ = directory.Remove(".gitignore")
		return fmt.Errorf("retained-evidence .gitignore cannot be written: %w", err)
	}
	return nil
}

// keepOrRepairIgnore keeps an existing non-empty regular .gitignore, writes
// the ignore rule into an empty one, and refuses a directory, symlink or any
// other non-regular entry, which Git cannot use as the ignore file.
func keepOrRepairIgnore(directory *os.Root) error {
	info, err := directory.Lstat(".gitignore")
	if err != nil {
		return fmt.Errorf("retained-evidence .gitignore cannot be inspected: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("retained-evidence .gitignore is not a regular file (%v)", info.Mode().Type())
	}
	if info.Size() > 0 {
		return nil
	}
	file, err := directory.OpenFile(".gitignore", os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("retained-evidence .gitignore cannot be repaired: %w", err)
	}
	if opened, err := file.Stat(); err != nil || !os.SameFile(info, opened) || opened.Size() != 0 {
		_ = file.Close()
		return errors.New("retained-evidence .gitignore changed during repair")
	}
	if err := writeIgnoreFile(file); err != nil {
		return fmt.Errorf("retained-evidence .gitignore cannot be repaired: %w", err)
	}
	return nil
}

// writeIgnoreFile writes the ignore rule and closes file; tests replace it to
// inject a failure after the file exists.
var writeIgnoreFile = func(file *os.File) error {
	_, err := file.WriteString(retainedIgnore)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
