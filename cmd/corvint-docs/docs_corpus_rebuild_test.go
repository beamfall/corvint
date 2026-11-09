package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// recordedBy re-encodes artifact as recorded by another builder engine and attested revision,
// with its content digest recomputed, as an artifact from an earlier release would be.
func recordedBy(t *testing.T, artifact, engine string) string {
	t.Helper()
	a, err := doccorpus.ParseArtifact([]byte(artifact))
	if err != nil {
		t.Fatal(err)
	}
	c := *a
	c.Builder.Revision, c.Builder.Engine, c.SHA256 = strings.Repeat("a", 40), engine, ""
	unsigned, err := doccorpus.Encode(c)
	if err != nil {
		t.Fatal(err)
	}
	c.SHA256 = doccorpus.Digest(unsigned)
	data, err := doccorpus.Encode(c)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestDCXV1495BuildRebuildsFromEmbeddedManifest: docs corpus build --artifact rebuilds an artifact
// a verified read refuses as built by another derivation, from its embedded manifest alone, to
// the bytes build --manifest produces; it takes exactly one of --manifest or --artifact, refuses
// an artifact whose digests do not hold with no output, and never writes the artifact file
// (DCX-V1-495, DCX-V1-496, DCX-V1-497).
func TestDCXV1495BuildRebuildsFromEmbeddedManifest(t *testing.T) {
	defer doccorpus.TrustUnattestedBuilderForTest()()
	root := taskContextRepository(t)
	rev := cemGit(t, root, "rev-parse", "HEAD")
	built := writeCorpusFixture(t, root, rev, "cache")
	stale := recordedBy(t, built, "native-contextindex/"+doccorpus.Schema+"/derivation-0/"+runtime.Version())
	cemWrite(t, root, "stale.json", stale)
	code, _, stderr := corpusCLI(t, root, "docs", "corpus", "search", "--artifact", "stale.json", "--query", "Split")
	if code != 2 || !strings.Contains(stderr, "derivation version differs (artifact 0, running builder "+doccorpus.DerivationVersion+")") ||
		!strings.Contains(stderr, "docs corpus build --artifact") {
		t.Fatalf("stale read: %d %s", code, stderr)
	}
	for _, path := range []string{"stale.json", "corpus.json"} {
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--artifact", path)
		if code != 0 || out != built {
			t.Fatalf("build --artifact %s: %d %s", path, code, stderr)
		}
	}
	if after, err := os.ReadFile(filepath.Join(root, "stale.json")); err != nil || string(after) != stale {
		t.Fatalf("build --artifact wrote the artifact it read (%v)", err)
	}
	for _, args := range [][]string{{}, {"--manifest", "corpus-input.json", "--artifact", "corpus.json"}, {"--manifest=", "--artifact=corpus.json"}, {"--artifact="}} {
		code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus", "build"}, args...)...)
		if code != 2 || out != "" || !strings.Contains(stderr, "exactly one of --manifest or --artifact") {
			t.Fatalf("build %v: %d %q %s", args, code, out, stderr)
		}
	}
	cemWrite(t, root, "tampered.json", strings.Replace(stale, `"manifest":{`, `"manifest":{ `, 1))
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--artifact", "tampered.json")
	if code != 2 || out != "" || !strings.Contains(stderr, "corpus-refused") || !strings.Contains(stderr, "artifact digest mismatch") {
		t.Fatalf("build --artifact of a tampered artifact: %d %q %s", code, out, stderr)
	}
}

// TestDCXV1498MCPServesArtifactsAcrossBuilderUpgrades: a corpus MCP server answers its first call
// from an artifact another attested builder revision of the same derivation recorded; one whose
// derivation differs is refused, naming the version change and the rebuild; once the operator
// rebuilds it into a new file and renames it over the old one, the running server names the
// restart and a restarted server answers (DCX-V1-498).
func TestDCXV1498MCPServesArtifactsAcrossBuilderUpgrades(t *testing.T) {
	defer doccorpus.TrustUnattestedBuilderForTest()()
	root := taskContextRepository(t)
	rev := cemGit(t, root, "rev-parse", "HEAD")
	built := writeCorpusFixture(t, root, rev, "cache")
	ctx := context.Background()
	search := func(r *corpusbridge.Registry) *corpusbridge.ToolFailure {
		_, _, failure, protocol := r.Call(ctx, "corvint.docs_search", []byte(`{"query":"Split"}`))
		if protocol != nil {
			t.Fatalf("protocol error %v", protocol)
		}
		return failure
	}
	serve := func(path string) *corpusbridge.Registry {
		r, startup := corpusbridge.New(root, path)
		if startup != nil {
			t.Fatal(startup)
		}
		return r
	}
	cemWrite(t, root, "other-revision.json", recordedBy(t, built, "native-contextindex/"+doccorpus.Schema+"/derivation-"+doccorpus.DerivationVersion+"/"+runtime.Version()))
	if failure := search(serve("other-revision.json")); failure != nil {
		t.Fatalf("another attested revision of the same derivation: %+v", failure)
	}
	cemWrite(t, root, "stale.json", recordedBy(t, built, "native-contextindex/"+doccorpus.Schema+"/derivation-0/"+runtime.Version()))
	running := serve("stale.json")
	failure := search(running)
	if failure == nil || failure.Code != "corpus-refused" || !strings.Contains(failure.Message, "derivation version differs") ||
		!strings.Contains(failure.Message, "docs corpus build --artifact") {
		t.Fatalf("stale derivation: %+v", failure)
	}
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--artifact", "stale.json")
	if code != 0 {
		t.Fatal(stderr)
	}
	cemWrite(t, root, "stale.json.new", out)
	if err := os.Rename(filepath.Join(root, "stale.json.new"), filepath.Join(root, "stale.json")); err != nil {
		t.Fatal(err)
	}
	if failure := search(running); failure == nil || !strings.Contains(failure.Message, "restart with the new artifact") {
		t.Fatalf("running server after the rebuild: %+v", failure)
	}
	if failure := search(serve("stale.json")); failure != nil {
		t.Fatalf("restarted server on the rebuilt artifact: %+v", failure)
	}
}
