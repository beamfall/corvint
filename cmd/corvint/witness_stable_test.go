package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1160WitnessStableRoute (CEM-V1-160, CEM-V1-161): `corvint witness
// --stable-artifact-root` prints corvint-witness-stable/0 only over a stable
// verifier ACCEPT, and a wrong base, edited map bytes, a tampered artifact or
// a missing artifact root each exit 2 with no stdout.
func TestCEMV1160WitnessStableRoute(t *testing.T) {
	change := stablefixture.NewChange(t)
	stable := change.Stable(t)
	witnessCLI := func(base string, extra ...string) (int, string, string) {
		return runCLI(t, append([]string{"--root", change.Root, "witness", "--base", base, "--json"}, extra...)...)
	}
	code, out, stderr := witnessCLI(change.Base, "--stable-artifact-root", stable.ArtifactRoot)
	if code != 0 || stderr != "" {
		t.Fatalf("stable witness: exit %d stderr %q", code, stderr)
	}
	var report struct {
		Profile string `json:"profile"`
		Sources []struct {
			Status string `json:"status"`
			Stable struct {
				Verification struct {
					Outcome   string `json:"outcome"`
					MapSha256 string `json:"mapSha256"`
				} `json:"verification"`
			} `json:"stable"`
		} `json:"sources"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Profile != "corvint-witness-stable/0" || report.Sources[0].Status != "BOUND" ||
		report.Sources[0].Stable.Verification.Outcome != "ACCEPT" || report.Sources[0].Stable.Verification.MapSha256 != stablefixture.Digest(stable.Map) {
		t.Fatalf("stable witness report: %s", out)
	}
	refused := func(name string, code int, out, stderr string, reason string) {
		t.Helper()
		if code != 2 || out != "" || !strings.Contains(stderr, "stable-witness-refused") || !strings.Contains(stderr, reason) {
			t.Fatalf("%s: exit %d out %q stderr %q", name, code, out, stderr)
		}
	}
	code, out, stderr = witnessCLI(change.ProducerTarget, "--stable-artifact-root", stable.ArtifactRoot)
	refused("wrong base", code, out, stderr, "stable verifier returned")
	code, out, stderr = witnessCLI(change.Base)
	refused("missing artifact root", code, out, stderr, "UNSUPPORTED")
	code, out, stderr = witnessCLI(change.Base, "--stable-artifact-root", filepath.Join(t.TempDir(), "absent"))
	refused("absent artifact root", code, out, stderr, "stable verifier returned")

	artifact := filepath.Join(stable.ArtifactRoot, filepath.FromSlash(stable.ArtifactPaths[0]))
	original, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte(`{"fixture":"tampered"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, stderr = witnessCLI(change.Base, "--stable-artifact-root", stable.ArtifactRoot)
	refused("tampered artifact", code, out, stderr, "stable verifier returned")
	if err := os.WriteFile(artifact, original, 0o644); err != nil {
		t.Fatal(err)
	}

	edited := strings.Replace(string(stable.Map), stable.CriterionID, strings.Repeat("0", len(stable.CriterionID)), -1)
	if edited == string(stable.Map) {
		t.Fatal("edit did not change the map")
	}
	if err := os.WriteFile(filepath.Join(change.Root, filepath.FromSlash(wire.ExcludedCEMPath)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	change.Git("add", "-f", wire.ExcludedCEMPath)
	change.Git("commit", "-qm", "edited stable sidecar")
	code, out, stderr = witnessCLI(change.Base, "--stable-artifact-root", stable.ArtifactRoot)
	refused("edited map bytes", code, out, stderr, "stable verifier returned")

	change.Git("rm", "-q", "--cached", wire.ExcludedCEMPath)
	change.Git("commit", "-qm", "no sidecar")
	code, out, stderr = witnessCLI(change.Base, "--stable-artifact-root", stable.ArtifactRoot)
	refused("no cem/1.0 map", code, out, stderr, "requires a cem/1.0 map")
	code, out, stderr = witnessCLI(change.Base)
	if code != 0 || stderr != "" || !strings.Contains(out, `"corvint-witness/0"`) {
		t.Fatalf("0.x route without the stable flag: exit %d stderr %q", code, stderr)
	}
}
