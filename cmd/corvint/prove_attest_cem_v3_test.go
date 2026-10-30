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

	"github.com/Beamfall/corvint/internal/attest"
)

// TestCAXV0001ProveAttestCEMV3CountsAndCeilings checks CAX-V0-001 end to
// end: prove --attest-cem-v3 signs the cem/v3 predicate with the map's counts
// and exactly the applied ceilings, the verifier reports them, --attest-cem-v2
// output is unchanged, and the two flags are mutually exclusive.
func TestCAXV0001ProveAttestCEMV3CountsAndCeilings(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	if _, v2 := proveStableCEM(t, f); hexSum(v2) != inTreeCEMV2StdoutSha256 {
		t.Fatalf("cem/v2 stdout changed: %s", v2)
	}
	keyPath, publicKey := proveTestKey(t)
	common := []string{"--root", f.repo, "prove", "--cem", "stable.json", "--expected-base", f.base,
		"--target", f.target, "--artifacts", f.artifacts, "--attest-key", keyPath}
	stdout, stderr, code := runProveArguments(t, append(append([]string{}, common...), "--attest-cem-v3", "--max-unknown", "0")...)
	if code != 0 {
		t.Fatalf("prove --attest-cem-v3: exit %d stderr %s", code, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(string(stdout), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two envelopes: %s", stdout)
	}
	var envelope struct{ Payload string }
	if err := json.Unmarshal([]byte(lines[1]), &envelope); err != nil {
		t.Fatal(err)
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"predicateType":"`+attest.CEMPredicateTypeV3+`"`) ||
		!strings.Contains(string(payload), `"ceilings":{"maxUnknown":0},"counts":{"mechanical":0,"supported":2,"total":2,"unknown":0}`) {
		t.Fatalf("cem/v3 statement = %s", payload)
	}
	envelopePath := filepath.Join(t.TempDir(), "cem.dsse.json")
	if err := os.WriteFile(envelopePath, []byte(lines[1]), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code := runProveCLI(t, f.repo, "--verify-cem-attestation", envelopePath,
		"--attest-public-key", writeProvePublicKey(t, publicKey), "--cem", "stable.json", "--artifacts", f.artifacts, "--target", f.target)
	claim, _ := receipt["cem"].(map[string]any)
	counts, _ := claim["counts"].(map[string]any)
	ceilings, _ := claim["ceilings"].(map[string]any)
	if code != 0 || receipt["status"] != attest.CEMVerified || receipt["predicateType"] != attest.CEMPredicateTypeV3 ||
		counts["total"] != float64(2) || counts["supported"] != float64(2) || len(ceilings) != 1 || ceilings["maxUnknown"] != float64(0) {
		t.Fatalf("verify cem/v3: exit %d receipt %v stderr %s", code, receipt, stderr)
	}
	stdout, stderr, code = runProveArguments(t, append(append([]string{}, common...), "--attest-cem-v2", "--attest-cem-v3")...)
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "not allowed with argument --attest-cem-v2") {
		t.Fatalf("v2 and v3 together: exit %d stdout %q stderr %s", code, stdout, stderr)
	}
}

func hexSum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
