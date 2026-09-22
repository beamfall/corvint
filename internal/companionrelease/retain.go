package companionrelease

import (
	"fmt"
	"os"
	"path/filepath"
)

// retainBundle stages files into a fresh scratch directory under scratch,
// then atomically renames that directory into place as
// outputParent/dirName. It never overwrites an existing directory: dirName
// must be unclaimed, and a caller retrying with the same name after a
// partial failure gets a clear refusal rather than silently merged output.
//
// outputParent must not be inside either checkout root the caller built
// from; companionrelease.Run enforces that before calling this.
func retainBundle(scratch, outputParent, dirName string, files []ArchiveEntry) (string, error) {
	target := filepath.Join(outputParent, dirName)
	if _, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("refusing to overwrite existing retained output %s", target)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat retained output target: %w", err)
	}

	staging := filepath.Join(scratch, "retain-staging-"+dirName)
	if err := os.RemoveAll(staging); err != nil {
		return "", err
	}
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return "", err
	}
	for _, f := range files {
		full := filepath.Join(staging, f.Path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return "", err
		}
		if err := os.WriteFile(full, f.Data, os.FileMode(f.Mode)); err != nil {
			return "", fmt.Errorf("write %s: %w", f.Path, err)
		}
	}

	if err := os.MkdirAll(outputParent, 0o700); err != nil {
		return "", err
	}
	if err := os.Rename(staging, target); err != nil {
		return "", fmt.Errorf("atomic retain rename: %w", err)
	}
	if err := os.Chmod(target, 0o700); err != nil {
		return "", err
	}
	return target, nil
}
