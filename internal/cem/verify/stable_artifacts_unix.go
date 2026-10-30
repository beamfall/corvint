//go:build darwin || linux

package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func stableFileRoot(f *os.File) (*os.Root, error) {
	prefix := "/dev/fd"
	if runtime.GOOS == "linux" {
		prefix = "/proc/self/fd"
	}
	return os.OpenRoot(fmt.Sprintf("%s/%d", prefix, f.Fd()))
}
func stableArtifactUnavailable() error {
	return cemcode.New("artifact-unavailable", "artifact path or identity unavailable")
}
func stableSameDirectory(before, after os.FileInfo) bool {
	return before != nil && after != nil && before.IsDir() && after.IsDir() && os.SameFile(before, after) && before.Mode() == after.Mode()
}
func stableSameRegular(before, after os.FileInfo) bool {
	return before != nil && after != nil && before.Mode().IsRegular() && after.Mode().IsRegular() && os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime() == after.ModTime()
}

func stableOpenRoot(path string) (*stableArtifactRoot, func(), error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, nil, stableArtifactUnavailable()
	}
	// Do not normalize a symlink away. The explicit root's full named ancestry
	// must consist of directories, and retain those identities across every read.
	names := []string{string(filepath.Separator)}
	current := string(filepath.Separator)
	if path != current {
		for _, part := range strings.Split(strings.TrimPrefix(path, current), string(filepath.Separator)) {
			current = filepath.Join(current, part)
			names = append(names, current)
		}
	}
	infos := make([]os.FileInfo, len(names))
	for i, name := range names {
		info, e := os.Lstat(name)
		if e != nil || !info.IsDir() {
			return nil, nil, stableArtifactUnavailable()
		}
		infos[i] = info
	}
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, nil, stableArtifactUnavailable()
	}
	r, e := stableFileRoot(f)
	if e != nil {
		f.Close()
		return nil, nil, stableArtifactUnavailable()
	}
	closeRoot := func() { r.Close(); f.Close() }
	validate := func() error {
		for i, name := range names {
			info, e := os.Lstat(name)
			if e != nil || !stableSameDirectory(infos[i], info) {
				return stableArtifactUnavailable()
			}
		}
		opened, e := f.Stat()
		if e != nil || !stableSameDirectory(infos[len(infos)-1], opened) {
			return stableArtifactUnavailable()
		}
		rooted, e := r.Stat(".")
		if e != nil || !stableSameDirectory(opened, rooted) {
			return stableArtifactUnavailable()
		}
		return nil
	}
	if e = validate(); e != nil {
		closeRoot()
		return nil, nil, e
	}
	return &stableArtifactRoot{root: r, validate: validate}, closeRoot, nil
}

func stableReadArtifact(root *stableArtifactRoot, path string, bound int) ([]byte, error) {
	if e := root.validate(); e != nil {
		return nil, e
	}
	parent := root.root
	parts := strings.Split(path, "/")
	checks := []func() error{root.validate}
	// Root.OpenFile may follow a confined symlink even with O_NOFOLLOW. The
	// explicit Lstat/type and pre/open/post identity checks therefore govern
	// admission; flags and descriptor confinement are additional bounds only.
	// These checks do not promise immunity to hostile concurrent ABA replacement.
	for _, name := range parts[:len(parts)-1] {
		before, e := parent.Lstat(name)
		if e != nil || !before.IsDir() {
			return nil, stableArtifactUnavailable()
		}
		f, e := parent.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if e != nil {
			return nil, stableArtifactUnavailable()
		}
		defer f.Close()
		child, e := stableFileRoot(f)
		if e != nil {
			return nil, stableArtifactUnavailable()
		}
		defer child.Close()
		edgeParent := parent
		check := func() error {
			named, e := edgeParent.Lstat(name)
			if e != nil || !stableSameDirectory(before, named) {
				return stableArtifactUnavailable()
			}
			opened, e := f.Stat()
			if e != nil || !stableSameDirectory(before, opened) {
				return stableArtifactUnavailable()
			}
			rooted, e := child.Stat(".")
			if e != nil || !stableSameDirectory(before, rooted) {
				return stableArtifactUnavailable()
			}
			return nil
		}
		if e = check(); e != nil {
			return nil, e
		}
		checks = append(checks, check)
		parent = child
	}
	name := parts[len(parts)-1]
	before, e := parent.Lstat(name)
	if e != nil || !before.Mode().IsRegular() {
		return nil, stableArtifactUnavailable()
	}
	f, e := parent.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, stableArtifactUnavailable()
	}
	defer f.Close()
	checkLeaf := func() error {
		named, e := parent.Lstat(name)
		if e != nil || !stableSameRegular(before, named) {
			return stableArtifactUnavailable()
		}
		opened, e := f.Stat()
		if e != nil || !stableSameRegular(before, opened) {
			return stableArtifactUnavailable()
		}
		return nil
	}
	if e = checkLeaf(); e != nil {
		return nil, e
	}
	b, e := stableReadBoundedFile(f, bound)
	if e != nil {
		return nil, e
	}
	if e = checkLeaf(); e != nil {
		return nil, e
	}
	for _, check := range checks {
		if e = check(); e != nil {
			return nil, e
		}
	}
	return b, nil
}
