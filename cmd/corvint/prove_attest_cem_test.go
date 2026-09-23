package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
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

// emitCEMAttestation runs `prove --cem ... --attest-key PEM --attest-cem` on
// the CEM fixture with an in-test key and returns the repository, the CEM
// envelope (stdout line 2) saved outside it, and the matching public key PEM.
// The FPK-V0-015 proof envelope (line 1) is saved beside it as proof.dsse.json.
func emitCEMAttestation(t *testing.T) (root, envelopePath, publicKeyPath string) {
	t.Helper()
	root, base := proveCEMRepository(t)
	keyPath, publicKey := proveTestKey(t)
	stdout, stderr, code := runProveArguments(t, "--root", root, "prove", "--cem", ".corvint/change.cem.json",
		"--expected-base", base, "--target", "HEAD", "--attest-key", keyPath, "--attest-cem")
	lines := strings.Split(strings.TrimSuffix(string(stdout), "\n"), "\n")
	if code != 0 || len(lines) != 2 {
		t.Fatalf("emit exit %d, %d lines: %s", code, len(lines), stderr)
	}
	directory := t.TempDir()
	envelopePath = filepath.Join(directory, "cem.dsse.json")
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyPath = filepath.Join(directory, "signer.pub")
	if err := os.WriteFile(envelopePath, []byte(lines[1]), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "proof.dsse.json"), []byte(lines[0]), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, envelopePath, publicKeyPath
}

// TestProveCEMAttestOutputIsUnchangedWithoutAttestCEM pins FPK-V0-031's
// compatibility clause: without --attest-cem, --attest and --attest-key print
// exactly the FPK-V0-015 statement or envelope rebuilt here from the plain
// document, and with it that same line comes first, byte for byte.
func TestProveCEMAttestOutputIsUnchangedWithoutAttestCEM(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	common := []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD"}
	_, plain, stderr, code := runProveCLI(t, root, common...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(plain, &document); err != nil {
		t.Fatal(err)
	}
	statement, err := attest.Statement(bytes.TrimSuffix(plain, []byte("\n")), cemSubjects(".corvint/change.cem.json", mapBytes, document.Revision))
	if err != nil {
		t.Fatal(err)
	}
	keyPath, _ := proveTestKey(t)
	key, err := attest.ReadPrivateKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := attest.Envelope(statement, key)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		flags []string
		want  []byte
	}{
		{[]string{"--attest"}, statement},
		{[]string{"--attest-key", keyPath}, envelope},
	} {
		want := append(append([]byte{}, test.want...), '\n')
		_, stdout, stderr, code := runProveCLI(t, root, append(common, test.flags...)...)
		if code != 0 || !bytes.Equal(stdout, want) {
			t.Fatalf("%v: exit %d, output differs from FPK-V0-015 bytes: %s", test.flags, code, stderr)
		}
		withCEM, stderr, code := runProveArguments(t, append([]string{"--root", root, "prove"}, append(append(common, test.flags...), "--attest-cem")...)...)
		if code != 0 || !bytes.HasPrefix(withCEM, want) || bytes.Count(withCEM, []byte("\n")) != 2 {
			t.Fatalf("%v --attest-cem: exit %d, first line not the unchanged output: %s", test.flags, code, stderr)
		}
	}
}

// TestProveCEMAttestationRoundTripsThroughTheCLI checks FPK-V0-031 emission
// and verification: the second line verifies against the committed map with
// its independently computed digest and size, and the repository is unchanged.
func TestProveCEMAttestationRoundTripsThroughTheCLI(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	before := treeDigest(t, root)
	receipt, _, stderr, code := runProveCLI(t, root, "--verify-cem-attestation", envelopePath,
		"--attest-public-key", publicKeyPath, "--cem", ".corvint/change.cem.json")
	if code != 0 {
		t.Fatalf("verify exit %d: %s", code, stderr)
	}
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(mapBytes)
	claim, _ := receipt["cem"].(map[string]any)
	digest, _ := claim["digest"].(map[string]any)
	if receipt["status"] != attest.CEMVerified || receipt["predicateType"] != attest.CEMPredicateType ||
		digest["sha256"] != hex.EncodeToString(sum[:]) || claim["size"] != float64(len(mapBytes)) ||
		claim["name"] != ".corvint/change.cem.json" || receipt["reason"] != nil {
		t.Fatalf("verification receipt: %v", receipt)
	}
	if treeDigest(t, root) != before {
		t.Fatal("verification changed the repository")
	}
}

// TestProveCEMAttestationVerifyRefusesAChangedMap checks FPK-V0-031: map bytes
// that differ from the signed digest exit 2 with attest-cem-mismatch and print
// nothing.
func TestProveCEMAttestationVerifyRefusesAChangedMap(t *testing.T) {
	t.Parallel()
	root, envelopePath, publicKeyPath := emitCEMAttestation(t)
	mapPath := filepath.Join(root, ".corvint", "change.cem.json")
	mapBytes, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mapPath, append(mapBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stdout, stderr, code := runProveCLI(t, root, "--verify-cem-attestation", envelopePath,
		"--attest-public-key", publicKeyPath, "--cem", ".corvint/change.cem.json")
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
			"--attest-public-key", keyPath, "--cem", ".corvint/change.cem.json")
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
			"--attest-public-key", publicKeyPath, "--cem", ".corvint/change.cem.json")
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
		return base64.StdEncoding.EncodeToString(bytes.Replace(statement, []byte(`"size":`), []byte(`"size":1`), 1))
	})
	proofEnvelope := filepath.Join(filepath.Dir(envelopePath), "proof.dsse.json")
	for _, test := range [][2]string{
		{envelopePath, otherKeyPath}, {payloadType, publicKeyPath}, {payload, publicKeyPath}, {proofEnvelope, publicKeyPath},
	} {
		verifyCEMAttestationRefusal(t, root, "attest-verification-failed", "--verify-cem-attestation", test[0],
			"--attest-public-key", test[1], "--cem", ".corvint/change.cem.json")
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

// TestProveCEMAttestCEMRefusesInvalidArguments checks FPK-V0-031's emission
// grammar: --attest-cem with a value, repeated, or in impact mode (outside CEM
// mode) exits 2 with invalid-arguments and prints nothing.
func TestProveCEMAttestCEMRefusesInvalidArguments(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	common := []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD", "--attest"}
	for _, arguments := range [][]string{
		append(append([]string{}, common...), "--attest-cem=yes"),
		append(append([]string{}, common...), "--attest-cem", "--attest-cem"),
		{"--attest-cem", "docs/rule.txt"},
	} {
		verifyCEMAttestationRefusal(t, root, "invalid-arguments", arguments...)
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
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(root), "change.cem.json"), mapBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "oversized.cem.json"), bytes.Repeat([]byte(" "), wire.MaxMapBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mapPath := range []string{filepath.Join(root, ".corvint", "change.cem.json"), "../change.cem.json", "missing.cem.json", "oversized.cem.json"} {
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
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
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
	insideDir := filepath.Join(root, ".corvint", "inside")
	if err := os.Mkdir(insideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(insideDir, "change.cem.json"), mapBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	// Relative, so this symlink resolves entirely inside root: os.Root's own
	// containment (which refuses only an absolute or root-escaping symlink)
	// would follow it, so only the explicit per-component Lstat check refuses it.
	if err := os.Symlink(filepath.Join(".corvint", "inside"), filepath.Join(root, "inlink")); err != nil {
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

// TestProveCEMAttestationVerifiesAV1Predicate checks FPK-V0-034 at the CLI: a
// cem/v1 envelope verifies with its predicateType, baseRevision and
// patchSha256 in the receipt, VERIFIED with --cem and NOT_RUN without, while a
// cem/0 receipt carries neither new member.
func TestProveCEMAttestationVerifiesAV1Predicate(t *testing.T) {
	t.Parallel()
	root, _ := proveCEMRepository(t)
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseMap(mapBytes)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	statement, err := attest.CEMStatementV1(".corvint/change.cem.json", mapBytes)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := attest.Envelope(statement, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	envelopePath, publicKeyPath := filepath.Join(directory, "cem.dsse.json"), filepath.Join(directory, "signer.pub")
	if err := os.WriteFile(envelopePath, envelope, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		arguments []string
		status    string
	}{
		{[]string{"--cem", ".corvint/change.cem.json"}, attest.CEMVerified},
		{nil, attest.CEMNotRun},
	} {
		arguments := append([]string{"--verify-cem-attestation", envelopePath, "--attest-public-key", publicKeyPath}, test.arguments...)
		receipt, _, stderr, code := runProveCLI(t, root, arguments...)
		claim, _ := receipt["cem"].(map[string]any)
		if code != 0 || receipt["status"] != test.status || receipt["predicateType"] != attest.CEMPredicateTypeV1 ||
			claim["baseRevision"] != document.BaseRevision || claim["patchSha256"] != document.PatchSha256 {
			t.Fatalf("exit %d, receipt %v, stderr %s", code, receipt, stderr)
		}
	}
	v0Root, v0Envelope, v0PublicKey := emitCEMAttestation(t)
	_, stdout, stderr, code := runProveCLI(t, v0Root, "--verify-cem-attestation", v0Envelope,
		"--attest-public-key", v0PublicKey, "--cem", ".corvint/change.cem.json")
	if code != 0 || bytes.Contains(stdout, []byte("baseRevision")) || bytes.Contains(stdout, []byte("patchSha256")) {
		t.Fatalf("cem/0 receipt exit %d: %s %s", code, stdout, stderr)
	}
}
