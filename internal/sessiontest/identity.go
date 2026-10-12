package sessiontest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Identify binds head, tree and the bytes of every dirty path (STR-V0-017).
// A path that is gone is "deleted"; a symlink binds its target text, never
// what it points at, because that is what Git would commit. A dirty
// directory is refused.
func Identify(root, head, tree string, dirty []string) (SourceIdentity, error) {
	paths := append([]string(nil), dirty...)
	sort.Strings(paths)
	identity := SourceIdentity{Head: head, Tree: tree, Paths: make([]PathDigest, 0, len(paths))}
	for _, name := range paths {
		digest, err := pathDigest(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return SourceIdentity{}, fmt.Errorf("dirty path %s: %w", name, err)
		}
		identity.Paths = append(identity.Paths, PathDigest{Path: name, Digest: digest})
	}
	return identity, nil
}

func pathDigest(name string) (string, error) {
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return "deleted", nil
	}
	if err != nil {
		return "", err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(name)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256([]byte(target))
		return "symlink:sha256:" + hex.EncodeToString(sum[:]), nil
	case info.IsDir():
		// Git reports a dirty submodule or nested repository as one directory
		// path; its contents cannot be bound here, so refuse rather than bind
		// a constant that no later edit inside it would change.
		return "", errors.New("it is a directory (a submodule or nested repository) whose contents cannot be bound")
	case !info.Mode().IsRegular():
		return "special:" + info.Mode().Type().String(), nil
	}
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
