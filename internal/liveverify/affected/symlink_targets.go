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
	// Root is the tree's physical path, every link of its path resolved, in
	// slash form: a target that climbs above the tree is resolved from it.
	Root string
}

// WithSymlinks returns a copy of the graph whose plans read links as the
// tree's symbolic links (AFP-V0-9965). The graph is not changed, and a graph
// without this call plans as before.
func (graph *Graph) WithSymlinks(links Symlinks) *Graph {
	copied := *graph
	copied.symlinks = &Symlinks{Listed: links.Listed, Targets: maps.Clone(links.Targets), Changed: slices.Clone(links.Changed), Root: links.Root}
	return &copied
}

// TreeSymlinks lists the AFP-V0-9965 links of tree for a plan of dirty
// measured from commit: every link at commit, from repository's Git objects,
// with its target read from tree, and every dirty path that is a link at
// commit or in tree. A path at commit that is dirty is read from tree alone.
// A link whose target leaves the tree is resolved on the file system from
// the tree's physical path (physicalPath), so a target that climbs out and
// re-enters by the repository's own name, or names the tree through an alias
// of an ancestor (/tmp and /private/tmp), gets the repository path it reaches
// as its target, one that cannot be evaluated gets ".", and one that stays
// outside keeps its text. Missing inputs, a tree whose physical path cannot be
// read, a Git failure or a listing that reaches MaxStatusBytes return a list
// that is not Listed.
func TreeSymlinks(ctx context.Context, gitExecutable, repository, tree, commit string, dirty []string) Symlinks {
	if gitExecutable == "" || repository == "" || tree == "" || commit == "" {
		return Symlinks{}
	}
	physical, err := filepath.EvalSymlinks(tree)
	if err != nil {
		return Symlinks{}
	}
	root := filepath.ToSlash(physical)
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
	links := Symlinks{Listed: true, Targets: map[string]string{}, Root: root}
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
		links.Targets[name] = linkTarget(repository, tree, root, name)
	}
	for _, relative := range dirty {
		if !ValidRelativePath(relative) {
			continue
		}
		info, err := os.Lstat(filepath.Join(tree, filepath.FromSlash(relative)))
		switch {
		case err == nil && info.Mode()&fs.ModeSymlink != 0:
			changed[relative] = true
			links.Targets[relative] = linkTarget(repository, tree, root, relative)
		case err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR):
			// A path whose kind cannot be read may be a link.
			changed[relative] = true
		}
	}
	links.Changed = slices.Sorted(maps.Keys(changed))
	reentered := map[string]string{}
	for link := range links.Targets {
		position, inside := resolveSymlink(link, links.Targets, root)
		if inside || position == "" {
			continue
		}
		reached, err := physicalPath(position)
		if err != nil {
			reentered[link] = "."
		} else if relative, inside := physicalTarget(reached, root); inside {
			reentered[link] = relative
		}
	}
	maps.Copy(links.Targets, reentered)
	return links
}

// errUnresolved is a path physicalPath cannot evaluate: a dangling link or a
// `..` in the part of the path that does not exist.
var errUnresolved = errors.New("path cannot be resolved")

// physicalPath resolves the absolute slash path lexical as the kernel would:
// its longest existing prefix, cut on raw components so a `..` after a link
// climbs from the link's target, with every link followed, then the rest,
// which must hold no `..` and must not start at an entry that exists but
// cannot be followed (a dangling link).
func physicalPath(lexical string) (string, error) {
	components := strings.Split(lexical, "/")
	for count := len(components); count >= 1; count-- {
		rest := components[count:]
		if slices.Contains(rest, "..") {
			return "", errUnresolved
		}
		prefix := strings.Join(components[:count], "/")
		if count == 1 {
			prefix += "/"
		}
		resolved, err := filepath.EvalSymlinks(filepath.FromSlash(prefix))
		if err == nil {
			return filepath.ToSlash(filepath.Join(append([]string{resolved}, rest...)...)), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if _, err := os.Lstat(filepath.FromSlash(prefix)); !errors.Is(err, fs.ErrNotExist) {
			return "", errUnresolved
		}
	}
	return "", errUnresolved
}

// insideRoot returns the slash path physical as a clean repository path when
// it is root or lies below it.
func insideRoot(physical, root string) (string, bool) {
	if physical == root {
		return ".", true
	}
	relative, below := strings.CutPrefix(physical, strings.TrimSuffix(root, "/")+"/")
	return relative, below && relative != ""
}

// physicalTarget is the Targets value of a link that reaches the physical
// slash path reached: the repository path when it is root or lies below it,
// and "." when it does so only under a case fold, or when either holds a
// byte past ASCII, whose folding and normalisation are not modelled. A
// case-insensitive file system opens `../REPO/x` beside repo, and
// EvalSymlinks keeps the case it was given (AFP-V0-9965). It returns false
// for a path outside.
func physicalTarget(reached, root string) (string, bool) {
	if inside, ok := insideRoot(reached, root); ok {
		return inside, true
	}
	if !asciiText(reached) || !asciiText(root) {
		return ".", true
	}
	if _, ok := insideRoot(strings.ToLower(reached), strings.ToLower(root)); ok {
		return ".", true
	}
	return "", false
}

// linkTarget is the Targets value of the link at relative in tree: its text
// joined to its directory, or the rest of an absolute text below tree or
// repository by name, neither cleaned; for any other absolute text, the
// repository path it reaches on the file system from the
// physical root (an alias of an ancestor), empty when it reaches none, and "."
// when the text or that path cannot be read.
func linkTarget(repository, tree, root, relative string) string {
	text, err := os.Readlink(filepath.Join(tree, filepath.FromSlash(relative)))
	if err != nil || text == "" {
		return "."
	}
	if filepath.IsAbs(text) {
		// The text is cut, not cleaned: a component may be a link that a
		// later `..` climbs from, which resolveSymlink follows.
		for _, root := range []string{tree, repository} {
			if text == root {
				return "."
			}
			if inside, below := strings.CutPrefix(text, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator)); below {
				if inside == "" {
					// `<root>/` names the root itself.
					return "."
				}
				return filepath.ToSlash(inside)
			}
		}
		reached, err := physicalPath(filepath.ToSlash(text))
		if err != nil {
			return "."
		}
		if inside, ok := physicalTarget(reached, root); ok {
			return inside
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
// path that leaves the repository: with the absolute position it reaches,
// root joined with the rest of the path unresolved, when it climbs above the
// root (physicalPath resolves it), and empty for a link to an absolute target
// outside. With no root, a climb above the root resolves to ".".
func resolveSymlink(start string, targets map[string]string, root string) (string, bool) {
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
				if root == "" {
					return ".", true
				}
				return strings.Join(append([]string{strings.TrimSuffix(root, "/"), ".."}, pending...), "/"), false
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
		if region, inside := resolveSymlink(link, links.Targets, links.Root); inside {
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

// asciiText reports whether text holds only ASCII bytes, the only bytes whose
// case folding physicalTarget models.
func asciiText(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= 0x80 {
			return false
		}
	}
	return true
}
