package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCEMExportBundleVerifiesOffline: `cem export` writes a bundle whose
// manifest bytes match the envelope's manifest digest (RCB-V0-001,
// RCB-V0-006); an output inside the worktree is refused (RCB-V0-005). The map
// is cem/1.0, exported through the stable route (CEM-V1-210).
func TestCEMExportBundleVerifiesOffline(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t)
	bind := commitExportMap(t, fixture)
	bundle := filepath.Join(t.TempDir(), "bundle")
	code, out, stderr := runCLI(t, "--root", fixture.root, "cem", "export", "--map", fixture.mapPath,
		"--expected-base", fixture.base, "--target", bind, "--artifacts", fixture.root, "--output", bundle)
	if code != 0 {
		t.Fatalf("export exited %d: %s", code, stderr)
	}
	var envelope struct {
		OK             bool   `json:"ok"`
		Mutates        bool   `json:"mutates"`
		ManifestSha256 string `json:"manifestSha256"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil || !envelope.OK || envelope.Mutates {
		t.Fatalf("envelope %s: %v", out, err)
	}
	// The 0.x offline shell verifier is retired (RCB-V0-006, V1-1278); the
	// stable manifest binds its own digest.
	manifest, err := os.ReadFile(filepath.Join(bundle, "manifest.json"))
	if err != nil || digestBytes(manifest) != envelope.ManifestSha256 {
		t.Fatalf("manifest digest: %v envelope %s", err, out)
	}

	code, _, stderr = runCLI(t, "--root", fixture.root, "cem", "export", "--map", fixture.mapPath,
		"--expected-base", fixture.base, "--target", bind, "--artifacts", fixture.root, "--output", filepath.Join(fixture.root, "bundle"))
	if code != 2 || !strings.Contains(stderr, `"code": "bundle-output-refused"`) {
		t.Fatalf("in-worktree output: %d %s", code, stderr)
	}
}

// commitExportMap commits the fixture's CEM sidecar and returns the bind
// commit, which RCB-V0-001 requires as the export target.
func commitExportMap(t *testing.T, fixture lrfFixture) string {
	t.Helper()
	cemGit(t, fixture.root, "add", fixture.mapPath)
	cemGit(t, fixture.root, "commit", "-qm", "bind")
	return cemGit(t, fixture.root, "rev-parse", "HEAD")
}
