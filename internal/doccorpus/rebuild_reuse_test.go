package doccorpus

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// rebuildFrom runs RebuildArtifact on data from the manifest it embeds, as docs corpus build
// --artifact does, and returns the encoded result, whether it reused data's records, and how
// many compiler builds ran.
func rebuildFrom(t *testing.T, root string, data []byte) ([]byte, bool, int64, error) {
	t.Helper()
	m, err := ManifestOfArtifact(data)
	if err != nil {
		t.Fatal(err)
	}
	return rebuildWith(t, root, m, data)
}

// rebuildWith is rebuildFrom with the manifest given, for an artifact whose embedded manifest
// ManifestOfArtifact itself refuses.
func rebuildWith(t *testing.T, root string, m Manifest, data []byte) ([]byte, bool, int64, error) {
	t.Helper()
	before := BuildsForTest()
	a, reused, err := RebuildArtifact(context.Background(), root, m, data)
	builds := BuildsForTest() - before
	if err != nil {
		return nil, reused, builds, err
	}
	out, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	return out, reused, builds, nil
}

// fullBuild is the bytes build --manifest writes for the manifest data embeds.
func fullBuild(t *testing.T, root string, data []byte) []byte {
	t.Helper()
	m, err := ManifestOfArtifact(data)
	if err != nil {
		t.Fatal(err)
	}
	a, err := BuildAdmitted(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// otherRevision records a as built by another attested revision of this builder's derivation.
func otherRevision(c *Artifact) { c.Builder.Revision = strings.Repeat("b", 40) }

// TestDCXV1532RebuildReusesSameDerivation: build --artifact of an artifact recorded by another
// attested revision of the running derivation reuses its records without a compiler build and
// prints exactly the bytes a full build of the same manifest prints, for a single record, two
// shards and a many-shard inventory (DCX-V1-532). It runs alone, since it counts builds.
func TestDCXV1532RebuildReusesSameDerivation(t *testing.T) {
	small := func(t *testing.T) (string, Manifest) { return largeAdoptionCorpus(t, 48, 6) }
	for name, corpus := range map[string]func(*testing.T) (string, Manifest){
		"record":    func(t *testing.T) (string, Manifest) { return adoptionFixture(t, nil) },
		"shards":    func(t *testing.T) (string, Manifest) { return shardAdoptionFixture(t, nil) },
		"inventory": small,
	} {
		t.Run(name, func(t *testing.T) {
			root, m := corpus(t)
			a, err := BuildAdmitted(context.Background(), root, m)
			if err != nil {
				t.Fatal(err)
			}
			for label, data := range map[string][]byte{"same builder": encodeArtifact(t, a), "other revision": rehashed(t, a, otherRevision)} {
				want := fullBuild(t, root, data)
				got, reused, builds, err := rebuildFrom(t, root, data)
				if err != nil {
					t.Fatalf("%s: %v", label, err)
				}
				if !reused || builds != 0 {
					t.Fatalf("%s: reused=%v after %d builds, want the records reused without a build", label, reused, builds)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("%s: reused rebuild differs from a full build", label)
				}
				if _, err := OpenRead(context.Background(), root, got, ReadOptions{}); err != nil {
					t.Fatalf("%s: reused rebuild is not readable: %v", label, err)
				}
			}
		})
	}
}

func encodeArtifact(t *testing.T, a *Artifact) []byte {
	t.Helper()
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestDCXV1533RebuildRederivesOtherwise: build --artifact rederives in full, to the same bytes,
// an artifact of another or an unrecorded derivation (which the verified read still refuses),
// one whose recorded digests or tree a full build would not reproduce, and any artifact when the
// running builder is unattested; a pin refusal is the full build's own refusal (DCX-V1-533). It
// runs alone, since it counts builds and turns the builder trust off.
func TestDCXV1533RebuildRederivesOtherwise(t *testing.T) {
	root, a, data := builtFixture(t)
	want := fullBuild(t, root, data)
	unrecorded := func(c *Artifact) {
		c.Builder.Revision = strings.Repeat("a", 40)
		c.Builder.Engine = "native-contextindex/" + Schema + "/" + runtime.Version()
	}
	for name, edit := range map[string]func(*Artifact){
		"other derivation":      staleDerivation,
		"unrecorded derivation": unrecorded,
		"other engine":          func(c *Artifact) { otherRevision(c); c.Builder.Engine += "-other" },
		"manifest digest":       func(c *Artifact) { otherRevision(c); c.ManifestSHA256 = Digest([]byte("other")) },
		"profile digest":        func(c *Artifact) { otherRevision(c); c.ProfileSHA256 = Digest([]byte("other")) },
		"tree":                  func(c *Artifact) { otherRevision(c); c.Tree = strings.Repeat("c", 40) },
	} {
		stale := rehashed(t, a, edit)
		got, reused, builds, err := rebuildWith(t, root, a.Manifest, stale)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if reused || builds != 1 || !bytes.Equal(got, want) {
			t.Fatalf("%s: reused=%v after %d builds; want one full build to the same bytes", name, reused, builds)
		}
		if name == "unrecorded derivation" {
			if _, err := OpenRead(context.Background(), root, stale, ReadOptions{}); err == nil || !strings.Contains(err.Error(), "derivation version differs (artifact unrecorded") {
				t.Fatalf("the verified read admitted an unrecorded derivation: %v", err)
			}
		}
	}

	t.Run("input refusal", func(t *testing.T) {
		bad := rehashed(t, a, func(c *Artifact) {
			c.Manifest.Inputs = append([]Input(nil), c.Manifest.Inputs...)
			c.Manifest.Inputs[0].SHA256 = Digest([]byte("other bytes"))
			c.ManifestSHA256 = testHash(t, c.Manifest)
		})
		m, err := ManifestOfArtifact(bad)
		if err != nil {
			t.Fatal(err)
		}
		_, full := BuildAdmitted(context.Background(), root, m)
		_, reused, _, err := rebuildFrom(t, root, bad)
		if full == nil || err == nil || err.Error() != full.Error() || reused {
			t.Fatalf("rebuild refusal %v differs from the full build's %v", err, full)
		}
	})

	t.Run("unattested builder", func(t *testing.T) {
		if attestedBuilder(currentBuilder()) {
			t.Skip("this test binary records an attested build")
		}
		unattestedBuilderTrusted.Store(false)
		t.Cleanup(func() { unattestedBuilderTrusted.Store(true) })
		got, reused, builds, err := rebuildFrom(t, root, data)
		if err != nil {
			t.Fatal(err)
		}
		if reused || builds != 1 || !bytes.Equal(got, want) {
			t.Fatalf("unattested builder reused=%v after %d builds", reused, builds)
		}
	})
}

// largeAdoptionCorpus is a committed inventory of files Go sources, each declared by one adoption
// records provider as a module, a symbol and a claim, three anchors, over shards /1 shards: the
// shape of #1664's corpus at about 9,500 files.
func largeAdoptionCorpus(t *testing.T, files, shards int) (string, Manifest) {
	t.Helper()
	root, _ := fixture(t)
	body := strings.Repeat("// filler line for the synthetic corpus source file\n", 40)
	sources := make(map[string][]byte, files)
	for i := 0; i < files; i++ {
		p := fmt.Sprintf("src/app/m%03d/c%05d.go", i%300, i)
		sources[p] = []byte(fmt.Sprintf("package m%03d\n\n// Handler%05d handles.\nfunc Handler%05d() int { return %d }\n%s", i%300, i, i, i, body))
		writeFixture(t, root, p, sources[p])
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "inventory sources")
	rev := git(t, root, "rev-parse", "HEAD")
	m, err := InventoryScopes(context.Background(), root, rev, []string{"src"}, ManifestSchemaV2, "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	blobs := map[string]string{}
	for _, in := range m.Inputs {
		blobs[in.Path] = in.Blob
	}
	paths := []string{}
	per := (files + shards - 1) / shards
	for s := 0; s < shards; s++ {
		record := ProviderRecord{Schema: AdoptionProviderSchema, ID: "inv", Version: "1", Source: m.Repository, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{}, Capabilities: []CapabilityDeclaration{{"subjects", "present", "explicit provider subjects"}, {"claims", "present", "explicit claims"}}}
		for i := s * per; i < (s+1)*per && i < files; i++ {
			p := fmt.Sprintf("src/app/m%03d/c%05d.go", i%300, i)
			data := sources[p]
			ev := func(line int, symbol string) Evidence {
				excerpt, _ := span(data, line, line)
				anchor := Anchor{Repository: m.Repository.ID, Revision: m.Repository.Revision, Path: p, Blob: blobs[p], SHA256: Digest(data), Start: line, End: line, SpanSHA256: Digest(excerpt), Symbol: symbol, Authority: "external-provider", Kind: "declared", Reason: "inventory unit declared at its exact source line"}
				return Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{anchor}, Limitations: []string{"provider declaration"}}
			}
			symbol := fmt.Sprintf("Handler%05d", i)
			sid := "inv:symbol:" + p + ":Handler"
			record.Subjects = append(record.Subjects, Subject{ID: "inv:file:" + p, Kind: "module", Name: p, Provider: "inv", Evidence: ev(1, "")},
				Subject{ID: sid, Kind: "symbol", Name: symbol, Provider: "inv", Evidence: ev(4, symbol)})
			record.Claims = append(record.Claims, Claim{ID: "inv:claim:" + strconv.Itoa(i), Subject: sid, Text: symbol + " returns its index for the caller", Provider: "inv", Evidence: ev(3, "")})
		}
		raw, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		p := fmt.Sprintf("shards/s%03d.json", s)
		writeFixture(t, root, p, raw)
		paths = append(paths, p)
	}
	git(t, root, "add", "shards")
	git(t, root, "commit", "-qm", "inventory shards")
	srev := git(t, root, "rev-parse", "HEAD")
	m.Scopes = append(m.Scopes, Scope{Path: "shards", Revision: srev})
	for _, p := range paths {
		raw, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		m.Inputs = append(m.Inputs, Input{Path: p, Revision: srev, Blob: git(t, root, "rev-parse", srev+":"+p), SHA256: Digest(raw), Provider: "inv", Purpose: "provider"})
	}
	m.Providers = append(m.Providers, Provider{ID: "inv", Kind: "records", Version: "1", Revision: srev, Shards: paths})
	return root, m
}

func writeFixture(t testing.TB, root, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
		t.Fatal(err)
	}
}
