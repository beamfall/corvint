package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/attest"
)

// inTreeCEMV2StdoutSha256 pins the unsigned `prove --cem stable.json
// --attest-cem-v2` stdout of the receipt-authentication case. It was observed
// identical at the CEM-V1-641 base (bebabe67e29f) and after it, so an in-tree
// map's two statements stay byte-identical (CEM-V1-641).
const inTreeCEMV2StdoutSha256 = "f74620a60613ce1bb8bea246584e5e8b007c11a64feda6c8a39de5de902b3367"

// TestCEMV1641ProveCEMSignsAnOutOfTreeMapByContent checks CEM-V1-641: prove
// --cem over an absolute out-of-tree map names it external:<sha256> in both
// statements, so two locations of the same bytes sign byte-identical output
// that carries no host path; the in-tree output is unchanged; the attestation
// verifier reads the absolute map too; and an absolute path inside the
// repository or a symlinked map refuses.
func TestCEMV1641ProveCEMSignsAnOutOfTreeMapByContent(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	_, inTree := proveStableCEM(t, f)
	if sum := sha256.Sum256(inTree); hex.EncodeToString(sum[:]) != inTreeCEMV2StdoutSha256 {
		t.Fatalf("in-tree stdout changed: sha256 %x\n%s", sum, inTree)
	}
	mapBytes, err := os.ReadFile(filepath.Join(f.repo, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	mapSum := sha256.Sum256(mapBytes)
	locator := "external:" + hex.EncodeToString(mapSum[:])
	keyPath, publicKey := proveTestKey(t)
	emit := func(directory string) []byte {
		t.Helper()
		path := filepath.Join(directory, "adopter.cem.json")
		if err := os.WriteFile(path, mapBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runProveArguments(t, "--root", f.repo, "prove", "--cem", path, "--expected-base", f.base,
			"--target", f.target, "--artifacts", f.artifacts, "--attest-key", keyPath, "--attest-cem-v2")
		if code != 0 || strings.Contains(string(stdout), directory) {
			t.Fatalf("external map %s: exit %d stderr %s stdout %s", path, code, stderr, stdout)
		}
		return stdout
	}
	first, second := t.TempDir(), t.TempDir()
	signed := emit(first)
	if again := emit(second); string(again) != string(signed) {
		t.Fatalf("the same map at two locations signed different bytes:\n%s\n%s", signed, again)
	}
	lines := strings.Split(strings.TrimSuffix(string(signed), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two envelopes: %s", signed)
	}
	for index, line := range lines {
		var envelope struct{ Payload string }
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatal(err)
		}
		payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
		if err != nil {
			t.Fatal(err)
		}
		var statement struct{ Subject []struct{ Name string } }
		if err := json.Unmarshal(payload, &statement); err != nil || len(statement.Subject) == 0 || statement.Subject[0].Name != locator {
			t.Fatalf("envelope %d subjects: %s", index, payload)
		}
	}
	envelopePath := filepath.Join(t.TempDir(), "cem.dsse.json")
	if err := os.WriteFile(envelopePath, []byte(lines[1]), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKeyPath := writeProvePublicKey(t, publicKey)
	receipt, _, stderr, code := runProveCLI(t, f.repo, "--verify-cem-attestation", envelopePath, "--attest-public-key", publicKeyPath,
		"--cem", filepath.Join(second, "adopter.cem.json"), "--artifacts", f.artifacts, "--target", f.target)
	if code != 0 || receipt["status"] != attest.CEMVerified || receipt["artifactsStatus"] != attest.CEMVerified {
		t.Fatalf("verify over the external map: exit %d receipt %v stderr %s", code, receipt, stderr)
	}

	inside := filepath.Join(f.repo, "stable.json")
	link := filepath.Join(t.TempDir(), "link.cem.json")
	if err := os.Symlink(filepath.Join(first, "adopter.cem.json"), link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{inside, link, filepath.Join(first, "missing.cem.json")} {
		stdout, stderr, code := runProveArguments(t, "--root", f.repo, "prove", "--cem", path, "--expected-base", f.base,
			"--target", f.target, "--artifacts", f.artifacts, "--attest-cem-v2")
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"code"`) {
			t.Fatalf("%s: exit %d stdout %q stderr %s", path, code, stdout, stderr)
		}
	}
}

// writeProvePublicKey saves publicKey as a PKIX PEM outside the repository.
func writeProvePublicKey(t *testing.T, publicKey ed25519.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "signer.pub")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
