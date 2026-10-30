package contextindex

import (
	"context"
	jsonv2 "encoding/json/v2"
	"path"
	"unicode/utf8"
)

// This file exposes the index's admission rules to the application map's incremental build
// (AMAP-V0-245), which must know, without building an index, that a content edit leaves the set of
// indexed sources and the index's refusals as they were. Each function delegates to the rule
// buildEvidence applies, so the two cannot drift.

// SourcePathAdmitted reports whether the index considers the path p at all (admittedEntries): a
// valid UTF-8 path that is not forbidden and has an admitted suffix or is a Makefile or go.mod.
func SourcePathAdmitted(p string) bool {
	if !utf8.ValidString(p) || forbiddenPath(p) != "" {
		return false
	}
	base := path.Base(p)
	return ImpactPathAdmitted(p) || base == "Makefile" || base == "go.mod"
}

// SourceAdmitted reports whether the index makes the blob data at p a source: an admitted path
// within the per-source size bound that is neither a Git LFS pointer nor a generated file
// (pinnedFrom). A false answer means the path is absent from Index.Sources.
func SourceAdmitted(p string, data []byte) bool {
	return SourcePathAdmitted(p) && len(data) <= maxSourceBytes && pinnedFrom(treeEntry{path: p}, data).exclusionReason() == ""
}

// SourceSizeBound is the largest blob the index admits as a source.
const SourceSizeBound = maxSourceBytes

// WebConfigShaped reports whether data might be read as a tsconfig/jsconfig by the web import
// resolver: it is JSON once JSONC comments and trailing commas are removed. A config `extends`
// may name any indexed path, so a source that is not shaped like this at all parses to no config
// whatever its content. It is deliberately wider than parseWebConfig.
func WebConfigShaped(data []byte) bool {
	clean, ok := jsoncToJSON(string(data))
	if !ok {
		return false
	}
	var value any
	return jsonv2.Unmarshal([]byte(clean), &value) == nil
}

// CheckRevisionAdmission returns the refusal BuildRevisionContext gives at revision for the size
// of the candidate set (the source count and the aggregate byte bound), or nil, after reading only
// the tree listing: no blob is read.
func CheckRevisionAdmission(ctx context.Context, root, revision string) error {
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	identity, err := revisionIdentity(ctx, root, revision)
	if err != nil {
		return err
	}
	entries, err := readTreeEntries(ctx, root, identity)
	if err != nil {
		return err
	}
	_, candidates, _ := admittedEntries(entries)
	if err := validateBlobAdmission(candidates); err != nil {
		return err
	}
	_, err = uniqueBlobEntries(candidates)
	return err
}
