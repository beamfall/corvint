package stepverify

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
)

const maxPointerBytes = 8192

func pointer(root *os.Root, name string) (string, error) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPointerBytes {
		return "", ErrUnsupported
	}
	data, _, err := regular(root, name, &inventoryBudget{})
	if err != nil {
		return "", err
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" || !textSafe(text) {
		return "", ErrUnsupported
	}
	return text, nil
}
func pointerPath(base, target string) (string, error) {
	if !filepath.IsAbs(target) {
		target = filepath.Join(base, target)
	}
	target = filepath.Clean(target)
	if !absolute(target) {
		return "", ErrUnsupported
	}
	return target, nil
}

// Resolve the primary/reciprocal pointer chain through no-follow descriptors
// before the existing immutable Git authority reader may inspect that boundary.
func worktreeDirectories(rootPath string) (string, string, error) {
	root, err := safeopen.Root(rootPath)
	if err != nil {
		return "", "", ErrUnsupported
	}
	defer root.Close()
	marker, err := root.Lstat(".git")
	if err != nil {
		return "", "", ErrUnsupported
	}
	if marker.IsDir() {
		dir, err := safeopen.SubRoot(root, ".git")
		if err != nil {
			return "", "", ErrUnsupported
		}
		dir.Close()
		path := filepath.Join(rootPath, ".git")
		return path, path, nil
	}
	line, err := pointer(root, ".git")
	if err != nil {
		return "", "", err
	}
	target, ok := strings.CutPrefix(line, "gitdir: ")
	if !ok {
		return "", "", ErrUnsupported
	}
	gitdir, err := pointerPath(rootPath, target)
	if err != nil {
		return "", "", err
	}
	dir, err := safeopen.Root(gitdir)
	if err != nil {
		return "", "", ErrUnsupported
	}
	defer dir.Close()
	back, err := pointer(dir, "gitdir")
	if err != nil {
		return "", "", err
	}
	back, err = pointerPath(gitdir, back)
	if err != nil || back != filepath.Join(rootPath, ".git") {
		return "", "", ErrUnsupported
	}
	common := gitdir
	if _, err := dir.Lstat("commondir"); !os.IsNotExist(err) {
		if err != nil {
			return "", "", ErrUnsupported
		}
		value, err := pointer(dir, "commondir")
		if err != nil {
			return "", "", err
		}
		common, err = pointerPath(gitdir, value)
		if err != nil {
			return "", "", err
		}
	}
	shared, err := safeopen.Root(common)
	if err != nil {
		return "", "", ErrUnsupported
	}
	shared.Close()
	return gitdir, common, nil
}

type boundary struct{ path, identity string }

func boundaryAt(path string) (boundary, error) {
	root, err := safeopen.Root(path)
	if err != nil {
		return boundary{}, ErrUnsupported
	}
	defer root.Close()
	info, err := root.Stat(".")
	if err != nil {
		return boundary{}, ErrUnsupported
	}
	identity, _, _, ok := nativeInfo(info)
	if !ok {
		return boundary{}, ErrUnsupported
	}
	return boundary{path, identity}, nil
}
func outsideBound(ctx context.Context, path string, bounds []boundary) error {
	for _, bound := range bounds {
		if path == bound.path || strings.HasPrefix(path, strings.TrimSuffix(bound.path, "/")+"/") {
			return ErrUnsupported
		}
	}
	for {
		if ctx.Err() != nil {
			return ErrUnsupported
		}
		root, err := safeopen.Root(path)
		if err != nil {
			return ErrUnsupported
		}
		info, err := root.Stat(".")
		root.Close()
		if err != nil {
			return ErrUnsupported
		}
		identity, _, _, ok := nativeInfo(info)
		if !ok {
			return ErrUnsupported
		}
		for _, bound := range bounds {
			if identity == bound.identity {
				return ErrUnsupported
			}
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		path = parent
	}
}
func outside(ctx context.Context, path string, roots []string) error {
	bounds := []boundary{}
	for _, r := range roots {
		b, err := boundaryAt(r)
		if err != nil {
			return err
		}
		bounds = append(bounds, b)
	}
	return outsideBound(ctx, path, bounds)
}
func readOutside(ctx context.Context, path string, bounds []boundary) ([]byte, error) {
	if !absolute(path) || !safeopen.Supported {
		return nil, ErrInput
	}
	parent := filepath.Dir(path)
	if outsideBound(ctx, parent, bounds) != nil {
		return nil, ErrUnsupported
	}
	root, err := safeopen.Root(parent)
	if err != nil {
		return nil, ErrUnsupported
	}
	defer root.Close()
	data, _, err := regular(root, filepath.Base(path), &inventoryBudget{})
	if err != nil {
		return nil, err
	}
	if outsideBound(ctx, parent, bounds) != nil {
		return nil, ErrUnsupported
	}
	return data, nil
}

// ReadBoundInput retains the trusted complete before-state authority boundary
// when post-step metadata is unavailable. Current discoverable boundaries are
// also excluded. This permits content findings without executing unadmitted Git.
func ReadBoundInput(ctx context.Context, path string, d Declaration, h Host, before State) ([]byte, error) {
	d, h, err := inputs(d, h)
	if err != nil {
		return nil, err
	}
	before, err = DecodeState(Encode(before), d, h)
	if err != nil {
		return nil, err
	}
	bounds := []boundary{}
	for _, c := range before.Checkouts {
		bounds = append(bounds, boundary{c.Root, c.Entries[0].Identity})
		for _, part := range []struct{ label, path string }{{"git", c.GitDir}, {"common", c.CommonDir}} {
			identity := ""
			for _, entry := range c.AdminEntries {
				if entry.Path == part.label && entry.Kind == "DIRECTORY" {
					identity = entry.Identity
				}
			}
			if identity == "" {
				return nil, ErrInput
			}
			bounds = append(bounds, boundary{part.path, identity})
			// A still-present authority root's current identity also blocks aliases after ownership drift.
			if current, err := boundaryAt(part.path); err == nil {
				bounds = append(bounds, current)
			}
		}
		if current, err := boundaryAt(c.Root); err == nil {
			bounds = append(bounds, current)
		}
		gitdir, common, err := worktreeDirectories(c.Root)
		if err == nil {
			for _, path := range []string{gitdir, common} {
				current, err := boundaryAt(path)
				if err != nil {
					return nil, err
				}
				bounds = append(bounds, current)
			}
		}
	}
	return readOutside(ctx, path, bounds)
}
