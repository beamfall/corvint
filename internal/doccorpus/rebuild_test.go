package doccorpus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// staleDerivation records the fixture artifact as built by a derivation this builder no longer
// runs, so the verified read refuses it with `builder differs:`.
func staleDerivation(c *Artifact) {
	c.Builder.Revision = strings.Repeat("a", 40)
	c.Builder.Engine = "native-contextindex/" + Schema + "/derivation-0/" + runtime.Version()
}

// resigned replaces the content digest of artifact bytes edited below the struct level, as a
// deliberate editor would.
func resigned(data []byte) []byte {
	at := bytes.LastIndex(data, tailMember)
	digest := hashArtifactTail(data).digest
	return append(append(bytes.Clone(data[:at]), `,"sha256":"`+digest+`"}`...), '\n')
}

// TestDCXV1495RebuildFromEmbeddedManifest: the manifest an artifact embeds rebuilds it. A stale
// artifact, recorded by another derivation and holding a member this builder does not decode,
// yields its manifest without a build, and building that manifest reproduces the bytes a current
// build of the same inputs produces (DCX-V1-495).
func TestDCXV1495RebuildFromEmbeddedManifest(t *testing.T) {
	t.Parallel()
	root, a, data := builtFixture(t)
	stale := rehashed(t, a, staleDerivation)
	stale = resigned(bytes.Replace(stale, []byte(`{"schema":`), []byte(`{"future_records":[1],"schema":`), 1))
	if _, err := ParseArtifact(stale); err == nil {
		t.Fatal("the closed artifact decode admitted an unknown member; the fixture does not exercise it")
	}
	for name, in := range map[string][]byte{"current": data, "stale": stale} {
		m, err := ManifestOfArtifact(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !equalEncoding(t, m, a.Manifest) {
			t.Fatalf("%s: embedded manifest differs from the one built", name)
		}
		rebuilt, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out, err := Encode(rebuilt)
		if err != nil || !bytes.Equal(out, data) {
			t.Fatalf("%s: rebuilt bytes differ from a current build (%v)", name, err)
		}
		if _, err := OpenRead(context.Background(), root, out, ReadOptions{}); err != nil {
			t.Fatalf("%s: rebuilt artifact is not readable: %v", name, err)
		}
	}
}

// TestDCXV1496RebuildRefusesUnverifiedManifest: the rebuild refuses bytes whose content digest,
// canonical tail, schema, embedded manifest or manifest digest does not hold before any build,
// bytes over their schema's size bound, and a pinned input that no longer matches Git at build
// (DCX-V1-496). Not parallel: it counts the process's builds.
func TestDCXV1496RebuildRefusesUnverifiedManifest(t *testing.T) {
	root, a, data := builtFixture(t)
	changed := *a
	changed.Manifest.Inputs = append([]Input(nil), a.Manifest.Inputs...)
	changed.Manifest.Inputs[0].SHA256 = strings.Repeat("0", 64)
	cases := []struct {
		name, want string
		data       []byte
	}{
		{"digest", "artifact digest mismatch", bytes.Replace(data, []byte(`"manifest":{`), []byte(`"manifest":{ `), 1)},
		{"noncanonical tail", "noncanonical artifact", append(bytes.TrimSuffix(data, []byte("}\n")), " }\n"...)},
		{"not JSON", "invalid artifact JSON", []byte("not json\n")},
		{"schema", "unsupported artifact schema", rehashed(t, a, func(c *Artifact) { c.Schema = "other-schema/1" })},
		{"schema size bound", "input bound exceeded", resigned(bytes.Replace(data, []byte(`{"schema":`), []byte(`{"future_records":"`+strings.Repeat("x", MaxBytes)+`","schema":`), 1))},
		{"no manifest", "artifact has no embedded manifest", resigned([]byte(`{"schema":"` + Schema + `","sha256":""}` + "\n"))},
		{"open manifest", "invalid closed JSON input", resigned(bytes.Replace(data, []byte(`"manifest":{`), []byte(`"manifest":{"extra":1,`), 1))},
		{"manifest digest", "artifact manifest digest mismatch", rehashed(t, a, func(c *Artifact) { c.ManifestSHA256 = strings.Repeat("0", 64) })},
		{"edited manifest", "artifact manifest digest mismatch", rehashed(t, a, func(c *Artifact) { c.Manifest = changed.Manifest })},
	}
	for _, tc := range cases {
		before := BuildsForTest()
		_, err := ManifestOfArtifact(tc.data)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want a refusal naming %q", tc.name, err, tc.want)
		}
		if BuildsForTest() != before {
			t.Errorf("%s: the refused manifest read ran a build", tc.name)
		}
	}
	// A pinned input digest the embedded manifest records consistently but Git no longer holds is
	// refused by the build that verifies every pin.
	repinned := rehashed(t, a, func(c *Artifact) { c.Manifest, c.ManifestSHA256 = changed.Manifest, testHash(t, changed.Manifest) })
	m, err := ManifestOfArtifact(repinned)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(context.Background(), root, m); err == nil || !strings.Contains(err.Error(), "input changed after pinning") {
		t.Fatalf("build of a manifest whose pin Git no longer holds: %v", err)
	}
}

// TestDCXV1497ReadNeverRebuilds: a verified read of a stale artifact refuses, names the explicit
// rebuild, runs no build and leaves the artifact file unchanged (DCX-V1-497). Not parallel: it
// counts the process's builds.
func TestDCXV1497ReadNeverRebuilds(t *testing.T) {
	root, a, _ := builtFixture(t)
	stale := rehashed(t, a, staleDerivation)
	if err := os.WriteFile(filepath.Join(root, "stale-corpus.json"), stale, 0o600); err != nil {
		t.Fatal(err)
	}
	before := BuildsForTest()
	for name, read := range map[string]func() error{
		"bytes": func() error { _, err := OpenRead(context.Background(), root, stale, ReadOptions{}); return err },
		"file": func() error {
			_, err := OpenReadFile(context.Background(), root, "stale-corpus.json", ReadOptions{})
			return err
		},
	} {
		err := read()
		if err == nil || !strings.Contains(err.Error(), "derivation version differs") || !strings.Contains(err.Error(), "docs corpus build --artifact") {
			t.Errorf("%s: %v, want a refusal naming the version change and the rebuild", name, err)
		}
	}
	if BuildsForTest() != before {
		t.Fatalf("a refused read ran %d builds", BuildsForTest()-before)
	}
	if after, err := os.ReadFile(filepath.Join(root, "stale-corpus.json")); err != nil || !bytes.Equal(after, stale) {
		t.Fatalf("a refused read changed the artifact file (%v)", err)
	}
}

func equalEncoding(t *testing.T, x, y any) bool {
	t.Helper()
	return testHash(t, x) == testHash(t, y)
}
