package doccorpus

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

// ManifestOfArtifact returns the manifest an artifact embeds, for docs corpus build --artifact to
// rebuild it from (DCX-V1-495). It reads only the members it needs, so an artifact whose records
// a later derivation no longer decodes still yields its manifest, and it checks no builder,
// derivation or pin: that is the refusal the rebuild repairs. It refuses bytes whose recorded
// content digest or manifest digest does not hold, that exceed the size bound of their schema,
// or whose manifest does not decode closed (DCX-V1-496). Build then verifies every pinned input against Git.
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

// RebuildArtifact is docs corpus build --artifact: the admitted artifact m, the manifest data
// embeds as ManifestOfArtifact returns it, builds to (DCX-V1-495). When the running builder is attested and data is a canonical artifact
// whose recorded builder the verified read admits, its derivation equal to this one (DCX-V1-471),
// the build reuses data's derived records instead of rederiving them: every pinned input is
// still reread from Git and verified once, and only the builder and content digest are rewritten,
// to the bytes a full build prints (DCX-V1-532, DCX-V1-533). Any other artifact, an unrecorded
// derivation included, is built in full. reused reports which path ran.
func RebuildArtifact(ctx context.Context, root string, m Manifest, data []byte) (a *Artifact, reused bool, err error) {
	if a, ok, err := reuseArtifact(ctx, root, m, data); err != nil || ok {
		return a, ok, err
	}
	a, err = BuildAdmitted(ctx, root, m)
	return a, false, err
}

// reuseArtifact returns data's records under the running builder, ok false when they cannot be
// reused and the rebuild must rederive them. A pin refusal is the full build's own first refusal.
func reuseArtifact(ctx context.Context, root string, m Manifest, data []byte) (*Artifact, bool, error) {
	running := currentBuilder()
	if !rebuildAttested(running) {
		return nil, false, nil
	}
	old, err := ParseArtifact(data)
	if err != nil || derivationOf(old.Builder) == "" || sameDerivation(old.Builder, running) != nil {
		return nil, false, nil
	}
	want, err := Encode(m)
	if err != nil {
		return nil, false, nil
	}
	got, err := Encode(old.Manifest)
	manifest, merr := hashValue(m)
	profile, perr := hashValue(m.Profile)
	if err != nil || merr != nil || perr != nil || !bytes.Equal(got, want) || manifest != old.ManifestSHA256 || profile != old.ProfileSHA256 || (old.Schema == SchemaV2) != (m.Schema == ManifestSchemaV2) {
		return nil, false, nil
	}
	if err := validateManifest(m); err != nil {
		return nil, false, err
	}
	c := &compiler{root: root, manifest: m, indexes: map[string]*contextindex.Index{}, sources: map[string]contextindex.Source{}}
	if err := c.pin(gitstatus.WithIsolation(ctx)); err != nil {
		return nil, false, err
	}
	if c.indexes[m.Repository.Revision].Revision != old.Tree {
		return nil, false, nil
	}
	c.sources, c.indexes = nil, nil
	old.Builder, old.SHA256 = running, ""
	digest, err := hashValue(old)
	if err != nil {
		return nil, false, err
	}
	old.SHA256 = digest
	if _, err := Encode(old); err != nil {
		return nil, false, err
	}
	if err := AdmitIndex(ctx, old); err != nil {
		return nil, false, err
	}
	return old, true, nil
}
