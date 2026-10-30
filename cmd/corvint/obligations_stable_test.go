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
	code, out, stderr = runCLI(t, arguments(change.ProducerTarget)...)
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

// TestCEMV1639ObligationsAllowAbsentSidecar (CEM-V1-639): an accepted map that
// is not committed at the target refuses by default with the fix named, and
// --allow-absent-sidecar admits it with binding.sidecar ABSENT; the exact
// sidecar keeps its bytes with or without the flag, and the flag needs the
// stable context.
func TestCEMV1639ObligationsAllowAbsentSidecar(t *testing.T) {
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
	arguments := func(target string, extra ...string) []string {
		return append([]string{"obligations", "--cem", cemPath, "--impact", impactPath, "--repository", change.Root,
			"--expected-base", change.Base, "--target", target, "--stable-artifact-root", stable.ArtifactRoot}, extra...)
	}
	code, exact, stderr := runCLI(t, arguments(stable.Target)...)
	if code != 0 || stderr != "" || strings.Contains(exact, `"sidecar"`) {
		t.Fatalf("exact sidecar: exit %d out %s stderr %q", code, exact, stderr)
	}
	if code, flagged, _ := runCLI(t, arguments(stable.Target, "--allow-absent-sidecar")...); code != 0 || flagged != exact {
		t.Fatalf("the flag changed an exact sidecar's document: exit %d\n%s\n%s", code, flagged, exact)
	}
	// ProducerTarget carries the same change without the committed sidecar.
	code, out, stderr := runCLI(t, arguments(change.ProducerTarget)...)
	if code != 2 || out != "" || !strings.Contains(stderr, "sidecar is ABSENT") || !strings.Contains(stderr, "--allow-absent-sidecar") {
		t.Fatalf("absent sidecar default: exit %d out %q stderr %q", code, out, stderr)
	}
	code, out, stderr = runCLI(t, arguments(change.ProducerTarget, "--allow-absent-sidecar")...)
	if code != 0 || stderr != "" {
		t.Fatalf("absent sidecar admitted: exit %d stderr %q", code, stderr)
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		t.Fatal(err)
	}
	binding := document["binding"].(map[string]any)
	if binding["sidecar"] != "ABSENT" || binding["cem_sha256"] != stablefixture.Digest(stable.Map) || binding["target_revision"] != change.ProducerTarget {
		t.Fatalf("absent binding: %s", out)
	}
	for _, invalid := range [][]string{
		{"obligations", "--cem", cemPath, "--impact", impactPath, "--allow-absent-sidecar"},
		arguments(change.ProducerTarget, "--allow-absent-sidecar", "--allow-absent-sidecar"),
	} {
		if code, out, stderr := runCLI(t, invalid...); code != 2 || out != "" || !strings.Contains(stderr, "--allow-absent-sidecar") {
			t.Fatalf("%v: exit %d out %q stderr %q", invalid, code, out, stderr)
		}
	}
}
