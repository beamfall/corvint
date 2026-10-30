package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/attest"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// emitCEMAttestation runs `prove --attest-key PEM --attest-cem-v2` on the
// receipt-authentication case (CEM-V1-214: a cem/0 statement attests no map
// since CEM-V1-210) and returns the repository, the CEM envelope (stdout line
// 2) saved outside it, and the matching public key PEM. The map is
// stable.json at the repository root; the FPK-V0-015 proof envelope (line 1)
// is saved beside the CEM envelope as proof.dsse.json.
func emitCEMAttestation(t *testing.T) (root, envelopePath, publicKeyPath string) {
	t.Helper()
	f := materializeCEMV2Case(t)
	lines, envelopePath, publicKeyPath := emitCEMV2Attestation(t, f)
	if err := os.WriteFile(filepath.Join(filepath.Dir(envelopePath), "proof.dsse.json"), []byte(lines[0]), 0o600); err != nil {
		t.Fatal(err)
	}
	return f.repo, envelopePath, publicKeyPath
}

// TestProveCEMAttestationVerifyRefusesAChangedMap checks FPK-V0-031: map bytes
// that differ from the signed digest exit 2 with attest-cem-mismatch and print
// nothing.
func TestProveCEMAttestationVerifyRefusesAChangedMap(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	mapPath := filepath.Join(root, "stable.json")
	mapBytes, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mapPath, append(mapBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stdout, stderr, code := runProveCLI(t, root, "--verify-cem-attestation", envelopePath,
		"--attest-public-key", publicKeyPath, "--cem", "stable.json")
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"attest-cem-mismatch"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// TestProveCEMAttestationVerifyDisclosesMissingMapBytes checks FPK-V0-031:
// without --cem the signed claim is printed as NOT_RUN with its reason, never
// VERIFIED.
func TestProveCEMAttestationVerifyDisclosesMissingMapBytes(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	receipt, _, stderr, code := runProveCLI(t, root, "--verify-cem-attestation", envelopePath,
		"--attest-public-key", publicKeyPath)
	if code != 0 {
		t.Fatalf("verify exit %d: %s", code, stderr)
	}
	if receipt["status"] != attest.CEMNotRun || receipt["reason"] != "cem-bytes-not-supplied" {
		t.Fatalf("verification receipt: %v", receipt)
	}
}

// verifyCEMAttestationRefusal runs verify-cem mode and fails unless it exits 2
// with empty stdout and code on stderr, so no refusal prints a partial receipt.
func verifyCEMAttestationRefusal(t *testing.T, root, code string, arguments ...string) {
	t.Helper()
	_, stdout, stderr, exit := runProveCLI(t, root, arguments...)
	if exit != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"`+code+`"`) {
		t.Fatalf("%v: exit=%d stdout=%q stderr=%q, want %s", arguments, exit, stdout, stderr, code)
	}
}

// TestProveCEMAttestationVerifyRefusesAnUnusablePublicKey checks FPK-V0-031: a
// missing key file, a private-key PEM, and a key file over 16 KiB exit 2 with
// attest-public-key-unavailable.
func TestProveCEMAttestationVerifyRefusesAnUnusablePublicKey(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	privateKeyPath, _ := proveTestKey(t)
	publicPEM, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(t.TempDir(), "oversized.pub")
	if err := os.WriteFile(oversized, append(publicPEM, bytes.Repeat([]byte("\n"), 16*1024)...), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, keyPath := range []string{filepath.Join(t.TempDir(), "missing.pub"), privateKeyPath, oversized} {
		verifyCEMAttestationRefusal(t, root, "attest-public-key-unavailable", "--verify-cem-attestation", envelopePath,
			"--attest-public-key", keyPath, "--cem", "stable.json")
	}
}

// TestProveCEMAttestationVerifyRefusesAnUnreadableEnvelope checks FPK-V0-031: a
// missing envelope, and the valid envelope padded with JSON whitespace past
// 8 MiB, exit 2 with attest-envelope-unavailable.
func TestProveCEMAttestationVerifyRefusesAnUnreadableEnvelope(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	envelope, err := os.ReadFile(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(t.TempDir(), "oversized.dsse.json")
	padding := bytes.Repeat([]byte(" "), maxAttestationEnvelopeBytes+1-len(envelope))
	if err := os.WriteFile(oversized, append(envelope, padding...), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(t.TempDir(), "missing.dsse.json"), oversized} {
		verifyCEMAttestationRefusal(t, root, "attest-envelope-unavailable", "--verify-cem-attestation", path,
			"--attest-public-key", publicKeyPath, "--cem", "stable.json")
	}
}

// TestProveCEMAttestationVerifyNamesTheRetiredPredicate checks CEM-V1-218 at
// the CLI: a cem/0 or cem/v1 envelope that verifies under the key exits 2 with
// attest-verification-failed naming cem/v2 as the only CEM predicate, with or
// without --cem.
func TestProveCEMAttestationVerifyNamesTheRetiredPredicate(t *testing.T) {
	t.Parallel()
	root, _, _ := emitCEMAttestation(t)
	directory := t.TempDir()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyPath := filepath.Join(directory, "signer.pub")
	if err := os.WriteFile(publicKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, predicateType := range []string{"https://corvint-context.dev/attestation/cem/0", "https://corvint-context.dev/attestation/cem/v1"} {
		statement, err := json.Marshal(map[string]any{
			"_type":         "https://in-toto.io/Statement/v1",
			"subject":       []any{map[string]any{"name": "stable.json", "digest": map[string]any{"sha256": strings.Repeat("0", 64)}}},
			"predicateType": predicateType,
			"predicate":     map[string]any{"name": "stable.json"},
		})
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := attest.Envelope(statement, privateKey)
		if err != nil {
			t.Fatal(err)
		}
		envelopePath := filepath.Join(directory, "retired.dsse.json")
		if err := os.WriteFile(envelopePath, envelope, 0o600); err != nil {
			t.Fatal(err)
		}
		for _, extra := range [][]string{nil, {"--cem", "stable.json"}} {
			arguments := append([]string{"--verify-cem-attestation", envelopePath, "--attest-public-key", publicKeyPath}, extra...)
			_, stdout, stderr, exit := runProveCLI(t, root, arguments...)
			if exit != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"attest-verification-failed"`) ||
				!strings.Contains(stderr, attest.CEMPredicateTypeV2+" is the only CEM predicate") {
				t.Fatalf("%s %v: exit=%d stdout=%q stderr=%q", predicateType, extra, exit, stdout, stderr)
			}
		}
	}
}

// TestProveCEMAttestationVerifyRefusesAnEnvelopeThatDoesNotVerify checks
// FPK-V0-031: another signer's key, a changed payloadType, a changed payload,
// and the FPK-V0-015 proof envelope signed by the same key (wrong
// predicateType) exit 2 with attest-verification-failed, never VERIFIED.
func TestProveCEMAttestationVerifyRefusesAnEnvelopeThatDoesNotVerify(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	directory := t.TempDir()
	otherPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherDER, err := x509.MarshalPKIXPublicKey(otherPublic)
	if err != nil {
		t.Fatal(err)
	}
	otherKeyPath := filepath.Join(directory, "other.pub")
	if err := os.WriteFile(otherKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: otherDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	altered := func(name, member string, change func(string) string) string {
		var envelope map[string]any
		if err := json.Unmarshal(original, &envelope); err != nil {
			t.Fatal(err)
		}
		envelope[member] = change(envelope[member].(string))
		encoded, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	payloadType := altered("payload-type.dsse.json", "payloadType", func(string) string { return "application/json" })
	payload := altered("payload.dsse.json", "payload", func(encoded string) string {
		statement, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		return base64.StdEncoding.EncodeToString(bytes.Replace(statement, []byte("{"), []byte("{ "), 1))
	})
	proofEnvelope := filepath.Join(filepath.Dir(envelopePath), "proof.dsse.json")
	for _, test := range [][2]string{
		{envelopePath, otherKeyPath}, {payloadType, publicKeyPath}, {payload, publicKeyPath}, {proofEnvelope, publicKeyPath},
	} {
		verifyCEMAttestationRefusal(t, root, "attest-verification-failed", "--verify-cem-attestation", test[0],
			"--attest-public-key", test[1], "--cem", "stable.json")
	}
}

// TestProveCEMAttestationVerifyRefusesInvalidArguments checks FPK-V0-031's
// verify-mode grammar: a missing, empty, repeated, or unknown flag exits 2 with
// invalid-arguments before any file is read.
func TestProveCEMAttestationVerifyRefusesInvalidArguments(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	for _, arguments := range [][]string{
		{"--verify-cem-attestation", envelopePath},
		{"--verify-cem-attestation", envelopePath, "--attest-public-key", publicKeyPath, "--cem="},
		{"--verify-cem-attestation", envelopePath, "--attest-public-key", publicKeyPath, "--attest-public-key", publicKeyPath},
		{"--verify-cem-attestation", envelopePath, "--attest-public-key", publicKeyPath, "--attest-key", publicKeyPath},
	} {
		verifyCEMAttestationRefusal(t, root, "invalid-arguments", arguments...)
	}
}

// TestCEMV1218RetiredAttestationFlagsAreUnrecognized checks CEM-V1-218 and the
// amended FPK-V0-031/050 grammar: the retired --attest-cem and --attest-cem-v1
// flags, bare, with a value, repeated, beside each other or outside CEM mode,
// exit 2 with invalid-arguments naming the flag and print nothing. Only
// --attest-cem-v2 attests a map.
func TestCEMV1218RetiredAttestationFlagsAreUnrecognized(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	common := []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD", "--attest"}
	with := func(extra ...string) []string { return append(append([]string{}, common...), extra...) }
	for _, arguments := range [][]string{
		with("--attest-cem"),
		with("--attest-cem=yes"),
		with("--attest-cem", "--attest-cem"),
		with("--attest-cem-v1"),
		with("--attest-cem-v1=yes"),
		with("--attest-cem-v1", "--attest-cem-v1"),
		with("--attest-cem", "--attest-cem-v1"),
		{"--attest-cem", "docs/rule.txt"},
		{"--attest-cem-v1", "docs/rule.txt"},
	} {
		verifyCEMAttestationRefusal(t, root, "invalid-arguments", arguments...)
		if _, _, stderr, _ := runProveCLI(t, root, arguments...); !strings.Contains(stderr, "unrecognized arguments: --attest-cem") {
			t.Fatalf("%v: refusal names no retired flag: %s", arguments, stderr)
		}
	}
}

// TestProveCEMAttestationVerifyRefusesAnUnavailableMap checks FPK-V0-031: a
// --cem path that is absolute, climbs out of the root, is missing, or exceeds
// the CEM map bound exits 2 with map-unavailable and prints nothing. The
// signed map is copied to where the climbing path points, so only the path
// check refuses it.
func TestProveCEMAttestationVerifyRefusesAnUnavailableMap(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	mapBytes, err := os.ReadFile(filepath.Join(root, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(root), "change.cem.json"), mapBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "oversized.cem.json"), bytes.Repeat([]byte(" "), wire.MaxMapBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mapPath := range []string{filepath.Join(root, "stable.json"), "../change.cem.json", "missing.cem.json", "oversized.cem.json"} {
		verifyCEMAttestationRefusal(t, root, "map-unavailable", "--verify-cem-attestation", envelopePath,
			"--attest-public-key", publicKeyPath, "--cem", mapPath)
	}
}

// TestProveCEMAttestationVerifyRefusesASymlinkedOrCaseFoldedGitMap checks
// FPK-V0-031's readAttestedMap: a --cem path is refused, before any read,
// when a path component under --root is itself a symlink (even one that
// resolves back inside root) or case-folds equal to .git, mirroring the
// prove --cem refusal.
func TestProveCEMAttestationVerifyRefusesASymlinkedOrCaseFoldedGitMap(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	mapBytes, err := os.ReadFile(filepath.Join(root, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "change.cem.json"), mapBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(root, "outlink")); err != nil {
		t.Fatal(err)
	}
	insideDir := filepath.Join(root, "inside")
	if err := os.Mkdir(insideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(insideDir, "change.cem.json"), mapBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	// Relative, so this symlink resolves entirely inside root: os.Root's own
	// containment (which refuses only an absolute or root-escaping symlink)
	// would follow it, so only the explicit per-component Lstat check refuses it.
	if err := os.Symlink("inside", filepath.Join(root, "inlink")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "corvint-secret.txt"), mapBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mapPath := range []string{
		"outlink/change.cem.json", // symlinked parent, resolves outside root
		"inlink/change.cem.json",  // symlinked parent, resolves inside root
		".GIT/corvint-secret.txt", // case-folds equal to .git
	} {
		verifyCEMAttestationRefusal(t, root, "map-unavailable", "--verify-cem-attestation", envelopePath,
			"--attest-public-key", publicKeyPath, "--cem", mapPath)
	}
}

// TestProveCEMAttestFailsWhenTheCEMStatementCannotBeBuilt checks FPK-V0-031's
// attest-failed refusal through attestProof, the only seam: the CLI reaches it
// only with a map the CEM verifier already accepted, so an empty name and
// non-CEM bytes are supplied directly, and no output is returned. The proof
// document itself is attestable, so the refusal comes from the CEM statement.
func TestProveCEMAttestFailsWhenTheCEMStatementCannotBeBuilt(t *testing.T) {
	t.Parallel()
	document := []byte(`{"profile":"falsifiable-packet/0","revision":"` + strings.Repeat("a", 40) + `"}`)
	subjects := cemSubjects(".corvint/change.cem.json", []byte(`{}`), strings.Repeat("a", 40))
	if _, err := attestProof(document, subjects, nil, ""); err != nil {
		t.Fatalf("attestProof without a CEM statement: %v", err)
	}
	for name, cem := range map[string]*cemAttestation{
		"empty name":    {name: "", bytes: []byte(`{}`)},
		"non-CEM bytes": {name: ".corvint/change.cem.json", bytes: []byte(`{"not":"a cem"}`)},
	} {
		output, err := attestProof(document, subjects, cem, "")
		var refusal *gokernel.Error
		if !errors.As(err, &refusal) || refusal.Code != "attest-failed" || output != nil {
			t.Errorf("%s: attestProof = %q, %v; want attest-failed and no output", name, output, err)
		}
	}
}
