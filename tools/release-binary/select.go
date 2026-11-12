// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"strings"
)

// Selection is what one release tag asks the pipeline to build (ARTIFACT-BIN-V0-005).
type Selection struct {
	Tag      string   `json:"tag"`
	Scope    string   `json:"scope"`
	Version  string   `json:"version"`
	Binaries []string `json:"binaries"`
}

// selectTag maps `vX.Y.Z` to every product binary, `SET/vX.Y.Z` to one version set and
// `NAME/vX.Y.Z` to one product binary. Support and withheld binaries and unknown names refuse.
func selectTag(catalog Catalog, tag string) (Selection, error) {
	scope, versionTag, scoped := strings.Cut(tag, "/")
	if !scoped {
		scope, versionTag = "", tag
	}
	version, ok := strings.CutPrefix(versionTag, "v")
	if !ok || !versionPattern.MatchString(version) {
		return Selection{}, fmt.Errorf("bad-tag: %q must end in vMAJOR.MINOR.PATCH[-pre]", tag)
	}
	selection := Selection{Tag: tag, Version: version}
	switch {
	case scope == "":
		selection.Scope = "all"
		selection.Binaries = catalog.products("")
	case catalog.VersionSets[scope] != "":
		selection.Scope = "set:" + scope
		selection.Binaries = catalog.products(scope)
	default:
		binary, known := catalog.find(scope)
		if !known {
			return Selection{}, fmt.Errorf("unknown-binary: %q names no product binary or version set in the catalog", scope)
		}
		if binary.Role != "product" {
			return Selection{}, refuseUnreleased(binary)
		}
		selection.Scope = "binary"
		selection.Binaries = []string{binary.Name}
	}
	if len(selection.Binaries) == 0 {
		return Selection{}, fmt.Errorf("empty-selection: %q selects no product binary", tag)
	}
	return selection, nil
}
