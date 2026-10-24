package doccorpus

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"
)

// derivationFixtureDigests pins, per derivation version, the digest of the fixture corpus built
// with the builder, the content digest and every commit and tree id normalized. A change that
// alters it alters the records a build derives from the same inputs: increment DerivationVersion
// and record the new digest here (DCP-V1-471).
var derivationFixtureDigests = map[string]string{
	"1": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
}

// TestDCPV1471VerifiedReadAcrossBuilderRevisions: the verified read admits an artifact recorded
// by another attested builder revision with the same builder schema, derivation version and
// engine, and refuses, naming what differs, one whose derivation version, engine or schema
// differs or whose builder revision is unattested (DCP-V1-471).
func TestDCPV1471VerifiedReadAcrossBuilderRevisions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a, _ := builtFixture(t)
	other := rehashed(t, a, func(c *Artifact) { c.Builder.Revision = strings.Repeat("a", 40) })
	read, err := OpenRead(ctx, root, other, ReadOptions{})
	if err != nil || read.validation != ValidationVerified || read.Builder.Revision != strings.Repeat("a", 40) {
		t.Fatalf("verified read refused another attested revision of the same derivation: %v", err)
	}
	engine := func(e string) func(*Artifact) {
		return func(c *Artifact) { c.Builder.Revision, c.Builder.Engine = strings.Repeat("a", 40), e }
	}
	cases := []struct {
		name, want string
		edit       func(*Artifact)
	}{
		{"derivation", "derivation version differs (artifact 0, running builder " + DerivationVersion + "); rebuild the corpus",
			engine("native-contextindex/" + Schema + "/derivation-0/" + runtime.Version())},
		{"engine before the version", "derivation version differs (artifact unrecorded, running builder " + DerivationVersion + ")",
			engine("native-contextindex/" + Schema + "/" + runtime.Version())},
		{"go toolchain", "engine differs (artifact native-contextindex/" + Schema + "/derivation-" + DerivationVersion + "/go0.0.0",
			engine("native-contextindex/" + Schema + "/derivation-" + DerivationVersion + "/go0.0.0")},
		{"builder schema", "builder schema differs (artifact corvint-evidence-corpus/0", func(c *Artifact) {
			c.Builder.Revision, c.Builder.Version = strings.Repeat("a", 40), "corvint-evidence-corpus/0"
		}},
		{"dirty", "builder revision is unrecorded or dirty", func(c *Artifact) { c.Builder.Revision = strings.Repeat("a", 40) + "+dirty" }},
	}
	for _, tc := range cases {
		_, err := OpenRead(ctx, root, rehashed(t, a, tc.edit), ReadOptions{})
		if err == nil || !strings.Contains(err.Error(), "builder differs: ") || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want a refusal naming %q", tc.name, err, tc.want)
		}
	}
	if derivationOf(currentBuilder()) != DerivationVersion {
		t.Fatalf("running builder engine %q does not record derivation %s", currentBuilder().Engine, DerivationVersion)
	}
}

// TestDCPV1471DerivationVersionGuard fails when the records the fixture corpus derives change
// without a new DerivationVersion (DCP-V1-471).
func TestDCPV1471DerivationVersionGuard(t *testing.T) {
	t.Parallel()
	_, a, _ := builtFixture(t)
	normal := *a
	normal.Builder, normal.SHA256, normal.ManifestSHA256 = Builder{}, "", ""
	data, err := Encode(normal)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{a.Tree, a.Manifest.Repository.Revision}
	for _, in := range a.Manifest.Inputs {
		ids = append(ids, in.Revision)
	}
	for _, id := range ids {
		if id != "" {
			data = bytes.ReplaceAll(data, []byte(id), []byte("<commit>"))
		}
	}
	data = bytes.ReplaceAll(data, []byte(a.Manifest.BuiltAt), []byte("<built-at>"))
	got := Digest(data)
	if want := derivationFixtureDigests[DerivationVersion]; got != want {
		t.Fatalf("fixture derivation digest %s, pinned %q for derivation %s: a change that alters the derived records must increment DerivationVersion and pin the new digest", got, want, DerivationVersion)
	}
}
