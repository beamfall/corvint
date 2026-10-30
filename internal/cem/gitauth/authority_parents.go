package gitauth

import (
	"context"
	"os"
	"sort"
	"strings"
)

const authorityDirectoryWitnessLimit = (4<<20)/2 + 1 // each parent consumes at least one path byte plus slash

const authorityParentHandles = 30 // root + cached parents; two reopen scratch slots remain.

type authorityParent struct {
	root       *os.Root
	path       string
	pins, used int
}
type authorityParents struct {
	ctx      context.Context
	rootPath string
	cache    map[string]*authorityParent
	observed map[string]os.FileInfo
	clock    int
}

func newAuthorityParents(ctx context.Context, path string, root *os.Root, info os.FileInfo) *authorityParents {
	return &authorityParents{ctx: ctx, rootPath: path, cache: map[string]*authorityParent{"": {root: root, pins: 1}}, observed: map[string]os.FileInfo{"": info}}
}
func authoritySplit(path string) (string, string) {
	i := strings.LastIndexByte(path, '/')
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}
func (p *authorityParents) touch(n *authorityParent)   { p.clock++; n.used = p.clock }
func (p *authorityParents) release(n *authorityParent) { n.pins-- }
func (p *authorityParents) evict() error {
	var oldest *authorityParent
	for _, n := range p.cache {
		if n.pins == 0 && (oldest == nil || n.used < oldest.used) {
			oldest = n
		}
	}
	if oldest == nil {
		return unavailable("authority directory handle budget")
	}
	delete(p.cache, oldest.path)
	return oldest.root.Close()
}

// get retains one pin for the caller. Eviction affects only descriptors: every
// observed namespace edge survives as metadata and is revalidated after hashing.
// Reopening walks only through retained bound directory handles, never an
// absolute pathname or a timestamp-only replacement for the byte scan.
func (p *authorityParents) get(path string) (*authorityParent, error) {
	if p.ctx.Err() != nil {
		return nil, unavailable("authority snapshot cancelled")
	}
	prefix := path
	var current *authorityParent
	for {
		if current = p.cache[prefix]; current != nil {
			break
		}
		prefix, _ = authoritySplit(prefix)
	}
	current.pins++
	p.touch(current)
	remainder := strings.TrimPrefix(path, prefix)
	remainder = strings.TrimPrefix(remainder, "/")
	if remainder == "" {
		return current, nil
	}
	for _, part := range strings.Split(remainder, "/") {
		if p.ctx.Err() != nil {
			p.release(current)
			return nil, unavailable("authority snapshot cancelled")
		}
		end := len(current.path) + len(part)
		if current.path != "" {
			end++
		}
		nextPath := path[:end] // retain the admitted leaf string, not quadratic copied prefixes
		if cached := p.cache[nextPath]; cached != nil {
			cached.pins++
			p.touch(cached)
			p.release(current)
			current = cached
			continue
		}
		before, err := current.root.Lstat(part)
		if err != nil || !before.IsDir() {
			p.release(current)
			return nil, unavailable("authority directory unavailable")
		}
		child, err := openAuthorityDirectory(current.root, part)
		if err != nil {
			p.release(current)
			return nil, unavailable("authority directory open")
		}
		info, err := child.Stat(".")
		after, e := current.root.Lstat(part)
		if err != nil || e != nil || !sameAuthorityMetadata(before, info) || !sameAuthorityMetadata(info, after) {
			child.Close()
			p.release(current)
			return nil, unavailable("authority directory changed")
		}
		if original, ok := p.observed[nextPath]; ok {
			if !sameAuthorityMetadata(original, info) {
				child.Close()
				p.release(current)
				return nil, unavailable("authority reopened directory changed")
			}
		} else {
			if len(p.observed) >= authorityDirectoryWitnessLimit {
				child.Close()
				p.release(current)
				return nil, unavailable("authority directory metadata bound")
			}
			p.observed[nextPath] = info
		}
		p.release(current)
		for len(p.cache) >= authorityParentHandles {
			if err := p.evict(); err != nil {
				child.Close()
				return nil, err
			}
		}
		current = &authorityParent{root: child, path: nextPath, pins: 1}
		p.touch(current)
		p.cache[nextPath] = current
	}
	return current, nil
}
func (p *authorityParents) close() {
	for name, n := range p.cache {
		if name != "" {
			n.root.Close()
		}
	}
}
func (p *authorityParents) check() error {
	root, err := os.Lstat(p.rootPath)
	if err != nil || !sameAuthorityMetadata(root, p.observed[""]) {
		return unavailable("authority root changed")
	}
	paths := make([]string, 0, len(p.observed))
	for path := range p.observed {
		if path != "" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		child, err := p.get(path)
		if err != nil {
			return err
		}
		parentPath, name := authoritySplit(path)
		parent, err := p.get(parentPath)
		if err != nil {
			p.release(child)
			return err
		}
		named, e := parent.root.Lstat(name)
		held, h := child.root.Stat(".")
		p.release(parent)
		p.release(child)
		if e != nil || h != nil || !sameAuthorityMetadata(p.observed[path], named) || !sameAuthorityMetadata(named, held) {
			return unavailable("authority namespace changed")
		}
	}
	root, err = os.Lstat(p.rootPath)
	if err != nil || !sameAuthorityMetadata(root, p.observed[""]) {
		return unavailable("authority root changed")
	}
	return nil
}
