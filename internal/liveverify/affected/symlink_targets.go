// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// FrontierShellSymlinkTarget prefixes the AFP-V0-9965 unknown detail
// "shell:symlink-target:<path>" of a dirty path that is a symbolic link of
// the planned tree, lies below one, or lies at or below a link's target. A
// shell unit names a path it reads through a link by the link's path, so no
// token of it names the target's paths (V1-5368).
const FrontierShellSymlinkTarget = "shell:symlink-target"

// FrontierShellSymlinkInventoryUnknown is the AFP-V0-9965 unknown detail of a
// plan whose tree's links could not be listed.
const FrontierShellSymlinkInventoryUnknown = "shell:symlink-inventory-unknown"

// maxSymlinkHops bounds the links one target resolves through; a longer chain
// resolves to the repository root, so every dirty path is UNKNOWN.
const maxSymlinkHops = 40

// Symlinks is the AFP-V0-9965 plan input: the symbolic links of the tree a
// plan is built over. It is a plan input only, never part of the
// observed graph, its digest or the graph cache.
type Symlinks struct {
	// Listed is false when the links could not be listed: a plan with a
	// dirty path then carries FrontierShellSymlinkInventoryUnknown.
	Listed bool
	// Targets maps each link present in the tree to its target as a slash
	// path from the repository root: the link's directory joined with its
	// text, not yet cleaned, so a component may be another link, `.` or
	// `..`. An empty value is a target outside the repository, and "." one
	// that could not be read.
	Targets map[string]string
	// Changed lists each dirty path that is a link at the measured commit or
	// in the tree.
	Changed []string
}

// WithSymlinks returns a copy of the graph whose plans read links as the
// tree's symbolic links (AFP-V0-9965). The graph is not changed, and a graph
// without this call plans as before.
func (graph *Graph) WithSymlinks(links Symlinks) *Graph {
	copied := *graph
	copied.symlinks = &Symlinks{Listed: links.Listed, Targets: maps.Clone(links.Targets), Changed: slices.Clone(links.Changed)}
	return &copied
}

// TreeSymlinks lists the AFP-V0-9965 links of tree for a plan of dirty
// measured from commit: every link at commit, from repository's Git objects,
// with its target read from tree, and every dirty path that is a link at
// commit or in tree. A path at commit that is dirty is read from tree alone.
// Missing inputs, a Git failure or a listing that reaches MaxStatusBytes
// return a list that is not Listed.
func TreeSymlinks(ctx context.Context, gitExecutable, repository, tree, commit string, dirty []string) Symlinks {
	if gitExecutable == "" || repository == "" || tree == "" || commit == "" {
		return Symlinks{}
	}
	deadline, cancel := context.WithTimeout(ctx, StatusDeadline)
	defer cancel()
	raw, err := boundedGitRunner(gitExecutable)(deadline, repository, MaxStatusBytes, "ls-tree", "-r", "-z", "--full-tree", commit)
	if err != nil || len(raw) >= MaxStatusBytes {
		return Symlinks{}
	}
	isDirty := make(map[string]bool, len(dirty))
	for _, relative := range dirty {
		isDirty[relative] = true
	}
	links := Symlinks{Listed: true, Targets: map[string]string{}}
	changed := map[string]bool{}
	for _, record := range strings.Split(string(raw), "\x00") {
		if record == "" {
			continue
		}
		meta, name, found := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !found || len(fields) != 3 {
			return Symlinks{}
		}
		if fields[0] != "120000" {
			continue
		}
		if isDirty[name] {
			changed[name] = true
			continue
		}
		links.Targets[name] = linkTarget(repository, tree, name)
	}
	for _, relative := range dirty {
		if !ValidRelativePath(relative) {
			continue
		}
		info, err := os.Lstat(filepath.Join(tree, filepath.FromSlash(relative)))
		switch {
		case err == nil && info.Mode()&fs.ModeSymlink != 0:
			changed[relative] = true
			links.Targets[relative] = linkTarget(repository, tree, relative)
		case err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR):
			// A path whose kind cannot be read may be a link.
			changed[relative] = true
		}
	}
	links.Changed = slices.Sorted(maps.Keys(changed))
	return links
}

// linkTarget is the Targets value of the link at relative in tree: its text
// joined to its directory, empty when an absolute text lies outside both tree
// and repository, and "." when the text cannot be read.
func linkTarget(repository, tree, relative string) string {
	text, err := os.Readlink(filepath.Join(tree, filepath.FromSlash(relative)))
	if err != nil || text == "" {
		return "."
	}
	if filepath.IsAbs(text) {
		for _, root := range []string{tree, repository} {
			if inside, err := filepath.Rel(root, text); err == nil && inside != ".." && !strings.HasPrefix(inside, ".."+string(filepath.Separator)) && !filepath.IsAbs(inside) {
				return filepath.ToSlash(inside)
			}
		}
		return ""
	}
	cut := strings.LastIndexByte(relative, '/')
	if cut < 0 {
		return filepath.ToSlash(text)
	}
	return relative[:cut] + "/" + filepath.ToSlash(text)
}

// resolveSymlink resolves the slash path start from the repository root one
// component at a time, following each link of targets it meets before a
// later `..` climbs, as the kernel does. It returns the clean repository path
// and true, "." and true after more than maxSymlinkHops links, or false for a
// path that leaves the repository.
func resolveSymlink(start string, targets map[string]string) (string, bool) {
	pending := strings.Split(start, "/")
	var current []string
	hops := 0
	for len(pending) > 0 {
		component := pending[0]
		pending = pending[1:]
		switch component {
		case "", ".":
			continue
		case "..":
			if len(current) == 0 {
				return "", false
			}
			current = current[:len(current)-1]
			continue
		}
		current = append(current, component)
		target, link := targets[strings.Join(current, "/")]
		if !link {
			continue
		}
		hops++
		if hops > maxSymlinkHops {
			return ".", true
		}
		if target == "" {
			return "", false
		}
		current = nil
		pending = append(strings.Split(target, "/"), pending...)
	}
	if len(current) == 0 {
		return ".", true
	}
	return strings.Join(current, "/"), true
}

// symlinkTargets returns the AFP-V0-9965 unknowns of a plan over normalized
// in a graph built with the shell plugin and given the tree's links: one
// per dirty path that is a link, lies below one, or lies at or below a
// link's resolved target, or one for the whole plan when the links could not
// be listed.
func (graph *Graph) symlinkTargets(normalized []string) []Unknown {
	if graph.symlinks == nil || len(normalized) == 0 || !slices.Contains(graph.languages, "shell") {
		return nil
	}
	links := graph.symlinks
	if !links.Listed {
		return []Unknown{{Reason: UnknownLanguageFrontier, Detail: FrontierShellSymlinkInventoryUnknown}}
	}
	regions := make([]string, 0, len(links.Targets))
	for link := range links.Targets {
		if region, inside := resolveSymlink(link, links.Targets); inside {
			regions = append(regions, region)
		}
	}
	slices.Sort(regions)
	regions = slices.Compact(regions)
	var unknown []Unknown
	for _, dirtyPath := range normalized {
		if slices.Contains(links.Changed, dirtyPath) || throughLink(dirtyPath, links.Targets) || belowAny(dirtyPath, regions) {
			unknown = append(unknown, Unknown{Reason: UnknownLanguageFrontier, Detail: FrontierShellSymlinkTarget + ":" + dirtyPath})
		}
	}
	return unknown
}

// throughLink reports whether dirtyPath or one of its ancestors is a link of
// targets.
func throughLink(dirtyPath string, targets map[string]string) bool {
	for prefix := dirtyPath; prefix != ""; {
		if _, link := targets[prefix]; link {
			return true
		}
		cut := strings.LastIndexByte(prefix, '/')
		if cut < 0 {
			break
		}
		prefix = prefix[:cut]
	}
	return false
}

// belowAny reports whether dirtyPath equals or lies below one of regions,
// "." being the repository root.
func belowAny(dirtyPath string, regions []string) bool {
	for _, region := range regions {
		if region == "." || dirtyPath == region || strings.HasPrefix(dirtyPath, region+"/") {
			return true
		}
	}
	return false
}
