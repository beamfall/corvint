package doccorpus

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"strings"
)

// ManifestOfArtifact returns the manifest an artifact embeds, for docs corpus build --artifact to
// rebuild it from (DCP-V1-495). It reads only the members it needs, so an artifact whose records
// a later derivation no longer decodes still yields its manifest, and it checks no builder,
// derivation or pin: that is the refusal the rebuild repairs. It refuses bytes whose recorded
// content digest or manifest digest does not hold, that exceed the size bound of their schema,
// or whose manifest does not decode closed (DCP-V1-496). Build then verifies every pinned input against Git.
func ManifestOfArtifact(data []byte) (Manifest, error) {
	if len(data) > MaxCorpusBytes {
		return Manifest{}, fail("input bound exceeded")
	}
	var header struct {
		Schema         string         `json:"schema"`
		Manifest       jsontext.Value `json:"manifest"`
		ManifestSHA256 string         `json:"manifest_sha256"`
		SHA256         string         `json:"sha256"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Manifest{}, fail("invalid artifact JSON")
	}
	tail := hashArtifactTail(data)
	if err := tail.suffix(&Artifact{SHA256: header.SHA256}); err != nil {
		return Manifest{}, err
	}
	if tail.digest != header.SHA256 {
		return Manifest{}, fail("artifact digest mismatch")
	}
	if !strings.HasPrefix(header.Schema, "corvint-evidence-corpus/") {
		return Manifest{}, fail("unsupported artifact schema")
	}
	if len(data) > encodingLimit(Artifact{Schema: header.Schema}) {
		return Manifest{}, fail("input bound exceeded")
	}
	if len(header.Manifest) == 0 {
		return Manifest{}, fail("artifact has no embedded manifest")
	}
	m, err := ParseManifest(header.Manifest)
	if err != nil {
		return Manifest{}, err
	}
	digest, err := hashValue(m)
	if err != nil || digest != header.ManifestSHA256 {
		return Manifest{}, fail("artifact manifest digest mismatch")
	}
	return m, nil
}
