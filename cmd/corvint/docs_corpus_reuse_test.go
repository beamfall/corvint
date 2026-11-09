package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// TestDCXV1531CompactProviderCommand: docs corpus compact-provider prints exactly the producer's
// /2 encoding of a /1 provider record, and refuses a /2 or missing input with no output
// (DCX-V1-531).
func TestDCXV1531CompactProviderCommand(t *testing.T) {
	t.Parallel()
	root, _ := typedCorpusCLIFixture(t)
	raw, err := os.ReadFile(filepath.Join(root, "provider.json"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := doccorpus.CompactProvider("provider.json", raw)
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "compact-provider", "--input", "provider.json")
	if code != 0 || out != string(want) || !strings.Contains(out, `"schema":"`+doccorpus.AdoptionProviderSchemaV2+`"`) {
		t.Fatalf("compact-provider: %d %s", code, stderr)
	}
	cemWrite(t, root, "compact.json", out)
	for _, input := range []string{"compact.json", "missing.json"} {
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "compact-provider", "--input", input)
		if code != 2 || out != "" || stderr == "" {
			t.Fatalf("compact-provider %s: %d %q %s", input, code, out, stderr)
		}
	}
}

// TestDCXV1532BuildArtifactReusesRecords: docs corpus build --artifact of an artifact another
// attested revision of the running derivation recorded prints the bytes build --manifest prints
// without a compiler build, and one of another derivation is still rebuilt in full to the same
// bytes (DCX-V1-532, DCX-V1-533). It runs alone, since it counts builds.
func TestDCXV1532BuildArtifactReusesRecords(t *testing.T) {
	defer doccorpus.TrustUnattestedBuilderForTest()()
	root := taskContextRepository(t)
	rev := cemGit(t, root, "rev-parse", "HEAD")
	built := writeCorpusFixture(t, root, rev, "cache")
	for name, engine := range map[string]string{"other-revision.json": doccorpus.DerivationVersion, "stale.json": "0"} {
		cemWrite(t, root, name, recordedBy(t, built, "native-contextindex/"+doccorpus.Schema+"/derivation-"+engine+"/"+runtime.Version()))
		before := doccorpus.BuildsForTest()
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--artifact", name)
		builds := doccorpus.BuildsForTest() - before
		if code != 0 || out != built {
			t.Fatalf("build --artifact %s: %d %s", name, code, stderr)
		}
		if want := int64(map[bool]int{true: 0, false: 1}[name == "other-revision.json"]); builds != want {
			t.Fatalf("build --artifact %s ran %d builds, want %d", name, builds, want)
		}
	}
}
