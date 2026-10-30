package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCAXV0034ProvePairedEvidenceSignsTheRange checks CAX-V0-034 end to end:
// prove --attest-cem-v3 --paired-evidence signs the document's range pins and
// SHA-256 as pairedRange, the verifier's receipt reports them, and the flag
// is refused without --attest-cem-v3.
func TestCAXV0034ProvePairedEvidenceSignsTheRange(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	mapBytes, err := os.ReadFile(filepath.Join(f.repo, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	stable, err := wire.ParseStable(mapBytes)
	if err != nil {
		t.Fatal(err)
	}
	paired, err := wire.EncodePairedEvidence(wire.PairedEvidence{
		Change: wire.PairedChange{BaseRevision: stable.Change.BaseRevision, PatchSha256: stable.Change.PatchSha256},
		Citations: []wire.PairedCitation{{BlobOid: strings.Repeat("b", 40), HunkID: stable.Change.Hunks[0].ID,
			Path: "tests/calc_test.go", Relation: "test-claim", Span: wire.PairedSpan{End: 8}, SpanSha256: strings.Repeat("c", 64)}},
		Profile: wire.PairedEvidenceProfile,
		Range:   wire.PairedRange{BaseRevision: strings.Repeat("1", 40), HeadRevision: strings.Repeat("2", 40), Name: "tests"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.repo, "paired.json"), paired, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(paired)
	keyPath, publicKey := proveTestKey(t)
	common := []string{"--root", f.repo, "prove", "--cem", "stable.json", "--expected-base", f.base,
		"--target", f.target, "--artifacts", f.artifacts, "--attest-key", keyPath, "--paired-evidence", "paired.json"}
	stdout, stderr, code := runProveArguments(t, append(append([]string{}, common...), "--attest-cem-v3")...)
	if code != 0 {
		t.Fatalf("prove --paired-evidence: exit %d stderr %s", code, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(string(stdout), "\n"), "\n")
	var envelope struct{ Payload string }
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &envelope); err != nil {
		t.Fatal(err)
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"evidenceSha256":"`+hex.EncodeToString(digest[:])+`"`) {
		t.Fatalf("cem/v3 statement lacks the paired range: %s", payload)
	}
	envelopePath := filepath.Join(t.TempDir(), "cem.dsse.json")
	if err := os.WriteFile(envelopePath, []byte(lines[len(lines)-1]), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code := runProveCLI(t, f.repo, "--verify-cem-attestation", envelopePath,
		"--attest-public-key", writeProvePublicKey(t, publicKey), "--cem", "stable.json", "--artifacts", f.artifacts, "--target", f.target)
	claim, _ := receipt["cem"].(map[string]any)
	pin, _ := claim["pairedRange"].(map[string]any)
	if code != 0 || pin["name"] != "tests" || pin["headRevision"] != strings.Repeat("2", 40) || pin["evidenceSha256"] != hex.EncodeToString(digest[:]) {
		t.Fatalf("verify paired cem/v3: exit %d receipt %v stderr %s", code, receipt, stderr)
	}
	stdout, stderr, code = runProveArguments(t, append(append([]string{}, common...), "--attest-cem-v2")...)
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "argument --paired-evidence: requires --attest-cem-v3") {
		t.Fatalf("--paired-evidence with cem/v2: exit %d stdout %q stderr %s", code, stdout, stderr)
	}
}
