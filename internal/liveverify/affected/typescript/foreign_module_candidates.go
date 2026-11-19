// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// foreignModuleCandidates certifies only ordinary repository paths naming a
// file or directory without a package manifest; suffixes do not restrict filenames.
// Directory package resolution (including main and future manifest forms) is
// unproved: preserve uncertainty rather than guess a complete set of targets.
func foreignModuleCandidates(root *affected.Source, candidate string) ([]string, bool) {
	if !affected.ValidRelativePath(candidate) {
		return nil, true
	}
	tokens := []string{candidate}
	for _, c := range candidate {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			strings.ContainsRune("/ ._-@#?", c)) {
			return tokens, true
		}
	}
	tokens = append(tokens, candidate+".json", path.Join(candidate, "index.json"))
	// Only absent paths, regular files and manifest-free directories prove a
	// complete candidate set. Probe the file first: probing a regular file's
	// child yields ENOTDIR and makes the observation uncacheable.
	info, err := root.Stat(candidate)
	if errors.Is(err, fs.ErrNotExist) {
		return tokens, false
	}
	if err != nil {
		return tokens, true
	}
	if info.Mode().IsRegular() {
		return tokens, false
	}
	if !info.IsDir() {
		return tokens, true
	}
	// Even a dotted name can denote a directory; a manifest can redirect its
	// read outside that directory. Missing/unreadable metadata is not absence.
	_, err = root.Stat(path.Join(candidate, "package.json"))
	if !errors.Is(err, fs.ErrNotExist) {
		return tokens, true
	}
	return tokens, false
}

// commonJSLiteralCandidates certifies only an exact regular JSON file. The
// foreign data candidates do not model CommonJS source or directory resolution;
// every other form retains uncertainty, including future loader extensions.
func commonJSLiteralCandidates(root *affected.Source, candidate string) ([]string, bool) {
	tokens, bounded := foreignModuleCandidates(root, candidate)
	if bounded || path.Ext(candidate) != ".json" {
		return tokens, true
	}
	info, err := root.Stat(candidate)
	return tokens, err != nil || !info.Mode().IsRegular()
}
