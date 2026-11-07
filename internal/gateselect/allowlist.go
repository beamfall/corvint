// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

// AllowedUnknown is the one allowlist of plan.unknown entries a narrowed run
// may carry (AFP-V0-982): the deciding driver's unknownNarrowing (CIN-V0-860)
// and the shadow `corvint test` (sessiontest, AFP-V0-028) both decide through
// it. A LANGUAGE_FRONTIER names a category, not a unit, except a Go frontier
// other than build-constraint variants and the nested-module frontier, which
// may hide a Go reader; an UNOWNED_DIRTY_PATH is read only by the Go packages
// rules (b) to (d) select; a NO_SELECTABLE_TEST unit gets no test from a FULL
// run either. Every other entry forces FULL.
func AllowedUnknown(reason, detail string) bool {
	switch reason {
	case "LANGUAGE_FRONTIER":
		return !strings.HasPrefix(detail, "go:") ||
			detail == "go:build-constraint-variants" || detail == "go:nested-module-frontier"
	case "UNOWNED_DIRTY_PATH", "NO_SELECTABLE_TEST":
		return true
	}
	return false
}

// UnobservedModule returns the module directory of a well-formed AFP-V0-650
// unobserved-module frontier, "go:unobserved-module:<directory>:<path>": a
// clean, local, slash-separated directory that is the only split whose path
// lies below it. The driver admits such an entry beside AllowedUnknown, and
// `corvint test` does on every profile: outside the ci profile the module's
// own suite runs it (STR-V0-064), and on the ci profile it names that suite as
// a check CI runs outside the Go shards (AFP-V0-2742); both read it here
// (AFP-V0-982).
func UnobservedModule(reason, detail string) (string, bool) {
	rest, ok := strings.CutPrefix(detail, "go:unobserved-module:")
	if reason != "LANGUAGE_FRONTIER" || !ok {
		return "", false
	}
	found, count := "", 0
	for index := strings.IndexByte(rest, ':'); index >= 0; {
		directory, dirty := rest[:index], rest[index+1:]
		if strings.HasPrefix(dirty, directory+"/") {
			found, count = directory, count+1
		}
		next := strings.IndexByte(rest[index+1:], ':')
		if next < 0 {
			break
		}
		index += next + 1
	}
	if count != 1 || found == "." || !filepath.IsLocal(found) || path.Clean(found) != found || strings.ContainsRune(found, '\\') {
		return "", false
	}
	return found, true
}

// BinaryConsumer returns the file of a well-formed AFP-V0-762
// UNSELECTED_BINARY_CONSUMER entry: a clean, relative, slash-separated path
// with no backslash or control character, judged the same on every host. The
// file is no Go test; it runs the built binary of a Go command the change
// reaches, so no Go run, narrowed or FULL, runs it. The driver admits such an
// entry beside AllowedUnknown, and `corvint test` names it as a check still
// required on every profile (STR-V0-750); both read it here (AFP-V0-982).
func BinaryConsumer(reason, file string) (string, bool) {
	if reason != "UNSELECTED_BINARY_CONSUMER" || file == "." || !fs.ValidPath(file) || strings.ContainsRune(file, '\\') || strings.ContainsFunc(file, unicode.IsControl) {
		return "", false
	}
	return file, true
}
