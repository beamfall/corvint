package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
)

// TestCEMV1162CorpusStableOperation (CEM-V1-162, CEM-V1-163): `docs corpus
// cem-stable` prints corvint-corpus-cem-stable/1 only over a stable verifier
// ACCEPT; a wrong base, edited map bytes, a tampered artifact or a missing
// artifact root exits 2 with no stdout; a partial binding is an argument
// error; and the 0.x `cem` operation still refuses a cem/1.0 map.
func TestCEMV1162CorpusStableOperation(t *testing.T) {
	change := stablefixture.NewChange(t)
	stable := change.Stable(t)
	writeCorpusFixture(t, change.Root, change.Base, "docs/rule.txt")
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(change.Root, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("stable.cem.json", stable.Map)
	edited := strings.ReplaceAll(string(stable.Map), stable.CriterionID, strings.Repeat("0", len(stable.CriterionID)))
	write("edited.cem.json", []byte(edited))
	project := func(cem, base, artifacts string) (int, string, string) {
		return corpusCLI(t, change.Root, "docs", "corpus", "cem-stable", "--artifact", "corpus.json", "--cem", cem,
			"--expected-base", base, "--target", stable.Target, "--stable-artifact-root", artifacts)
	}
	code, out, stderr := project("stable.cem.json", change.Base, stable.ArtifactRoot)
	if code != 0 || stderr != "" {
		t.Fatalf("stable projection: exit %d stderr %q", code, stderr)
	}
	var receipt struct {
		Schema       string `json:"schema"`
		Outcome      string `json:"outcome"`
		CEMSHA256    string `json:"cem_sha256"`
		Verification struct {
			Outcome string `json:"outcome"`
		} `json:"cem_verification"`
		Claims []map[string]any `json:"claims"`
	}
	if err := json.Unmarshal([]byte(out), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Schema != "corvint-corpus-cem-stable/1" || receipt.Outcome != "PROJECTED" || receipt.Verification.Outcome != "ACCEPT" ||
		receipt.CEMSHA256 != stablefixture.Digest(stable.Map) || len(receipt.Claims) == 0 {
		t.Fatalf("stable projection receipt: %s", out)
	}
	refused := func(name string, code int, out, stderr string) {
		t.Helper()
		if code != 2 || out != "" || !strings.Contains(stderr, "corpus-stable-refused") {
			t.Fatalf("%s: exit %d out %q stderr %q", name, code, out, stderr)
		}
	}
	code, out, stderr = project("stable.cem.json", change.ProducerTarget, stable.ArtifactRoot)
	refused("wrong base", code, out, stderr)
	code, out, stderr = project("edited.cem.json", change.Base, stable.ArtifactRoot)
	refused("edited map bytes", code, out, stderr)
	code, out, stderr = project("stable.cem.json", change.Base, filepath.Join(t.TempDir(), "absent"))
	refused("missing artifact root", code, out, stderr)
	artifact := filepath.Join(stable.ArtifactRoot, filepath.FromSlash(stable.ArtifactPaths[0]))
	if err := os.WriteFile(artifact, []byte(`{"fixture":"tampered"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, stderr = project("stable.cem.json", change.Base, stable.ArtifactRoot)
	refused("tampered artifact", code, out, stderr)

	code, out, stderr = corpusCLI(t, change.Root, "docs", "corpus", "cem-stable", "--artifact", "corpus.json", "--cem", "stable.cem.json",
		"--expected-base", change.Base, "--target", stable.Target)
	if code != 2 || out != "" || !strings.Contains(stderr, "missing required corpus option") {
		t.Fatalf("partial binding: exit %d out %q stderr %q", code, out, stderr)
	}
	code, out, _ = corpusCLI(t, change.Root, "docs", "corpus", "cem", "--artifact", "corpus.json", "--cem", "stable.cem.json")
	if code != 2 || out != "" {
		t.Fatalf("0.x cem operation over cem/1.0: exit %d out %q", code, out)
	}
}
