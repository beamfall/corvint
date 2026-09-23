package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMExportBundleVerifiesOffline: `cem export` writes a bundle the offline
// verifier accepts, and both agree on the manifest digest (RCB-V0-001,
// RCB-V0-006); an output inside the worktree is refused (RCB-V0-005).
func TestCEMExportBundleVerifiesOffline(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t, wire.Spec02)
	bind := commitExportMap(t, fixture)
	bundle := filepath.Join(t.TempDir(), "bundle")
	code, out, stderr := runCLI(t, "--root", fixture.root, "cem", "export", "--map", fixture.mapPath,
		"--expected-base", fixture.base, "--target", bind, "--output", bundle)
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
	verified, err := exec.Command("sh", "../../script/verify-receipt-bundle.sh", bundle).CombinedOutput()
	if err != nil {
		t.Fatalf("verifier: %v\n%s", err, verified)
	}
	lines := strings.Split(strings.TrimSpace(string(verified)), "\n")
	if lines[0] != "manifest sha256 "+envelope.ManifestSha256 || !strings.HasPrefix(lines[1], "MATCH receipts/cem.json ") ||
		lines[len(lines)-1] != "PASS" {
		t.Fatalf("verifier output:\n%s", verified)
	}

	code, _, stderr = runCLI(t, "--root", fixture.root, "cem", "export", "--map", fixture.mapPath,
		"--expected-base", fixture.base, "--target", bind, "--output", filepath.Join(fixture.root, "bundle"))
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
