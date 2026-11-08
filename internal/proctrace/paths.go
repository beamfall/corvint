// SPDX-License-Identifier: Apache-2.0

// RSP-V0-007 and RSP-V0-049: repository paths and conservative write provenance.
package proctrace

import (
	"os"
	"path/filepath"
	"strings"
)

type collector struct {
	q       Request
	r       *Result
	paths   map[Path]bool
	created map[string]bool
}

func within(root, name string) (string, bool) {
	rel, err := filepath.Rel(root, name)
	return filepath.ToSlash(rel), err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolve also resolves the existing prefix of a missing lookup.
func resolve(name string) (string, error) {
	result, err := filepath.EvalSymlinks(name)
	if err == nil {
		return result, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(name)
	if parent == name {
		return "", err
	}
	prefix, err := resolve(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(prefix, filepath.Base(name)), nil
}
func (c *collector) repository(name string) (string, string) {
	id, rel := "", ""
	longest := -1
	for _, repo := range c.q.Repositories {
		if p, ok := within(repo.Root, name); ok && len(repo.Root) > longest {
			id, rel, longest = repo.ID, p, len(repo.Root)
		}
	}
	return id, rel
}
func (c *collector) path(lexical, resolved string, reading bool) (Path, bool) {
	id, p := c.repository(lexical)
	rid, rp := c.repository(resolved)
	if rid == "" {
		class := "other"
		longest := -1
		for _, outside := range c.q.Outside {
			if _, ok := within(outside.Root, resolved); ok && len(outside.Root) > longest {
				class, longest = outside.Class, len(outside.Root)
			}
		}
		c.r.Outside[class]++
		if reading && class == "other" {
			mark(&c.r.Reads, Incomplete, "undeclared-outside-read")
		}
		// A repository symlink into an excluded class still has a lexical dependency.
		if id == "" {
			return Path{}, false
		}
	}
	path := Path{Repository: id, Path: p, ResolvedRepository: rid, ResolvedPath: rp}
	if !c.paths[path] {
		if len(c.paths) >= c.q.PathCap {
			c.r.unavailable(Incomplete, "path-cap")
			return Path{}, false
		}
		c.paths[path] = true
	}
	return path, true
}
func appendUnique[T comparable](entries *[]T, value T) {
	for _, old := range *entries {
		if old == value {
			return
		}
	}
	*entries = append(*entries, value)
}
func (c *collector) read(lexical, resolved string) {
	p, ok := c.path(lexical, resolved, true)
	if ok && !c.created[resolved] {
		appendUnique(&c.r.Reads.Entries, p)
	}
}
func (c *collector) write(lexical, resolved, kind string, created bool) {
	// RSP-V0-007/021: every successful mutation depends on the prior path
	// unless the caller proved exclusive creation or replacement by a tree-owned
	// file. Unknown write forms cannot establish ownership, even with this bit.
	created = created && (kind == "create" || kind == "replace")
	if !created {
		c.read(lexical, resolved)
	}
	p, ok := c.path(lexical, resolved, false)
	if ok {
		appendUnique(&c.r.Writes.Entries, Write{Path: p, Kind: kind})
	}
	if created && ok {
		c.created[resolved] = true
	}
}
