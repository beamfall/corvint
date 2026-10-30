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

// errPathChanged is the typed refusal for a dirty path replaced between its
// Lstat and its open (STR-V0-060).
var errPathChanged = errors.New("it changed while it was digested")

// pathDigestSwapHook, when set by a test, runs between the Lstat and the open
// so a swap in that window can be exercised deterministically.
var pathDigestSwapHook func(name string)

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
	if pathDigestSwapHook != nil {
		pathDigestSwapHook(name)
	}
	// Open without following a final symlink or blocking on a FIFO, then
	// digest only the regular file the handle proves is the one the Lstat saw
	// (STR-V0-060; V1-2129).
	file, err := openInput(name)
	if err != nil {
		// A filesystem can hand a freed inode number straight to the
		// replacement (ext4 does), so a changed file type also proves a swap.
		if now, lerr := os.Lstat(name); lerr != nil || !os.SameFile(info, now) || now.Mode().Type() != info.Mode().Type() {
			return "", fmt.Errorf("%w: %v", errPathChanged, err)
		}
		return "", err
	}
	defer file.Close()
	if opened, err := file.Stat(); err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return "", errPathChanged
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
