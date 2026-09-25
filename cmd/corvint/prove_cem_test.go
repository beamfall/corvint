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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/attest"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// proveCEMRepository is the CEM end-to-end fixture: one committed change, a
// cem/0.2 map prepared for it, hunk 1 cited to committed evidence, and the map
// committed so the target-side sidecar check can bind it.
func proveCEMRepository(t *testing.T) (root, base string) {
	t.Helper()
	root, base, target := cemRepo(t)
	if code, _, stderr := runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", target); code != 0 {
		t.Fatalf("prepare: %s", stderr)
	}
	if code, _, stderr := runCLI(t, "--root", root, "cem", "cite",
		"--map", ".corvint/change.cem.json", "--hunk", "1",
		"--evidence-path", "docs/rule.txt", "--lines", "1:1", "--relation", "specification"); code != 0 {
		t.Fatalf("cite: %s", stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "candidate")
	return root, base
}

func TestProveCEMAcceptedMapProvesSupportedHunksAndWritesNothing(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	before := treeDigest(t, root)
	arguments := []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD"}
	receipt, first, stderr, code := runProveCLI(t, root, arguments...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if receipt["state"] != "READY" || receipt["mutates"] != false || receipt["profile"] != proveProfile {
		t.Fatalf("envelope: %v", receipt)
	}
	rows := proofRows(t, receipt)
	if len(rows) != 1 {
		t.Fatalf("one hunk, %d rows: %v", len(rows), rows)
	}
	row := rows[0]
	if row["falsifier"] != falsifierVerifier || row["falsified"] != falsifiedPass || row["authority"] != "cem-supported" ||
		row["path"] != "src/app.txt" || !strings.HasPrefix(stringAt(row, "result"), "hunk:sha256:") || stringAt(row, "blob_hash") == "" {
		t.Fatalf("row: %v", row)
	}
	if proofField(t, receipt, "proven_results") != 1 || proofField(t, receipt, "failed_results") != 0 {
		t.Fatalf("proof: %v", receipt["proof"])
	}
	assertProofClaims(t, receipt)
	verifyCode, verifyStdout, verifyStderr := runCLI(t, "--root", root, "cem", "verify", "--map", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD")
	if verifyCode != 0 {
		t.Fatalf("cem verify: %d %s", verifyCode, verifyStderr)
	}
	var verifyEnvelope map[string]any
	if err := json.Unmarshal([]byte(verifyStdout), &verifyEnvelope); err != nil {
		t.Fatal(err)
	}
	embedded, _ := json.Marshal(receipt["packet"])
	expected, _ := json.Marshal(verifyEnvelope)
	if string(embedded) != string(expected) {
		t.Fatalf("embedded document differs from cem verify:\n%s\n%s", embedded, expected)
	}
	if !wire.IsGitOid(stringAt(receipt, "revision")) {
		t.Fatalf("revision %v", receipt["revision"])
	}
	if _, second, _, _ := runProveCLI(t, root, arguments...); string(first) != string(second) {
		t.Fatal("two runs over one tree differ")
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove --cem changed the repository")
	}
}

func TestProveCEMRejectedMapFailsEveryClaimRow(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	receipt, _, stderr, code := runProveCLI(t, root, "--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if receipt["state"] != "REJECTED" || proofField(t, receipt, "failed_results") != 1 || proofField(t, receipt, "proven_results") != 0 {
		t.Fatalf("state %v proof %v", receipt["state"], receipt["proof"])
	}
	if row := proofRows(t, receipt)[0]; row["falsified"] != falsifiedFail {
		t.Fatalf("row: %v", row)
	}
}

func TestProveCEMUnknownHunkIsNotAClaim(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	if code, out, _ := runCLI(t, "--root", root, "cem", "mark",
		"--map", ".corvint/change.cem.json", "--hunk", "1", "--disposition", "unknown", "--reason", "insufficient-evidence"); code != 0 {
		t.Fatalf("mark: %s", out)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "unknown")
	receipt, _, stderr, code := runProveCLI(t, root, "--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	row := proofRows(t, receipt)[0]
	if row["falsifier"] != falsifierNone || row["falsified"] != falsifiedNotRun || row["authority"] != "cem-unknown" {
		t.Fatalf("row: %v", row)
	}
	if receipt["state"] != "UNPROVEN" || proofField(t, receipt, "unproven_results") != 1 {
		t.Fatalf("state %v proof %v", receipt["state"], receipt["proof"])
	}
}

func TestProveCEMRejectsBadArgumentsWithoutOutputOrWrite(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	before := treeDigest(t, root)
	for _, test := range []struct {
		arguments []string
		code      string
	}{
		{[]string{"--cem"}, "invalid-arguments"},
		{[]string{"--cem", ".corvint/change.cem.json", "--bogus", "x"}, "invalid-arguments"},
		{[]string{"--cem", ".corvint/change.cem.json", "--target", "HEAD", "--target", "HEAD"}, "invalid-arguments"},
		{[]string{"--cem", ".corvint/change.cem.json", "--target", "HEAD"}, "expected-base-required"},
		{[]string{"--cem", ".corvint/change.cem.json", "--expected-base", base}, "target-required"},
		{[]string{"--cem", ".corvint/missing.cem.json", "--expected-base", base, "--target", "HEAD"}, "map-unavailable"},
		{[]string{"--cem", "../outside.cem.json", "--expected-base", base, "--target", "HEAD"}, "map-unavailable"},
		{[]string{"--cem", filepath.Join(root, ".corvint", "change.cem.json"), "--expected-base", base, "--target", "HEAD"}, "map-unavailable"},
	} {
		_, stdout, stderr, code := runProveCLI(t, root, test.arguments...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, test.code) {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", test.arguments, code, stdout, stderr)
		}
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("a rejected prove --cem invocation wrote to the repository")
	}
}

// TestProveCEMRefusesAMapPathThroughASymlinkedParentOrCaseFoldedGit checks
// that a --cem path is refused, before any read, when a path component that
// exists under root is itself a symlink (even one that resolves back inside
// root) or when a component case-folds equal to .git: FPK-V0-012's lexical
// "repository-relative" check alone would let a symlinked parent or a
// case-insensitive filesystem's `.GIT` escape it.
func TestProveCEMRefusesAMapPathThroughASymlinkedParentOrCaseFoldedGit(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	outsideDir := t.TempDir()
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
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
	// treeDigest is not used here: it walks and reads every file under root,
	// and os.ReadFile on a directory symlink (the fixture this test needs)
	// fails, so the no-write check is the exit/stdout/stderr assertion below.
	for _, mapPath := range []string{
		"outlink/change.cem.json",              // symlinked parent, resolves outside root
		"inlink/change.cem.json",               // symlinked parent, resolves inside root
		".GIT/corvint-secret.txt",              // case-folds equal to .git
		filepath.Join(".corvint", ".Git", "x"), // case-folds equal to .git mid-path (nonexistent, refused lexically)
	} {
		_, stdout, stderr, code := runProveCLI(t, root, "--cem", mapPath, "--expected-base", base, "--target", "HEAD")
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "map-unavailable") {
			t.Errorf("%s: exit=%d stdout=%q stderr=%q, want map-unavailable", mapPath, code, stdout, stderr)
		}
	}
}

func TestJudgeVerifierVerdicts(t *testing.T) {
	t.Parallel()
	supported := proveRow{Falsifier: falsifierVerifier, basis: []string{"evidence:1"}}
	accepted := map[string]any{"valid": true, "drift": []any{}}
	stale := map[string]any{"valid": true, "drift": []any{map[string]any{"evidenceId": "evidence:1", "status": "stale"}}}
	relocated := map[string]any{"valid": true, "drift": []any{map[string]any{"evidenceId": "evidence:1", "status": "relocated"}}}
	rejected := map[string]any{"valid": false, "drift": []any{}}
	for _, test := range []struct {
		name         string
		row          proveRow
		verification map[string]any
		want         string
	}{
		{"accepted", supported, accepted, falsifiedPass},
		{"relocated evidence", supported, relocated, falsifiedPass},
		{"stale evidence", supported, stale, falsifiedFail},
		{"rejected map", supported, rejected, falsifiedFail},
		{"unknown hunk", proveRow{Falsifier: falsifierNone}, accepted, falsifiedNotRun},
	} {
		if got := judgeVerifier(test.row, test.verification); got != test.want {
			t.Errorf("%s: got %s want %s", test.name, got, test.want)
		}
	}
	for authority, want := range map[string]string{"cem-supported": falsifierVerifier, "cem-mechanical": falsifierVerifier, "cem-unknown": falsifierNone, "cem-other": falsifierNone} {
		if got := falsifierFor(proveRow{Kind: "cem-hunk", Authority: authority, Path: "src/app.txt"}); got != want {
			t.Errorf("%s: got %s want %s", authority, got, want)
		}
	}
}

func proveTestKey(t *testing.T) (string, ed25519.PublicKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "signer.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, publicKey
}

func TestProveCEMAttestWrapsTheDocumentAndSignsIt(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	before := treeDigest(t, root)
	common := []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD"}
	plain, plainStdout, stderr, code := runProveCLI(t, root, common...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	statement, statementStdout, stderr, code := runProveCLI(t, root, append(common, "--attest")...)
	if code != 0 {
		t.Fatalf("attest exit %d: %s", code, stderr)
	}
	if statement["_type"] != "https://in-toto.io/Statement/v1" || statement["predicateType"] != attest.PredicateType {
		t.Fatalf("statement: %v", statement)
	}
	predicate, _ := json.Marshal(statement["predicate"])
	document, _ := json.Marshal(plain)
	if string(predicate) != string(document) {
		t.Fatalf("predicate differs from the plain document:\n%s\n%s", predicate, document)
	}
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	mapDigest := sha256.Sum256(mapBytes)
	head := strings.TrimSpace(cemGit(t, root, "rev-parse", "HEAD"))
	subjects, _ := json.Marshal(statement["subject"])
	want := `[{"digest":{"sha256":"` + hex.EncodeToString(mapDigest[:]) + `"},"name":".corvint/change.cem.json"},{"digest":{"gitCommit":"` + head + `"},"name":"git:` + head + `"}]`
	if string(subjects) != want {
		t.Fatalf("subjects:\n%s\n%s", subjects, want)
	}
	if stringAt(plain, "revision") != head {
		t.Fatalf("revision %v is not HEAD %s", plain["revision"], head)
	}
	keyPath, publicKey := proveTestKey(t)
	_, envelope, stderr, code := runProveCLI(t, root, append(common, "--attest-key", keyPath)...)
	if code != 0 {
		t.Fatalf("attest-key exit %d: %s", code, stderr)
	}
	payload, err := attest.Verify(bytes.TrimSuffix(envelope, []byte("\n")), publicKey)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if string(payload) != string(bytes.TrimSuffix(statementStdout, []byte("\n"))) {
		t.Fatal("signed payload differs from the unsigned statement")
	}
	if len(plainStdout) == 0 || treeDigest(t, root) != before {
		t.Fatal("attestation changed the repository")
	}
}

// TestProveCEMAttestVerifiesOfflineWithCosignSemantics independently checks
// FPK-V0-015's public-key DSSE/in-toto contract without the Corvint verifier.
func TestProveCEMAttestVerifiesOfflineWithCosignSemantics(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	privateKeyPath, publicKey := proveTestKey(t)
	publicKeyPath := filepath.Join(t.TempDir(), "cosign.pub")
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o644); err != nil {
		t.Fatal(err)
	}
	arguments := []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD", "--attest-key", privateKeyPath}
	_, envelope, stderr, code := runProveCLI(t, root, arguments...)
	if code != 0 {
		t.Fatalf("attest-key exit %d: %s", code, stderr)
	}
	payload, err := verifyOfflineCosignEnvelope(bytes.TrimSpace(envelope), publicKeyPath)
	if err != nil {
		t.Fatalf("offline verify: %v", err)
	}
	var statement map[string]any
	if err := json.Unmarshal(payload, &statement); err != nil {
		t.Fatal(err)
	}
	if statement["_type"] != "https://in-toto.io/Statement/v1" || statement["predicateType"] != attest.PredicateType {
		t.Fatalf("in-toto statement: %v", statement)
	}

	var tampered map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(envelope), &tampered); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(stringAt(tampered, "payload"))
	if err != nil || len(decoded) == 0 {
		t.Fatalf("decode payload: %v", err)
	}
	decoded[len(decoded)/2] ^= 1
	tampered["payload"] = base64.StdEncoding.EncodeToString(decoded)
	tamperedEnvelope, err := json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyOfflineCosignEnvelope(tamperedEnvelope, publicKeyPath); err == nil {
		t.Fatal("offline verifier accepted a one-byte-tampered payload")
	}
}

func verifyOfflineCosignEnvelope(envelope []byte, publicKeyPath string) ([]byte, error) {
	var document struct {
		Payload     string `json:"payload"`
		PayloadType string `json:"payloadType"`
		Signatures  []struct {
			Sig string `json:"sig"`
		} `json:"signatures"`
	}
	if err := json.Unmarshal(envelope, &document); err != nil {
		return nil, err
	}
	if document.PayloadType != "application/vnd.in-toto+json" || len(document.Signatures) != 1 {
		return nil, fmt.Errorf("not one DSSE in-toto signature")
	}
	payload, err := base64.StdEncoding.DecodeString(document.Payload)
	if err != nil {
		return nil, err
	}
	signature, err := base64.StdEncoding.DecodeString(document.Signatures[0].Sig)
	if err != nil {
		return nil, err
	}
	publicPEM, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(publicPEM)
	if block == nil || block.Type != "PUBLIC KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, fmt.Errorf("not one PKIX public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an Ed25519 public key")
	}
	pae := []byte("DSSEv1 " + strconv.Itoa(len(document.PayloadType)) + " " + document.PayloadType + " " + strconv.Itoa(len(payload)) + " ")
	pae = append(pae, payload...)
	if !ed25519.Verify(publicKey, pae, signature) {
		return nil, fmt.Errorf("DSSE signature does not verify")
	}
	return payload, nil
}

func TestProveCEMAttestRejectsBadFlagsAndKeys(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	common := []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD"}
	bogusKey := filepath.Join(t.TempDir(), "bogus.pem")
	if err := os.WriteFile(bogusKey, []byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		arguments []string
		code      string
	}{
		{append(common, "--attest=yes"), "invalid-arguments"},
		{append(common, "--attest", "--attest"), "invalid-arguments"},
		{append(common, "--attest-key"), "invalid-arguments"},
		{append(common, "--attest-key="), "invalid-arguments"},
		{append(common, "--attest-key", filepath.Join(t.TempDir(), "missing.pem")), "attest-key-unavailable"},
		{append(common, "--attest-key", bogusKey), "attest-key-unavailable"},
	} {
		_, stdout, stderr, code := runProveCLI(t, root, test.arguments...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, test.code) {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", test.arguments, code, stdout, stderr)
		}
	}
}

// TestProveCEMAttestRefusesAMapPathThatIsNotUTF8 checks FPK-V0-015's
// canonical-subject refusal: an accepted map whose path bytes are not valid
// UTF-8 is exit 2 `attest-failed` with nothing on stdout, never a subject name
// with U+FFFD. Where the filesystem refuses such a name (APFS returns EILSEQ),
// the CLI cannot open the map, so the refusal is checked through attestProof,
// the seam runProve calls, with the real prove document for the same map.
func TestProveCEMAttestRefusesAMapPathThatIsNotUTF8(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	name := ".corvint/change-\xff.cem.json"
	common := []string{"--expected-base", base, "--target", "HEAD", "--attest"}
	err = os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), mapBytes, 0o644)
	if err == nil {
		// The file system accepted the name, so the map is itself a non-UTF-8 untracked path.
		// The worktree status read discloses it in display form (V1-0314), so the CLI
		// reaches attestation, which refuses the subject name.
		_, stdout, stderr, code := runProveCLI(t, root, append([]string{"--cem", name}, common...)...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"attest-failed"`) {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		return
	}
	if !errors.Is(err, syscall.EILSEQ) {
		t.Fatal(err)
	}
	plain, document, stderr, code := runProveCLI(t, root, "--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	document = bytes.TrimSuffix(document, []byte("\n"))
	revision := stringAt(plain, "revision")
	if _, err := attestProof(document, cemSubjects(".corvint/change.cem.json", mapBytes, revision), nil, ""); err != nil {
		t.Fatalf("attestProof with a UTF-8 map path: %v", err)
	}
	output, err := attestProof(document, cemSubjects(name, mapBytes, revision), nil, "")
	var refusal *gokernel.Error
	if !errors.As(err, &refusal) || refusal.Code != "attest-failed" || output != nil {
		t.Fatalf("attestProof = %q, %v; want attest-failed and no output", output, err)
	}
}
