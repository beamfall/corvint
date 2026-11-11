// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// AllowedUnknown is the one allowlist of plan.unknown entries a narrowed run
// may carry (AFP-V0-982): the deciding driver's unknownNarrowing (CIN-V0-860)
// and the shadow `corvint test` (sessiontest, AFP-V0-028) both decide through
// it. Of the LANGUAGE_FRONTIER entries it admits only the two Go frontiers that
// hide no Go reader and a driver frontier, whose make targets and CI jobs no
// adapter or declared command runs, narrowed or FULL. Every other frontier the
// plan names bears on the change, so it is no category to ignore: a non-Go one
// adds its language's declared suite through FrontierSuite, and a Go one may
// hide a Go reader (AFP-V0-2750). An UNOWNED_DIRTY_PATH is read only by the Go
// packages rules (b) to (d) select; a NO_SELECTABLE_TEST unit gets no test from
// a FULL run either. Every other entry forces FULL. The AFP-V0-012 selector,
// which holds the Go index, first runs everything for an unowned path no Go
// rule attributes (NCM-V1-001, `unowned-path`), so wherever that selector
// decides the run an entry admitted here is an attributed one.
func AllowedUnknown(reason, detail string) bool {
	switch reason {
	case "LANGUAGE_FRONTIER":
		return detail == "go:build-constraint-variants" || detail == "go:nested-module-frontier" ||
			strings.HasPrefix(detail, driverFrontier)
	case "UNOWNED_DIRTY_PATH", "NO_SELECTABLE_TEST":
		return true
	}
	return false
}

// driverFrontier prefixes a frontier of the make and CI driver namespace
// (AZU-V0-001), whose units no language suite runs.
const driverFrontier = "driver:"

// frontierLanguage is a plain plugin name, safe to echo in an audit line and to
// look up in the declared full scope.
var frontierLanguage = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// FrontierSuite returns the language of a LANGUAGE_FRONTIER whose detail is
// "<language>:..." for a plain language name other than go and driver
// (AFP-V0-2750). Such an entry bears on the change and may hide a reader of a
// changed or deleted path that no Go selection covers, so both selectors keep
// the run narrowed only by adding that language's declared suite: `corvint
// test` runs it (or names it as a check on the ci profile, or records its
// declared no-suite assertion), and the driver names it in a `suite` audit
// line. Any other detail is not admitted here.
func FrontierSuite(reason, detail string) (string, bool) {
	language, _, ok := strings.Cut(detail, ":")
	if reason != "LANGUAGE_FRONTIER" || !ok || language == "go" || language+":" == driverFrontier || !frontierLanguage.MatchString(language) {
		return "", false
	}
	return language, true
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

// E2EFlowUnknown returns the detail of a well-formed flow-binding entry of
// `corvint affected` (AFU-V1-1652): E2E_UNMAPPED_CHANGE names a clean relative
// changed path that reaches a flow-bound consumer's command while no reviewed
// flow link explains it, and E2E_FLOWS_INVALID names the binding declaration
// itself. Either leaves the consumer it concerns selected or named as a check
// (the binding only narrows when every path is explained), so the Go
// selection is unchanged: the driver admits the entry beside BinaryConsumer,
// and `corvint test` names no further check for it.
func E2EFlowUnknown(reason, detail string) (string, bool) {
	switch reason {
	case "E2E_UNMAPPED_CHANGE":
		if detail == "." || !fs.ValidPath(detail) || strings.ContainsRune(detail, '\\') || strings.ContainsFunc(detail, unicode.IsControl) {
			return "", false
		}
		return detail, true
	case "E2E_FLOWS_INVALID":
		if detail != ".corvint/e2e-flows.json" {
			return "", false
		}
		return detail, true
	}
	return "", false
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
