package testevidence

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// retainedIgnore ignores everything in its directory, itself included, so
// retained evidence never shows in `git status` (LPCV-V0-084).
const retainedIgnore = "*\n"

// corvintDirectory is the top component of the retained-evidence location.
var corvintDirectory = strings.Split(testvaliditydoc.EvidenceDirectory, "/")[0]

// IgnoreRetained keeps a retention out of `git status` (LPCV-V0-084). When the
// retention created the worktree's .corvint directory, the whole directory is
// Corvint-owned state and .corvint/.gitignore ignores it; a .corvint that
// already existed may hold tracked project files, so only the evidence
// directory itself is ignored. Neither file is ever overwritten. A provider
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

func writeIgnore(directory *os.Root) error {
	file, err := directory.OpenFile(".gitignore", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("retained-evidence .gitignore cannot be created: %w", err)
	}
	_, err = file.WriteString(retainedIgnore)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("retained-evidence .gitignore cannot be written: %w", err)
	}
	return nil
}
