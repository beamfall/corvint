package worksource

import (
	"errors"
	"os"
	"path/filepath"
)

// RunParentEnvironment names the explicit operator override for the directory
// that holds work run roots and ordinary source acquisition scratch (WQO-V0-094).
const RunParentEnvironment = "CORVINT_WORK_RUN_PARENT"

// ScratchParent returns the directory that holds work run roots and ordinary
// source acquisition scratch. Ambient TMPDIR is never consulted: without the
// override the parent is the fixed "/tmp". A set CORVINT_WORK_RUN_PARENT is
// honored only when it names an absolute, canonical (symlink-free) existing
// directory owned by the current user with mode 0700; any other value, empty
// included, is refused rather than ignored, so a misconfiguration cannot fall
// back to the shared default unnoticed (WQO-V0-094).
func ScratchParent() (string, error) {
	value, set := os.LookupEnv(RunParentEnvironment)
	if !set {
		return "/tmp", nil
	}
	if err := privateParent(value); err != nil {
		return "", errors.New(RunParentEnvironment + " " + err.Error())
	}
	return value, nil
}

func privateParent(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("is not an absolute clean path")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return errors.New("does not resolve to an existing directory")
	}
	if canonical != path {
		return errors.New("is not canonical: it names or passes through a symlink")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return errors.New("is not a directory")
	}
	if info.Mode().Perm() != 0700 {
		return errors.New("is not mode 0700")
	}
	if owner, known := fileOwner(info); !known || owner != os.Getuid() {
		return errors.New("is not owned by the current user")
	}
	return safeAncestors(path)
}

// safeAncestors refuses a parent that another user could rename or replace
// between the check and allocation: every ancestor must be owned by root or the
// current user and, unless sticky, writable by neither group nor others.
func safeAncestors(path string) error {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			return errors.New("has an ancestor that is not a directory")
		}
		if owner, known := fileOwner(info); !known || (owner != 0 && owner != os.Getuid()) {
			return errors.New("has an ancestor owned by another user")
		}
		if info.Mode().Perm()&0022 != 0 && info.Mode()&os.ModeSticky == 0 {
			return errors.New("has an ancestor writable by other users")
		}
		if parent := filepath.Dir(dir); parent == dir {
			return nil
		}
	}
}
