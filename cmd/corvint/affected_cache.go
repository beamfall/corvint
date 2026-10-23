package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedPathsFromMaxBytes bounds one --paths-from file (AFP-V0-395): the
// --path bound of 4096 canonical paths, each well under 1 KiB.
const affectedPathsFromMaxBytes = 4 << 20

// affectedProgramMaxBytes bounds the executable read for the cache identity.
const affectedProgramMaxBytes = 1 << 30

func setAffectedOnce(field *string, name, value string) error {
	if *field != "" || value == "" {
		return argumentError(name + " requires exactly one nonempty value")
	}
	*field = value
	return nil
}

// resolveAffectedBatchAndCache reads --paths-from into the --path set and
// resolves --cache, both after --root is known.
func resolveAffectedBatchAndCache(invocation *affectedInvocation) error {
	if invocation.Cache != "" {
		resolved, err := resolveAffectedCache(invocation.Root, invocation.Cache)
		if err != nil {
			return err
		}
		invocation.Cache = resolved
	}
	if invocation.PathsFrom != "" {
		paths, err := readAffectedPathsFrom(invocation.Root, invocation.PathsFrom)
		if err != nil {
			return err
		}
		for _, value := range paths {
			if err := addAffectedPath(invocation, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// readAffectedPathsFrom reads one bounded regular file, relative to root
// unless absolute, of LF-separated --path values; blank lines are skipped and
// every other line must be a canonical repository-relative path (AFP-V0-395).
// --cache is resolved first, so a refused cache placement reads nothing. A file with no path is refused rather than planning the
// worktree instead.
func readAffectedPathsFrom(root, name string) ([]string, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(root, name)
	}
	info, err := os.Stat(name)
	if err != nil || !info.Mode().IsRegular() {
		return nil, argumentError("--paths-from must name a readable regular file")
	}
	if info.Size() > affectedPathsFromMaxBytes {
		return nil, argumentError(fmt.Sprintf("--paths-from is over the %d-byte bound", affectedPathsFromMaxBytes))
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, argumentError("--paths-from must name a readable regular file")
	}
	defer file.Close()
	if opened, err := file.Stat(); err != nil || !opened.Mode().IsRegular() {
		return nil, argumentError("--paths-from must name a readable regular file")
	}
	body, err := io.ReadAll(io.LimitReader(file, affectedPathsFromMaxBytes+1))
	if err != nil || len(body) > affectedPathsFromMaxBytes {
		return nil, argumentError("--paths-from must name a readable regular file within its bound")
	}
	if strings.HasPrefix(string(body), "\ufeff") {
		return nil, argumentError("--paths-from must not begin with a byte order mark")
	}
	paths := []string{}
	for _, line := range strings.Split(string(body), "\n") {
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, "\r") {
			return nil, argumentError("--paths-from lines end in LF, not CRLF; write --path for a name that ends in a carriage return")
		}
		if !affected.ValidRelativePath(line) {
			return nil, argumentError("--paths-from line must be a repository-relative canonical path, got " + pythonRepr(line))
		}
		paths = append(paths, line)
	}
	if len(paths) == 0 {
		return nil, argumentError("--paths-from names no path")
	}
	return paths, nil
}

// resolveAffectedCache makes the --cache directory absolute, resolving the
// symlinks of its nearest existing ancestor, and refuses one at or under the
// repository root or under any .git directory: a read command writes no
// repository content (AFP-V0-390, AGENTS.md invariant 4).
func resolveAffectedCache(root, value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", argumentError("invalid --cache")
	}
	resolved := resolveExistingAncestor(absolute)
	for _, component := range strings.Split(filepath.ToSlash(resolved), "/") {
		if strings.EqualFold(component, ".git") {
			return "", argumentError("--cache must name a directory outside the repository and outside any .git directory")
		}
	}
	if within(resolved, root) || underSameFile(resolved, root) {
		return "", argumentError("--cache must name a directory outside the repository root: a read command writes no repository content")
	}
	return resolved, nil
}

// underSameFile reports whether root is the same directory as candidate or
// one of its existing ancestors, which a textual comparison misses on a
// case-insensitive file system or through a firmlink.
func underSameFile(candidate, root string) bool {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false
	}
	for current := candidate; ; current = filepath.Dir(current) {
		if info, err := os.Stat(current); err == nil && os.SameFile(info, rootInfo) {
			return true
		}
		if filepath.Dir(current) == current {
			return false
		}
	}
}

// resolveExistingAncestor resolves the symlinks of the deepest existing
// ancestor of absolute and rejoins the components that do not exist yet.
func resolveExistingAncestor(absolute string) string {
	rest := []string{}
	current := absolute
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(append([]string{resolved}, rest...)...)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return absolute
		}
		rest = append([]string{filepath.Base(current)}, rest...)
		current = parent
	}
}

func within(candidate, root string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// affectedProgramIdentity is the SHA-256 of the running executable, the
// cache key's program member: an entry written by any other build of corvint,
// whose plugins may observe differently, is never trusted (AFP-V0-391). It is
// empty when the executable cannot be read, which disables the cache.
var affectedProgramIdentity = sync.OnceValue(func() string {
	name, err := os.Executable()
	if err != nil {
		return ""
	}
	file, err := os.Open(name)
	if err != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	copied, err := io.Copy(hash, io.LimitReader(file, affectedProgramMaxBytes+1))
	if err != nil || copied > affectedProgramMaxBytes {
		return ""
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
})

// affectedCacheObserved receives every cache outcome; tests replace it.
var affectedCacheObserved = func(affected.CacheOutcome) {}

// affectedBuildGraph builds the plan graph, through the verified cache when
// --cache named one (AFP-V0-390 to AFP-V0-393).
func affectedBuildGraph(ctx context.Context, invocation affectedInvocation) (*affected.Graph, error) {
	if invocation.Cache == "" {
		return affected.Build(ctx, invocation.Root, affectedLanguages()...)
	}
	graph, outcome, err := affected.BuildCached(ctx, invocation.Root, invocation.Cache, affectedProgramIdentity(), affectedLanguages()...)
	if err == nil {
		affectedCacheObserved(outcome)
	}
	return graph, err
}
