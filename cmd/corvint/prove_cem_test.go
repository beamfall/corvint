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

// proveCEMRepository is the CEM argument fixture: one committed change, a
// cem/1.0 map prepared for it (CEM-V1-214), hunk 1 cited to committed
// evidence, and the map committed. Only refusals run over it; an accepted
// proof runs over the receipt-authentication case (materializeCEMV2Case).
func proveCEMRepository(t *testing.T) (root, base string) {
	t.Helper()
	root, base, target := cemRepo(t)
	if code, _, stderr := runCLI(t, "--root", root, "cem", "prepare", "--spec", wire.StableSpec, "--base", base, "--target", target); code != 0 {
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

// proveStableCEM runs `prove --attest-cem-v2` unsigned over the
// receipt-authentication case and returns the prove document (the FPK-V0-015
// statement's predicate) and the raw stdout. A cem/1.0 map is judged only
// through the stable verifier (CEM-V1-210, CEM-V1-214).
func proveStableCEM(t *testing.T, f cemV2Fixture, extra ...string) (map[string]any, []byte) {
	t.Helper()
	arguments := append([]string{"--root", f.repo, "prove", "--cem", "stable.json", "--expected-base", f.base,
		"--target", f.target, "--artifacts", f.artifacts, "--attest-cem-v2"}, extra...)
	stdout, stderr, code := runProveArguments(t, arguments...)
	lines := strings.Split(strings.TrimSuffix(string(stdout), "\n"), "\n")
	if code != 0 || len(lines) != 2 {
		t.Fatalf("prove exit %d, %d lines: %s", code, len(lines), stderr)
	}
	var statement struct {
		Predicate map[string]any `json:"predicate"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &statement); err != nil {
		t.Fatal(err)
	}
	return statement.Predicate, stdout
}

func TestProveCEMAcceptedMapProvesSupportedHunksAndWritesNothing(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	before := treeDigest(t, f.repo)
	receipt, first := proveStableCEM(t, f)
	if receipt["state"] != "READY" || receipt["mutates"] != false || receipt["profile"] != proveProfile {
		t.Fatalf("envelope: %v", receipt)
	}
	rows := proofRows(t, receipt)
	if len(rows) != 2 {
		t.Fatalf("two hunks, %d rows: %v", len(rows), rows)
	}
	for _, row := range rows {
		if row["falsifier"] != falsifierVerifier || row["falsified"] != falsifiedPass || row["authority"] != "cem-supported" ||
			!strings.HasPrefix(stringAt(row, "result"), "hunk:sha256:") || stringAt(row, "blob_hash") == "" {
			t.Fatalf("row: %v", row)
		}
	}
	if proofField(t, receipt, "proven_results") != 2 || proofField(t, receipt, "failed_results") != 0 {
		t.Fatalf("proof: %v", receipt["proof"])
	}
	assertProofClaims(t, receipt)
	verifyCode, verifyStdout, verifyStderr := runCLI(t, "--root", f.repo, "cem", "verify", "--map", "stable.json",
		"--expected-base", f.base, "--target", f.target, "--artifacts", f.artifacts)
	if verifyCode != 0 {
		t.Fatalf("cem verify: %d %s", verifyCode, verifyStderr)
	}
	var verifyEnvelope map[string]any
	if err := json.Unmarshal([]byte(verifyStdout), &verifyEnvelope); err != nil {
		t.Fatal(err)
	}
	if embedded, expected := mustJSON(t, receipt["packet"]), mustJSON(t, verifyEnvelope); string(embedded) != string(expected) {
		t.Fatalf("embedded document differs from cem verify:\n%s\n%s", embedded, expected)
	}
	if stringAt(receipt, "revision") != f.target {
		t.Fatalf("revision %v is not the target %s", receipt["revision"], f.target)
	}
	if _, second := proveStableCEM(t, f); string(first) != string(second) {
		t.Fatal("two runs over one tree differ")
	}
	if after := treeDigest(t, f.repo); after != before {
		t.Fatal("prove --cem changed the repository")
	}
}

// TestCEMV1210ProveRefusesEveryMapOutsideTheStableRoute checks CEM-V1-210 for
// prove: without --attest-cem-v2 a cem/1.0 map, and a map relabelled to any
// retired cem/0.x spec, exits 2 with unsupported-spec and an empty stdout; a
// 0.x refusal carries the recovery line and the 1.0 refusal names the stable
// route. The retired --attest-cem and --attest-cem-v1 flags are unrecognized
// arguments before any map is read (CEM-V1-218). A stable map the
// verifier does not accept is refused with attest-cem-v2-not-accepted.
func TestCEMV1210ProveRefusesEveryMapOutsideTheStableRoute(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, root)
	common := []string{"--expected-base", base, "--target", "HEAD"}
	for _, spec := range []string{"cem/0.1", "cem/0.2", "cem/0.3", wire.StableSpec} {
		name := ".corvint/relabelled-" + strings.ReplaceAll(spec, "/", "-") + ".cem.json"
		relabelled := bytes.Replace(mapBytes, []byte(`"spec":"`+wire.StableSpec+`"`), []byte(`"spec":"`+spec+`"`), 1)
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), relabelled, 0o644); err != nil {
			t.Fatal(err)
		}
		want := wire.RetiredSpecFix
		if spec == wire.StableSpec {
			want = "prove judges a cem/1.0 map only with --attest-cem-v2"
		}
		for _, flags := range [][]string{nil, {"--attest"}} {
			arguments := append(append([]string{"--cem", name}, common...), flags...)
			_, stdout, stderr, code := runProveCLI(t, root, arguments...)
			if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"unsupported-spec"`) || !strings.Contains(stderr, want) {
				t.Fatalf("%v: exit=%d stdout=%q stderr=%q", arguments, code, stdout, stderr)
			}
		}
		for _, flag := range []string{"--attest-cem", "--attest-cem-v1"} {
			arguments := append(append([]string{"--cem", name}, common...), flag)
			_, stdout, stderr, code := runProveCLI(t, root, arguments...)
			if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"invalid-arguments"`) || !strings.Contains(stderr, "unrecognized arguments: "+flag) {
				t.Fatalf("%v: exit=%d stdout=%q stderr=%q", arguments, code, stdout, stderr)
			}
		}
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("a refused prove --cem invocation wrote to the repository")
	}
	f := materializeCEMV2Case(t)
	_, stdout, stderr, code := runProveCLI(t, f.repo, "--cem", "stable.json", "--expected-base", f.base, "--target", f.base,
		"--artifacts", f.artifacts, "--attest-cem-v2")
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"attest-cem-v2-not-accepted"`) {
		t.Fatalf("rejected stable map: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
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
		{[]string{"--cem", ".corvint/change.cem.json", "--target", "HEAD", "--artifacts", root, "--attest-cem-v2"}, "expected-base-required"},
		{[]string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--artifacts", root, "--attest-cem-v2"}, "target-required"},
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

// TestCEMProofOfAnUnknownHunkIsNotAClaim checks the proof rows that the
// stable route builds from an accepted map (compileStableCEMProof): an
// unknown hunk is a cem-unknown row with no falsifier, NOT_RUN, counted as
// unproven, and a map with no supported hunk is UNPROVEN, never READY. The
// 0.x CLI test of this was retired with the 0.x route (CEM-V1-214).
func TestCEMProofOfAnUnknownHunkIsNotAClaim(t *testing.T) {
	t.Parallel()
	accepted := map[string]any{"valid": true, "drift": []any{}}
	document := wire.Map{Hunks: []wire.Hunk{{ID: "hunk:sha256:1", Path: "src/app.txt", NewRange: wire.Range{Start: 3}, Disposition: "unknown", Reason: "insufficient-evidence"}}}
	rows := cemRows(&document)
	for index := range rows {
		rows[index].Falsified = judgeVerifier(rows[index], accepted)
	}
	if len(rows) != 1 || rows[0].Authority != "cem-unknown" || rows[0].Falsifier != falsifierNone || rows[0].Falsified != falsifiedNotRun {
		t.Fatalf("rows = %+v, want one cem-unknown row with no falsifier, NOT_RUN", rows)
	}
	summary := summarizeCEMProof(rows)
	if summary.UnprovenResults != 1 || summary.ProvenResults != 0 || cemState(accepted, summary) != "UNPROVEN" {
		t.Fatalf("summary %+v state %s, want one unproven result and UNPROVEN", summary, cemState(accepted, summary))
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

// TestProveCEMAttestWrapsTheDocumentAndSignsIt checks FPK-V0-015 over a
// cem/1.0 map (CEM-V1-214): the first line is the in-toto statement whose
// predicate is the prove document and whose subjects are the map digest and
// the accepted target commit, and --attest-key signs exactly that statement.
func TestProveCEMAttestWrapsTheDocumentAndSignsIt(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	before := treeDigest(t, f.repo)
	_, unsigned := proveStableCEM(t, f)
	statementLine := strings.SplitN(string(unsigned), "\n", 2)[0]
	var statement map[string]any
	if err := json.Unmarshal([]byte(statementLine), &statement); err != nil {
		t.Fatal(err)
	}
	if statement["_type"] != "https://in-toto.io/Statement/v1" || statement["predicateType"] != attest.PredicateType {
		t.Fatalf("statement: %v", statement)
	}
	mapBytes, err := os.ReadFile(filepath.Join(f.repo, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	mapDigest := sha256.Sum256(mapBytes)
	subjects, _ := json.Marshal(statement["subject"])
	want := `[{"digest":{"sha256":"` + hex.EncodeToString(mapDigest[:]) + `"},"name":"stable.json"},{"digest":{"gitCommit":"` + f.target + `"},"name":"git:` + f.target + `"}]`
	if string(subjects) != want {
		t.Fatalf("subjects:\n%s\n%s", subjects, want)
	}
	predicate, _ := statement["predicate"].(map[string]any)
	if stringAt(predicate, "revision") != f.target {
		t.Fatalf("revision %v is not the target %s", predicate["revision"], f.target)
	}
	keyPath, publicKey := proveTestKey(t)
	signed, stderr, code := runProveArguments(t, "--root", f.repo, "prove", "--cem", "stable.json", "--expected-base", f.base,
		"--target", f.target, "--artifacts", f.artifacts, "--attest-key", keyPath, "--attest-cem-v2")
	if code != 0 {
		t.Fatalf("attest-key exit %d: %s", code, stderr)
	}
	payload, err := attest.Verify([]byte(strings.SplitN(string(signed), "\n", 2)[0]), publicKey)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if string(payload) != statementLine {
		t.Fatal("signed payload differs from the unsigned statement")
	}
	if treeDigest(t, f.repo) != before {
		t.Fatal("attestation changed the repository")
	}
}

// TestProveCEMAttestVerifiesOfflineWithCosignSemantics independently checks
// FPK-V0-015's public-key DSSE/in-toto contract without the Corvint verifier,
// on the proof envelope a cem/1.0 map's prove emits (CEM-V1-214).
func TestProveCEMAttestVerifiesOfflineWithCosignSemantics(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	lines, _, publicKeyPath := emitCEMV2Attestation(t, f)
	envelope := []byte(lines[0])
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
	f := materializeCEMV2Case(t)
	root := f.repo
	common := []string{"--cem", "stable.json", "--expected-base", f.base, "--target", f.target, "--artifacts", f.artifacts, "--attest-cem-v2"}
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
	f := materializeCEMV2Case(t)
	root := f.repo
	mapBytes, err := os.ReadFile(filepath.Join(root, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	name := "stable-\xff.json"
	common := []string{"--expected-base", f.base, "--target", f.target, "--artifacts", f.artifacts, "--attest-cem-v2"}
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
	plain, _ := proveStableCEM(t, f)
	document, err := json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	revision := stringAt(plain, "revision")
	if _, err := attestProof(document, cemSubjects("stable.json", mapBytes, revision), nil, ""); err != nil {
		t.Fatalf("attestProof with a UTF-8 map path: %v", err)
	}
	output, err := attestProof(document, cemSubjects(name, mapBytes, revision), nil, "")
	var refusal *gokernel.Error
	if !errors.As(err, &refusal) || refusal.Code != "attest-failed" || output != nil {
		t.Fatalf("attestProof = %q, %v; want attest-failed and no output", output, err)
	}
}
