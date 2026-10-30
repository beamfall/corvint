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
// and record the new digest here (DCX-V1-471).
var derivationFixtureDigests = map[string]string{
	"1": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 2 adds derived declarations (DCS-V1-482); the fixture holds no source input a
	// records provider owns, so its digest is unchanged (DCS-V1-485).
	"2": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 3 derives declarations in additional repositories too (DCS-V1-549); the
	// fixture declares no additional repository, so its digest is unchanged.
	"3": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 4 derives declarations up to the record and output byte bounds and names the
	// bound in one gap (DCS-V1-568, DCS-V1-569); the fixture reaches neither bound, so its
	// digest is unchanged.
	"4": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 5 adds the methods of TypeScript and JavaScript class bodies as declarations
	// (DCS-V1-571); the fixture holds no web source, so its digest is unchanged.
	"5": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 6 derives the declarations of inputs other providers' records anchor first when
	// a bound drops some (DCS-V1-575); the fixture reaches no bound, so its digest is unchanged.
	"6": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 7 adds the top-level keys of exported TypeScript and JavaScript object literals
	// as declarations (DCS-V1-583); the fixture holds no web source, so its digest is unchanged.
	"7": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 8 adds module claims for exported object keys (DCP-V1-760);
	// this fixture has no exported object, so its digest is unchanged.
	"8": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 9 requires publish evidence for an npm package, adds the gem rules, names deploy
	// environments and unsupported CI, and grades first-party references (DCI-V1-006 to
	// DCI-V1-010); the fixture declares no repository inventory, so its digest is unchanged.
	"9": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 10 records an over-bound scope input as a declared gap, ties a publish job to the
	// manifest directory it publishes, infers a deploy environment from its name and reads
	// CircleCI jobs (DCI-V1-021 to DCI-V1-024); the fixture holds no over-bound input and declares
	// no repository inventory, so its digest is unchanged.
	"10": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
	// Derivation 11 reads prd, sbx and eph as environment words, matches a deploy job on its
	// words, and applies declared environment words and deploy jobs (DCI-V1-027 to DCI-V1-029);
	// the fixture declares no repository inventory, so its digest is unchanged.
	"11": "a571c3440bc8df0f5e1c5f758ce85b78511dc24d6820a32961311e272d1a3568",
}

// TestDCXV1471VerifiedReadAcrossBuilderRevisions: the verified read admits an artifact recorded
// by another attested builder revision with the same builder schema, derivation version and
// engine, and refuses, naming what differs, one whose derivation version, engine or schema
// differs or whose builder revision is unattested (DCX-V1-471).
func TestDCXV1471VerifiedReadAcrossBuilderRevisions(t *testing.T) {
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
		{"dirty", "builder revision is unrecorded, dirty or not a Git object ID", func(c *Artifact) { c.Builder.Revision = strings.Repeat("a", 40) + "+dirty" }},
		{"empty revision", "builder revision is unrecorded, dirty or not a Git object ID", func(c *Artifact) { c.Builder.Revision = "" }},
		{"malformed revision", "builder revision is unrecorded, dirty or not a Git object ID", func(c *Artifact) { c.Builder.Revision = "garbage" }},
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

// TestDCXV1471DerivationVersionGuard fails when the records the fixture corpus derives change
// without a new DerivationVersion (DCX-V1-471).
func TestDCXV1471DerivationVersionGuard(t *testing.T) {
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
