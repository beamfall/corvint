package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
)

// TestCEMV1135ObligationsStableMode (CEM-V1-135): the four stable flags select
// external-frontier-obligations-stable/0 over a verified cem/1.0 map, a wrong
// base refuses with the verifier code and no stdout, and a partial stable
// context is an argument error.
func TestCEMV1135ObligationsStableMode(t *testing.T) {
	change := stablefixture.NewChange(t)
	stable := change.Stable(t)
	scratch := t.TempDir()
	cemPath, impactPath := filepath.Join(scratch, "stable.cem.json"), filepath.Join(scratch, "impact.json")
	receipt := `{"ok":true,"tool":"impact","context":{"profile":"impact"}}`
	for path, content := range map[string][]byte{cemPath: stable.Map, impactPath: []byte(receipt)} {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	arguments := func(base string) []string {
		return []string{"obligations", "--cem", cemPath, "--impact", impactPath, "--repository", change.Root,
			"--expected-base", base, "--target", stable.Target, "--stable-artifact-root", stable.ArtifactRoot}
	}
	code, out, stderr := runCLI(t, arguments(change.Base)...)
	if code != 0 || stderr != "" {
		t.Fatalf("stable obligations: exit %d stderr %q", code, stderr)
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		t.Fatal(err)
	}
	binding := document["binding"].(map[string]any)
	if document["schema"] != "external-frontier-obligations-stable/0" || binding["cem_sha256"] != stablefixture.Digest(stable.Map) ||
		binding["target_revision"] != stable.Target || document["state"] != "unknown" {
		t.Fatalf("stable sidecar: %s", out)
	}
	code, out, stderr = runCLI(t, arguments(change.LegacyTarget)...)
	if code != 2 || out != "" || !strings.Contains(stderr, `"code"`) {
		t.Fatalf("wrong base: exit %d out %q stderr %q", code, out, stderr)
	}
	code, out, stderr = runCLI(t, "obligations", "--cem", cemPath, "--impact", impactPath, "--stable-artifact-root", stable.ArtifactRoot)
	if code != 2 || out != "" || !strings.Contains(stderr, "stable mode requires") {
		t.Fatalf("partial stable context: exit %d out %q stderr %q", code, out, stderr)
	}
	code, out, stderr = runCLI(t, "obligations", "--cem", cemPath, "--impact", impactPath)
	if code != 2 || out != "" || !strings.Contains(stderr, "invalid-obligations-input") {
		t.Fatalf("frozen sidecar over cem/1.0: exit %d out %q stderr %q", code, out, stderr)
	}
}
