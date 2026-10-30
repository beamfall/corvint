package worksource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

// VerifyMaterialization checks raw pinned bytes without consulting ambient Git.
// Extra paths, including generated private .git, are a separate run manifest.
func (source *Source) VerifyMaterialization(ctx context.Context, directory string) error {
	return source.readEntries(ctx, directory, false)
}

func (source *Source) readEntries(ctx context.Context, directory string, capture bool) error {
	before, err := os.Lstat(directory)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid materialization root")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(before, opened) {
		return errors.New("materialization root drift")
	}
	total := 0
	seen := make(map[[2]uint64]string)
	for i := range source.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry := &source.Entries[i]
		raw, err := readEntry(root, *entry, fileDevice(opened), seen)
		if err != nil {
			return fmt.Errorf("source entry %s: %w", entry.Path, err)
		}
		total += len(raw)
		if total > maxBytes {
			return errors.New("materialization byte limit")
		}
		if objectID(source.Identity.ObjectFormat, "blob", raw) != entry.BlobOID {
			return fmt.Errorf("source bytes differ: %s", entry.Path)
		}
		if !capture && !bytes.Equal(raw, entry.Raw) {
			return errors.New("source byte drift")
		}
		if capture {
			entry.Raw = raw
		}
	}
	if err := validateLinks(source.Entries); err != nil {
		return err
	}
	for _, entry := range source.Entries {
		if entry.Mode != "120000" {
			continue
		}
		resolved, err := root.Stat(entry.Path)
		if err != nil || !resolved.Mode().IsRegular() || fileDevice(resolved) != fileDevice(opened) {
			return errors.New("symlink does not resolve to contained tracked file")
		}
	}
	final, err := os.Lstat(directory)
	if err != nil || !os.SameFile(opened, final) || opened.Mode() != final.Mode() {
		return errors.New("materialization root drift")
	}
	return nil
}

func readEntry(root *os.Root, entry Entry, device uint64, seen map[[2]uint64]string) ([]byte, error) {
	components := strings.Split(entry.Path, "/")
	directory := root
	logical := ""
	// Hold every parent descriptor until the leaf and parent identities are checked.
	var checks []func() error
	defer func() {
		for i := len(checks) - 1; i >= 0; i-- {
			_ = checks[i]()
		}
	}()
	for _, name := range components[:len(components)-1] {
		logical = path.Join(logical, name)
		expected, err := directory.Lstat(name)
		if err != nil || !expected.IsDir() || fileDevice(expected) != device {
			return nil, errors.New("unsupported parent or mount")
		}
		child, err := directory.OpenRoot(name)
		if err != nil {
			return nil, err
		}
		actual, err := child.Stat(".")
		if err != nil || !os.SameFile(expected, actual) || expected.Mode() != actual.Mode() {
			child.Close()
			return nil, errors.New("parent drift")
		}
		if err := bindEntryIdentity(seen, logical, actual); err != nil {
			child.Close()
			return nil, err
		}
		parent := directory
		checks = append(checks, func() error {
			defer child.Close()
			final, err := parent.Lstat(name)
			if err != nil || !os.SameFile(actual, final) || actual.Mode() != final.Mode() {
				return errors.New("parent drift")
			}
			return nil
		})
		directory = child
	}
	name := components[len(components)-1]
	before, err := directory.Lstat(name)
	if err != nil || fileDevice(before) != device {
		return nil, errors.New("unavailable entry or mount")
	}
	if err := bindEntryIdentity(seen, entry.Path, before); err != nil {
		return nil, err
	}
	var raw []byte
	if entry.Mode == "120000" {
		if before.Mode()&os.ModeSymlink == 0 {
			return nil, errors.New("symlink mode drift")
		}
		target, err := directory.Readlink(name)
		if err != nil {
			return nil, err
		}
		raw = []byte(target)
	} else {
		raw, err = readRegular(directory, name, before, entry.Mode)
		if err != nil {
			return nil, err
		}
	}
	after, err := directory.Lstat(name)
	if err != nil || !sameEntry(before, after) {
		return nil, errors.New("entry drift")
	}
	for i := len(checks) - 1; i >= 0; i-- {
		if err := checks[i](); err != nil {
			return nil, err
		}
	}
	checks = nil
	return raw, nil
}
func sameEntry(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}
func readRegular(root *os.Root, name string, before os.FileInfo, mode string) ([]byte, error) {
	if !before.Mode().IsRegular() || fileLinks(before) != 1 || before.Size() > maxFileBytes {
		return nil, errors.New("unsupported regular entry")
	}
	executable := before.Mode().Perm()&0111 != 0
	if executable != (mode == "100755") {
		return nil, errors.New("executable mode drift")
	}
	file, err := root.OpenFile(name, noFollowReadFlags(), 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !sameEntry(before, opened) {
		return nil, errors.New("opened entry drift")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil || len(raw) > maxFileBytes {
		return nil, errors.New("incomplete entry read")
	}
	after, err := file.Stat()
	if err != nil || !sameEntry(opened, after) || int64(len(raw)) != opened.Size() {
		return nil, errors.New("read entry drift")
	}
	return raw, nil
}
func validateLinks(entries []Entry) error {
	byPath := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	for _, entry := range entries {
		if entry.Mode != "120000" {
			continue
		}
		seen := make(map[string]bool)
		current := entry
		for current.Mode == "120000" {
			if seen[current.Path] {
				return errors.New("cyclic source symlink")
			}
			seen[current.Path] = true
			target := string(current.Raw)
			if !utf8.ValidString(target) || strings.HasPrefix(target, "/") || strings.Contains(target, "\\") || strings.ContainsRune(target, 0) {
				return errors.New("unsupported symlink target")
			}
			resolved := path.Clean(path.Join(path.Dir(current.Path), target))
			if !validPath(resolved) {
				return errors.New("escaping source symlink")
			}
			var ok bool
			current, ok = byPath[resolved]
			if !ok {
				return errors.New("dangling or directory source symlink")
			}
		}
	}
	return nil
}

// The actual filesystem mapping, rather than a guessed Unicode normalization
// rule, must preserve every distinct Git path and directory spelling.
func bindEntryIdentity(seen map[[2]uint64]string, path string, info os.FileInfo) error {
	key := fileIdentity(info)
	if previous, ok := seen[key]; ok && previous != path {
		return errors.New("ambiguous filesystem path mapping")
	}
	seen[key] = path
	return nil
}
