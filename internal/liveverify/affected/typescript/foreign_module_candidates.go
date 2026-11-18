// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"errors"
	"io/fs"
	"path"

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
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '/' || c == ' ' || c == '.' || c == '_' || c == '-' || c == '@' || c == '#') {
			return tokens, true
		}
	}
	tokens = append(tokens, candidate+".json", path.Join(candidate, "index.json"))
	// Even a dotted name can denote a directory; a manifest can redirect its
	// read outside that directory. Missing/unreadable metadata is not absence.
	_, err := root.Stat(path.Join(candidate, "package.json"))
	if !errors.Is(err, fs.ErrNotExist) {
		// An existing regular file cannot also be a package directory. Stat of
		// its child may return ENOTDIR instead of fs.ErrNotExist.
		if info, statErr := root.Stat(candidate); statErr == nil && info.Mode().IsRegular() {
			return tokens, false
		}
		return tokens, true
	}
	return tokens, false
}
