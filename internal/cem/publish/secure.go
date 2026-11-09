package publish

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// PublishSecure publishes through held directory descriptors. It rejects
// symlinks in every traversed component, locks the parent directory, syncs the
// file before promotion, and syncs the directory after the atomic rename.
func PublishSecure(ctx context.Context, output Output, createParents bool) error {
	select {
	case <-ctx.Done():
		return securePublishError(output.Relative)
	default:
	}
	if _, err := output.Root.resolveOutput(output.Relative); err != nil {
		return err
	}
	before, err := os.Lstat(output.Root.path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, output.Root.info) {
		return securePublishError(output.Relative)
	}
	root, err := os.OpenRoot(output.Root.path)
	if err != nil {
		return securePublishError(output.Relative)
	}
	defer root.Close()
	parentFile, err := root.Open(".")
	if err != nil {
		return securePublishError(output.Relative)
	}
	opened, err := parentFile.Stat()
	if err != nil || !opened.IsDir() || !os.SameFile(opened, output.Root.info) {
		parentFile.Close()
		return securePublishError(output.Relative)
	}
	lockFile, err := root.Open(".")
	if err != nil {
		parentFile.Close()
		return securePublishError(output.Relative)
	}
	if err := lockSecureDirectory(ctx, lockFile); err != nil {
		lockFile.Close()
		parentFile.Close()
		return securePublishError(output.Relative)
	}
	defer func() {
		unlockSecureDirectory(lockFile)
		lockFile.Close()
	}()
	parentRoot := root
	segments := strings.Split(output.Relative, "/")
	for _, segment := range segments[:len(segments)-1] {
		nextFile, nextRoot, openErr := secureChild(parentRoot, parentFile, segment, createParents)
		if parentRoot != root {
			parentRoot.Close()
		}
		parentFile.Close()
		if openErr != nil {
			return securePublishError(output.Relative)
		}
		parentFile, parentRoot = nextFile, nextRoot
	}
	if parentRoot != root {
		defer parentRoot.Close()
	}
	defer parentFile.Close()
	return securePromote(parentRoot, parentFile, segments[len(segments)-1], output)
}

func secureChild(parent *os.Root, parentFile *os.File, name string, create bool) (*os.File, *os.Root, error) {
	info, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) && create {
		if err := parent.Mkdir(name, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, nil, err
		}
		if err := parentFile.Sync(); err != nil {
			return nil, nil, err
		}
		info, err = parent.Lstat(name)
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, nil, errors.New("unsafe output parent")
	}
	file, root, err := openSecureDirectory(parent, name)
	if err != nil {
		return nil, nil, err
	}
	descriptor, statErr := file.Stat()
	if statErr != nil || !descriptor.IsDir() || !os.SameFile(info, descriptor) {
		file.Close()
		root.Close()
		return nil, nil, errors.New("output parent changed")
	}
	return file, root, nil
}

func securePromote(parent *os.Root, parentFile *os.File, name string, output Output) error {
	if info, err := parent.Lstat(name); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return securePublishError(output.Relative)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return securePublishError(output.Relative)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return securePublishError(output.Relative)
	}
	temporary := "." + name + ".tmp-" + hex.EncodeToString(suffix)
	file, err := openSecureMember(parent, temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return securePublishError(output.Relative)
	}
	cleanup := true
	defer func() {
		file.Close()
		if cleanup {
			_ = parent.Remove(temporary)
		}
	}()
	if _, err := io.Copy(file, bytes.NewReader(output.Data)); err != nil {
		return securePublishError(output.Relative)
	}
	if err := file.Sync(); err != nil {
		return securePublishError(output.Relative)
	}
	if err := file.Close(); err != nil {
		return securePublishError(output.Relative)
	}
	if err := parent.Rename(temporary, name); err != nil {
		return securePublishError(output.Relative)
	}
	cleanup = false
	if err := parentFile.Sync(); err != nil {
		return securePublishError(output.Relative)
	}
	return nil
}

func securePublishError(relative string) error {
	return cemcode.New(cemcode.PublishFailed, "output %q could not be published safely", relative)
}
