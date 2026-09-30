package stepverify

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
)

type inventoryBudget struct {
	entries int
	bytes   int64
}

func regular(root *os.Root, name string, b *inventoryBudget) ([]byte, os.FileInfo, error) {
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() {
		return nil, nil, ErrUnsupported
	}
	_, stamp, links, ok := nativeInfo(before)
	if !ok || links != 1 || before.Size() < 0 || before.Size() > MaxFileBytes {
		return nil, nil, ErrUnsupported
	}
	f, err := safeopen.InRoot(root, name, os.O_RDONLY, 0, false)
	if err != nil {
		return nil, nil, ErrUnsupported
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, nil, ErrDrift
	}
	_, openedStamp, _, ok := nativeInfo(opened)
	if !ok || stamp != openedStamp {
		return nil, nil, ErrDrift
	}
	if b.bytes+before.Size() > MaxTotalBytes {
		return nil, nil, ErrUnsupported
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil || int64(len(data)) != before.Size() {
		return nil, nil, ErrDrift
	}
	after, err := f.Stat()
	if err != nil {
		return nil, nil, ErrDrift
	}
	_, afterStamp, _, ok := nativeInfo(after)
	leaf, err := root.Lstat(name)
	if err != nil || !ok || afterStamp != stamp || !os.SameFile(before, leaf) {
		return nil, nil, ErrDrift
	}
	_, leafStamp, _, ok := nativeInfo(leaf)
	if !ok || leafStamp != stamp {
		return nil, nil, ErrDrift
	}
	b.bytes += int64(len(data))
	return data, before, nil
}
func entry(path string, info os.FileInfo, data []byte) (Entry, error) {
	identity, _, _, ok := nativeInfo(info)
	if !ok {
		return Entry{}, ErrUnsupported
	}
	e := Entry{Path: path, Mode: uint32(info.Mode()), Digest: hash(data), Identity: identity}
	switch {
	case info.IsDir():
		e.Kind = "DIRECTORY"
	case info.Mode().IsRegular():
		e.Kind = "FILE"
		e.Size = info.Size()
	case info.Mode()&os.ModeSymlink != 0:
		e.Kind = "SYMLINK"
		e.Size = int64(len(data))
	default:
		return Entry{}, ErrUnsupported
	}
	return e, nil
}

// Every traversal/open is relative to a held directory descriptor. Paths in
// the result are labels, never the authority used to open children.
func inventory(ctx context.Context, path string, b *inventoryBudget, worktree bool) ([]Entry, map[string][]byte, error) {
	entries := []Entry{}
	raw := map[string][]byte{}
	if !safeopen.Supported {
		return entries, raw, ErrUnsupported
	}
	root, err := safeopen.Root(path)
	if err != nil {
		return entries, raw, ErrUnsupported
	}
	defer root.Close()
	var walk func(*os.Root, string, int) error
	walk = func(dir *os.Root, prefix string, depth int) error {
		if ctx.Err() != nil || depth > MaxDepth {
			return ErrUnsupported
		}
		before, err := dir.Stat(".")
		if err != nil {
			return ErrUnsupported
		}
		_, stamp, _, ok := nativeInfo(before)
		if !ok {
			return ErrUnsupported
		}
		e, err := entry(prefix, before, nil)
		if err != nil {
			return err
		}
		entries = append(entries, e)
		b.entries++
		if b.entries > MaxEntries {
			return ErrUnsupported
		}
		fd, err := safeopen.InRoot(dir, ".", os.O_RDONLY, 0, true)
		if err != nil {
			return ErrUnsupported
		}
		names := []string{}
		for {
			items, readErr := fd.ReadDir(128)
			for _, item := range items {
				names = append(names, item.Name())
				if len(names)+b.entries > MaxEntries {
					fd.Close()
					return ErrUnsupported
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				fd.Close()
				return ErrUnsupported
			}
		}
		fd.Close()
		sort.Strings(names)
		for _, name := range names {
			if worktree && depth == 0 && name == ".git" {
				continue
			}
			if ctx.Err() != nil || !literal(name) || name == ".git" {
				return ErrUnsupported
			}
			rel := name
			if prefix != "." {
				rel = prefix + "/" + name
			}
			if !literal(rel) {
				return ErrUnsupported
			}
			info, err := dir.Lstat(name)
			if err != nil {
				return ErrUnsupported
			}
			if info.IsDir() {
				child, err := safeopen.SubRoot(dir, name)
				if err != nil {
					return ErrUnsupported
				}
				pinned, err := child.Stat(".")
				if err != nil || !os.SameFile(info, pinned) {
					child.Close()
					return ErrDrift
				}
				err = walk(child, rel, depth+1)
				child.Close()
				if err != nil {
					return err
				}
			} else {
				var data []byte
				if info.Mode().IsRegular() {
					data, info, err = regular(dir, name, b)
				} else if info.Mode()&os.ModeSymlink != 0 {
					_, stamp, _, ok := nativeInfo(info)
					if !ok {
						return ErrUnsupported
					}
					var target string
					target, err = dir.Readlink(name)
					data = []byte(target)
					after, statErr := dir.Lstat(name)
					if statErr != nil {
						return ErrDrift
					}
					_, afterStamp, _, ok := nativeInfo(after)
					if !ok || stamp != afterStamp || !os.SameFile(info, after) || len(data) > 1024 {
						return ErrDrift
					}
				} else {
					return ErrUnsupported
				}
				if err != nil {
					return err
				}
				e, err := entry(rel, info, data)
				if err != nil {
					return err
				}
				entries = append(entries, e)
				b.entries++
				if b.entries > MaxEntries {
					return ErrUnsupported
				}
				if !worktree {
					raw[rel] = data
				}
			}
			after, err := dir.Lstat(name)
			if err != nil || !os.SameFile(info, after) {
				return ErrDrift
			}
		}
		after, err := dir.Stat(".")
		if err != nil {
			return ErrDrift
		}
		_, afterStamp, _, ok := nativeInfo(after)
		if !ok || stamp != afterStamp || !os.SameFile(before, after) {
			return ErrDrift
		}
		return nil
	}
	err = walk(root, ".", 0)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, raw, err
}

// ReadInput reads a bounded regular single-link host input through no-follow
// descriptors. When d is present, aliases of checkout/authority ancestry refuse.
func ReadInput(ctx context.Context, path string, d *Declaration) ([]byte, error) {
	if !absolute(path) || !safeopen.Supported {
		return nil, ErrInput
	}
	if d != nil {
		roots := []string{}
		for _, c := range append([]Checkout{d.Author}, d.ReadOnly...) {
			gitdir, common, err := worktreeDirectories(c.Root)
			if err != nil {
				return nil, ErrUnsupported
			}
			roots = append(roots, c.Root, gitdir, common)
		}
		if outside(ctx, filepath.Dir(path), roots) != nil {
			return nil, ErrUnsupported
		}
	}
	root, err := safeopen.Root(filepath.Dir(path))
	if err != nil {
		return nil, ErrUnsupported
	}
	defer root.Close()
	data, _, err := regular(root, filepath.Base(path), &inventoryBudget{})
	return data, err
}

func scopeAncestors(root string, paths []string) error {
	dir, err := safeopen.Root(root)
	if err != nil {
		return ErrUnsupported
	}
	defer dir.Close()
	for _, path := range paths {
		p := strings.TrimSuffix(path, "/")
		if !strings.HasSuffix(path, "/") {
			p = filepath.Dir(p)
		}
		parts := strings.Split(p, "/")
		current := ""
		for _, part := range parts {
			if part == "." {
				continue
			}
			if current != "" {
				current += "/"
			}
			current += part
			info, err := dir.Lstat(current)
			if os.IsNotExist(err) {
				break
			}
			if err != nil || !info.IsDir() {
				return ErrUnsupported
			}
			held, err := safeopen.SubRoot(dir, current)
			if err != nil {
				return ErrUnsupported
			}
			held.Close()
		}
	}
	return nil
}
